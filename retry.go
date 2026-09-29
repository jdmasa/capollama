package main

import (
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/sashabaranov/go-openai"
)

// retryPause is how long a request that failed for a reason other than the
// file waits before it is sent again. A model that is still loading, a server
// that restarted or ran out of memory usually recovers within that time.
const retryPause = 90 * time.Second

// filePattern matches what the servers answer when the image itself is the
// problem, such as a format the extension lied about or a corrupt file.
var filePattern = regexp.MustCompile(`(?i)failed to (read|load|decode) image|image or audio file|` +
	`invalid image|unsupported image|image format|cannot identify image|could not decode`)

// FileProblem reports whether err is about the file rather than the model or
// the server, in which case sending it again cannot help and the run should go
// on to the next file.
func FileProblem(err error) bool {
	if err == nil {
		return false
	}
	// Chat has already raised the context where it could, so a context error
	// left over means this image is too large for any size it may pick.
	if _, tooSmall := ContextTooSmall(err); tooSmall {
		return true
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.HTTPStatusCode {
		case http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType:
			return true
		}
	}
	return filePattern.MatchString(err.Error())
}

// withRetry sends a request and, when it fails for a reason other than the
// file, waits pause and sends it once more. A second failure is returned, so
// the run moves on instead of stopping, and the next file gets its own chance.
func withRetry(call func() (string, error), pause time.Duration, wait func(time.Duration), onPause func(error)) (string, error) {
	response, err := call()
	if err == nil || FileProblem(err) {
		return response, err
	}
	onPause(err)
	wait(pause)
	return call()
}

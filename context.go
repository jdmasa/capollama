package main

import (
	"regexp"
	"strconv"
)

// maxContextSize caps what capollama will ask for on its own. Context is
// allocated on the server, so growing it without a limit could push a model out
// of its memory. Image tokens saturate around 4038 for any photo of 2048px or
// more, so this is far above what a picture can need.
const maxContextSize = 16384

// contextHeadroom leaves room for the answer and the prompt's own tokens on top
// of what the failed request reported.
const contextHeadroom = 512

// promptTokensPattern reads the token count out of an Ollama context error,
// which reports what the request actually needed:
//
//	{"error":{"message":"request (4126 tokens) exceeds the available context
//	size (4096 tokens)","n_prompt_tokens":4126,"n_ctx":4096}}
var promptTokensPattern = regexp.MustCompile(`"n_prompt_tokens":\s*(\d+)`)

// ContextTooSmall reports whether err is the server saying the request did not
// fit, and how many tokens it needed. The count is absent from older servers
// that only phrase it in the message, so ok can be true with tokens at zero.
func ContextTooSmall(err error) (tokens int, ok bool) {
	if err == nil {
		return 0, false
	}
	text := err.Error()
	if !contextErrorPattern.MatchString(text) {
		return 0, false
	}
	if match := promptTokensPattern.FindStringSubmatch(text); match != nil {
		if n, convErr := strconv.Atoi(match[1]); convErr == nil {
			return n, true
		}
	}
	return 0, true
}

var contextErrorPattern = regexp.MustCompile(`context size|exceed_context_size`)

// NextContextSize returns a context large enough for a request of promptTokens
// that is also larger than current, rounded up to a power of two so that a run
// settles on one size and the server reloads the model once rather than per
// image. It returns 0 when no size within the cap would help.
func NextContextSize(promptTokens, current int) int {
	if current >= maxContextSize {
		return 0
	}

	size := 4096
	if current >= size {
		size = current * 2
	}
	// An unreported token count still doubles, which is how a server that only
	// phrases the error in words gets a second chance.
	for promptTokens > 0 && size < promptTokens+contextHeadroom {
		size *= 2
	}

	if size > maxContextSize {
		// The cap may still hold the request itself, with less room for the
		// answer than asked for. That beats not trying.
		if maxContextSize > current && (promptTokens == 0 || maxContextSize > promptTokens) {
			return maxContextSize
		}
		return 0
	}
	return size
}

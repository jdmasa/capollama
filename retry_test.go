package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func TestFileProblem(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New(`ollama API error: Failed to load image or audio file`), true},
		{fmt.Errorf("failed to read image: %w", errors.New("permission denied")), true},
		{errors.New(`request (17000 tokens) exceeds the available context size (16384 tokens)`), true},
		{fmt.Errorf("OpenAI API error: %w", &openai.APIError{HTTPStatusCode: 413, Message: "too large"}), true},
		{errors.New(`OpenAI API error: invalid image data`), true},
		// The model or the server, which may recover.
		{errors.New(`ollama API error: model "qwen2.5vl" not found, try pulling it first`), false},
		{errors.New(`dial tcp 127.0.0.1:11434: connect: connection refused`), false},
		{errors.New(`ollama API error: model runner has unexpectedly stopped`), false},
		{fmt.Errorf("OpenAI API error: %w", &openai.APIError{HTTPStatusCode: 503, Message: "busy"}), false},
	}
	for _, c := range cases {
		if got := FileProblem(c.err); got != c.want {
			t.Errorf("FileProblem(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

func TestWithRetry(t *testing.T) {
	serverDown := errors.New("connection refused")
	badFile := errors.New("Failed to load image or audio file")

	cases := []struct {
		name    string
		results []error
		calls   int
		waits   int
		wantErr error
	}{
		{"succeeds", []error{nil}, 1, 0, nil},
		{"bad file is not retried", []error{badFile}, 1, 0, badFile},
		{"recovers after the pause", []error{serverDown, nil}, 2, 1, nil},
		{"gives up after one retry", []error{serverDown, serverDown, nil}, 2, 1, serverDown},
	}
	for _, c := range cases {
		calls, waits := 0, 0
		_, err := withRetry(func() (string, error) {
			err := c.results[calls]
			calls++
			return "", err
		}, retryPause, func(d time.Duration) {
			if d != retryPause {
				t.Errorf("%s: waited %s, want %s", c.name, d, retryPause)
			}
			waits++
		}, func(error) {})
		if calls != c.calls || waits != c.waits || err != c.wantErr {
			t.Errorf("%s: %d calls, %d waits, err %v; want %d, %d, %v",
				c.name, calls, waits, err, c.calls, c.waits, c.wantErr)
		}
	}
}

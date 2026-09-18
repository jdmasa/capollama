package main

import (
	"errors"
	"testing"
)

func TestContextTooSmall(t *testing.T) {
	ollama := errors.New(`ollama API error: {"error":{"code":400,"message":"request (4126 tokens) ` +
		`exceeds the available context size (4096 tokens), try increasing it",` +
		`"type":"exceed_context_size_error","n_prompt_tokens":4126,"n_ctx":4096}}`)
	if tokens, ok := ContextTooSmall(ollama); !ok || tokens != 4126 {
		t.Errorf("got %d, %v, want 4126, true", tokens, ok)
	}

	// A server that only phrases it in words is still recognised, without a count.
	worded := errors.New("ollama API error: request exceeds the available context size, try increasing it")
	if tokens, ok := ContextTooSmall(worded); !ok || tokens != 0 {
		t.Errorf("got %d, %v, want 0, true", tokens, ok)
	}

	for _, err := range []error{
		nil,
		errors.New("connection refused"),
		errors.New(`{"error":"model 'qwen2.5vl:3b' not found"}`),
	} {
		if _, ok := ContextTooSmall(err); ok {
			t.Errorf("%v must not read as a context error", err)
		}
	}
}

func TestNextContextSize(t *testing.T) {
	cases := []struct {
		name            string
		tokens, current int
		want            int
	}{
		{"a photo against the server default", 4126, 0, 8192},
		{"an unreported count doubles from the default", 0, 0, 4096},
		{"an unreported count doubles again", 0, 4096, 8192},
		{"a count that needs two doublings", 9000, 0, 16384},
		{"headroom pushes it over a power of two", 8000, 0, 16384},
		{"already raised, still short", 16000, 8192, 16384},
		// Beyond the cap there is nothing sensible left to ask for.
		{"needs more than the cap", 20000, 0, 0},
		{"already at the cap", 4126, 16384, 0},
		{"already past the cap", 4126, 32768, 0},
	}
	for _, c := range cases {
		if got := NextContextSize(c.tokens, c.current); got != c.want {
			t.Errorf("%s: NextContextSize(%d, %d) = %d, want %d", c.name, c.tokens, c.current, got, c.want)
		}
	}
}

// A run must settle on one size, as each change reloads the model server side.
func TestNextContextSizeSettles(t *testing.T) {
	size := NextContextSize(4126, 0)
	if again := NextContextSize(4103, size); again != 0 && again <= size {
		t.Errorf("a later photo re-raised the context to %d from %d", again, size)
	}
}

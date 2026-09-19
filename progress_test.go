package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                                 "0s",
		45 * time.Second:                  "45s",
		90 * time.Second:                  "1m30s",
		59*time.Minute + 59*time.Second:   "59m59s",
		time.Hour + 2*time.Minute:         "1h02m",
		25*time.Hour + 30*time.Minute:     "25h30m",
		2*time.Hour + 59*time.Minute + 59: "2h59m",
	}
	for d, want := range cases {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestProgressEstimate(t *testing.T) {
	p := newProgress(10)

	// Nothing has been captioned, so there is no history to estimate from.
	if _, ok := p.remaining(1); ok {
		t.Error("an estimate before the first image is a guess, not an estimate")
	}

	p.Done(10 * time.Second)
	// Starting image 2 leaves 9 to go, itself included, at 10s each.
	if eta, ok := p.remaining(2); !ok || eta != 90*time.Second {
		t.Errorf("remaining(2) = %v, %v, want 1m30s, true", eta, ok)
	}

	// A slower second image drags the average up: 15s each, 8 left.
	p.Done(20 * time.Second)
	if eta, ok := p.remaining(3); !ok || eta != 120*time.Second {
		t.Errorf("remaining(3) = %v, %v, want 2m0s, true", eta, ok)
	}

	// Skipped images cost nothing, so they must not flatten the average.
	if p.captioned != 2 {
		t.Errorf("captioned = %d, want 2", p.captioned)
	}
}

func TestProgressLine(t *testing.T) {
	var out bytes.Buffer
	p := newProgress(450)
	p.out = &out

	p.Starting(12, "/holiday/IMG_0012.jpg")
	if got := out.String(); !strings.HasPrefix(got, "[12/450] /holiday/IMG_0012.jpg") {
		t.Errorf("got %q", got)
	}
	if strings.Contains(out.String(), "eta") {
		t.Error("the first image cannot carry an estimate")
	}

	out.Reset()
	p.Done(30 * time.Second)
	p.Starting(13, "/holiday/IMG_0013.jpg")
	if got := out.String(); !strings.Contains(got, "eta ") {
		t.Errorf("an estimate was due by now, got %q", got)
	}
}

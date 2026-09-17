package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSniffFormat(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "JPEG"},
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, "PNG"},
		{"jpeg xl codestream", []byte{0xFF, 0x0A, 0x00}, "JPEG XL"},
		{"jpeg xl container", []byte{0x00, 0x00, 0x00, 0x0C, 'J', 'X', 'L', ' ', 0x0D, 0x0A, 0x87, 0x0A}, "JPEG XL"},
		{"gif", []byte("GIF89a..."), "GIF"},
		{"bmp", []byte("BMxx"), "BMP"},
		{"tiff little endian", []byte("II*\x00xx"), "TIFF"},
		{"tiff big endian", []byte("MM\x00*xx"), "TIFF"},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBP"), "WebP"},
		{"heic", append([]byte{0, 0, 0, 0x20}, []byte("ftypheic")...), "HEIC"},
		{"avif", append([]byte{0, 0, 0, 0x20}, []byte("ftypavif")...), "AVIF"},
		{"svg", []byte("<svg xmlns=..."), "SVG"},
		// A RIFF container that is not WebP, and an ISO box that is not HEIC.
		{"wav", []byte("RIFF\x00\x00\x00\x00WAVE"), ""},
		{"mp4", append([]byte{0, 0, 0, 0x20}, []byte("ftypisom")...), ""},
		{"text", []byte("not an image"), ""},
		{"empty", nil, ""},
		{"too short", []byte{0xFF}, ""},
	}
	for _, c := range cases {
		if got := SniffFormat(c.data); got != c.want {
			t.Errorf("%s: SniffFormat = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSniffFile(t *testing.T) {
	dir := t.TempDir()

	// A JPEG XL file under a .jpg name is what set this check off.
	path := filepath.Join(dir, "liar.jpg")
	if err := os.WriteFile(path, []byte{0xFF, 0x0A, 0x00, 0x01}, 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := SniffFile(path); err != nil || got != "JPEG XL" {
		t.Errorf("SniffFile = %q, %v, want \"JPEG XL\", nil", got, err)
	}

	// A file shorter than the header must not fail, only stay unrecognised.
	short := filepath.Join(dir, "short.jpg")
	if err := os.WriteFile(short, []byte{0xFF}, 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := SniffFile(short); err != nil || got != "" {
		t.Errorf("SniffFile = %q, %v, want \"\", nil", got, err)
	}

	// An empty file reports io.EOF rather than pretending to be an image.
	empty := filepath.Join(dir, "empty.jpg")
	if err := os.WriteFile(empty, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := SniffFile(empty); err == nil {
		t.Error("an empty file should report an error")
	}

	if _, err := SniffFile(filepath.Join(dir, "missing.jpg")); err == nil {
		t.Error("a missing file should report an error")
	}
}

func TestSupportedFormat(t *testing.T) {
	for _, format := range []string{"JPEG", "PNG"} {
		if !SupportedFormat(format) {
			t.Errorf("%s must be supported", format)
		}
		if SkipReason(format) != "" {
			t.Errorf("%s must not be skipped", format)
		}
	}
	for _, format := range []string{"JPEG XL", "HEIC", "WebP", ""} {
		if SupportedFormat(format) {
			t.Errorf("%q must not be supported", format)
		}
		if SkipReason(format) == "" {
			t.Errorf("%q needs a skip reason", format)
		}
	}
}

func TestMimeType(t *testing.T) {
	cases := map[string]string{"JPEG": "image/jpeg", "PNG": "image/png", "": "image/jpeg"}
	for format, want := range cases {
		if got := MimeType(format); got != want {
			t.Errorf("MimeType(%q) = %q, want %q", format, got, want)
		}
	}
}

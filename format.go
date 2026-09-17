package main

import (
	"bytes"
	"fmt"
	"os"
)

// headerSize is enough for every signature checked below. The ISO base media
// signature used by HEIC and AVIF sits at offset 4 and is 8 bytes wide.
const headerSize = 16

// Formats the vision APIs decode. Anything else is reported and skipped, as
// both Ollama and the OpenAI image endpoints reject it.
var supportedFormats = map[string]string{
	"JPEG": "image/jpeg",
	"PNG":  "image/png",
}

// SniffFormat names the image format of data by its magic bytes, or returns an
// empty string when the header matches nothing known. Extensions lie: images
// carrying a .jpg name while actually being JPEG XL or HEIC are common in
// libraries converted by phones and photo managers.
func SniffFormat(data []byte) string {
	switch {
	case len(data) < 3:
		return ""

	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "JPEG"
	case bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "PNG"

	// Recognised but not decodable by the vision APIs. Naming them makes the
	// skip message tell you what the file really is.
	case bytes.HasPrefix(data, []byte{0xFF, 0x0A}),
		bytes.HasPrefix(data, []byte{0x00, 0x00, 0x00, 0x0C, 'J', 'X', 'L', ' ', 0x0D, 0x0A, 0x87, 0x0A}):
		return "JPEG XL"
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return "GIF"
	case bytes.HasPrefix(data, []byte("BM")):
		return "BMP"
	case bytes.HasPrefix(data, []byte("II*\x00")), bytes.HasPrefix(data, []byte("MM\x00*")):
		return "TIFF"
	case bytes.HasPrefix(data, []byte("RIFF")) && len(data) >= 12 && bytes.Equal(data[8:12], []byte("WEBP")):
		return "WebP"
	case bytes.HasPrefix(data, []byte("<?xml")), bytes.HasPrefix(data, []byte("<svg")):
		return "SVG"
	}

	// ISO base media formats carry their brand right after the box header.
	if len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")) {
		switch string(data[8:12]) {
		case "heic", "heix", "heim", "heis", "hevc", "hevx", "mif1", "msf1":
			return "HEIC"
		case "avif", "avis":
			return "AVIF"
		}
	}
	return ""
}

// SniffFile reads the header of path and names its format. An unreadable file
// reports the error so the caller can say why it was skipped.
func SniffFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	header := make([]byte, headerSize)
	n, err := file.Read(header)
	if err != nil && n == 0 {
		return "", err
	}
	return SniffFormat(header[:n]), nil
}

// SupportedFormat reports whether a sniffed format can be sent to the model.
func SupportedFormat(format string) bool {
	_, ok := supportedFormats[format]
	return ok
}

// MimeType returns the media type to declare for a sniffed format.
func MimeType(format string) string {
	if mime, ok := supportedFormats[format]; ok {
		return mime
	}
	return "image/jpeg"
}

// SkipReason explains why a file cannot be sent to the model, or returns an
// empty string when it can.
func SkipReason(format string) string {
	switch {
	case SupportedFormat(format):
		return ""
	case format == "":
		return "not a JPEG or PNG"
	default:
		return fmt.Sprintf("%s, which the vision API cannot read, despite the file name", format)
	}
}

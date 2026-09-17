package main

import (
	"strings"
	"testing"

	"github.com/alexflint/go-arg"
)

func TestCaptionFileName(t *testing.T) {
	cases := []struct {
		path string
		xmp  bool
		want string
	}{
		// Sidecars keep the full image name, as metadata tools expect.
		{"/photos/image.jpg", true, "/photos/image.jpg.xmp"},
		{"/photos/image.jpeg", true, "/photos/image.jpeg.xmp"},
		{"/photos/image.PNG", true, "/photos/image.PNG.xmp"},
		// Text captions replace the extension.
		{"/photos/image.jpg", false, "/photos/image.txt"},
		{"/photos/holiday.2024.png", false, "/photos/holiday.2024.txt"},
	}
	for _, c := range cases {
		if got := captionFileName(c.path, c.xmp); got != c.want {
			t.Errorf("captionFileName(%q, %v) = %q, want %q", c.path, c.xmp, got, c.want)
		}
	}
}

// Struct tags are raw strings, so this guards against the escapes in the
// defaults reaching the model as literal backslashes.
func TestPromptDefaults(t *testing.T) {
	var args cmdArgs
	parser, err := arg.NewParser(arg.Config{}, &args)
	if err != nil {
		t.Fatal(err)
	}
	if err := parser.Parse([]string{"image.jpg"}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(args.SinglePassPrompt, "\nKEYWORDS:") {
		t.Errorf("the single pass prompt needs a real newline before its second label:\n%q", args.SinglePassPrompt)
	}
	for name, prompt := range map[string]string{
		"--prompt":             args.Prompt,
		"--single-pass-prompt": args.SinglePassPrompt,
	} {
		if strings.Contains(prompt, `\n`) || strings.Contains(prompt, `\"`) {
			t.Errorf("%s default holds an unprocessed escape:\n%q", name, prompt)
		}
	}
	if args.Language != "English" {
		t.Errorf("default language = %q, want English", args.Language)
	}
}

func TestKeywordOptionsHaveNoStop(t *testing.T) {
	// --force-one-sentence must not reach the keyword or single passes, where
	// its stop token would cut the list off.
	if _, ok := keywordOptions()["stop"]; ok {
		t.Error("the keyword pass must not stop at a period")
	}
	if _, ok := singlePassOptions()["stop"]; ok {
		t.Error("the single pass must not stop at a period")
	}
	if _, ok := options(cmdArgs{ForceOneSentence: true})["stop"]; !ok {
		t.Error("--force-one-sentence must still stop the caption pass")
	}
}

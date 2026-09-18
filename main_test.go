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
	// DropEnglishOpener matches this clause exactly, so the two must not drift.
	if !strings.Contains(args.Prompt, `Start your response with "A ..."`) {
		t.Error("the default prompt no longer holds the clause DropEnglishOpener removes")
	}
	if parseLanguage("es").DropEnglishOpener(args.Prompt) == args.Prompt {
		t.Error("the English opener was not removed from the default prompt")
	}

	if args.Language != "English" {
		t.Errorf("default language = %q, want English", args.Language)
	}
}

func TestNumCtx(t *testing.T) {
	// Left alone by default, so the server keeps deciding.
	for name, opts := range map[string]map[string]any{
		"caption":     options(cmdArgs{}),
		"keyword":     keywordOptions(cmdArgs{}),
		"single pass": singlePassOptions(cmdArgs{}),
	} {
		if _, ok := opts["num_ctx"]; ok {
			t.Errorf("the %s pass must not send num_ctx unless asked", name)
		}
	}
	// A photo needs the whole context, so every pass has to carry the setting.
	args := cmdArgs{NumCtx: 8192}
	for name, opts := range map[string]map[string]any{
		"caption":     options(args),
		"keyword":     keywordOptions(args),
		"single pass": singlePassOptions(args),
	} {
		if opts["num_ctx"] != 8192 {
			t.Errorf("the %s pass sent num_ctx %v, want 8192", name, opts["num_ctx"])
		}
	}
}

func TestKeywordOptionsHaveNoStop(t *testing.T) {
	// --force-one-sentence must not reach the keyword or single passes, where
	// its stop token would cut the list off.
	if _, ok := keywordOptions(cmdArgs{})["stop"]; ok {
		t.Error("the keyword pass must not stop at a period")
	}
	if _, ok := singlePassOptions(cmdArgs{})["stop"]; ok {
		t.Error("the single pass must not stop at a period")
	}
	if _, ok := options(cmdArgs{ForceOneSentence: true})["stop"]; !ok {
		t.Error("--force-one-sentence must still stop the caption pass")
	}
}

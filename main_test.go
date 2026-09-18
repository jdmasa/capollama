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

	// The keyword defaults must not hand the model words it can echo as tags.
	for _, word := range []string{"archive", "image", "refusals"} {
		if strings.Contains(strings.ToLower(args.KeywordPrompt+" "+args.KeywordSystem), word) {
			t.Errorf("the keyword defaults still mention %q, which models return as a keyword", word)
		}
	}

	if args.Language != "English" {
		t.Errorf("default language = %q, want English", args.Language)
	}
}

func TestContextSizeIsAddedByTheClient(t *testing.T) {
	// The passes themselves say nothing about it, so one place decides.
	for name, opts := range map[string]map[string]any{
		"caption":     options(cmdArgs{NumCtx: 8192}),
		"keyword":     keywordOptions(cmdArgs{NumCtx: 8192}),
		"single pass": singlePassOptions(cmdArgs{NumCtx: 8192}),
	} {
		if _, ok := opts["num_ctx"]; ok {
			t.Errorf("the %s pass must leave num_ctx to the client", name)
		}
	}

	none := (&client{}).withContext(map[string]any{})
	if _, ok := none["num_ctx"]; ok {
		t.Error("a client with no size set must leave the server to decide")
	}
	set := (&client{numCtx: 8192}).withContext(map[string]any{})
	if set["num_ctx"] != 8192 {
		t.Errorf("sent num_ctx %v, want 8192", set["num_ctx"])
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

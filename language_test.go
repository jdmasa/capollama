package main

import (
	"strings"
	"testing"
)

func TestParseLanguage(t *testing.T) {
	cases := []struct{ in, name, code, tag string }{
		{"", "English", "en", ""},
		{"English", "English", "en", ""},
		{"en", "English", "en", ""},
		{"Spanish", "Spanish", "es", "es"},
		{"spanish", "Spanish", "es", "es"},
		{"es", "Spanish", "es", "es"},
		{"es-ES", "Spanish", "es-ES", "es-ES"},
		{"pt_BR", "Portuguese", "pt-BR", "pt-BR"},
		{" Catalan ", "Catalan", "ca", "ca"},
		// Unknown languages still reach the model by name, without a tag.
		{"Klingon", "Klingon", "", ""},
	}
	for _, c := range cases {
		l := parseLanguage(c.in)
		if l.Name != c.name || l.Code != c.code || l.Tag() != c.tag {
			t.Errorf("parseLanguage(%q) = {Name:%q Code:%q} tag %q, want {Name:%q Code:%q} tag %q",
				c.in, l.Name, l.Code, l.Tag(), c.name, c.code, c.tag)
		}
	}
}

func TestInstructKeywords(t *testing.T) {
	const prompt = "List the keywords."
	if got := parseLanguage("en").InstructKeywords(prompt); got != prompt {
		t.Errorf("English must not add an instruction, got %q", got)
	}
	// "your answer" was not enough: the model kept answering in English.
	if got, want := parseLanguage("es").InstructKeywords(prompt), prompt+"\nWrite the keywords in Spanish."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDropEnglishOpener(t *testing.T) {
	const withOpener = `Describe this image. Start your response with "A ..."`
	const without = "Describe this image."

	if got := parseLanguage("en").DropEnglishOpener(withOpener); got != withOpener {
		t.Errorf("English must keep its opener, got %q", got)
	}
	if got := parseLanguage("es").DropEnglishOpener(withOpener); got != without {
		t.Errorf("got %q, want %q", got, without)
	}
	if got := parseLanguage("es").DropEnglishOpener(without); got != without {
		t.Errorf("a prompt without the clause must be untouched, got %q", got)
	}
}

func TestInstruct(t *testing.T) {
	const prompt = "Describe this image."

	for _, value := range []string{"", "English", "en"} {
		if got := parseLanguage(value).Instruct(prompt); got != prompt {
			t.Errorf("parseLanguage(%q) must not add an instruction, got %q", value, got)
		}
		if got := parseLanguage(value).InstructSinglePass(prompt); got != prompt {
			t.Errorf("parseLanguage(%q) must not add a single pass instruction, got %q", value, got)
		}
	}

	if got, want := parseLanguage("es").Instruct(prompt), prompt+"\nWrite your answer in Spanish."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// The labels have to survive translation, as the parser looks for them.
	single := parseLanguage("es").InstructSinglePass(prompt)
	if single == prompt {
		t.Fatal("single pass instruction missing")
	}
	for _, want := range []string{"Spanish", "DESCRIPTION", "KEYWORDS"} {
		if !strings.Contains(single, want) {
			t.Errorf("single pass instruction missing %q: %q", want, single)
		}
	}
}

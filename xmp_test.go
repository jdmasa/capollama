package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseKeywords(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"comma line", "cat, outdoor, sunny", []string{"cat", "outdoor", "sunny"}},
		{"semicolons", "cat; outdoor; sunny", []string{"cat", "outdoor", "sunny"}},
		{"bullets with a lead-in", "Here are the keywords:\n- cat\n- outdoor\n- Cat\n", []string{"cat", "outdoor"}},
		{"json array", "```json\n[\"cat\", \"outdoor\"]\n```", []string{"cat", "outdoor"}},
		{"numbering", "1. cat\n2) outdoor\n", []string{"cat", "outdoor"}},
		{"quotes and stops", "\"cat\"; 'outdoor'.", []string{"cat", "outdoor"}},
		{"sentences are not keywords", "cat, " + strings.Repeat("x", maxKeywordLength+1), []string{"cat"}},
		{"empty", "", nil},
		// Words echoed back from the prompts rather than read off the image.
		{"instruction leakage", "motorcycle, no refusals, archive, image, chrome",
			[]string{"motorcycle", "chrome"}},
		{"only leakage", "keywords, tags, photo", nil},
	}
	for _, c := range cases {
		if got := ParseKeywords(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: ParseKeywords(%q) = %#v, want %#v", c.name, c.in, got, c.want)
		}
	}
}

func TestParseSinglePass(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantDesc string
		wantKw   []string
		wantOk   bool
	}{
		{"two lines", "DESCRIPTION: A fluffy orange cat on a deck.\nKEYWORDS: cat, outdoor, sunny",
			"A fluffy orange cat on a deck.", []string{"cat", "outdoor", "sunny"}, true},
		{"one line", "DESCRIPTION: A cat. KEYWORDS: cat, sunny",
			"A cat.", []string{"cat", "sunny"}, true},
		{"markdown", "**Description:** A cat on a deck.\n\n**Keywords:** cat, deck",
			"A cat on a deck.", []string{"cat", "deck"}, true},
		{"bullets and synonyms", "- Caption: A cat.\n- Tags:\n  - cat\n  - deck",
			"A cat.", []string{"cat", "deck"}, true},
		{"wrapped description", "DESCRIPTION: A cat\nsitting on a deck.\nKEYWORDS: cat",
			"A cat sitting on a deck.", []string{"cat"}, true},
		{"fenced", "```\nDESCRIPTION: A cat.\nKEYWORDS: cat\n```",
			"A cat.", []string{"cat"}, true},
		{"preamble before the labels", "Sure! Here you go.\nDESCRIPTION: A cat.\nKEYWORDS: cat",
			"A cat.", []string{"cat"}, true},
		{"keywords without a description label", "A cat sitting on a deck.\nKeywords: cat, deck",
			"A cat sitting on a deck.", []string{"cat", "deck"}, true},
		{"translated labels", "DESCRIPCIÓN: Un gato en la terraza.\nPALABRAS CLAVE: gato, terraza, sol",
			"Un gato en la terraza.", []string{"gato", "terraza", "sol"}, true},
		// A reply that keeps no format at all must still yield the caption.
		{"no labels at all", "A cat sitting on a deck.",
			"A cat sitting on a deck.", nil, false},
		{"description label only", "DESCRIPTION: A cat sitting on a deck.",
			"A cat sitting on a deck.", nil, false},
	}
	for _, c := range cases {
		desc, kw, ok := ParseSinglePass(c.in)
		if desc != c.wantDesc || !reflect.DeepEqual(kw, c.wantKw) || ok != c.wantOk {
			t.Errorf("%s:\n got  %q %#v %v\n want %q %#v %v", c.name, desc, kw, ok, c.wantDesc, c.wantKw, c.wantOk)
		}
	}
}

func TestBuildXMP(t *testing.T) {
	out := BuildXMP("A fluffy orange cat sitting on a sunny wooden deck outdoors.", []string{"cat", "outdoor", "sunny"}, "")
	for _, want := range []string{
		`<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">`,
		`xmlns:dc="http://purl.org/dc/elements/1.1/"`,
		`<rdf:li xml:lang="x-default">A fluffy orange cat sitting on a sunny wooden deck outdoors.</rdf:li>`,
		`<rdf:li>outdoor</rdf:li>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestBuildXMPEscapes(t *testing.T) {
	out := BuildXMP(`A photo of "Tom" & <Jerry>`, []string{"cat & mouse"}, "")
	if strings.Contains(out, "<Jerry>") {
		t.Errorf("description not escaped:\n%s", out)
	}
	if !strings.Contains(out, `<rdf:li>cat &amp; mouse</rdf:li>`) {
		t.Errorf("keyword not escaped:\n%s", out)
	}
}

func TestBuildXMPOmitsEmptySubject(t *testing.T) {
	if out := BuildXMP("A cat.", nil, ""); strings.Contains(out, "dc:subject") {
		t.Errorf("an empty keyword list must omit dc:subject:\n%s", out)
	}
}

func TestBuildXMPLanguage(t *testing.T) {
	out := BuildXMP("Un gato.", []string{"gato"}, "es")
	// x-default repeats the language entry, which is what the XMP spec asks for.
	for _, want := range []string{
		`<rdf:li xml:lang="x-default">Un gato.</rdf:li>`,
		`<rdf:li xml:lang="es">Un gato.</rdf:li>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(BuildXMP("A cat.", nil, ""), `xml:lang="en"`) {
		t.Error("English must stay x-default only")
	}
}

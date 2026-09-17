package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

// maxKeywordLength guards against the model answering with a sentence instead of
// a keyword. Anything longer is not a tag and gets dropped.
const maxKeywordLength = 64

// ParseKeywords turns whatever the keyword pass answered into a clean list of
// tags. Models answer with a comma separated line, a bullet list or a JSON
// array, so all three are accepted. Duplicates are removed case insensitively
// while keeping the order of the first occurrence.
func ParseKeywords(raw string) []string {
	text := stripCodeFence(strings.TrimSpace(raw))

	var parts []string
	if strings.HasPrefix(text, "[") {
		var list []string
		if err := json.Unmarshal([]byte(text), &list); err == nil {
			parts = list
		}
	}
	if parts == nil {
		parts = strings.FieldsFunc(text, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r'
		})
	}

	var keywords []string
	seen := map[string]bool{}
	for _, part := range parts {
		keyword := cleanKeyword(part)
		if keyword == "" || len(keyword) > maxKeywordLength {
			continue
		}
		// Drops lead-ins like "Here are the keywords:" that some models prepend.
		if strings.HasSuffix(keyword, ":") {
			continue
		}
		if key := strings.ToLower(keyword); !seen[key] {
			seen[key] = true
			keywords = append(keywords, keyword)
		}
	}
	return keywords
}

// stripCodeFence removes a surrounding markdown code fence including its
// optional language tag.
func stripCodeFence(text string) string {
	if !strings.HasPrefix(text, "```") {
		return text
	}
	if _, rest, found := strings.Cut(text, "\n"); found {
		text = rest
	}
	if index := strings.LastIndex(text, "```"); index >= 0 {
		text = text[:index]
	}
	return strings.TrimSpace(text)
}

// cleanKeyword strips list markers, numbering, quotes and trailing punctuation
// from a single entry.
func cleanKeyword(part string) string {
	keyword := strings.TrimSpace(part)
	keyword = strings.TrimLeft(keyword, "-*•#\t ")

	// Removes "1." or "2)" style numbering.
	if index := strings.IndexAny(keyword, ".)"); index > 0 && index <= 3 {
		if strings.Trim(keyword[:index], "0123456789") == "" {
			keyword = strings.TrimSpace(keyword[index+1:])
		}
	}

	// A keyword never legitimately starts or ends with a quote or a period.
	return strings.Trim(keyword, "\"'. \t")
}

// BuildXMP renders an XMP sidecar holding the caption as dc:description and the
// keywords as dc:subject. dc:subject is left out when there are no keywords.
//
// lang is the BCP 47 tag the caption was written in, or empty for English. When
// set, the caption is written twice: once as x-default, which is what readers
// that ignore languages pick up, and once tagged with the language itself. The
// XMP spec expects the x-default item to repeat one of the other items, so the
// duplication is intended. dc:subject is an unordered bag of plain text and
// carries no language qualifier.
func BuildXMP(description string, keywords []string, lang string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b, `<x:xmpmeta xmlns:x="adobe:ns:meta/" x:xmptk="%s">`+"\n",
		xmlEscape(appName+" "+strings.TrimSpace(fullVersion)))
	b.WriteString(` <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` + "\n")
	b.WriteString(`  <rdf:Description rdf:about=""` + "\n")
	b.WriteString(`    xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n")

	b.WriteString(`   <dc:description>` + "\n")
	b.WriteString(`    <rdf:Alt>` + "\n")
	fmt.Fprintf(&b, `     <rdf:li xml:lang="x-default">%s</rdf:li>`+"\n", xmlEscape(description))
	if lang != "" {
		fmt.Fprintf(&b, `     <rdf:li xml:lang="%s">%s</rdf:li>`+"\n", xmlEscape(lang), xmlEscape(description))
	}
	b.WriteString(`    </rdf:Alt>` + "\n")
	b.WriteString(`   </dc:description>` + "\n")

	if len(keywords) > 0 {
		b.WriteString(`   <dc:subject>` + "\n")
		b.WriteString(`    <rdf:Bag>` + "\n")
		for _, keyword := range keywords {
			fmt.Fprintf(&b, `     <rdf:li>%s</rdf:li>`+"\n", xmlEscape(keyword))
		}
		b.WriteString(`    </rdf:Bag>` + "\n")
		b.WriteString(`   </dc:subject>` + "\n")
	}

	b.WriteString(`  </rdf:Description>` + "\n")
	b.WriteString(` </rdf:RDF>` + "\n")
	b.WriteString(`</x:xmpmeta>` + "\n")
	return b.String()
}

func xmlEscape(text string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(text)); err != nil {
		return ""
	}
	return buf.String()
}

// Labels accepted by the single pass parser. Models drift between synonyms, so
// the common ones are all treated as the same field.
// The prompt asks for English labels even when the caption itself is written in
// another language, but models translate them anyway, so the common
// translations are accepted as well.
var (
	descriptionLabels = []string{
		"description:", "caption:",
		"descripción:", "descripcion:", "leyenda:",
		"description :", "légende:", "legende:",
		"beschreibung:", "descrizione:", "descrição:", "descricao:", "descripció:",
	}
	keywordLabels = []string{
		"keywords:", "keyword:", "tags:", "subject:",
		"palabras clave:", "palabras-clave:", "etiquetas:",
		"mots-clés:", "mots clés:", "mots-cles:",
		"schlüsselwörter:", "schlagwörter:", "stichwörter:",
		"parole chiave:", "palavras-chave:", "palavras chave:", "paraules clau:",
	}
)

// findLabel returns where the earliest of labels starts in lower (searching
// from index from) and where it ends. Matching happens on an already lowercased
// copy of the text so the indexes stay valid for the original.
func findLabel(lower string, labels []string, from int) (start, after int, found bool) {
	start = -1
	for _, label := range labels {
		index := strings.Index(lower[from:], label)
		if index < 0 {
			continue
		}
		index += from
		if !found || index < start {
			start, after, found = index, index+len(label), true
		}
	}
	return start, after, found
}

// ParseSinglePass splits the combined reply of the single pass into caption and
// keywords. It searches for the labels anywhere in the text, so a model that
// answers on one line is handled as well as one that uses two. When no keyword
// label shows up, ok is false and the whole reply is returned as the caption,
// which keeps a malformed answer from losing the description too.
func ParseSinglePass(raw string) (description string, keywords []string, ok bool) {
	text := stripCodeFence(strings.TrimSpace(raw))
	lower := strings.ToLower(text)

	_, descriptionAt, hasDescription := findLabel(lower, descriptionLabels, 0)
	searchFrom := 0
	if hasDescription {
		searchFrom = descriptionAt
	}
	keywordStart, keywordAt, hasKeywords := findLabel(lower, keywordLabels, searchFrom)

	switch {
	case hasDescription && hasKeywords:
		description = text[descriptionAt:keywordStart]
	case hasDescription:
		description = text[descriptionAt:]
	case hasKeywords:
		description = text[:keywordStart]
	default:
		description = text
	}

	if hasKeywords {
		keywords = ParseKeywords(text[keywordAt:])
	}
	return cleanLine(description), keywords, hasKeywords
}

// cleanLine folds a possibly multi line answer into one caption line and strips
// the markdown decoration models like to wrap it in.
func cleanLine(text string) string {
	line := strings.Join(strings.Fields(text), " ")
	// Bullets, numbering decoration and stray label punctuation can sit on
	// either end once the labels themselves are cut away.
	line = strings.Trim(line, "*_#`-•: \t")
	return strings.Trim(line, `"' `)
}

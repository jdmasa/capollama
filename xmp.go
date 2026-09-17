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
func BuildXMP(description string, keywords []string) string {
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

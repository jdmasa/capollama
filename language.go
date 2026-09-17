package main

import (
	"fmt"
	"strings"
)

// languageNames maps a BCP 47 base code to the English name of the language.
// The name goes into the prompt, the code into the xml:lang attribute of
// dc:description, so both directions are needed.
var languageNames = map[string]string{
	"ar": "Arabic",
	"ca": "Catalan",
	"cs": "Czech",
	"da": "Danish",
	"de": "German",
	"el": "Greek",
	"en": "English",
	"es": "Spanish",
	"eu": "Basque",
	"fi": "Finnish",
	"fr": "French",
	"gl": "Galician",
	"he": "Hebrew",
	"hi": "Hindi",
	"hu": "Hungarian",
	"id": "Indonesian",
	"it": "Italian",
	"ja": "Japanese",
	"ko": "Korean",
	"nl": "Dutch",
	"no": "Norwegian",
	"pl": "Polish",
	"pt": "Portuguese",
	"ro": "Romanian",
	"ru": "Russian",
	"sv": "Swedish",
	"tr": "Turkish",
	"uk": "Ukrainian",
	"vi": "Vietnamese",
	"zh": "Chinese",
}

// language is the output language of the captions, as both the name the model
// is asked to write in and the tag the sidecar is labelled with.
type language struct {
	Name string // English name of the language, e.g. "Spanish"
	Code string // BCP 47 tag for xml:lang, empty when the name is not known
}

// parseLanguage accepts either a name ("Spanish") or a code ("es", "es-ES",
// "pt-BR"). An unknown value is passed to the model as given, with no code, so
// that a language missing from the table still works for the caption itself.
func parseLanguage(value string) language {
	value = strings.TrimSpace(value)
	if value == "" {
		return language{Name: "English", Code: "en"}
	}

	// A code may carry a region ("es-ES"), which is kept in the tag but not
	// used for the lookup.
	base := strings.ToLower(value)
	if index := strings.IndexAny(base, "-_"); index > 0 {
		base = base[:index]
	}
	if name, ok := languageNames[base]; ok {
		return language{Name: name, Code: strings.ReplaceAll(value, "_", "-")}
	}

	for code, name := range languageNames {
		if strings.EqualFold(name, value) {
			return language{Name: name, Code: code}
		}
	}
	return language{Name: value}
}

// IsEnglish reports whether captions are written in the default language, in
// which case no instruction is added to the prompts.
func (l language) IsEnglish() bool {
	return strings.EqualFold(l.Name, "English")
}

// Instruct appends the language instruction to a prompt.
func (l language) Instruct(prompt string) string {
	if l.IsEnglish() {
		return prompt
	}
	return prompt + "\n" + fmt.Sprintf("Write your answer in %s.", l.Name)
}

// InstructSinglePass appends the language instruction to the combined prompt.
// The labels have to stay English there, as they are what the parser looks for.
func (l language) InstructSinglePass(prompt string) string {
	if l.IsEnglish() {
		return prompt
	}
	return prompt + "\n" + fmt.Sprintf(
		"Write the description and the keywords in %s, but keep the labels DESCRIPTION and KEYWORDS in English.", l.Name)
}

// Tag is the value for the xml:lang attribute of the language specific
// dc:description entry, or an empty string when only x-default is written.
func (l language) Tag() string {
	if l.IsEnglish() {
		return ""
	}
	return l.Code
}

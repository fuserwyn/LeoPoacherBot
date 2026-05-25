package prompts

import (
	_ "embed"
	"strings"
)

//go:embed data/shared_persona.txt
var embeddedSharedPersona string

//go:embed data/shared_tone.txt
var embeddedSharedTone string

//go:embed data/shared_language.txt
var embeddedSharedLanguage string

//go:embed data/shared_glossary.txt
var embeddedSharedGlossary string

func joinBlocks(blocks ...string) string {
	var b strings.Builder
	for _, block := range blocks {
		s := strings.TrimSpace(block)
		if s == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(s)
	}
	return b.String()
}

// FoundationBlocks — PERSONA + TONE + LANGUAGE (canonical v1.3 §0).
func FoundationBlocks() string {
	return joinBlocks(embeddedSharedPersona, embeddedSharedTone, embeddedSharedLanguage)
}

// FoundationWithGlossary — foundation + GLOSSARY (для лички и справочных промтов).
func FoundationWithGlossary() string {
	return joinBlocks(FoundationBlocks(), embeddedSharedGlossary)
}

// ComposeSystemPrompt склеивает shared foundation и task-specific тело.
func ComposeSystemPrompt(taskBody string, withGlossary bool) string {
	if withGlossary {
		return joinBlocks(FoundationWithGlossary(), taskBody)
	}
	return joinBlocks(FoundationBlocks(), taskBody)
}

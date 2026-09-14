package story

import (
	_ "embed"
	"fmt"
	"strings"
)

// Historical English text from Project Gutenberg #10483 (1916 anthology).
//
//go:embed necklace.txt
var sourceText string

func SourceText() string { return sourceText }

// Exact historical passages, never reconstructed by the model. Offsets use
// UTF-8 bytes in the embedded source, not an invented page number.
func sourceExcerpts() map[string]Excerpt {
	starts := []string{
		"He stopped, stupefied, distracted, on seeing that his wife was crying.",
		"All at once she discovered, in a black satin box, a splendid diamond",
		"She danced with delight, with passion, intoxicated with pleasure,",
		"They looked in the folds of her dress, in the folds of her cloak, in the",
		"They found, in a shop at the Palais Royal, a string of diamonds which",
		"At the end of ten years they had paid everything,",
		"\"Oh, my poor Mathilde! Why, my necklace was paste.",
	}
	out := map[string]Excerpt{}
	for i, start := range starts {
		from := strings.Index(sourceText, start)
		if from < 0 {
			panic("missing historical source anchor")
		}
		to := strings.Index(sourceText[from:], "\n\n")
		if to < 0 {
			to = len(sourceText) - from
		}
		to += from
		out[fmt.Sprintf("e%d", i+1)] = Excerpt{Text: sourceText[from:to], StartByte: from, EndByte: to}
	}
	return out
}

package llm

import (
	"github.com/sachn-cs/promptsheon/internal/models"
	"unicode"
)

// EstimateTokens provides a rough token count estimate.
// Uses ~4 characters per token as a heuristic for English text.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	// Count words and apply heuristic
	words := 0
	inWord := false
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if inWord {
				words++
				inWord = false
			}
		} else {
			inWord = true
		}
	}
	if inWord {
		words++
	}

	// Rough estimate: ~1.3 tokens per word for English
	tokens := int(float64(words) * 1.3)

	// Ensure minimum of 1 token for non-empty text
	if tokens == 0 && len(text) > 0 {
		tokens = 1
	}

	return tokens
}

// EstimateCost calculates estimated cost in USD for a given token count and model.
func EstimateCost(inputTokens, outputTokens int, model string) float64 {
	usage := models.Usage{
		PromptTokens:     inputTokens,
		CompletionTokens: outputTokens,
		TotalTokens:      inputTokens + outputTokens,
	}
	return CalculateCost(model, usage)
}

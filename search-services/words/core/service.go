package core

import (
	"strconv"
	"strings"
	"unicode"
)

type normalizer struct {
	stemmer         Stemmer
	stopWordChecker StopWordChecker
	logger          Logger
}

func NewNormalizer(stemmer Stemmer, stopWordChecker StopWordChecker, logger Logger) Normalizer {
	return &normalizer{
		stemmer:         stemmer,
		stopWordChecker: stopWordChecker,
		logger:          logger}
}

func (n *normalizer) Normalize(phrase string) []string {
	tokens := strings.FieldsFunc(phrase, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	seen := make(map[string]struct{})

	for _, token := range tokens {
		word := strings.ToLower(token)

		if _, err := strconv.Atoi(word); err == nil {
			seen[word] = struct{}{}
			continue
		}

		if n.stopWordChecker.IsStopWord(word) {
			continue
		}

		stemmed, err := n.stemmer.Stem(word)
		if err != nil {
			n.logger.Error("failed to stem word", "word", word, "error", err)
			stemmed = word
		}

		seen[stemmed] = struct{}{}
	}

	result := make([]string, 0, len(seen))
	for w := range seen {
		result = append(result, w)
	}
	return result
}

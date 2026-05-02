package stopwords

import (
	"github.com/kljensen/snowball/english"
	"yadro.com/course/words/core"
)

type checker struct{}

func NewEnglishStopWordChecker() core.StopWordChecker {
	return &checker{}
}

func (c *checker) IsStopWord(word string) bool {
	return english.IsStopWord(word)
}

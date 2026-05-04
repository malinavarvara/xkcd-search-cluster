package stemming

import (
	"github.com/kljensen/snowball"
	"yadro.com/course/words/core"
)

type snowballStemmer struct{}

func NewSnowballStemmer() core.Stemmer {
	return &snowballStemmer{}
}

func (s *snowballStemmer) Stem(word string) (string, error) {
	return snowball.Stem(word, "english", true)
}

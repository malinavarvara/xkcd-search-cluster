package core

type Normalizer interface {
	Normalize(phrase string) []string
}

type Stemmer interface {
	Stem(word string) (string, error)
}

type StopWordChecker interface {
	IsStopWord(word string) bool
}

type Logger interface {
	Error(msg string, args ...any)
}

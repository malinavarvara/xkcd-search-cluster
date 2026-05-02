package stopwords

import "testing"

func TestEnglishStopWords(t *testing.T) {
	c := NewEnglishStopWordChecker()
	if !c.IsStopWord("the") {
		t.Error("expected 'the' to be a stopword")
	}
	if c.IsStopWord("golang") {
		t.Error("'golang' should not be a stopword")
	}
}

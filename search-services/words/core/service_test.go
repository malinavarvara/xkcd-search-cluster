package core

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

type mockStemmer struct {
	failWord string
}

func (m *mockStemmer) Stem(word string) (string, error) {
	if word == m.failWord {
		return "", fmt.Errorf("stemming error")
	}
	if len(word) > 3 && word[len(word)-3:] == "ing" {
		return word[:len(word)-3], nil
	}
	return word, nil
}

type mockStopWords struct {
	stops map[string]struct{}
}

func (m *mockStopWords) IsStopWord(word string) bool {
	_, ok := m.stops[word]
	return ok
}

type mockLogger struct {
	called bool
}

func (m *mockLogger) Error(msg string, args ...any) {
	m.called = true
}

func TestNormalizer_Normalize(t *testing.T) {
	stemmer := &mockStemmer{failWord: "errorword"}
	stopWords := &mockStopWords{
		stops: map[string]struct{}{"the": {}, "a": {}},
	}
	logger := &mockLogger{}

	normalizer := NewNormalizer(stemmer, stopWords, logger)

	t.Run("Full_Success_Flow", func(t *testing.T) {
		phrase := "The 2 running dogs!"

		got := normalizer.Normalize(phrase)
		sort.Strings(got)
		want := []string{"2", "dogs", "runn"}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("Normalize(%q) = %v, want %v", phrase, got, want)
		}
	})

	t.Run("Handle_Stemming_Error", func(t *testing.T) {
		logger.called = false
		phrase := "errorword"

		got := normalizer.Normalize(phrase)

		if len(got) != 1 || got[0] != "errorword" {
			t.Errorf("Normalize(%q) = %v, want [%q]", phrase, got, "errorword")
		}
		if !logger.called {
			t.Error("expected logger.Error to be called on stemming error, but it wasn't")
		}
	})

	t.Run("Empty_And_Special_Chars", func(t *testing.T) {
		phrase := "!!! ??? 123"
		got := normalizer.Normalize(phrase)

		if len(got) != 1 || got[0] != "123" {
			t.Errorf("Normalize(%q) = %v, want [%q]", phrase, got, "123")
		}
	})
}

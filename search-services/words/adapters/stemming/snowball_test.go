package stemming

import "testing"

func TestSnowballStemmer(t *testing.T) {
	s := NewSnowballStemmer()
	res, err := s.Stem("running")
	if err != nil || res != "run" {
		t.Errorf("stemming failed: %v, got %s", err, res)
	}
}

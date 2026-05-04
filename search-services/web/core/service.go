package core

import (
	"fmt"
)

type WebService struct {
	store ComicStore
}

func NewWebService(store ComicStore) *WebService {
	return &WebService{
		store: store,
	}
}

func (s *WebService) SearchComics(query string) ([]Comic, error) {
	if query == "" {
		return nil, fmt.Errorf("%w: search query is empty", ErrInvalidInput)
	}

	comics, err := s.store.Search(query)
	if err != nil {
		return nil, fmt.Errorf("core search: %w", err)
	}

	if len(comics) == 0 {
		return nil, ErrNotFound
	}

	return comics, nil
}

package core

import (
	"fmt"
)

// WebService реализует бизнес-логику веб-интерфейса.
type WebService struct {
	store ComicStore // Интерфейс, определенный в ports.go[cite: 5]
}

// NewWebService создает новый экземпляр сервиса.
func NewWebService(store ComicStore) *WebService {
	return &WebService{
		store: store,
	}
}

// SearchComics обрабатывает поисковый запрос, валидирует его и обращается к стору.
func (s *WebService) SearchComics(query string) ([]Comic, error) {
	if query == "" {
		return nil, fmt.Errorf("%w: search query is empty", ErrInvalidInput)
	}

	// Вызов порта (адаптера API)[cite: 3, 5]
	comics, err := s.store.Search(query)
	if err != nil {
		// Оборачиваем ошибку, чтобы сохранить контекст для логов[cite: 3]
		return nil, fmt.Errorf("core search: %w", err)
	}

	if len(comics) == 0 {
		return nil, ErrNotFound
	}

	return comics, nil
}

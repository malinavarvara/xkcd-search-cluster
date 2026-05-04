package core

type ComicStore interface {
	Search(query string) ([]Comic, error)
}

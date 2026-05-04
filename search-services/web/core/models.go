package core

type Comic struct {
	ID     int
	Title  string
	ImgURL string
}

type SearchResponse struct {
	Items []Comic
}

type PageData struct {
	Comics  []Comic
	Query   string
	Error   string
	IsAdmin bool
}

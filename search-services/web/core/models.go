package core

type Comic struct {
	ID     int    //`json:"id"`
	Title  string //`json:"title"`
	ImgURL string //`json:"img"`
}

type SearchResponse struct {
	Items []Comic //`json:"items"`
}

type PageData struct {
	Comics  []Comic
	Query   string
	Error   string
	IsAdmin bool
}

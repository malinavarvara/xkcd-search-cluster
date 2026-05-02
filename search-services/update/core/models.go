package core

import "time"

type ServiceStatus string

const (
	StatusRunning ServiceStatus = "running"
	StatusIdle    ServiceStatus = "idle"
)

type DBStats struct {
	WordsTotal    int
	WordsUnique   int
	ComicsFetched int
}

type ServiceStats struct {
	DBStats
	ComicsTotal int
}

type Comics struct {
	Num         int
	ImgURL      string
	PublishedAt time.Time
	Words       []string
}

type XKCDInfo struct {
	ID          int
	URL         string
	Title       string
	SafeTitle   string
	Alt         string
	Description string
	Year        string
	Month       string
	Day         string
}

package rules

import "time"

type FetchMeta struct{ Bytes int64 }
type File struct {
	Name      string    `json:"name"`
	SourceURL string    `json:"source_url"`
	SHA256    string    `json:"sha256"`
	Bytes     int64     `json:"bytes"`
	FetchedAt time.Time `json:"fetched_at"`
}
type Manifest struct {
	Generation string `json:"generation"`
	Files      []File `json:"files"`
}

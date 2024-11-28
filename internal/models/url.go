package models

import "time"

type URL struct {
	ID          int64     `json:"id"`
	OriginalURL string    `json:"original_url"`
	ShortCode   string    `json:"short_code"`
	Visits      int       `json:"visits"`
	CreatedAt   time.Time `json:"created_at"`
}

type URLResponse struct {
	URL      URL    `json:"url"`
	ShortURL string `json:"short_url"`
}

type Stats struct {
	TotalURLs   int64  `json:"total_urls"`
	TotalVisits int64  `json:"total_visits"`
	LastCreated string `json:"last_created,omitempty"`
}

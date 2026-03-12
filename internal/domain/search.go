package domain

import "time"

type SavedSearch struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Query       string            `json:"query"`
	Filters     map[string]string `json:"filters"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type SearchHistory struct {
	ID        string    `json:"id"`
	Query     string    `json:"query"`
	Results   int       `json:"results"`
	Timestamp time.Time `json:"timestamp"`
}

type AdvancedSearchInput struct {
	Query        string            `json:"query"`
	BucketID     string            `json:"bucket_id"`
	Metadata     map[string]string `json:"metadata"`
	Tags         []string          `json:"tags"`
	MinSize      int64             `json:"min_size"`
	MaxSize      int64             `json:"max_size"`
	StartDate    *time.Time        `json:"start_date"`
	EndDate      *time.Time        `json:"end_date"`
	ContentTypes []string          `json:"content_types"`
	Limit        int               `json:"limit"`
}

package dto

import "time"

type SearchFilesInput struct {
	Query    string `form:"query" binding:"required"`
	BucketID string `form:"bucket_id"`
	Limit    int    `form:"limit"`
}

type SearchByMetadataInput struct {
	Metadata map[string]string `form:"metadata" binding:"required"`
	BucketID string            `form:"bucket_id"`
	Limit    int               `form:"limit"`
}

type SearchByTagsInput struct {
	Tags     []string `form:"tags" binding:"required"`
	BucketID string   `form:"bucket_id"`
	Limit    int      `form:"limit"`
}

type SearchByContentInput struct {
	Query    string `form:"query" binding:"required"`
	BucketID string `form:"bucket_id"`
	Limit    int    `form:"limit"`
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

type SearchResultOutput struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
}

type SearchResult struct {
	ID          string            `json:"id"`
	BucketID    string            `json:"bucket_id"`
	Key         string            `json:"key"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type"`
	Metadata    map[string]string `json:"metadata"`
	Tags        []string          `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	Relevance   float64           `json:"relevance,omitempty"`
}

type SearchSuggestionsInput struct {
	Query    string `form:"query" binding:"required"`
	BucketID string `form:"bucket_id"`
	Limit    int    `form:"limit"`
}

type SearchSuggestionsOutput struct {
	Suggestions []string `json:"suggestions"`
}

type SearchHistoryInput struct {
	Limit int `form:"limit"`
}

type SearchHistoryOutput struct {
	History []SearchHistoryItem `json:"history"`
}

type SearchHistoryItem struct {
	Query     string    `json:"query"`
	Timestamp time.Time `json:"timestamp"`
	Results   int       `json:"results"`
}

type SaveSearchInput struct {
	Name        string            `json:"name" binding:"required"`
	Query       string            `json:"query" binding:"required"`
	Filters     map[string]string `json:"filters"`
	Description string            `json:"description"`
}

type SaveSearchOutput struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
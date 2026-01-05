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
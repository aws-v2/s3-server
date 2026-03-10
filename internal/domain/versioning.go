package domain

type VersioningStatus string

const (
	VersioningEnabled   VersioningStatus = "Enabled"
	VersioningSuspended VersioningStatus = "Suspended"
)

type VersioningOutput struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"` // "Enabled", "Suspended", or ""
}

package dto

import (
	"time"
)

// PingResponse represents a simple ping-pong check.
type PingResponse struct {
	Response string    `json:"response"`
	PingedAt time.Time `json:"pinged_at"`
}

// StatusResponse shows system-level status.
type StatusResponse struct {
	Database  string    `json:"database"`
	Storage   string    `json:"storage"`
	CheckedAt time.Time `json:"checked_at"`
}

type Metrics struct {
	CPUPercent  float64 `json:"cpu_percent"`
	DiskTotalGB uint64  `json:"disk_total_gb"`
}

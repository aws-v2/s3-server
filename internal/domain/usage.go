package domain

import "time"

// UsageMeter accumulates usage for one request. It lives in the request
// context: the middleware creates it, handlers and services enrich it.
type UsageMeter struct {
	EventID, Kind, Direction          string // Direction: "ingress" | "egress"
	UserID, BucketID, OwnerID, Region string
	BytesIn, BytesOut                 int64
	PayloadOut                        int64 // logical file bytes, set by legacy handlers whose wire format inflates the body
	Objects                           int
	ReadErr, WriteErr                 error
	Start                             time.Time
}

// BillableOut is min(wire bytes, payload bytes) when a payload size is known.
func (m *UsageMeter) BillableOut() int64 {
	if m.PayloadOut > 0 && m.BytesOut > m.PayloadOut {
		return m.PayloadOut
	}
	return m.BytesOut
}

// UsageEvent is the replayable record published for billing.
type UsageEvent struct {
	EventID         string    `json:"event_id"`
	Kind            string    `json:"kind"`
	Direction       string    `json:"direction"`
	Status          string    `json:"status"`
	UserID          string    `json:"user_id"`  // who made the request (may be the nil UUID on token routes)
	OwnerID         string    `json:"owner_id"` // who is billed
	BucketID        string    `json:"bucket_id"`
	Region          string    `json:"region"`
	BytesIn         int64     `json:"bytes_in"`
	BytesOut        int64     `json:"bytes_out"`
	Objects         int       `json:"objects"`
	DurationMs      int64     `json:"duration_ms"`
	At              time.Time `json:"at"`
	BytesStored     int64     `json:"bytes_stored"`
	IntervalSeconds int64     `json:"interval_seconds"`
}



type BucketUsage struct {
	OwnerID, BucketID, Region string
	Bytes                     int64
	Objects                   int
}

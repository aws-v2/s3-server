package usage

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"strconv"
	"time"

	"s3/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type meterKey struct{}

const nilUUID = "00000000-0000-0000-0000-000000000000"

func isBillableUser(id string) bool { return id != "" && id != nilUUID }

// Publisher is swapped for a NATS JetStream publisher later.
type Publisher interface {
	Publish(ctx context.Context, e domain.UsageEvent) error
}

type logPublisher struct{}

func (logPublisher) Publish(_ context.Context, e domain.UsageEvent) error {
	b, _ := json.Marshal(e)
	log.Printf("[usage**] %s of kind %v", b, e.Kind)
	return nil
}

var publisher Publisher = logPublisher{}

func SetPublisher(p Publisher) { publisher = p }

var (
	ingressBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "s3_ingress_bytes_total", Help: "Bytes received on upload routes"},
		[]string{"kind", "status"}, // never label by user: cardinality
	)
	egressBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "s3_egress_bytes_total", Help: "Bytes sent on download routes"},
		[]string{"kind", "status"},
	)
	unattributedBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "s3_unattributed_bytes_total", Help: "Bytes with no billable owner"},
		[]string{"direction"},
	)
)

// FromContext returns the request's meter, or a throwaway one so callers need no nil checks.
func FromContext(ctx context.Context) *domain.UsageMeter {
	if m, ok := ctx.Value(meterKey{}).(*domain.UsageMeter); ok {
		return m
	}
	return &domain.UsageMeter{}
}

// ---------- ingress: count bytes read from the request body ----------

type countingBody struct {
	io.ReadCloser
	m *domain.UsageMeter
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.m.BytesIn += int64(n)
	if err != nil && err != io.EOF {
		b.m.ReadErr = err
	}
	return n, err
}

func MeterIngress(kind string) gin.HandlerFunc {
	log.Printf("Metering the ingress")
	return func(c *gin.Context) {
		m := &domain.UsageMeter{
			Kind:      kind,
			Direction: "ingress",
			EventID:   uuid.NewString(),
			Start:     time.Now(),
			UserID:    c.GetString("userId"), // empty on presign until the handler sets it
		}
		userId := c.GetString("userId")
		m.OwnerID = userId
		m.UserID = userId
		c.Request.Body = &countingBody{c.Request.Body, m}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), meterKey{}, m))

		c.Next()

		status := "completed"
		switch {
		case m.ReadErr != nil:
			status = "aborted"
		case c.Writer.Status() >= 300:
			status = "failed"
		}

		ingressBytes.WithLabelValues(kind, status).Add(float64(m.BytesIn))
		emit(m, status)
	}
}

// ---------- egress: count bytes written to the response ----------

type countingWriter struct {
	gin.ResponseWriter
	m *domain.UsageMeter
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.m.BytesOut += int64(n)
	if err != nil && w.m.WriteErr == nil {
		w.m.WriteErr = err
	}
	return n, err
}

func (w *countingWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.m.BytesOut += int64(n)
	if err != nil && w.m.WriteErr == nil {
		w.m.WriteErr = err
	}
	return n, err
}

func MeterEgress(kind string) gin.HandlerFunc {
	log.Printf("Metering the Egress")

	return func(c *gin.Context) {
		userId := c.GetString("userId")
		m := &domain.UsageMeter{
			Kind:      kind,
			Direction: "egress",
			EventID:   uuid.NewString(),
			Start:     time.Now(),
			BucketID:  c.Param("bucketId"), // service overwrites with the resolved ID
			Objects:   1,                   // one GET per request on these routes
		}
		m.OwnerID = userId
		m.UserID = userId

		c.Writer = &countingWriter{c.Writer, m}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), meterKey{}, m))

		c.Next()

		declared, _ := strconv.ParseInt(c.Writer.Header().Get("Content-Length"), 10, 64)

		status := "completed"
		switch {
		case c.Writer.Status() >= 400:
			status = "failed"
		case m.WriteErr != nil:
			status = "aborted" // client went away mid-write
		case declared > 0 && m.BytesOut < declared:
			status = "aborted" // truncated: client abort or source failure
		}

		m.BytesOut = m.BillableOut()
		egressBytes.WithLabelValues(kind, status).Add(float64(m.BytesOut))

		if status == "failed" {
			return // error body only, no file data delivered
		}
		emit(m, status)
	}
}

// ---------- shared ----------

// emit publishes one usage event. The bucket owner is billed; the requester
// is only a fallback when the bucket was never resolved.
func emit(m *domain.UsageMeter, status string) {
	log.Printf("Emitting kind %v", m.Kind)

	total := m.BytesIn + m.BytesOut

	owner := m.OwnerID
	if owner == "" && isBillableUser(m.UserID) {
		owner = m.UserID
	}
	log.Printf("Emitting kind 0%v", m.Kind)

	if owner == "" {
		unattributedBytes.WithLabelValues(m.Direction).Add(float64(total))
		return // nobody to bill
	}
	log.Printf("Emitting kind 1 %v", m.Kind)

	// Fresh context: the request context is already cancelled on client aborts,
	// and aborted events are exactly the ones we must not lose.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	log.Printf("Emitting kind 3 %v", m.Kind)

	err := publisher.Publish(ctx, domain.UsageEvent{
		EventID: m.EventID, Kind: m.Kind, Direction: m.Direction, Status: status,
		UserID: m.UserID, OwnerID: owner, BucketID: m.BucketID, Region: m.Region,
		BytesIn: m.BytesIn, BytesOut: m.BytesOut, Objects: m.Objects,
		DurationMs: time.Since(m.Start).Milliseconds(), At: time.Now(),
	})
	log.Printf("Emitting kind 3c %v", m.Kind)

	if err != nil {
		log.Printf("[usage] publish failed event_id=%s err=%v", m.EventID, err)
	}
	log.Printf("Emitting kind 3cc %v", m.Kind)

}

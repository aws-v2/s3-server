package usage

import (
	"context"
	"fmt"
	"log"
	"s3/internal/domain"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)


// type StorageRepo interface {
// 	SampleBucketUsage(ctx context.Context) ([]domain.BucketUsage, error)
// }

var storedBytes = promauto.NewGaugeVec(
	prometheus.GaugeOpts{Name: "s3_stored_bytes", Help: "Bytes stored at last sample"},
	[]string{"region"}, // never label by owner or bucket: cardinality
)

// RunStorageSampler samples on clock-aligned boundaries (00:00, 06:00, ... UTC for 6h).
func RunStorageSampler(ctx context.Context, repo domain.FileRepository, every time.Duration) {
	for {
		next := time.Now().UTC().Truncate(every).Add(every)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			sampleOnce(ctx, repo, next, every)
		}
	}
}

func sampleOnce(ctx context.Context, repo domain.FileRepository, at time.Time, every time.Duration) {
	log.Printf("[usage] storage running a sample omce")

	rows, err := repo.SampleBucketUsage(ctx)
	if err != nil {
		log.Printf("[usage] storage sample failed at=%s err=%v", at, err)
		return
	}
	perRegion := map[string]int64{}
	for _, r := range rows {
		perRegion[r.Region] += r.Bytes

		pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := publisher.Publish(pctx, domain.UsageEvent{
			EventID:         fmt.Sprintf("storage:%s:%d", r.BucketID, at.Unix()), // deterministic: replays never double bill
			Kind:            "storage",
			Status:          "sampled",
			OwnerID:         r.OwnerID,
			BucketID:        r.BucketID,
			Region:          r.Region,
			BytesStored:     r.Bytes,
			Objects:         r.Objects,
			IntervalSeconds: int64(every.Seconds()),
			At:              at,
		})
		cancel()
		if err != nil {
			log.Printf("[usage] storage publish failed bucket=%s err=%v", r.BucketID, err)
		}
	}
	for region, b := range perRegion {
		storedBytes.WithLabelValues(region).Set(float64(b))
	}
}

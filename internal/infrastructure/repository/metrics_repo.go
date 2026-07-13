package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"s3/internal/domain"
	"time"
)

type MetricsRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewMetricsRepo(db *sql.DB, logger *slog.Logger) *MetricsRepository {
	if logger == nil {
		logger = slog.Default()
	}
	return &MetricsRepository{
		db:     db,
		logger: logger.With("component", "metrics_repository"),
	}
}



func (r *MetricsRepository) Insert(
	ctx context.Context,
	hostID string,
	metric domain.DomainStats,
) error {
	log := r.logger.With(
		"method", "Insert",
		"vm_id", metric.VMID,
		"host_id", hostID,
		"vm_name", metric.Name,
		"state", metric.State,
	)

	log.DebugContext(ctx, "inserting vm metric",
		"cpu_used", metric.CPUUsed,
		"memory_used", metric.Memory,
		"disk_read", metric.DiskRead,
		"disk_write", metric.DiskWrite,
		"net_rx", metric.NetRx,
		"net_tx", metric.NetTx,
	)

	start := time.Now()

	_, err := r.db.ExecContext(ctx, `
        INSERT INTO vm_metrics (
            vm_id,
            host_id,
            name,
            state,
            cpu_used,
            memory_used,
            disk_read,
            disk_write,
            net_rx,
            net_tx,
            created_at
        )
        VALUES (
            $1, $2, $3, $4, $5,
            $6, $7, $8, $9, $10,
            NOW()
        )
    `,
		metric.VMID,
		hostID,
		metric.Name,
		metric.State,
		metric.CPUUsed,
		metric.Memory,
		metric.DiskRead,
		metric.DiskWrite,
		metric.NetRx,
		metric.NetTx,
	)

	elapsed := time.Since(start)

	if err != nil {
		log.ErrorContext(ctx, "failed to insert vm metric",
			"error", err,
			"duration_ms", elapsed.Milliseconds(),
		)
		return err
	}

	log.InfoContext(ctx, "vm metric inserted",
		"duration_ms", elapsed.Milliseconds(),
	)

	return nil
}

func (r *MetricsRepository) GetVMsRequiringAction(ctx context.Context, sleepDuration, terminateDuration time.Duration) ([]domain.VMActionTarget, error) {
	return nil, nil
}

var _ domain.MetricsRepository = (*MetricsRepository)(nil)

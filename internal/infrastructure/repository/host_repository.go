package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"s3/internal/domain"

	"github.com/lib/pq"
)

func (r *PostgresRepository) UpsertHost(ctx context.Context, host *domain.Host) error {

	query := `
		INSERT INTO hosts (id, hosttype,hostname, ip, ssh_user, ssh_private_key, cpu_total, cpu_used, ram_total, ram_free, disk_total, disk_free, status, last_heartbeat, available_templates)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)	
		ON CONFLICT (id) DO UPDATE SET
		hosttype=	EXCLUDED.hosttype,
		hostname = EXCLUDED.hostname,
			ip = EXCLUDED.ip,
			ssh_user = CASE 
				WHEN hosts.ssh_user IS NOT NULL AND hosts.ssh_user != '' AND hosts.ssh_user != 'root' THEN hosts.ssh_user 
				ELSE EXCLUDED.ssh_user 
			END,
			ssh_private_key = CASE
				WHEN EXCLUDED.ssh_private_key != '' THEN EXCLUDED.ssh_private_key
				ELSE hosts.ssh_private_key
			END,
			cpu_total = EXCLUDED.cpu_total,
			cpu_used = EXCLUDED.cpu_used,
			ram_total = EXCLUDED.ram_total,
			ram_free = EXCLUDED.ram_free,
			disk_total = EXCLUDED.disk_total,
			disk_free = EXCLUDED.disk_free,
			available_templates = EXCLUDED.available_templates,
			status = EXCLUDED.status,
			last_heartbeat = EXCLUDED.last_heartbeat
	`
	_, err := r.db.Exec(query,
		host.ID, host.HostType, host.Hostname, host.IP, host.SSHUser, host.SSHPrivateKey,
		host.CPUTotal, host.CPUUsed, host.RAMTotal, host.RAMFree,
		host.DiskTotal, host.DiskFree, host.Status, host.LastHeartbeat,
		pq.Array(host.AvailableTemplates),
	)


	if err != nil {
		return fmt.Errorf("failed to upsert host: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetHostByID(ctx context.Context, hostID string) (*domain.Host, error) {
	query := `
		SELECT id, hosttype, hostname, ip, cpu_total, cpu_used, ram_total, ram_free,
		       disk_total, disk_free, status, ssh_user, last_heartbeat, created_at,
		       available_templates, ssh_private_key
		FROM hosts
		WHERE id = $1
	`

	host, err := r.scanHost(r.db.QueryRowContext(ctx, query, hostID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get host: %w", err)
	}
	return host, nil
}

func (r *PostgresRepository) ListHostsByType(ctx context.Context, hostType string) ([]domain.Host, error) {
	query := `
		SELECT id, hosttype, hostname, ip, cpu_total, cpu_used, ram_total, ram_free,
		       disk_total, disk_free, status, ssh_user, last_heartbeat, created_at,
		       available_templates, ssh_private_key
		FROM hosts
		WHERE hosttype = $1
		ORDER BY disk_free DESC, cpu_used ASC, last_heartbeat DESC
	`

	rows, err := r.db.QueryContext(ctx, query, hostType)
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts: %w", err)
	}
	defer rows.Close()

	var hosts []domain.Host
	for rows.Next() {
		host, err := r.scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, *host)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

type hostScanner interface {
	Scan(dest ...interface{}) error
}

func (r *PostgresRepository) scanHost(scanner hostScanner) (*domain.Host, error) {
	var host domain.Host
	err := scanner.Scan(
		&host.ID,
		&host.HostType,
		&host.Hostname,
		&host.IP,
		&host.CPUTotal,
		&host.CPUUsed,
		&host.RAMTotal,
		&host.RAMFree,
		&host.DiskTotal,
		&host.DiskFree,
		&host.Status,
		&host.SSHUser,
		&host.LastHeartbeat,
		&host.CreatedAt,
		pq.Array(&host.AvailableTemplates),
		&host.SSHPrivateKey,
	)
	if err != nil {
		return nil, err
	}
	return &host, nil
}

var _ domain.HostRepository = (*PostgresRepository)(nil)

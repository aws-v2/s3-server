package application

import (
	"context"
	"fmt"
	"s3/internal/domain"
)

type HostScheduler struct {
	hostRepo domain.HostRepository
}

func NewHostScheduler(hostRepo domain.HostRepository) *HostScheduler {
	return &HostScheduler{hostRepo: hostRepo}
}

func (s *HostScheduler) SelectHost(ctx context.Context, hostType string) (*domain.Host, error) {
	hosts, err := s.hostRepo.ListHostsByType(ctx, hostType)
	if err != nil {
		return nil, err
	}

	var best *domain.Host
	for i := range hosts {
		host := hosts[i]
		if host.Status != "active" {
			continue
		}
		if best == nil ||
			host.DiskFree > best.DiskFree ||
			(host.DiskFree == best.DiskFree && host.CPUUsed < best.CPUUsed) {
			best = &host
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no active %s hosts available", hostType)
	}
	return best, nil
}

var _ domain.HostScheduler = (*HostScheduler)(nil)

package application

import (
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
)

// HealthService checks system and storage health.
type HealthService struct {
	repo    domain.RepositoryPort
	storage domain.StoragePort
	system domain.SystemPort
}


// Constructor
func NewHealthService(repo domain.RepositoryPort, storage domain.StoragePort, system domain.SystemPort) *HealthService {
	return &HealthService{
		repo:    repo,
		storage: storage,
		system:system,
	}
}

func (s *HealthService) Ping() *dto.PingResponse {
	return &dto.PingResponse{
		Response: "Pong",
		PingedAt: time.Now(),
	}
}

func (s *HealthService) Status() *dto.StatusResponse {
	dbStatus := "ok"
	storageStatus := "ok"
	if err := s.system.HealthCheck(); err != nil {
	    dbStatus = "unreachable"
	}
	return &dto.StatusResponse{
		Database: dbStatus,
		Storage:  storageStatus,
		CheckedAt: time.Now(),
	}
}



func (s * HealthService)GetMetrics() *dto.Metrics {
	cpuPercent, _ := cpu.Percent(0, false)
	diskStat, _ := disk.Usage("/")
	metrics := dto.Metrics{
		CPUPercent:    cpuPercent[0],
		DiskTotalGB:   diskStat.Total / 1024 / 1024 / 1024,
	}
	return &metrics

}
package application

import (
	"context"
	"log"
	"s3/internal/domain"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type MetricsService struct{
	metricsRepo domain.MetricsRepository
	hostRepo domain.HostRepository
	key string 

}




func NewMetricsService(metricsRepo domain.MetricsRepository, hostRepo domain.HostRepository) MetricsService{
	return MetricsService{
		metricsRepo: metricsRepo,
		hostRepo: hostRepo,
		key: "key",
	}
}
type HeartbeatResponse struct {
	EC2PublicKey string `json:"ec2_public_key"`
}


func (s *MetricsService) HandleHeartbeat(req domain.HeartbeatRequest,ctx *gin.Context) (*HeartbeatResponse, error) {
	log.Printf("[host-service] Heartbeat from host=%s ip=%s cpu_used=%v ram_free=%v vms_count=%d", 
		req.HostID, req.IP, req.CPUUsed, req.RAMFree, len(req.VMs))

	sshUser := req.SSHUser
	parts := strings.Split(req.Hostname, ":")
	if len(parts) >= 7 && parts[6] != "root" && parts[6] != "" {
		log.Printf("[host-service] Extracted SSH user '%s' from heartbeat hostname", parts[6])
		sshUser = parts[6]
	}

	host := &domain.Host{
		ID:                 req.HostID,
		Hostname:           req.Hostname,
		HostType: req.HostType,
		IP:                 req.IP,
		SSHUser:            sshUser,
		CPUTotal:           req.CPUTotal,
		CPUUsed:            req.CPUUsed,
		RAMTotal:           req.RAMTotal,
		RAMFree:            req.RAMFree,
		DiskTotal:          req.DiskTotal,
		DiskFree:           req.DiskFree,
		AvailableTemplates: req.AvailableTemplates,
		SSHPrivateKey:      req.SSHPrivateKey,
		Status:             "active",
		LastHeartbeat:      time.Now(),
	}

	if s.hostRepo != nil {
		if err := s.hostRepo.UpsertHost(ctx.Request.Context(), host); err != nil {
			log.Printf("[host-service] Failed to update host %s: %v", req.HostID, err)
			return nil, err
		}
	}

	for _, vm := range req.VMs {
		metric := domain.DomainStats{
			VMID:      vm.VMID,
			HostID:    req.HostID,
			Name:      vm.Name,
			CPUUsed:   vm.CPUUsed,
			Memory:    vm.Memory,
			DiskRead:  vm.DiskRead,
			DiskWrite: vm.DiskWrite,
			NetRx:     vm.NetRx,
			NetTx:     vm.NetTx,
			State:     vm.State,
			CreatedAt: time.Now(),
		}
		log.Printf("  -> [vm-met8rics] name=%-15s state=%-10s cpu=%.2f ram=%d disk(r/w)=%d/%d net(rx/tx)=%d/%d",
			metric.Name, metric.State, metric.CPUUsed, metric.Memory, metric.DiskRead, metric.DiskWrite, metric.NetRx, metric.NetTx)
		go func(hID string, m domain.DomainStats) {
			if err := s.metricsRepo.Insert(context.Background(), hID, m); err != nil {
				log.Printf("[host-service] Failed to insert metrics for VM %s: %v", m.VMID, err)
			}
		}(host.ID, metric)
	}

	return &HeartbeatResponse{
		EC2PublicKey: s.key,
	}, nil
}

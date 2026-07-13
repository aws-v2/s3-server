package domain

import "time"

type HeartbeatRequest struct {
	HostType string `json:"hosttype"`
	HostID             string        `json:"host_id"`
	Hostname           string        `json:"hostname"`
	IP                 string        `json:"ip"`
	CPUTotal           float64       `json:"cpu_total"`
	CPUUsed            float64       `json:"cpu_used"`
	RAMTotal           float64       `json:"ram_total"`
	RAMFree            float64       `json:"ram_free"`
	DiskTotal          float64       `json:"disk_total"`
	DiskFree           float64       `json:"disk_free"`
	AvailableTemplates []string      `json:"available_templates"`
	SSHPrivateKey      string        `json:"ssh_private_key"`
	SSHUser            string        `json:"ssh_user"`
	VMs                []DomainStats `json:"vms"`
}



type DomainStats struct {
	VMID      string  `json:"vmid"`
	HostID    string  `json:"hostid"`
	CreatedAt time.Time `json:"created_at"`
	Name      string  `json:"name"`
	State     string  `json:"state"`
	CPUUsed   float64 `json:"cpu_used"`   // Changed to float64
	Memory    uint64  `json:"memory_kb"`  // Changed to uint64
	DiskRead  uint64  `json:"disk_read"`  // Changed to uint64
	DiskWrite uint64  `json:"disk_write"` // Changed to uint64
	NetRx     uint64  `json:"net_rx"`     // Changed to uint64
	NetTx     uint64  `json:"net_tx"`     // Changed to uint64
}


type Host struct {
	HostType string `json:"hosttype"`
	ID            string    `json:"id"`
	Hostname      string    `json:"hostname"`
	IP            string    `json:"ip"`
	CPUTotal      float64       `json:"cpu_total"`
	CPUUsed       float64       `json:"cpu_used"`
	RAMTotal      float64       `json:"ram_total"` // in MB
	RAMFree       float64       `json:"ram_free"`  // in MB
	DiskTotal     float64       `json:"disk_total"` // in GB
	DiskFree      float64       `json:"disk_free"`  // in GB
	Status        string    `json:"status"`    // active, inactive, maintenance
	SSHUser       string    `json:"ssh_user"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	CreatedAt     time.Time `json:"created_at"`
	AvailableTemplates []string  `json:"available_templates"`
	SSHPrivateKey      string    `json:"ssh_private_key"`
}

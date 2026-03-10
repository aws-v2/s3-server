package domain

import "time"

type AccessPoint struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	BucketID      string    `json:"bucket_id"`
	NetworkOrigin string    `json:"network_origin"` // e.g., 'internet', 'vpc'
	VpcID         string    `json:"vpc_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ListAccessPointsOutput struct {
	AccessPoints  []AccessPoint `json:"access_points"`
	AvailableVPCs []VPC         `json:"available_vpcs"`
}

type CreateAccessPointInput struct {
	Name          string `json:"name"`
	NetworkOrigin string `json:"networkOrigin"`
	VpcID         string `json:"vpcId,omitempty"`
}

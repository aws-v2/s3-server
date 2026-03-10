package application

import (
	"context"
	"fmt"
	"s3/internal/domain"
	"time"

	"github.com/google/uuid"
)

type AccessPointService struct {
	repo domain.RepositoryPort
	net  domain.NetworkPort
}

func NewAccessPointService(repo domain.RepositoryPort, net domain.NetworkPort) *AccessPointService {
	return &AccessPointService{
		repo: repo,
		net:  net,
	}
}

func (s *AccessPointService) resolveBucket(ctx context.Context, idOrName string, filterID string) (domain.Bucket, error) {
	// Try by ID first
	bucket, err := s.repo.GetBucketByID(ctx, idOrName, filterID)
	if err == nil {
		return bucket, nil
	}

	// Try by Name as fallback
	bucket, err = s.repo.GetBucketByName(ctx, idOrName, filterID)
	if err == nil {
		return bucket, nil
	}

	return domain.Bucket{}, fmt.Errorf("bucket not found: %s", idOrName)
}

func (s *AccessPointService) CreateAccessPoint(ctx context.Context, bucketID, name, networkOrigin, vpcID string) (*domain.AccessPoint, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	ap := &domain.AccessPoint{
		ID:            uuid.New().String(),
		Name:          name,
		BucketID:      bucket.ID,
		NetworkOrigin: networkOrigin,
		VpcID:         vpcID,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repo.SaveAccessPoint(ctx, ap); err != nil {
		return nil, err
	}

	return ap, nil
}

func (s *AccessPointService) ListAccessPoints(ctx context.Context, bucketID string) (*domain.ListAccessPointsOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	aps, err := s.repo.ListAccessPoints(ctx, bucket.ID)
	if err != nil {
		return nil, err
	}

	// Fetch VPCs for the dropdown
	vpcs, err := s.net.ListVPCs(ctx, actor.ID)
	if err != nil {
		fmt.Printf("Warning: failed to fetch VPCs from network service: %v\n", err)
		vpcs = []domain.VPC{} // Return empty list rather than failing
	}

	return &domain.ListAccessPointsOutput{
		AccessPoints:  aps,
		AvailableVPCs: vpcs,
	}, nil
}

func (s *AccessPointService) UpdateAccessPointOrigin(ctx context.Context, bucketID, name, networkOrigin, vpcID string) (*domain.AccessPoint, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	ap, err := s.repo.GetAccessPointByName(ctx, bucket.ID, name)
	if err != nil {
		return nil, err
	}

	ap.NetworkOrigin = networkOrigin
	ap.VpcID = vpcID
	ap.UpdatedAt = time.Now()

	if err := s.repo.UpdateAccessPoint(ctx, ap); err != nil {
		return nil, err
	}

	return ap, nil
}

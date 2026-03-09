package application

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"time"
)

// BucketService provides business logic for managing buckets.
type BucketService struct {
	repo    domain.RepositoryPort
	storage domain.StoragePort
}

type BucketAlreadyExists struct {
	Name string
}

// NewBucketService creates a new instance of BucketService.
func NewBucketService(repo domain.RepositoryPort, storage domain.StoragePort) *BucketService {
	return &BucketService{
		repo:    repo,
		storage: storage,
	}
}
func (e *BucketAlreadyExists) Error() string {
	return fmt.Sprintf("bucket %s already exists", e.Name)
}

func (s *BucketService) CreateBucket(ctx context.Context, input dto.CreateBucketInput) (*dto.CreateBucketOutput, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	// Try to create bucket in storage
	bucketId, err := s.storage.CreateBucket(ctx, input.Name)
	if err != nil {
		// Handle "bucket already exists" gracefully using type assertion
		if _, ok := err.(*BucketAlreadyExists); ok {
			return nil, fmt.Errorf("bucket already exists: %s", input.Name)
		}
		return nil, fmt.Errorf("%w", err)
	}

	// Determine versioning status
	vStatus := domain.VersioningSuspended
	if input.Versioning {
		vStatus = domain.VersioningEnabled
	}

	// Map tags
	var tags []domain.Tag
	for _, t := range input.Tags {
		tags = append(tags, domain.Tag{
			Key:   t.Key,
			Value: t.Value,
		})
	}

	arn := fmt.Sprintf("arn:serw:s3:%s:%s:bucket/%s", input.Region, input.OwnerId, input.Name)

	bucket := &domain.Bucket{
		ID:              bucketId,
		Name:            input.Name,
		OwnerID:         input.OwnerId,
		ARN:             arn,
		Region:          input.Region,
		BucketType:      input.BucketType,
		ObjectOwnership: input.ObjectOwnership,
		BlockPublicAccess: domain.BlockPublicAccess{
			BlockPublicAcls:       input.BlockPublicAccess.BlockPublicAcls,
			IgnorePublicAcls:      input.BlockPublicAccess.IgnorePublicAcls,
			BlockPublicPolicy:     input.BlockPublicAccess.BlockPublicPolicy,
			RestrictPublicBuckets: input.BlockPublicAccess.RestrictPublicBuckets,
		},
		VersioningStatus: vStatus,
		Tags:             tags,
		Encryption: domain.BucketEncryption{
			Type:             input.Encryption.Type,
			BucketKeyEnabled: input.Encryption.BucketKeyEnabled,
		},
		ObjectLock: input.ObjectLock,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Save metadata in repository
	bucketObject, err := s.repo.SaveBucket(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to save bucket metadata: %w", err)
	}

	// Return success
	return &dto.CreateBucketOutput{
		BucketID:  bucketObject.ID,
		Name:      input.Name,
		CreatedAt: bucket.CreatedAt,
	}, nil
}

func (s *BucketService) GetBucket(ctx context.Context, bucketID string) (*dto.GetBucketOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.repo.GetBucketByID(ctx, bucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	return &dto.GetBucketOutput{
		BucketID:   bucket.ID,
		Name:       bucket.Name,
		CreatedAt:  bucket.CreatedAt,
		BucketType: bucket.BucketType,
		Region:     bucket.Region,
	}, nil
}

func (s *BucketService) ListBuckets(ctx context.Context) ([]domain.Bucket, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}
	return s.repo.ListBuckets(ctx, filterID)
}

func (s *BucketService) UpdateBucket(ctx context.Context, bucketID string, input dto.UpdateBucketInput) (*dto.GetBucketOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.repo.GetBucketByID(ctx, bucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	if input.Name != "" {
		bucket.Name = input.Name
		bucket.UpdatedAt = time.Now()
	}
	error := s.storage.RenameBucket(ctx, bucket.Name, input.Name)
	if error != nil {
		return nil, fmt.Errorf("failed to rename bucket: %w", err)
	}

	updated, err := s.repo.UpdateBucket(ctx, &bucket, filterID)
	if err != nil {
		return nil, fmt.Errorf("failed to update bucket: %w", err)
	}

	return &dto.GetBucketOutput{
		BucketID:  updated.ID,
		Name:      updated.Name,
		CreatedAt: updated.CreatedAt,
	}, nil
}

func (s *BucketService) DeleteBucket(ctx context.Context, bucketID string) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	files, err := s.repo.ListFiles(ctx, bucketID)
	if err != nil {
		return fmt.Errorf("failed to check files: %w", err)
	}

	if len(files) > 0 {
		return fmt.Errorf("cannot delete bucket with files")
	}
	bucket, err := s.repo.GetBucketByName(ctx, bucketID, filterID)

	if err != nil {
		return fmt.Errorf("bucket not found: %w", err)
	}

	if err := s.storage.DeleteBucket(ctx, bucket.Name); err != nil {

		return fmt.Errorf("failed to delete from storage: %w", err)
	}
	if err := s.repo.DeleteBucket(ctx, bucketID); err != nil {

		return fmt.Errorf("failed to delete bucket: %w", err)
	}
	return nil
}

func (s *BucketService) EmptyBucket(ctx context.Context, bucketID string) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.repo.GetBucketByID(ctx, bucketID, filterID)
	if err != nil {
		return fmt.Errorf("bucket not found: %w", err)
	}

	// 1. Delete all objects from storage
	if err := s.storage.EmptyBucket(ctx, bucket.Name); err != nil {
		return fmt.Errorf("failed to empty storage: %w", err)
	}

	// 2. Delete all file metadata from repository
	if err := s.repo.DeleteFilesByBucket(ctx, bucketID); err != nil {
		return fmt.Errorf("failed to delete file metadata: %w", err)
	}

	return nil
}

func (s *BucketService) GetBucketStats(ctx context.Context, bucketID string) (*dto.BucketStatsOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.repo.GetBucketByID(ctx, bucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found or access denied: %w", err)
	}

	files, err := s.repo.ListFiles(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get files: %w", err)
	}

	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}

	return &dto.BucketStatsOutput{
		BucketID:   bucketID,
		TotalFiles: int64(len(files)),
		TotalSize:  totalSize,
	}, nil
}

func (s *BucketService) GetBucketPolicy(ctx context.Context, bucketID string) (*dto.BucketPolicyOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.repo.GetBucketByID(ctx, bucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	policyStr := ""
	if bucket.Policy != nil {
		policyStr = bucket.Policy.String()
	}

	return &dto.BucketPolicyOutput{
		BucketID: bucketID,
		Policy:   policyStr,
	}, nil
}

func (s *BucketService) UpdateBucketPolicy(ctx context.Context, bucketID string, input dto.UpdatePolicyInput, actorID string) error {
	// actorID is "user:<id>" or "role:admin"

	filterID := actorID
	if IsAdmin(actorID) {
		filterID = ""
	}

	bucket, err := s.repo.GetBucketByID(ctx, bucketID, filterID)
	if err != nil {
		return fmt.Errorf("bucket not found: %w", err)
	}

	// Permission: only owner or admin can update
	if actorID != fmt.Sprintf("user:%s", bucket.OwnerID) && !IsAdmin(actorID) {
		return errors.New("forbidden: only bucket owner or admin can update policy")
	}

	// Convert DTO -> domain.Policy
	policy := domain.Policy{
		Version: input.Version,
		Statement: []domain.Statement{
			{
				Effect:    domain.Effect(input.Effect),
				Action:    make([]domain.Action, 0, len(input.Actions)),
				Resource:  input.Resources,
				Principal: make([]domain.Principal, 0, len(input.Principals)),
				Condition: input.Conditions,
			},
		},
	}

	// convert string slices to typed slices
	for _, a := range input.Actions {
		policy.Statement[0].Action = append(policy.Statement[0].Action, domain.Action(a))
	}

	for _, p := range input.Principals {
		policy.Statement[0].Principal = append(policy.Statement[0].Principal, domain.Principal(p))
	}

	// validate
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("invalid policy: %w", err)
	}

	// set and save; increment version
	bucket.Policy = &policy
	bucket.UpdatedAt = time.Now()
	if err := s.repo.IncrementPolicyVersionAndUpdateBucket(ctx, &bucket); err != nil {
		return fmt.Errorf("failed to save policy: %w", err)
	}

	// audit / history - optional
	_ = s.repo.AppendPolicyHistory(ctx, bucketID, &policy, actorID)

	return nil
}

func (s *BucketService) SetBucketVersioning(ctx context.Context, bucketID string, enabled bool) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.repo.GetBucketByID(ctx, bucketID, filterID)
	if err != nil {
		return fmt.Errorf("bucket not found: %w", err)
	}

	// Set versioning in storage layer (MinIO)
	if err := s.storage.SetBucketVersioning(ctx, bucket.Name, enabled); err != nil {
		return fmt.Errorf("failed to set versioning in storage: %w", err)
	}

	// Persist versioning status to database
	status := domain.VersioningSuspended
	if enabled {
		status = domain.VersioningEnabled
	}
	if err := s.repo.SetBucketVersioning(ctx, bucketID, status); err != nil {
		return fmt.Errorf("failed to persist versioning status: %w", err)
	}

	return nil
}

// isAdmin checks whether the actor (like "user:abc123") is an admin.
// In MVP mode, we load admin IDs from an env var: ADMIN_USERS=user:abc123,user:def456
func IsAdmin(actor string) bool {
	admins := os.Getenv("ADMIN_USERS")

	// fallback for tests only — change or remove for production
	if strings.TrimSpace(admins) == "" {
		admins = "user:550e8400-e29b-41d4-a716-446655440000" // <-- dummy admin for testing; remove/override in prod
	}

	for _, a := range strings.Split(admins, ",") {
		if strings.TrimSpace(a) == actor {
			return true
		}
	}
	return false
}

func (s *BucketService) GetBucketVersioning(ctx context.Context, bucketID string) (*dto.VersioningOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Verify bucket ownership first
	_, err := s.repo.GetBucketByID(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	// Get versioning status from database
	status, err := s.repo.GetBucketVersioning(ctx, bucketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get versioning status: %w", err)
	}

	return &dto.VersioningOutput{
		Status: string(status),
	}, nil
}

func (s *BucketService) SetBucketLifecycle(ctx context.Context, bucketID string, input dto.SetLifecycleInput) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Verify bucket ownership first
	_, err := s.repo.GetBucketByID(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	for _, ruleInput := range input.Rules {
		// Map DTO to domain
		rule := domain.LifecycleRule{
			ID:                     ruleInput.ID,
			Prefix:                 ruleInput.Prefix,
			Status:                 ruleInput.Status,
			ExpirationDays:         ruleInput.ExpirationDays,
			TransitionDays:         ruleInput.TransitionDays,
			TransitionStorageClass: ruleInput.TransitionStorageClass,
		}

		// Marshal rule to JSON for storage
		ruleJSON, err := json.Marshal(rule)
		if err != nil {
			return fmt.Errorf("failed to marshal rule: %w", err)
		}

		if err := s.repo.UpsertLifecycleRule(ctx, bucketID, ruleJSON); err != nil {
			return fmt.Errorf("failed to save lifecycle rule: %w", err)
		}
	}
	return nil
}

func (s *BucketService) GetBucketLifecycle(ctx context.Context, bucketID string) ([]domain.LifecycleRule, error) {
	rules, err := s.repo.GetLifecycleRules(ctx, bucketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lifecycle rules: %w", err)
	}

	return rules, nil
}

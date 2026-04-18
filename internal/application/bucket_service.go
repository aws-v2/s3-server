package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"s3/internal/infrastructure/metrics"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BucketService provides business logic for managing buckets.
type BucketService struct {
	bucketRepo domain.BucketRepository
	fileRepo   domain.FileRepository
	policyRepo domain.PolicyRepository
	storage    domain.StoragePort
	metrics    *metrics.MetricsClient
}

type BucketAlreadyExists struct {
	Name string
}

func (e *BucketAlreadyExists) Error() string {
	return fmt.Sprintf("bucket %s already exists", e.Name)
}

// NewBucketService creates a new instance of BucketService.
func NewBucketService(
	repo domain.BucketRepository,
	fileRepo domain.FileRepository,
	policyRepo domain.PolicyRepository,
	storage domain.StoragePort,
	metrics *metrics.MetricsClient,
) *BucketService {
	return &BucketService{
		bucketRepo: repo,
		fileRepo:   fileRepo,
		policyRepo: policyRepo,
		storage:    storage,
		metrics:    metrics,
	}
}

func (s *BucketService) CreateBucket(ctx context.Context, input dto.CreateBucketInput) (*dto.CreateBucketOutput, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	// Check if logical name already exists for this user
	existing, err := s.bucketRepo.GetBucketByName(ctx, input.Name, input.OwnerId)
	if err == nil && existing.ID != "" {
		return nil, &BucketAlreadyExists{Name: input.Name}
	}

	// Generate ID and Storage name (Physical name in MinIO)
	bucketId := uuid.New().String()
	storageName := fmt.Sprintf("bucket-%s", strings.ReplaceAll(bucketId, "-", ""))

	// Try to create physical bucket in storage (MinIO)
	_, err = s.storage.CreateBucket(ctx, storageName)
	if err != nil {
		return nil, fmt.Errorf("failed to create physical bucket: %w", err)
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

log.Printf("DEBUG: OwnerId=%q, BucketName=%q", input.OwnerId, input.Name)


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
		ObjectLock:  input.ObjectLock,
		StorageName: storageName,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	// Save metadata in repository
	bucketObject, err := s.bucketRepo.SaveBucket(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to save bucket metadata: %w", err)
	}

	// Emit metrics for bucket creation (Tier 1)
	go s.emitMetrics(context.Background(), bucketObject.ID, bucketObject.OwnerID, bucketObject.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	// Return success
	return &dto.CreateBucketOutput{
		BucketID:  bucketObject.ID,
		Name:      input.Name,
		CreatedAt: bucket.CreatedAt,
	}, nil
}

func (s *BucketService) emitMetrics(ctx context.Context, bucketID, ownerID, region string, partial dto.S3IngestRequest) {
	if s.metrics == nil {
		return
	}

	// Enrich with basic info
	partial.BucketID = bucketID
	partial.OwnerID = ownerID
	partial.Region = region

	// Attempt to send
	_ = s.metrics.SendS3Metrics(ctx, partial)
}

func (s *BucketService) resolveBucket(ctx context.Context, idOrName string, filterID string) (domain.Bucket, error) {
	// Try by ID first
	bucket, err := s.bucketRepo.GetBucketByID(ctx, idOrName, filterID)
	if err == nil {
		return bucket, nil
	}

	// Try by Name as fallback
	bucket, err = s.bucketRepo.GetBucketByName(ctx, idOrName, filterID)
	if err == nil {
		return bucket, nil
	}

	return domain.Bucket{}, fmt.Errorf("bucket not found: %s", idOrName)
}

func (s *BucketService) GetBucketByName(ctx context.Context, name string) (*domain.Bucket, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.bucketRepo.GetBucketByName(ctx, name, filterID)
	if err != nil {
		return nil, err
	}

	return &bucket, nil
}

func (s *BucketService) GetBucket(ctx context.Context, bucketID string) (*dto.GetBucketOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	// Emit metrics for bucket detail (Tier 2 - Head/Get)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		HeadRequests: 1,
	})

	return &dto.GetBucketOutput{
		BucketID:   bucket.ID,
		Name:       bucket.Name,
		ARN:        bucket.ARN,
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
	buckets, err := s.bucketRepo.ListBuckets(ctx, filterID)
	if err == nil {
		// Emit metrics for List (Tier 2) - Note: This is an account-level list, but we can log it
		// For simplicity, we'll skip per-bucket metrics here unless a specific bucket was requested
	}
	return buckets, err
}

func (s *BucketService) UpdateBucket(ctx context.Context, bucketID string, input dto.UpdateBucketInput) (*dto.GetBucketOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}
	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	if input.Name != "" {
		bucket.Name = input.Name
		bucket.UpdatedAt = time.Now()
	}
	// Note: We don't rename the physical bucket in MinIO because StorageName is immutable/UUID.
	// We only update the logical Name in the database.
	// If you want to rename in storage: error := s.storage.RenameBucket(ctx, bucket.StorageName, ...)
	// But it's better to keep physical names stable.

	updated, err := s.bucketRepo.UpdateBucket(ctx, &bucket, filterID)
	if err != nil {
		return nil, fmt.Errorf("failed to update bucket: %w", err)
	}

	// Emit metrics for update (Tier 1)
	go s.emitMetrics(context.Background(), updated.ID, updated.OwnerID, updated.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

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

	files, err := s.fileRepo.ListFiles(ctx, bucketID)
	if err != nil {
		return fmt.Errorf("failed to check files: %w", err)
	}

	if len(files) > 0 {
		return fmt.Errorf("cannot delete bucket with files")
	}
	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	if err := s.storage.DeleteBucket(ctx, bucket.StorageName); err != nil {

		return fmt.Errorf("failed to delete from storage: %w", err)
	}
	if err := s.bucketRepo.DeleteBucket(ctx, bucketID); err != nil {

		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	// Emit metrics for delete (Tier 1/Free)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		DeleteRequests: 1,
	})

	return nil
}

func (s *BucketService) EmptyBucket(ctx context.Context, bucketID string) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	// 1. Delete all objects from storage
	if err := s.storage.EmptyBucket(ctx, bucket.StorageName); err != nil {
		return fmt.Errorf("failed to empty storage: %w", err)
	}

	// 2. Delete all file metadata from repository
	if err := s.fileRepo.DeleteFilesByBucket(ctx, bucketID); err != nil {
		return fmt.Errorf("failed to delete file metadata: %w", err)
	}

	// Emit metrics for empty/delete (Tier 1/Free)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		DeleteRequests: 1,
	})

	return nil
}

func (s *BucketService) GetBucketStats(ctx context.Context, bucketID string) (*dto.BucketStatsOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	files, err := s.fileRepo.ListFiles(ctx, bucket.ID)
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

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
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

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}
	// Permission: only owner or admin can update
	if actorID != bucket.OwnerID && !IsAdmin(actorID) {
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
	if err := s.policyRepo.IncrementPolicyVersionAndUpdateBucket(ctx, &bucket); err != nil {
		return fmt.Errorf("failed to save policy: %w", err)
	}

	// audit / history - optional
	_ = s.policyRepo.AppendPolicyHistory(ctx, bucketID, &policy, actorID)

	return nil
}

func (s *BucketService) SetBucketVersioning(ctx context.Context, bucketID string, enabled bool) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	// Set versioning in storage layer (MinIO)
	if err := s.storage.SetBucketVersioning(ctx, bucket.StorageName, enabled); err != nil {
		return fmt.Errorf("failed to set versioning in storage: %w", err)
	}

	// Persist versioning status to database
	status := domain.VersioningSuspended
	if enabled {
		status = domain.VersioningEnabled
	}
	if err := s.bucketRepo.SetBucketVersioning(ctx, bucketID, status); err != nil {
		return fmt.Errorf("failed to persist versioning status: %w", err)
	}

	// Emit metrics for versioning (Tier 1)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return nil
}

// isAdmin checks whether the actor (like "user:abc123") is an admin.
// In MVP mode, we load admin IDs from an env var: ADMIN_USERS=user:abc123,user:def456
func IsAdmin(actorID string) bool {
	admins := os.Getenv("ADMIN_USERS")

	// fallback for tests only
	if strings.TrimSpace(admins) == "" {
		admins = "550e8400-e29b-41d4-a716-446655440000" // Dummy admin ID
	}

	for _, a := range strings.Split(admins, ",") {
		adminID := strings.TrimSpace(a)
		// Handle both "user:UUID" and "UUID" formats in the environment variable
		adminID = strings.TrimPrefix(adminID, "user:")

		if adminID == actorID {
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
	_, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	// Get versioning status from database
	status, err := s.bucketRepo.GetBucketVersioning(ctx, bucketID)
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
	_, err := s.resolveBucket(ctx, bucketID, filterID)
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

		if err := s.bucketRepo.UpsertLifecycleRule(ctx, bucketID, ruleJSON); err != nil {
			return fmt.Errorf("failed to save lifecycle rule: %w", err)
		}
	}

	// Emit metrics for lifecycle (Tier 1)
	// We need bucket info for owner/region
	if bucket, err := s.resolveBucket(ctx, bucketID, filterID); err == nil {
		go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
			PutRequests: 1,
		})
	}

	return nil
}

func (s *BucketService) GetBucketLifecycle(ctx context.Context, bucketID string) ([]domain.LifecycleRule, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Verify bucket ownership and resolve name
	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	rules, err := s.bucketRepo.GetLifecycleRules(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lifecycle rules: %w", err)
	}

	return rules, nil
}

func (s *BucketService) SetBucketBlockPublicAccess(ctx context.Context, bucketID string, input dto.SetBlockPublicAccessInput) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	config := domain.BlockPublicAccess{
		BlockPublicAcls:       input.BlockPublicAcls,
		IgnorePublicAcls:      input.IgnorePublicAcls,
		BlockPublicPolicy:     input.BlockPublicPolicy,
		RestrictPublicBuckets: input.RestrictPublicBuckets,
	}

	if input.BlockAll != nil {
		val := *input.BlockAll
		config.BlockPublicAcls = val
		config.IgnorePublicAcls = val
		config.BlockPublicPolicy = val
		config.RestrictPublicBuckets = val
	}

	if err := s.bucketRepo.SetBucketBlockPublicAccess(ctx, bucket.ID, config); err != nil {
		return fmt.Errorf("failed to update block public access: %w", err)
	}

	// Emit metrics (Tier 1)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return nil
}

func (s *BucketService) GetBucketCORS(ctx context.Context, bucketID string) (*dto.CORSConfiguration, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Verify bucket ownership
	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	cors, err := s.bucketRepo.GetBucketCORS(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cors configuration: %w", err)
	}

	if cors == nil {
		return &dto.CORSConfiguration{CORSRules: []dto.CORSRule{}}, nil
	}

	output := dto.CORSConfiguration{
		CORSRules: make([]dto.CORSRule, 0, len(cors.CORSRules)),
	}
	for _, r := range cors.CORSRules {
		output.CORSRules = append(output.CORSRules, dto.CORSRule{
			AllowedHeaders: r.AllowedHeaders,
			AllowedMethods: r.AllowedMethods,
			AllowedOrigins: r.AllowedOrigins,
			ExposeHeaders:  r.ExposeHeaders,
			MaxAgeSeconds:  r.MaxAgeSeconds,
			Test:           r.Test,
		})
	}

	return &output, nil
}

func (s *BucketService) SetBucketCORS(ctx context.Context, bucketID string, input dto.CORSConfiguration) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	cors := &domain.CORSConfiguration{
		CORSRules: make([]domain.CORSRule, 0, len(input.CORSRules)),
	}
	for _, r := range input.CORSRules {
		cors.CORSRules = append(cors.CORSRules, domain.CORSRule{
			AllowedHeaders: r.AllowedHeaders,
			AllowedMethods: r.AllowedMethods,
			AllowedOrigins: r.AllowedOrigins,
			ExposeHeaders:  r.ExposeHeaders,
			MaxAgeSeconds:  r.MaxAgeSeconds,
			Test:           r.Test,
		})
	}

	if err := s.bucketRepo.SetBucketCORS(ctx, bucket.ID, cors); err != nil {
		return fmt.Errorf("failed to save cors configuration: %w", err)
	}

	// Emit metrics (Tier 1)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return nil
}

func (s *BucketService) SetBucketEncryption(ctx context.Context, bucketID string, input dto.BucketEncryption) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	encryption := domain.BucketEncryption{
		Type:             input.Type,
		BucketKeyEnabled: input.BucketKeyEnabled,
	}

	if err := s.bucketRepo.SetBucketEncryption(ctx, bucket.ID, encryption); err != nil {
		return fmt.Errorf("failed to save encryption configuration: %w", err)
	}

	return nil
}

func (s *BucketService) GetBucketEncryption(ctx context.Context, bucketID string) (*dto.BucketEncryption, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	encryption, err := s.bucketRepo.GetBucketEncryption(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption configuration: %w", err)
	}

	return &dto.BucketEncryption{
		Type:             encryption.Type,
		BucketKeyEnabled: encryption.BucketKeyEnabled,
	}, nil
}

func (s *BucketService) GetBucketReplication(ctx context.Context, bucketID string) (*dto.ReplicationOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	repl, err := s.bucketRepo.GetBucketReplication(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get replication configuration: %w", err)
	}

	return &dto.ReplicationOutput{Replication: repl}, nil
}

func (s *BucketService) GetBucketTags(ctx context.Context, bucketID string) (*dto.TagsOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	tags, err := s.bucketRepo.GetBucketTags(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket tags: %w", err)
	}

	dtoTags := make([]dto.Tag, len(tags))
	for i, t := range tags {
		dtoTags[i] = dto.Tag{Key: t.Key, Value: t.Value}
	}

	return &dto.TagsOutput{Tags: dtoTags}, nil
}

func (s *BucketService) GetBucketNotifications(ctx context.Context, bucketID string) (*dto.NotificationsOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	notif, err := s.bucketRepo.GetBucketNotifications(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification configuration: %w", err)
	}

	return &dto.NotificationsOutput{Notifications: notif}, nil
}
func (s *BucketService) SetBucketObjectLock(ctx context.Context, bucketID string, input dto.ObjectLockInput) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	enabled := input.Status == "Enabled"
	if err := s.bucketRepo.SetBucketObjectLock(ctx, bucket.ID, enabled); err != nil {
		return fmt.Errorf("failed to save object lock configuration: %w", err)
	}

	return nil
}

func (s *BucketService) GetBucketObjectLock(ctx context.Context, bucketID string) (*dto.ObjectLockOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	enabled, err := s.bucketRepo.GetBucketObjectLock(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get object lock configuration: %w", err)
	}

	return &dto.ObjectLockOutput{Enabled: enabled}, nil
}

func (s *BucketService) SetBucketReplication(ctx context.Context, bucketID string, input dto.UpdateReplicationInput) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	if err := s.bucketRepo.SetBucketReplication(ctx, bucket.ID, input); err != nil {
		return fmt.Errorf("failed to save replication configuration: %w", err)
	}

	return nil
}

func (s *BucketService) SetBucketLogging(ctx context.Context, bucketID string, input dto.UpdateLoggingInput) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	if err := s.bucketRepo.SetBucketLogging(ctx, bucket.ID, input); err != nil {
		return fmt.Errorf("failed to save logging configuration: %w", err)
	}

	return nil
}

func (s *BucketService) GetBucketLogging(ctx context.Context, bucketID string) (*dto.LoggingOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	logging, err := s.bucketRepo.GetBucketLogging(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get logging configuration: %w", err)
	}

	if logging == nil {
		return &dto.LoggingOutput{Status: "Disabled"}, nil
	}

	m, ok := logging.(map[string]interface{})
	if !ok {
		return &dto.LoggingOutput{Status: "Disabled"}, nil
	}

	output := &dto.LoggingOutput{
		Status:       "Disabled",
		TargetBucket: "",
		TargetPrefix: "",
	}

	if status, ok := m["status"].(string); ok {
		output.Status = status
	}
	if targetBucket, ok := m["targetBucket"].(string); ok {
		output.TargetBucket = targetBucket
	}
	if targetPrefix, ok := m["targetPrefix"].(string); ok {
		output.TargetPrefix = targetPrefix
	}

	return output, nil
}

func (s *BucketService) SetBucketNotifications(ctx context.Context, bucketID string, input dto.UpdateNotificationsInput) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	if err := s.bucketRepo.SetBucketNotifications(ctx, bucket.ID, input); err != nil {
		return fmt.Errorf("failed to save notifications configuration: %w", err)
	}

	return nil
}

func (s *BucketService) SetBucketTags(ctx context.Context, bucketID string, input dto.UpdateTagsInput) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	var domainTags []domain.Tag
	for _, t := range input.Tags {
		domainTags = append(domainTags, domain.Tag{
			Key:   t.Key,
			Value: t.Value,
		})
	}

	if err := s.bucketRepo.SetBucketTags(ctx, bucket.ID, domainTags); err != nil {
		return fmt.Errorf("failed to save tags configuration: %w", err)
	}

	return nil
}

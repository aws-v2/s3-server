package application

import (
	"context"
	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
)

type SecurityService struct {
	bucketRepo domain.BucketRepository
}

func NewSecurityService(bucketRepo domain.BucketRepository) *SecurityService {
	return &SecurityService{
		bucketRepo: bucketRepo,
	}
}

func (s *SecurityService) AnalyzePosture(ctx context.Context, userID string) (*dto.SecuritySummaryOutput, error) {
	buckets, err := s.bucketRepo.ListBucketsByOwner(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets for security audit: %w", err)
	}


	report := &dto.SecuritySummaryOutput{
		Score:    100,
		Findings: []dto.SecurityFinding{},
	}

	for _, b := range buckets {
		// 1. Public Access Audit
		isPublic := false
		if !b.BlockPublicAccess.BlockPublicPolicy || !b.BlockPublicAccess.BlockPublicAcls {
			isPublic = true
			report.PublicBucketsCount++
			report.Findings = append(report.Findings, dto.SecurityFinding{
				ID:          "S3.1",
				Severity:    "high",
				BucketName:  b.Name,
				Description: "Public access is not fully blocked. This bucket might be accessible to anyone on the internet.",
				Remediation: "Enable all 'Block Public Access' settings in the bucket permissions tab.",
			})
		}

		// 2. Encryption Audit
		if b.Encryption.Type == "" || b.Encryption.Type == "None" {
			report.UnencryptedCount++
			report.Findings = append(report.Findings, dto.SecurityFinding{
				ID:          "S3.2",
				Severity:    "high",
				BucketName:  b.Name,
				Description: "Default encryption is disabled. Objects uploaded without encryption headers will be stored in plaintext.",
				Remediation: "Enable AES-256 or AWS-KMS default encryption under Bucket Settings.",
			})
		}

		// 3. Object Lock Audit (Optional but recommended for compliance)
		if !b.ObjectLock {
			report.Findings = append(report.Findings, dto.SecurityFinding{
				ID:          "S3.3",
				Severity:    "medium",
				BucketName:  b.Name,
				Description: "Object Lock is disabled. Data can be deleted or overwritten by anyone with delete permissions.",
				Remediation: "Enable Object Lock if this bucket stores sensitive or compliance-related data.",
			})
		}

		// 4. MFA Delete Check
		// In this simplified model, we'll flag any bucket without versioning as missing MFA protection
		// (actually MFA Delete requires versioning).
		if b.VersioningStatus != "Enabled" {
			report.MfaMissingCount++
			report.Findings = append(report.Findings, dto.SecurityFinding{
				ID:          "S3.4",
				Severity:    "low",
				BucketName:  b.Name,
				Description: "Versioning is disabled, preventing MFA Delete protection and recovery from accidental deletions.",
				Remediation: "Enable bucket versioning to allow for recovery and enhanced security.",
			})
		}

		// 5. Policy permissiveness (Basic heuristic)
		if b.Policy != nil && isPublic {
			// If it's public AND has a policy, it's a critical risk
			report.Findings = append(report.Findings, dto.SecurityFinding{
				ID:          "S3.5",
				Severity:    "critical",
				BucketName:  b.Name,
				Description: "Critical: Bucket has public access enabled AND a bucket policy attached.",
				Remediation: "Restrict the bucket policy to specific IAM roles or VPCs and block public access.",
			})
		}
	}

	// Phase 3 logic: Calculate score based on findings
	s.calculateScore(report)

	return report, nil
}

func (s *SecurityService) calculateScore(report *dto.SecuritySummaryOutput) {
	penalty := 0
	for _, f := range report.Findings {
		switch f.Severity {
		case "critical":
			penalty += 30
		case "high":
			penalty += 15
		case "medium":
			penalty += 5
		case "low":
			penalty += 2
		}
	}

	report.Score = 100 - penalty
	if report.Score < 0 {
		report.Score = 0
	}
}

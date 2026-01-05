package application

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"strings"
	"sync"
	"time"
)

type AnalyticsService struct {
	repo domain.RepositoryPort

	statsMu       sync.RWMutex
	totalRequests int
	byEndpoint    map[string]int
	byStatus      map[string]int
}

func NewAnalyticsService(repo domain.RepositoryPort) *AnalyticsService {
	return &AnalyticsService{
		repo:       repo,
		byEndpoint: make(map[string]int),
		byStatus:   make(map[string]int),
	}
}

func (s *AnalyticsService) GetStorageUsage(ctx context.Context) (*dto.GetStorageUsageOutput, error) {
	buckets, err := s.repo.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}

	var totalSize int64
	var totalFiles int
	bucketUsages := []dto.BucketUsageInfo{}

	for _, bucket := range buckets {
		files, _ := s.repo.ListFiles(ctx, bucket.ID)

		var bucketSize int64
		for _, file := range files {
			bucketSize += file.Size
		}

		totalSize += bucketSize
		totalFiles += len(files)

		bucketUsages = append(bucketUsages, dto.BucketUsageInfo{
			BucketID:      bucket.ID,
			BucketName:    bucket.Name,
			Size:          bucketSize,
			SizeFormatted: formatBytes(bucketSize),
			FileCount:     len(files),
		})
	}

	return &dto.GetStorageUsageOutput{
		TotalSize:          totalSize,
		TotalSizeFormatted: formatBytes(totalSize),
		TotalFiles:         totalFiles,
		BucketCount:        len(buckets),
		Buckets:            bucketUsages,
	}, nil
}

func (s *AnalyticsService) GetTrafficStats(ctx context.Context, input dto.GetTrafficStatsInput) (*dto.GetTrafficStatsOutput, error) {
	if input.StartDate.IsZero() {
		input.StartDate = time.Now().AddDate(0, 0, -30)
	}
	if input.EndDate.IsZero() {
		input.EndDate = time.Now()
	}

	logs, err := s.repo.GetAccessLogsByDateRange(ctx, input.StartDate, input.EndDate)
	if err != nil {
		return nil, err
	}

	var totalUploads, totalDownloads int64
	var uploadSize, downloadSize int64
	dailyMap := make(map[string]*dto.DailyTraffic)

	for _, log := range logs {
		dateStr := log.Timestamp.Format("2006-01-02")

		if _, exists := dailyMap[dateStr]; !exists {
			dailyMap[dateStr] = &dto.DailyTraffic{Date: dateStr}
		}

		switch log.Action {
		case "upload":
			totalUploads++
			uploadSize += log.Size
			dailyMap[dateStr].Uploads++
			dailyMap[dateStr].UploadSize += log.Size
		case "download":
			totalDownloads++
			downloadSize += log.Size
			dailyMap[dateStr].Downloads++
			dailyMap[dateStr].DownloadSize += log.Size
		}
	}

	daily := []dto.DailyTraffic{}
	for _, traffic := range dailyMap {
		daily = append(daily, *traffic)
	}

	return &dto.GetTrafficStatsOutput{
		Period:         fmt.Sprintf("%s to %s", input.StartDate.Format("2006-01-02"), input.EndDate.Format("2006-01-02")),
		TotalUploads:   totalUploads,
		TotalDownloads: totalDownloads,
		UploadSize:     uploadSize,
		DownloadSize:   downloadSize,
		Daily:          daily,
	}, nil
}

func (s *AnalyticsService) GetFileTypeDistribution(ctx context.Context) (*dto.GetFileTypeDistributionOutput, error) {
	buckets, _ := s.repo.ListBuckets(ctx)

	typeMap := make(map[string]*dto.FileTypeInfo)
	totalFiles := 0

	for _, bucket := range buckets {
		files, _ := s.repo.ListFiles(ctx, bucket.ID)

		for _, file := range files {
			contentType := file.ContentType
			if contentType == "" {
				contentType = "unknown"
			}

			if _, exists := typeMap[contentType]; !exists {
				typeMap[contentType] = &dto.FileTypeInfo{Type: contentType}
			}

			typeMap[contentType].Count++
			typeMap[contentType].TotalSize += file.Size
			totalFiles++
		}
	}

	types := []dto.FileTypeInfo{}
	for _, info := range typeMap {
		info.Percentage = float64(info.Count) / float64(totalFiles) * 100
		types = append(types, *info)
	}

	return &dto.GetFileTypeDistributionOutput{
		Types: types,
		Total: totalFiles,
	}, nil
}

func (s *AnalyticsService) GetBucketUsageOverTime(ctx context.Context, bucketID string, input dto.GetBucketUsageOverTimeInput) (*dto.GetBucketUsageOverTimeOutput, error) {
	days := input.Days
	if days == 0 {
		days = 30
	}

	files, _ := s.repo.ListFiles(ctx, bucketID)

	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}

	usage := []dto.UsageDataPoint{
		{
			Date:      time.Now().Format("2006-01-02"),
			Size:      totalSize,
			FileCount: len(files),
		},
	}

	return &dto.GetBucketUsageOverTimeOutput{
		BucketID: bucketID,
		Usage:    usage,
	}, nil
}

func (s *AnalyticsService) GetPopularFiles(ctx context.Context, input dto.GetPopularFilesInput) (*dto.GetPopularFilesOutput, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 10
	}

	popularFiles, err := s.repo.GetPopularFiles(ctx, limit)
	if err != nil {
		return nil, err
	}

	files := []dto.PopularFileInfo{}
	for _, pf := range popularFiles {
		files = append(files, dto.PopularFileInfo{
			FileID:      pf.FileID,
			Key:         pf.Key,
			AccessCount: pf.AccessCount,
			TotalSize:   pf.TotalSize,
		})
	}

	return &dto.GetPopularFilesOutput{Files: files}, nil
}

func (s *AnalyticsService) GetUserActivity(ctx context.Context, userID string) (*dto.GetUserActivityOutput, error) {
	logs, err := s.repo.GetAccessLogsByUser(ctx, userID, 100)
	if err != nil {
		return nil, err
	}

	var uploads, downloads, deletes int
	recent := []dto.UserActionInfo{}

	for _, log := range logs {
		switch log.Action {
		case "upload":
			uploads++
		case "download":
			downloads++
		case "delete":
			deletes++
		}

		if len(recent) < 20 {
			file, _ := s.repo.GetFileByID(ctx, log.FileID)
			key := ""
			if file != nil {
				key = file.Key
			}

			recent = append(recent, dto.UserActionInfo{
				Action:    log.Action,
				FileKey:   key,
				Timestamp: log.Timestamp,
			})
		}
	}

	return &dto.GetUserActivityOutput{
		UserID:        userID,
		TotalActions:  len(logs),
		Uploads:       uploads,
		Downloads:     downloads,
		Deletes:       deletes,
		RecentActions: recent,
	}, nil
}

func (s *AnalyticsService) ExportAnalytics(ctx context.Context, input dto.ExportAnalyticsInput) ([]byte, string, error) {
	stats, _ := s.GetStorageUsage(ctx)

	switch strings.ToLower(input.Format) {
	case "json":
		data, err := json.MarshalIndent(stats, "", "  ")
		return data, "application/json", err

	case "csv":
		var buf strings.Builder
		writer := csv.NewWriter(&buf)

		writer.Write([]string{"Bucket ID", "Bucket Name", "Size (bytes)", "File Count"})
		for _, bucket := range stats.Buckets {
			writer.Write([]string{
				bucket.BucketID,
				bucket.BucketName,
				fmt.Sprintf("%d", bucket.Size),
				fmt.Sprintf("%d", bucket.FileCount),
			})
		}
		writer.Flush()

		return []byte(buf.String()), "text/csv", nil

	default:
		return nil, "", fmt.Errorf("unsupported format: %s", input.Format)
	}
}

func (s *AnalyticsService) TrackRequest(method, path, status string) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	s.totalRequests++
	s.byEndpoint[method+" "+path]++
	s.byStatus[status]++
}

func (s *AnalyticsService) GetAPIUsage(ctx context.Context) (*dto.GetAPIUsageOutput, error) {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	// Copy maps to avoid race conditions during JSON marshaling later or modification
	endpoints := make(map[string]int)
	for k, v := range s.byEndpoint {
		endpoints[k] = v
	}

	statuses := make(map[string]int)
	for k, v := range s.byStatus {
		statuses[k] = v
	}

	return &dto.GetAPIUsageOutput{
		TotalRequests: s.totalRequests,
		ByEndpoint:    endpoints,
		ByStatus:      statuses,
	}, nil
}

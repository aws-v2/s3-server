package application

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"path"
	"strings"
	"time"

	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"s3/internal/infrastructure/metrics"
	"s3/internal/infrastructure/utils"

	"crypto/sha256"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UploadService struct {
	storage    domain.StoragePort
	bucketRepo domain.BucketRepository
	fileRepo   domain.FileRepository
	metrics    *metrics.MetricsClient
	events     domain.EventPublisher
	presign    *PresignService
}

func NewUploadService(
	storage domain.StoragePort,
	bucketRepo domain.BucketRepository,
	fileRepo domain.FileRepository,
	metrics *metrics.MetricsClient,
	events domain.EventPublisher,
	presign *PresignService,
) *UploadService {
	return &UploadService{
		storage:    storage,
		bucketRepo: bucketRepo,
		fileRepo:   fileRepo,
		metrics:    metrics,
		events:     events,
		presign:    presign,
	}
}

// UploadObjectReader uploads an object from a reader and emits a stored event if it's a game file.
func (s *UploadService) UploadObjectReader(ctx context.Context, bucketID, key string, reader io.Reader, size int64, contentType string, metadata map[string]string) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Resolve bucket
	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	// Calculate SHA256 while saving
	hash := sha256.New()
	teeReader := io.TeeReader(reader, hash)

	// Save object to storage
	err = s.storage.SaveObjectReader(ctx, bucket.StorageName, key, teeReader, size, metadata)
	if err != nil {
		return fmt.Errorf("failed to save object %s: %w", key, err)
	}

	sha256Value := hex.EncodeToString(hash.Sum(nil))

	// Save file metadata in DB
	file := domain.File{
		ID:        generateID(),
		BucketID:  bucket.ID,
		Key:       key,
		Size:      size,
		MimeType:  contentType,
		Metadata:  metadata,
		SHA256:    sha256Value,
		CreatedAt: time.Now(),
	}

	err = s.fileRepo.SaveFile(ctx, file)
	if err != nil {
		return fmt.Errorf("failed** to save file metadata for %s: %w", key, err)
	}

	// Emit metrics
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests:   1,
		BytesUploaded: size,
	})

	log.Printf("[S3] Checking if key %s is a game file...", key)

	// Normalize key (handle leading slash)
	cleanKey := strings.TrimPrefix(key, "/")

	// Pattern: uploads/games/{game_id}/game.zip (or any extension)
	isGameFile := strings.HasPrefix(cleanKey, "uploads/games/") &&
		(strings.HasSuffix(cleanKey, ".mp4") || strings.HasSuffix(cleanKey, ".zip"))

	if isGameFile || bucket.Name == "gamelift_games" {
		log.Printf("[S3] Recognized game-related upload in bucket %s, key %s", bucket.Name, cleanKey)
		parts := strings.Split(cleanKey, "/")
		if len(parts) >= 3 {
			gameIDStr := parts[2]
			var gameID int
			_, err := fmt.Sscanf(gameIDStr, "%d", &gameID)

			if err == nil && gameID > 0 {
				// Generate internal download URL for the backend/ec2 (valid for 1 hour)
				downloadURL, err := s.presign.GenerateSignedURL(uuid.New().String(), bucket.Name, cleanKey, "asset-id",
					"",
					"sha256", "GET", time.Now().Add(1*time.Hour), &file.ID)
				if err != nil {
					log.Printf("[S3] PRESIGN_DOWNLOAD_URL_GENERATION_FAILED",
						"error", err,
					)
					return fmt.Errorf("PRESIGN_DOWNLOAD_URL_GENERATION_FAILED someoneshould look at this  %w", err)

				}

				event := map[string]interface{}{
					"game_id":      gameID,
					"s3_arn":       fmt.Sprintf("arn:aws:s3:::%s/%s", bucket.StorageName, cleanKey),
					"download_url": downloadURL,
					"status":       "success",
				}
				subj := "dev.s3.v1.game.stored"
				log.Printf("[S3] Publishing completion event for Game %d to %s with internal link", gameID, subj)
				if err := s.events.PublishRaw(ctx, subj, event); err != nil {
					log.Printf("[S3] ERROR: Failed to publish NATS event: %v", err)
				} else {
					log.Printf("[S3] Successfully notified backend. Internal Link: %s", downloadURL)
				}
			}
		}
	}

	return nil
}

type UploadFileInput struct {
	Sha256              string              `json:"sha256"`
	BucketID            string              `json:"bucketId"`
	Prefix              string              `json:"prefix"`
	Files               []FileContent       `json:"files"`
	DestinationSettings DestinationSettings `json:"destinationSettings"`
	Properties          UploadProperties    `json:"properties"`
	Tags                []Tag               `json:"tags"`
	Metadata            []MetadataItem      `json:"metadata"`
}

type FileContent struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Type string `json:"type"`
	Data []byte `json:"-"`
}

type DestinationSettings struct {
	VersioningEnabled bool `json:"versioningEnabled"`
}

type UploadProperties struct {
	StorageClass     string `json:"storageClass"`
	EncryptionType   string `json:"encryptionType"`
	ChecksumFunction string `json:"checksumFunction"`
}

type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MetadataItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type UploadFileOutput struct {
	FileIDs   []string
	Result    string
	CreatedAt time.Time
	SHA256    string `json:"sha256"`
}

func (s *UploadService) resolveBucket(ctx context.Context, idOrName string, filterID string) (domain.Bucket, error) {
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

func (s *UploadService) UploadFile(
	ctx context.Context,
	input UploadFileInput,
) (*UploadFileOutput, error) {

	start := time.Now()

	userID, _ := ctx.Value("userId").(string)
	requestID, _ := ctx.Value("requestId").(string)

	filterID := userID
	role, _ := ctx.Value("role").(string)

	if role == "admin" {
		filterID = ""
	}

	log.Printf(
		"[UploadService] started request_id=%s user_id=%s bucket_id=%s files=%d",
		requestID,
		userID,
		input.BucketID,
		len(input.Files),
	)

	// Resolve bucket by ID or Name
	bucket, err := s.resolveBucket(ctx, input.BucketID, filterID)
	if err != nil {
		log.Printf(
			"[UploadService] bucket resolution failed request_id=%s bucket_id=%s error=%v",
			requestID,
			input.BucketID,
			err,
		)

		return nil, err
	}

	log.Printf(
		"[UploadService] bucket resolved request_id=%s storage_name=%s owner_id=%s",
		requestID,
		bucket.StorageName,
		bucket.OwnerID,
	)

	var fileIDs []string
	var totalBytes int64

	// Convert metadata slice to map
	metaMap := make(map[string]string)
	for _, m := range input.Metadata {
		metaMap[m.Key] = m.Value
	}

	var sha256Value string
	for _, f := range input.Files {
		fileStart := time.Now()

		totalBytes += f.Size

		log.Printf(
			"[UploadService] processing file request_id=%s filename=%s size=%d",
			requestID,
			f.Name,
			f.Size,
		)

		fileData := f.Data
		if len(fileData) == 0 {
			log.Printf(
				"[UploadService] using placeholder data request_id=%s filename=%s",
				requestID,
				f.Name,
			)

			fileData = []byte("placeholder data for " + f.Name)
		}

		objectKey := f.Name
		if input.Prefix != "" {
			objectKey = fmt.Sprintf(
				"%s/%s",
				strings.TrimSuffix(input.Prefix, "/"),
				f.Name,
			)
		}

		log.Printf(
			"[UploadService] saving object request_id=%s object_key=%s bucketname=%s ownerID=%s",
			requestID,
			objectKey,
			bucket.Name,
			bucket.OwnerID,
		)

		err = s.storage.SaveObject(
			ctx,
			bucket.StorageName,
			objectKey,
			fileData,
			metaMap,
		)

		if err != nil {
			log.Printf(
				"[UploadService] object save failed request_id=%s object_key=%s error=%v",
				requestID,
				objectKey,
				err,
			)

			return nil, fmt.Errorf(
				"failed to save object %s: %w",
				objectKey,
				err,
			)
		}

		log.Printf(
			"[UploadService] object saved request_id=%s object_key=%s",
			requestID,
			objectKey,
		)

		if input.Sha256 == "" {
			sha256Value = utils.CalculateSHA256Bytes(fileData)

		} else {
			sha256Value = input.Sha256

		}

		file := domain.File{
			ID:        generateID(),
			BucketID:  bucket.ID,
			Key:       objectKey,
			Size:      f.Size,
			MimeType:  f.Type,
			Metadata:  metaMap,
			SHA256:    sha256Value,
			CreatedAt: time.Now(),
		}

		log.Printf(
			"[UploadService] saving metadata request_id=%s file_id=%s object_key=%s",
			requestID,
			file.ID,
			objectKey,
		)

		err = s.fileRepo.SaveFile(ctx, file)
		if err != nil {
			log.Printf(
				"[UploadService] metadata save failed request_id=%s file_id=%s error=%v",
				requestID,
				file.ID,
				err,
			)

			return nil, fmt.Errorf(
				"failed to save file metadata for %s: %w",
				f.Name,
				err,
			)
		}

		log.Printf(
			"[UploadService] file completed request_id=%s file_id=%s duration=%s",
			requestID,
			file.ID,
			time.Since(fileStart),
		)

		fileIDs = append(fileIDs, file.ID)
	}

	log.Printf(
		"[UploadService] emitting metrics request_id=%s total_bytes=%d",
		requestID,
		totalBytes,
	)

	go s.emitMetrics(
		context.Background(),
		bucket.ID,
		bucket.OwnerID,
		bucket.Region,
		dto.S3IngestRequest{
			PutRequests:   int64(len(input.Files)),
			BytesUploaded: totalBytes,
		},
	)

	log.Printf(
		"[UploadService] completed request_id=%s uploaded_files=%d total_bytes=%d duration=%s",
		requestID,
		len(fileIDs),
		totalBytes,
		time.Since(start),
	)

	return &UploadFileOutput{
		FileIDs: fileIDs,
		SHA256:  sha256Value,
		Result: fmt.Sprintf(
			"Successfully processed %d files",
			len(input.Files),
		),
		CreatedAt: time.Now(),
	}, nil
}

func (s *UploadService) emitMetrics(ctx context.Context, bucketID, ownerID, region string, partial dto.S3IngestRequest) {
	if s.metrics == nil {
		return
	}

	partial.BucketID = bucketID
	partial.OwnerID = ownerID
	partial.Region = region

	_ = s.metrics.SendS3Metrics(ctx, partial)
}

// ExportOutput is the return type for all export functions (ExportSelect, ExportBucket, ExportFolder).
// It contains the zip file bytes and info about any files that could not be found.
type ExportOutput struct {
	ZipData       []byte                 `json:"zip_data"`
	FilesNotFound FilesNotFoundForExport `json:"files_not_found"`
}

// FilesNotFoundForExport tracks which requested files were not found in the bucket.
type FilesNotFoundForExport struct {
	Count int      `json:"count"`
	Data  []string `json:"data"`
}

// ExportSelect takes a list of file keys from the client, looks them up in the bucket,
// fetches the matching files from storage (MinIO), zips them, and returns the zip bytes.
//
// LEARNING NOTE (SET INTERSECTION):
// The client sends a list of file keys they want exported.
// We need to check which of those keys actually exist in the bucket.
//
// In Java, you'd use HashSet for this:
//
//	Set<String> requestedFiles = new HashSet<>(files);
//	Set<String> bucketKeys = new HashSet<>();
//	for (File f : repoFiles) bucketKeys.add(f.getKey());
//	requestedFiles.retainAll(bucketKeys);  // intersection
//
// In Go, we use map[string]bool as a set:
//
//	repoKeySet := make(map[string]bool)
//	for _, f := range repoFiles { repoKeySet[f.Key] = true }
//	if repoKeySet[requestedKey] { /* found */ }
//
// BUG FIXES from original implementation:
//   - make([]string, len(files)) pre-fills with empty strings, then append adds AFTER them
//     Fix: use make([]string, 0, len(files)) — zero length, pre-allocated capacity
//   - j < len(repo_files)-1 skips the LAST file in the repo
//     Fix: use the map-based lookup instead of nested loops entirely
func (s *UploadService) ExportSelect(ctx context.Context, files []string, bucketID string) (ExportOutput, error) {
	actor := ctx.Value("userId")
	filterID := fmt.Sprintf("%v", actor)
	if IsAdmin(filterID) {
		filterID = ""
	}

	// Resolve bucket
	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return ExportOutput{}, err
	}

	// Get all files in the bucket from the database
	repoFiles, err := s.fileRepo.ListFiles(ctx, bucket.ID)
	if err != nil {
		return ExportOutput{}, fmt.Errorf("failed to list files: %w", err)
	}

	// Build a set (map) of all keys that exist in the bucket.
	// This is the Go equivalent of Java's HashSet — O(1) lookup instead of O(n) nested loops.
	repoKeySet := make(map[string]string) // key -> storageName mapping
	for _, f := range repoFiles {
		repoKeySet[f.Key] = f.Key
	}

	// Partition the requested files into found vs not-found.
	// IMPORTANT: use make([]string, 0) not make([]string, len(files))
	// The latter pre-fills with empty strings, and append adds AFTER them.
	filesFound := make([]string, 0, len(files))
	filesNotFound := make([]string, 0)

	for _, requestedKey := range files {
		if _, exists := repoKeySet[requestedKey]; exists {
			filesFound = append(filesFound, requestedKey)
		} else {
			filesNotFound = append(filesNotFound, requestedKey)
		}
	}

	// Create zip from the found files
	zipData, err := s.createZipFromFiles(ctx, bucket.StorageName, filesFound)
	if err != nil {
		return ExportOutput{}, fmt.Errorf("failed to create zip: %w", err)
	}

	// Emit metrics
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		GetRequests:     int64(len(filesFound)),
		BytesDownloaded: int64(len(zipData)),
	})

	return ExportOutput{
		ZipData: zipData,
		FilesNotFound: FilesNotFoundForExport{
			Count: len(filesNotFound),
			Data:  filesNotFound,
		},
	}, nil
}

// ExportBucket exports ALL files in a bucket as a single zip file.
// The client provides a bucket name/ID, and we zip every file in it.
func (s *UploadService) ExportBucket(ctx context.Context, bucketID string) (ExportOutput, error) {
	actor := ctx.Value("userId")
	filterID := fmt.Sprintf("%v", actor)
	if IsAdmin(filterID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return ExportOutput{}, err
	}

	files, err := s.fileRepo.ListFiles(ctx, bucket.ID)
	if err != nil {
		return ExportOutput{}, fmt.Errorf("failed to list files: %w", err)
	}

	// Collect all non-folder file keys
	var keys []string
	for _, f := range files {
		if strings.HasSuffix(f.Key, "/") && f.Size == 0 {
			continue // skip folder markers
		}
		keys = append(keys, f.Key)
	}

	if len(keys) == 0 {
		return ExportOutput{}, fmt.Errorf("bucket has no files to export")
	}

	zipData, err := s.createZipFromFiles(ctx, bucket.StorageName, keys)
	if err != nil {
		return ExportOutput{}, fmt.Errorf("failed to create zip: %w", err)
	}

	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		GetRequests:     int64(len(keys)),
		BytesDownloaded: int64(len(zipData)),
	})

	return ExportOutput{
		ZipData: zipData,
		FilesNotFound: FilesNotFoundForExport{
			Count: 0,
			Data:  []string{},
		},
	}, nil
}

// ExportFolder exports all files within a specific folder (prefix) of a bucket as a zip.
// The client provides a bucket name/ID and a folder name (prefix), and we zip every file under that prefix.
//
// LEARNING NOTE:
// Folders in S3/MinIO are just key prefixes. A file "images/photo.png" is "in" the "images/" folder
// because its key starts with "images/". We use ListFilesByPrefix to find all files under the folder.
func (s *UploadService) ExportFolder(ctx context.Context, bucketID string, folderName string) (ExportOutput, error) {
	actor := ctx.Value("userId")
	filterID := fmt.Sprintf("%v", actor)
	if IsAdmin(filterID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return ExportOutput{}, err
	}

	// Ensure folder prefix ends with "/"
	prefix := folderName
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	files, err := s.fileRepo.ListFilesByPrefix(ctx, bucket.ID, prefix, 0)
	if err != nil {
		return ExportOutput{}, fmt.Errorf("failed to list files by prefix: %w", err)
	}

	// Collect non-folder file keys
	var keys []string
	for _, f := range files {
		if strings.HasSuffix(f.Key, "/") && f.Size == 0 {
			continue
		}
		keys = append(keys, f.Key)
	}

	if len(keys) == 0 {
		return ExportOutput{}, fmt.Errorf("no files found in folder: %s", folderName)
	}

	zipData, err := s.createZipFromFiles(ctx, bucket.StorageName, keys)
	if err != nil {
		return ExportOutput{}, fmt.Errorf("failed to create zip: %w", err)
	}

	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		GetRequests:     int64(len(keys)),
		BytesDownloaded: int64(len(zipData)),
	})

	// test:= make([]byte, 2)

	return ExportOutput{
		ZipData: zipData,
		FilesNotFound: FilesNotFoundForExport{
			Count: 0,
			Data:  []string{},
		},
	}, nil
}

// createZipFromFiles fetches each file from MinIO by its key and writes them all
// into a zip archive in memory.
//
// LEARNING NOTE:
// archive/zip works with an in-memory buffer (bytes.Buffer).
// For each file: GetObject returns the raw bytes -> zip.Writer.Create adds an entry -> Write the bytes.
// Finally, Close() flushes the zip central directory.
// This is the same pattern used in prefix_service.go's createZipArchive.
func (s *UploadService) createZipFromFiles(ctx context.Context, storageName string, keys []string) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	log.Printf("---->&%v", storageName)
	for _, key := range keys {
		// Fetch the file bytes from MinIO
		data, err := s.storage.GetObject(ctx, storageName, key)
		if err != nil {
			log.Printf("[ExportService] failed to get object %s: %v", key, err)
			continue // skip files that fail to download
		}

		// Create an entry in the zip with the full key as the path
		// This preserves folder structure inside the zip
		writer, err := zipWriter.Create(key)
		if err != nil {
			log.Printf("[ExportService] failed to create zip entry for %s: %v", key, err)
			continue
		}

		if _, err := writer.Write(data); err != nil {
			log.Printf("[ExportService] failed to write zip data for %s: %v", key, err)
			continue
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize zip: %w", err)
	}

	return buf.Bytes(), nil
}

// LEETCODE:
// these are all the files we have in the database (repo_files)
// these are the files you want us to zip (files),
// the set(files) must exist in the set(repo_files)
/*
in ajva this wouldhave been easy

Set<String> files_repo = new HashSet<String>();
files_repo.addAll(files)
Set<String> bucket_repo = new HashSet<String>();
bucket_repo.addAll(repo_files)


if(bucket_repo.containsAll(files_repo)){


}

*/

// data, err := s.storage.GetObject(ctx, bucket.StorageName, file.Key)
// if err != nil {
// 	log.Printf("[SERVICE] DownloadFile: storage.GetObject failed")
// 	return "", fmt.Errorf("Some error while gettin the bytes ofa file")
// }

// return output ,nil

// }

func (s *UploadService) CreateFolder(ctx context.Context, bucketID, folderName string) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	// Folder name must end with /
	folderKey := folderName
	if !strings.HasSuffix(folderKey, "/") {
		folderKey += "/"
	}

	// 1. Save 0-byte object to storage
	if err := s.storage.SaveObject(ctx, bucket.StorageName, folderKey, []byte{}, nil); err != nil {
		return fmt.Errorf("failed to create folder in storage: %w", err)
	}

	// 2. Save metadata to repository
	file := domain.File{
		ID:        generateID(),
		BucketID:  bucket.ID,
		Key:       folderKey,
		Size:      0,
		MimeType:  "application/x-directory",
		CreatedAt: time.Now(),
	}

	if err := s.fileRepo.SaveFile(ctx, file); err != nil {
		return fmt.Errorf("failed to save folder metadata: %w", err)
	}

	// Emit metrics for folder creation (Tier 1)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return nil
}

// Simple ID generator (you can use UUID library later)
// generateID returns a new unique identifier
func generateID() string {
	return uuid.New().String()
}

func (s *UploadService) GetFileInfo(ctx context.Context, bucketID, fileID string) (*dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Resolve bucket by ID or Name
	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	// Emit metrics for Head (Tier 2)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		HeadRequests: 1,
	})

	fmt.Println("---> %s", fileID)
	// Get file from DB (try ID first, then Key)
	// file, err := s.fileRepo.GetFileByID(ctx, fileID)
	file, err := s.fileRepo.GetFileByIDOrKey(ctx, fileID, bucketID)

	if err != nil {
		// Fallback to Key
		var errKey error
		file, errKey = s.fileRepo.GetFileByKey(ctx, bucket.ID, fileID)
		if errKey != nil {
			return nil, fmt.Errorf("file not found: %w", err)
		}
	}

	// Verify file belongs to bucket
	if file.BucketID != bucket.ID {
		return nil, fmt.Errorf("file not in specified bucket")
	}
	fileMetadata := make(map[string]string, 0)

	fileMetadata["arn"] = "arni"

	file.Metadata = fileMetadata

	return &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  bucket.Name,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: file.CreatedAt,
	}, nil
}

func (s *UploadService) GetFileInfoFolder(ctx context.Context, bucketID, fileID, folderID string) (*dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Resolve bucket by ID or Name
	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	// Emit metrics for Head (Tier 2)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		HeadRequests: 1,
	})

	// Get file from DB (try ID first, then Key)
	file, err := s.fileRepo.GetFileByIDOrKey(ctx, fileID, bucketID)
	if err != nil {
		// Fallback to Key
		var errKey error
		file, errKey = s.fileRepo.GetFileByKey(ctx, bucket.ID, fileID)
		if errKey != nil {
			return nil, fmt.Errorf("file not found: %w", err)
		}
	}

	// Verify file belongs to bucket
	if file.BucketID != bucket.ID {
		return nil, fmt.Errorf("file not in specified bucket")
	}

	return &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  bucket.Name,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: file.CreatedAt,
	}, nil
}

// ListFiles returns the full folder-tree structure of a bucket.
//
// LEARNING NOTE:
// This function builds a tree from flat file keys (e.g. "images/vacation/photo.png").
// Each key is split by "/" to determine which folder it belongs to.
// The result is a nested structure that the frontend can directly render
// as a file explorer UI — folders are collapsible, files are nested inside them.
//
// How the tree is built:
//   1. Fetch all files from DB (flat list of keys)
//   2. For each file key, split by "/" to get path segments
//   3. Insert into a recursive map: map[folderName] -> children
//   4. Convert the map into FolderNode/FileNode structs

func (s *UploadService) ListFiles(ctx context.Context, c *gin.Context,bucketName string) (*dto.BucketStructureOutput, error) {
	actor := c.GetString("userId")
	filterID := fmt.Sprintf("%v", actor)
	log.Printf("Updated filter id code: %s", filterID)
	if IsAdmin(filterID) {
		filterID = ""
	}

	// Resolve bucket by ID or Name
	bucket, err := s.resolveBucket(ctx, bucketName, filterID)
	if err != nil {
		return nil, err
	}

	// Emit metrics for List (Tier 2)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		ListRequests: 1,
	})

	// Get files from DB using bucket ID
	files, err := s.fileRepo.ListFiles(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// Build output structure
	output := &dto.BucketStructureOutput{
		BucketID:   bucket.ID,
		BucketName: bucket.Name,
		RootFiles:  []dto.FileNode{},
		Folders:    []dto.FolderNode{},
	}

	// folderMap holds the files grouped by their folder path.
	// Key = folder path (e.g. "images/vacation/"), Value = list of files in that folder.
	// Files at the root have key "".
	folderMap := make(map[string][]dto.FileNode)
	folderSet := make(map[string]bool)

	totalFiles := 0

	for _, file := range files {
		// Skip folder marker objects (0-byte objects with trailing slash)
		if strings.HasSuffix(file.Key, "/") && file.Size == 0 {
			folderSet[file.Key] = true
			continue
		}

		totalFiles++

		// Split key into directory + basename
		// e.g. "images/vacation/photo.png" -> dir="images/vacation", base="photo.png"
		dir := path.Dir(file.Key)   // returns "." for root-level files
		base := path.Base(file.Key) // returns the filename

		node := dto.FileNode{
			FileID:    file.ID,
			Name:      base,
			Key:       file.Key,
			Size:      file.Size,
			MimeType:  file.MimeType,
			SHA256:    file.SHA256,
			CreatedAt: file.CreatedAt,
			Metadata:  file.Metadata,
		}

		if dir == "." {
			// File is at bucket root
			folderMap[""] = append(folderMap[""], node)
		} else {
			// File is inside a folder
			folderPath := dir + "/"
			folderMap[folderPath] = append(folderMap[folderPath], node)
			// Register all parent folders in the set
			// e.g. for "a/b/c/file.txt", register "a/", "a/b/", "a/b/c/"
			parts := strings.Split(dir, "/")
			for i := range parts {
				parentPath := strings.Join(parts[:i+1], "/") + "/"
				folderSet[parentPath] = true
			}
		}
	}

	// Set root files
	output.RootFiles = folderMap[""]
	if output.RootFiles == nil {
		output.RootFiles = []dto.FileNode{}
	}

	// Build the folder tree recursively from the folder set
	output.Folders = s.buildFolderTree("", folderSet, folderMap)
	output.TotalFiles = totalFiles
	output.TotalFolders = len(folderSet)

	return output, nil
}

// buildFolderTree recursively constructs the folder hierarchy.
//
// LEARNING NOTE:
// This is a recursive tree-builder. Given a parent path (e.g. "images/"),
// it finds all direct child folders (e.g. "images/vacation/", "images/icons/")
// and builds FolderNode structs for each, recursing into sub-folders.
//
// A folder is a "direct child" of parentPath if:
//   - It starts with parentPath
//   - The remaining part (after parentPath) contains exactly one segment (no more "/")
func (s *UploadService) buildFolderTree(parentPath string, folderSet map[string]bool, folderMap map[string][]dto.FileNode) []dto.FolderNode {
	var folders []dto.FolderNode

	for folderPath := range folderSet {
		// Check if this folder is a direct child of parentPath
		if !strings.HasPrefix(folderPath, parentPath) {
			continue
		}

		remaining := strings.TrimPrefix(folderPath, parentPath)
		// Direct child has exactly one segment: "foldername/"
		// Skip deeper paths like "a/b/" when looking for children of ""
		trimmed := strings.TrimSuffix(remaining, "/")
		if trimmed == "" || strings.Contains(trimmed, "/") {
			continue
		}

		// Count files recursively in this folder and all sub-folders
		fileCount := s.countFilesRecursive(folderPath, folderMap)

		files := folderMap[folderPath]
		if files == nil {
			files = []dto.FileNode{}
		}

		folder := dto.FolderNode{
			Name:       trimmed,
			Path:       folderPath,
			Files:      files,
			SubFolders: s.buildFolderTree(folderPath, folderSet, folderMap),
			FileCount:  fileCount,
		}

		folders = append(folders, folder)
	}

	if folders == nil {
		return []dto.FolderNode{}
	}
	return folders
}

// countFilesRecursive counts all files under a folder path (including sub-folders).
func (s *UploadService) countFilesRecursive(folderPath string, folderMap map[string][]dto.FileNode) int {
	count := 0
	for fp, files := range folderMap {
		if strings.HasPrefix(fp, folderPath) {
			count += len(files)
		}
	}
	return count
}

func (s *UploadService) DownloadFile(ctx context.Context, bucketId, fileID string, userID string) ([]byte, *dto.FileInfoOutput, error) {
	filterID := userID
	if IsAdmin(userID) {
		filterID = ""
	}
	log.Printf("[SERVICE] DownloadFile: start actorID=%s bucketID=%s fileID=%s isAdmin=%v", userID, bucketId, fileID, IsAdmin(userID))

	bucket, err := s.resolveBucket(ctx, bucketId, filterID)
	if err != nil {
		log.Printf("[SERVICE] DownloadFile: resolveBucket failed bucketID=%s err=%v", bucketId, err)
		return nil, nil, err
	}

	log.Printf("[SERVICE] DownloadFile: bucket resolved bucketID=%s storageName=%s", bucket.ID, bucket.StorageName)

	var file *domain.File
	isSystemActor := userID == "00000000-0000-0000-0000-000000000000"

	if isSystemActor {
		log.Printf("[SERVICE] DownloadFile: system actor — resolving file by ID or key fileID=%s bucketID=%s", fileID, bucket.ID)
		file, err = s.fileRepo.GetFileByIDOrKey(ctx, fileID, bucket.ID)
	} else {
		log.Printf("[SERVICE] DownloadFile: regular actor%s resolving file strictly by ID fileID=%s", userID, fileID)
		file, err = s.fileRepo.GetFileByID(ctx, fileID)
	}

	if err != nil {
		log.Printf("[SERVICE] DownloadFile: file resolution failed fileID=%s err=%v", fileID, err)
		return nil, nil, fmt.Errorf("file not found: %w", err)
	}

	log.Printf("[SERVICE] DownloadFile: file resolved fileID=%s key=%s bucketID=%s size=%d", file.ID, file.Key, file.BucketID, file.Size)

	if file.BucketID != bucket.ID {
		log.Printf("[SERVICE] DownloadFile: bucket mismatch fileBucketID=%s requestedBucketID=%s", file.BucketID, bucket.ID)
		return nil, nil, fmt.Errorf("file not in specified bucket")
	}

	log.Printf("[SERVICE] DownloadFile: fetching object from storage storageName=%s key=%s", bucket.StorageName, file.Key)

	data, err := s.storage.GetObject(ctx, bucket.StorageName, file.Key)
	if err != nil {
		log.Printf("[SERVICE] DownloadFile: storage.GetObject failed storageName=%s key=%s err=%v", bucket.StorageName, file.Key, err)
		return nil, nil, fmt.Errorf("failed to retrieve file: %w", err)
	}

	log.Printf("[SERVICE] DownloadFile: object retrieved successfully key=%s bytes=%d", file.Key, len(data))

	metadata := &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  file.BucketID,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: file.CreatedAt,
	}

	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		GetRequests:     1,
		BytesDownloaded: int64(len(data)),
	})

	log.Printf("[SERVICE] DownloadFile: done fileID=%s metrics emitted async", fileID)

	return data, metadata, nil
}

func (s *UploadService) UpdateFileMetadata(ctx context.Context, bucketID, fileID string, input dto.UpdateFileMetadataInput) (*dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if file.BucketID != bucket.ID {
		return nil, fmt.Errorf("file not in specified bucket")
	}

	file.Metadata = input.Metadata

	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		return nil, fmt.Errorf("failed to update metadata: %w", err)
	}

	// Emit metrics for Metadata update (Tier 1)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  bucketID,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: file.CreatedAt,
	}, nil
}

func (s *UploadService) CopyFile(ctx context.Context, sourceBucketID, fileID string, input dto.CopyFileInput) (*dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	sourceBucket, err := s.resolveBucket(ctx, sourceBucketID, filterID)
	if err != nil {
		return nil, err
	}

	destBucket, err := s.resolveBucket(ctx, input.DestinationBucket, filterID)
	if err != nil {
		return nil, err
	}

	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if file.BucketID != sourceBucket.ID {
		return nil, fmt.Errorf("file not in specified bucket")
	}

	newKey := input.NewKey
	if newKey == "" {
		newKey = file.Key
	}

	// Copy in storage
	if err := s.storage.CopyObject(ctx, sourceBucket.StorageName, file.Key, destBucket.StorageName, newKey); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	// Create new file record
	newFile := domain.File{
		ID:        generateID(),
		BucketID:  destBucket.ID,
		Key:       newKey,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: time.Now(),
	}

	if err := s.fileRepo.SaveFile(ctx, newFile); err != nil {
		return nil, fmt.Errorf("failed to save file metadata: %w", err)
	}

	// Emit metrics for Copy (Source Get + Dest Put)
	go s.emitMetrics(context.Background(), sourceBucket.ID, sourceBucket.OwnerID, sourceBucket.Region, dto.S3IngestRequest{
		GetRequests: 1,
	})
	go s.emitMetrics(context.Background(), destBucket.ID, destBucket.OwnerID, destBucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return &dto.FileInfoOutput{
		FileID:    newFile.ID,
		BucketID:  input.DestinationBucket,
		Key:       newFile.Key,
		Size:      newFile.Size,
		MimeType:  newFile.MimeType,
		Metadata:  newFile.Metadata,
		SHA256:    newFile.SHA256,
		CreatedAt: newFile.CreatedAt,
	}, nil
}

func (s *UploadService) MoveFile(ctx context.Context, sourceBucketName, fileID string, input dto.MoveFileInput) (*dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	sourceBucket, err := s.resolveBucket(ctx, sourceBucketName, filterID)
	if err != nil {
		return nil, err
	}

	destBucket, err := s.resolveBucket(ctx, input.DestinationBucket, filterID)
	if err != nil {
		return nil, err
	}

	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if file.BucketID != sourceBucket.ID {
		return nil, fmt.Errorf("file not in specified bucket")
	}

	newKey := input.NewKey
	if newKey == "" {
		newKey = file.Key
	}

	// Copy to destination
	if err := s.storage.CopyObject(ctx, sourceBucket.StorageName, file.Key, destBucket.StorageName, newKey); err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	// Delete from source
	if err := s.storage.DeleteObject(ctx, sourceBucket.StorageName, file.Key); err != nil {
		return nil, fmt.Errorf("failed to delete source file: %w", err)
	}

	// Update DB record
	file.BucketID = destBucket.ID
	file.Key = newKey

	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		return nil, fmt.Errorf("failed to update file metadata: %w", err)
	}

	// Emit metrics for Move (Source Delete + Dest Put)
	go s.emitMetrics(context.Background(), sourceBucket.ID, sourceBucket.OwnerID, sourceBucket.Region, dto.S3IngestRequest{
		DeleteRequests: 1,
	})
	go s.emitMetrics(context.Background(), destBucket.ID, destBucket.OwnerID, destBucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  input.DestinationBucket,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: file.CreatedAt,
	}, nil
}

func (s *UploadService) GetFilesBySHA256(ctx context.Context, sha256 string) ([]dto.FileInfoOutput, error) {
	files, err := s.fileRepo.GetFilesBySHA256(ctx, sha256)
	if err != nil {
		return nil, fmt.Errorf("failed to get files by sha256: %w", err)
	}

	var output []dto.FileInfoOutput
	for _, file := range files {
		output = append(output, dto.FileInfoOutput{
			FileID:    file.ID,
			BucketID:  file.BucketID, // This will be the ID, might need resolve to Name
			Key:       file.Key,
			Size:      file.Size,
			MimeType:  file.MimeType,
			Metadata:  file.Metadata,
			SHA256:    file.SHA256,
			CreatedAt: file.CreatedAt,
		})
	}

	return output, nil
}

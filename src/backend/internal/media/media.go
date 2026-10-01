// Package media 实现图片校验、对象存储协调和媒体元数据持久化。
package media

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrTooLarge           = errors.New("media file too large")
	ErrUnsupportedFormat  = errors.New("unsupported media format")
	ErrInvalidImage       = errors.New("invalid image")
	ErrInvalidDimensions  = errors.New("invalid image dimensions")
	ErrEXIFForbidden      = errors.New("image contains EXIF metadata")
	ErrBusy               = errors.New("media upload already in progress")
	ErrStorageUnavailable = errors.New("media storage unavailable")
)

const (
	MaxFileSize  int64 = 10 << 20
	MaxDimension       = 8192
	MaxPixels          = 20_000_000
)

// Asset 同时承载持久化元数据和上传成功响应；对象键不暴露给浏览器业务逻辑。
type Asset struct {
	ID             uint64    `json:"id"`
	ObjectKey      string    `json:"-"`
	URL            string    `json:"url" gorm:"-"`
	MIMEType       string    `json:"mimeType"`
	SizeBytes      int64     `json:"sizeBytes"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	ChecksumSHA256 [32]byte  `json:"-"`
	Status         string    `json:"-"`
	CreatedAt      time.Time `json:"createdAt"`
}

type assetRecord struct {
	ID             uint64
	ObjectKey      string
	MIMEType       string
	SizeBytes      int64
	Width          int
	Height         int
	ChecksumSHA256 []byte
	Status         string
	CreatedAt      time.Time
}

func (assetRecord) TableName() string { return "media_assets" }

type Repository struct {
	database *gorm.DB
}

func NewRepository(database *gorm.DB) *Repository {
	return &Repository{database: database}
}

func (repository *Repository) Create(ctx context.Context, asset Asset) (Asset, error) {
	record := assetRecord{
		ObjectKey: asset.ObjectKey, MIMEType: asset.MIMEType, SizeBytes: asset.SizeBytes,
		Width: asset.Width, Height: asset.Height, ChecksumSHA256: asset.ChecksumSHA256[:],
		Status: asset.Status, CreatedAt: asset.CreatedAt,
	}
	if err := repository.database.WithContext(ctx).Create(&record).Error; err != nil {
		return Asset{}, fmt.Errorf("create media metadata: %w", err)
	}
	asset.ID = record.ID
	return asset, nil
}

type metadataRepository interface {
	Create(ctx context.Context, asset Asset) (Asset, error)
}

type ObjectStorage interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, mimeType, contentMD5 string) error
	Delete(ctx context.Context, key string) error
}

type Service struct {
	repository metadataRepository
	storage    ObjectStorage
	random     io.Reader
	publicBase string
	now        func() time.Time
	uploadSlot chan struct{}
}

func NewService(repository metadataRepository, storage ObjectStorage, random io.Reader, publicBase string) (*Service, error) {
	publicBase = strings.TrimRight(strings.TrimSpace(publicBase), "/")
	if repository == nil || storage == nil || random == nil || publicBase == "" {
		return nil, errors.New("media service requires repository, storage, random source, and public URL")
	}
	return &Service{
		repository: repository,
		storage:    storage,
		random:     random,
		publicBase: publicBase,
		now:        func() time.Time { return time.Now().UTC() },
		uploadSlot: make(chan struct{}, 1),
	}, nil
}

func (service *Service) Upload(ctx context.Context, file io.ReadSeeker) (Asset, error) {
	select {
	case service.uploadSlot <- struct{}{}:
		defer func() { <-service.uploadSlot }()
	default:
		return Asset{}, ErrBusy
	}

	info, err := inspectImage(file)
	if err != nil {
		return Asset{}, err
	}
	key, err := service.objectKey(info.extension)
	if err != nil {
		return Asset{}, fmt.Errorf("generate media object key: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return Asset{}, fmt.Errorf("rewind media for upload: %w", err)
	}
	uploadStarted := time.Now()
	if err := service.storage.Put(ctx, key, file, info.size, info.mimeType, info.contentMD5); err != nil {
		slog.Error("media object upload failed", "object_key", key, "size_bytes", info.size, "duration_ms", time.Since(uploadStarted).Milliseconds(), "error", err)
		return Asset{}, fmt.Errorf("%w: %v", ErrStorageUnavailable, err)
	}
	slog.Info("media object upload completed", "object_key", key, "size_bytes", info.size, "duration_ms", time.Since(uploadStarted).Milliseconds())

	asset := Asset{
		ObjectKey:      key,
		URL:            service.publicBase + "/" + key,
		MIMEType:       info.mimeType,
		SizeBytes:      info.size,
		Width:          info.width,
		Height:         info.height,
		ChecksumSHA256: info.checksum,
		Status:         "ready",
		CreatedAt:      service.now(),
	}
	created, err := service.repository.Create(ctx, asset)
	if err == nil {
		return created, nil
	}
	// COS 与 MySQL 无共同事务；元数据失败时立即补偿删除刚上传的对象。
	compensationContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if deleteErr := service.storage.Delete(compensationContext, key); deleteErr != nil {
		slog.Error("media compensation deletion failed", "object_key", key, "error", deleteErr)
		return Asset{}, fmt.Errorf("save media metadata: %w; compensate object deletion: %v", err, deleteErr)
	}
	slog.Warn("media object deleted after metadata failure", "object_key", key)
	return Asset{}, fmt.Errorf("save media metadata: %w", err)
}

func (service *Service) objectKey(extension string) (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := io.ReadFull(service.random, randomBytes); err != nil {
		return "", err
	}
	now := service.now()
	return fmt.Sprintf("media/%04d/%02d/%s.%s", now.Year(), now.Month(), hex.EncodeToString(randomBytes), extension), nil
}

// UnavailableStorage 让未配置 COS 的本地环境仍能启动，并对上传返回可重试错误。
type UnavailableStorage struct{}

func (UnavailableStorage) Put(context.Context, string, io.Reader, int64, string, string) error {
	return ErrStorageUnavailable
}

func (UnavailableStorage) Delete(context.Context, string) error { return nil }

package main

import (
	"crypto/rand"
	"errors"
	"net/url"
	"strings"

	"gorm.io/gorm"

	"hxy-blog/backend/internal/media"
)

type mediaConfig struct {
	enabled    bool
	bucketURL  string
	publicBase string
	secretID   string
	secretKey  string
}

func loadMediaConfig(getenv func(string) string) (mediaConfig, error) {
	config := mediaConfig{
		bucketURL:  strings.TrimSpace(getenv("MEDIA_COS_BUCKET_URL")),
		publicBase: strings.TrimRight(strings.TrimSpace(getenv("MEDIA_PUBLIC_BASE_URL")), "/"),
		secretID:   strings.TrimSpace(getenv("MEDIA_COS_SECRET_ID")),
		secretKey:  strings.TrimSpace(getenv("MEDIA_COS_SECRET_KEY")),
	}
	values := []string{config.bucketURL, config.publicBase, config.secretID, config.secretKey}
	setCount := 0
	for _, value := range values {
		if value != "" {
			setCount++
		}
	}
	if setCount == 0 {
		return config, nil
	}
	if setCount != len(values) {
		return mediaConfig{}, errors.New("MEDIA_COS_BUCKET_URL, MEDIA_PUBLIC_BASE_URL, MEDIA_COS_SECRET_ID, and MEDIA_COS_SECRET_KEY must be configured together")
	}
	publicURL, err := url.Parse(config.publicBase)
	if err != nil || publicURL.Scheme != "https" || publicURL.Host == "" || publicURL.Path != "" || publicURL.User != nil || publicURL.RawQuery != "" || publicURL.Fragment != "" {
		return mediaConfig{}, errors.New("MEDIA_PUBLIC_BASE_URL must be an HTTPS origin without a path")
	}
	config.enabled = true
	return config, nil
}

func buildMediaService(getenv func(string) string, database *gorm.DB) (*media.Service, error) {
	config, err := loadMediaConfig(getenv)
	if err != nil {
		return nil, err
	}
	var storage media.ObjectStorage = media.UnavailableStorage{}
	publicBase := "https://media.invalid"
	if config.enabled {
		storage, err = media.NewCOSStorage(config.bucketURL, config.secretID, config.secretKey)
		if err != nil {
			return nil, err
		}
		publicBase = config.publicBase
	}
	return media.NewService(media.NewRepository(database), storage, rand.Reader, publicBase)
}

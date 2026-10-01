package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

type COSStorage struct {
	client *cos.Client
}

func NewCOSStorage(bucketURL, secretID, secretKey string) (*COSStorage, error) {
	bucket, err := url.Parse(bucketURL)
	if err != nil || bucket.Scheme != "https" || bucket.Host == "" || bucket.Path != "" || bucket.User != nil || bucket.RawQuery != "" || bucket.Fragment != "" {
		return nil, errors.New("COS bucket URL must be an HTTPS origin without a path")
	}
	if secretID == "" || secretKey == "" {
		return nil, errors.New("COS credentials are required")
	}
	client := cos.NewClient(&cos.BaseURL{BucketURL: bucket}, &http.Client{
		Timeout: 30 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  secretID,
			SecretKey: secretKey,
		},
	})
	return &COSStorage{client: client}, nil
}

func (storage *COSStorage) Put(ctx context.Context, key string, body io.Reader, size int64, mimeType, contentMD5 string) error {
	// 桶关闭版本控制，显式禁止覆盖可让随机键碰撞安全失败。
	extraHeaders := http.Header{"x-cos-forbid-overwrite": []string{"true"}}
	_, err := storage.client.Object.Put(ctx, key, body, &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType:   mimeType,
			ContentLength: size,
			ContentMD5:    contentMD5,
			CacheControl:  "public, max-age=31536000, immutable",
			XOptionHeader: &extraHeaders,
		},
	})
	return err
}

func (storage *COSStorage) Delete(ctx context.Context, key string) error {
	_, err := storage.client.Object.Delete(ctx, key)
	return err
}

package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"testing"
	"time"
)

type repositoryStub struct {
	err   error
	asset Asset
}

func (stub *repositoryStub) Create(_ context.Context, asset Asset) (Asset, error) {
	stub.asset = asset
	asset.ID = 9
	return asset, stub.err
}

type storageStub struct {
	putKey     string
	putBody    []byte
	putSize    int64
	putMIME    string
	putMD5     string
	putErr     error
	deletedKey string
	deleteCtx  error
	deleteErr  error
}

func (stub *storageStub) Put(_ context.Context, key string, body io.Reader, size int64, mimeType, contentMD5 string) error {
	stub.putKey = key
	stub.putBody, _ = io.ReadAll(body)
	stub.putSize = size
	stub.putMIME = mimeType
	stub.putMD5 = contentMD5
	return stub.putErr
}

func (stub *storageStub) Delete(ctx context.Context, key string) error {
	stub.deletedKey = key
	stub.deleteCtx = ctx.Err()
	return stub.deleteErr
}

func TestServiceUploadsValidatedImage(t *testing.T) {
	repository := &repositoryStub{}
	storage := &storageStub{}
	service, err := NewService(repository, storage, bytes.NewReader(bytes.Repeat([]byte{0x2a}, 16)), "https://media.example.com/")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC) }
	imageBytes := encodePNG(t, 3, 2)

	asset, err := service.Upload(context.Background(), bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	wantKey := "media/2026/10/2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a.png"
	if asset.ID != 9 || asset.ObjectKey != wantKey || asset.URL != "https://media.example.com/"+wantKey {
		t.Fatalf("asset = %#v", asset)
	}
	if asset.MIMEType != "image/png" || asset.Width != 3 || asset.Height != 2 || asset.SizeBytes != int64(len(imageBytes)) {
		t.Fatalf("asset metadata = %#v", asset)
	}
	if storage.putKey != wantKey || !bytes.Equal(storage.putBody, imageBytes) {
		t.Fatalf("storage upload = key %q, bytes %d", storage.putKey, len(storage.putBody))
	}
	if storage.putSize != int64(len(imageBytes)) || storage.putMIME != "image/png" || storage.putMD5 == "" {
		t.Fatalf("storage metadata = size %d, MIME %q, MD5 %q", storage.putSize, storage.putMIME, storage.putMD5)
	}
}

func TestServiceCompensatesMetadataFailure(t *testing.T) {
	repository := &repositoryStub{err: errors.New("database unavailable")}
	storage := &storageStub{}
	service, _ := NewService(repository, storage, bytes.NewReader(make([]byte, 16)), "https://media.example.com")

	requestContext, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.Upload(requestContext, bytes.NewReader(encodePNG(t, 1, 1)))
	if err == nil || storage.deletedKey == "" || storage.deletedKey != storage.putKey || storage.deleteCtx != nil {
		t.Fatalf("Upload() error = %v, put = %q, deleted = %q, delete context = %v", err, storage.putKey, storage.deletedKey, storage.deleteCtx)
	}
}

func TestInspectImageRejectsUnsafeFiles(t *testing.T) {
	jpegWithEXIF := encodeJPEG(t, 2, 2)
	segment := []byte{0xff, 0xe1, 0x00, 0x08, 'E', 'x', 'i', 'f', 0, 0}
	jpegWithEXIF = append(append(jpegWithEXIF[:2:2], segment...), jpegWithEXIF[2:]...)

	tests := []struct {
		name string
		data []byte
		want error
	}{
		{name: "unsupported", data: []byte("not an image"), want: ErrUnsupportedFormat},
		{name: "corrupt png", data: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}, want: ErrInvalidImage},
		{name: "dimension", data: encodePNG(t, MaxDimension+1, 1), want: ErrInvalidDimensions},
		{name: "exif", data: jpegWithEXIF, want: ErrEXIFForbidden},
		{name: "too large", data: bytes.Repeat([]byte{'x'}, int(MaxFileSize+1)), want: ErrTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := inspectImage(bytes.NewReader(test.data))
			if !errors.Is(err, test.want) {
				t.Fatalf("inspectImage() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestInspectImageAcceptsWebP(t *testing.T) {
	// 固定的无元数据 WebP 夹具避免测试依赖外部文件或额外编码器。
	webp, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	if err != nil {
		t.Fatalf("decode WebP fixture: %v", err)
	}
	info, err := inspectImage(bytes.NewReader(webp))
	if err != nil {
		t.Fatalf("inspectImage(WebP) error = %v", err)
	}
	if info.mimeType != "image/webp" || info.width <= 0 || info.height <= 0 {
		t.Fatalf("WebP info = %#v", info)
	}
}

func encodePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buffer.Bytes()
}

func encodeJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	picture.Set(0, 0, color.White)
	if err := jpeg.Encode(&buffer, picture, nil); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	return buffer.Bytes()
}

package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"testing"
	"time"

	"golang.org/x/image/bmp"
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
	pngWithEXIF := addPNGEXIF(t, encodePNG(t, 2, 2))
	webpWithEXIF := addWebPEXIF(t, decodeWebPFixture(t))

	tests := []struct {
		name string
		data []byte
		want error
	}{
		{name: "unsupported", data: []byte("not an image"), want: ErrUnsupportedFormat},
		{name: "corrupt png", data: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}, want: ErrInvalidImage},
		{name: "dimension", data: encodePNG(t, MaxDimension+1, 1), want: ErrInvalidDimensions},
		{name: "jpeg exif", data: jpegWithEXIF, want: ErrEXIFForbidden},
		{name: "png exif", data: pngWithEXIF, want: ErrEXIFForbidden},
		{name: "webp exif", data: webpWithEXIF, want: ErrEXIFForbidden},
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
	webp := decodeWebPFixture(t)
	info, err := inspectImage(bytes.NewReader(webp))
	if err != nil {
		t.Fatalf("inspectImage(WebP) error = %v", err)
	}
	if info.mimeType != "image/webp" || info.width <= 0 || info.height <= 0 {
		t.Fatalf("WebP info = %#v", info)
	}
}

func TestInspectImageAcceptsBMPAndAnimatedGIF(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		mimeType  string
		extension string
	}{
		{name: "bmp", data: encodeBMP(t, 3, 2), mimeType: "image/bmp", extension: "bmp"},
		{name: "gif", data: encodeAnimatedGIF(t), mimeType: "image/gif", extension: "gif"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info, err := inspectImage(bytes.NewReader(test.data))
			if err != nil {
				t.Fatalf("inspectImage() error = %v", err)
			}
			if info.mimeType != test.mimeType || info.extension != test.extension || info.width != 3 || info.height != 2 {
				t.Fatalf("image info = %#v", info)
			}
		})
	}
}

func TestInspectImageRejectsCorruptLaterGIFFrame(t *testing.T) {
	data := encodeAnimatedGIF(t)
	_, err := inspectImage(bytes.NewReader(data[:len(data)-3]))
	if !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("inspectImage() error = %v, want %v", err, ErrInvalidImage)
	}
}

func TestInspectImageRejectsGIFFrameBudget(t *testing.T) {
	palette := color.Palette{color.Black, color.White}
	animation := &gif.GIF{Image: make([]*image.Paletted, maxGIFFrames+1), Delay: make([]int, maxGIFFrames+1)}
	for index := range animation.Image {
		animation.Image[index] = image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	}
	var buffer bytes.Buffer
	if err := gif.EncodeAll(&buffer, animation); err != nil {
		t.Fatalf("encode GIF: %v", err)
	}
	_, err := inspectImage(bytes.NewReader(buffer.Bytes()))
	if !errors.Is(err, ErrInvalidDimensions) {
		t.Fatalf("inspectImage() error = %v, want %v", err, ErrInvalidDimensions)
	}
}

func TestServiceKeepsAnimatedGIFBytes(t *testing.T) {
	repository := &repositoryStub{}
	storage := &storageStub{}
	service, err := NewService(repository, storage, bytes.NewReader(bytes.Repeat([]byte{0x2a}, 16)), "https://media.example.com")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	animation := encodeAnimatedGIF(t)
	asset, err := service.Upload(context.Background(), bytes.NewReader(animation))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if asset.MIMEType != "image/gif" || storage.putMIME != "image/gif" || !bytes.HasSuffix([]byte(asset.ObjectKey), []byte(".gif")) || !bytes.Equal(storage.putBody, animation) {
		t.Fatalf("GIF upload metadata or original bytes changed: asset = %#v, MIME = %q", asset, storage.putMIME)
	}
}

func addPNGEXIF(t *testing.T, source []byte) []byte {
	t.Helper()
	if len(source) < 33 {
		t.Fatal("PNG fixture is too short")
	}
	data := []byte("test metadata")
	chunk := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(data)))
	copy(chunk[4:8], "eXIf")
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[8+len(data):], crc32.ChecksumIEEE(chunk[4:8+len(data)]))
	result := make([]byte, 0, len(source)+len(chunk))
	result = append(result, source[:33]...)
	result = append(result, chunk...)
	return append(result, source[33:]...)
}

func addWebPEXIF(t *testing.T, source []byte) []byte {
	t.Helper()
	if len(source) < 12 || string(source[:4]) != "RIFF" || string(source[8:12]) != "WEBP" {
		t.Fatal("WebP fixture has an invalid RIFF header")
	}
	data := []byte("test metadata")
	chunk := make([]byte, 8+len(data))
	copy(chunk[:4], "EXIF")
	binary.LittleEndian.PutUint32(chunk[4:8], uint32(len(data)))
	copy(chunk[8:], data)
	if len(data)%2 == 1 {
		chunk = append(chunk, 0)
	}
	result := append(append([]byte(nil), source...), chunk...)
	binary.LittleEndian.PutUint32(result[4:8], uint32(len(result)-8))
	return result
}

func decodeWebPFixture(t *testing.T) []byte {
	t.Helper()
	// 固定的无元数据 WebP 夹具避免测试依赖外部文件或额外编码器。
	webp, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	if err != nil {
		t.Fatalf("decode WebP fixture: %v", err)
	}
	return webp
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

func encodeBMP(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := bmp.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("encode BMP: %v", err)
	}
	return buffer.Bytes()
}

func encodeAnimatedGIF(t *testing.T) []byte {
	t.Helper()
	palette := color.Palette{color.Black, color.White}
	frames := []*image.Paletted{
		image.NewPaletted(image.Rect(0, 0, 3, 2), palette),
		image.NewPaletted(image.Rect(0, 0, 3, 2), palette),
	}
	frames[1].SetColorIndex(0, 0, 1)
	var buffer bytes.Buffer
	if err := gif.EncodeAll(&buffer, &gif.GIF{Image: frames, Delay: []int{5, 5}}); err != nil {
		t.Fatalf("encode GIF: %v", err)
	}
	return buffer.Bytes()
}

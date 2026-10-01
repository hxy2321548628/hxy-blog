package media

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

type recordingTransport struct {
	request *http.Request
}

func (transport *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.request = request
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Request:    request,
	}, nil
}

func TestCOSStoragePutPreservesContentAndPreventsOverwrite(t *testing.T) {
	body := []byte("image bytes")
	bucketURL, err := url.Parse("https://bucket.example.com")
	if err != nil {
		t.Fatalf("parse bucket URL: %v", err)
	}
	transport := &recordingTransport{}
	storage := &COSStorage{client: cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{Transport: transport})}
	if err := storage.Put(context.Background(), "media/test.png", bytes.NewReader(body), int64(len(body)), "image/png", "YWJjZA=="); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	request := transport.request
	if request == nil || request.Method != http.MethodPut {
		t.Fatalf("request = %#v", request)
	}
	if request.Header.Get("Content-Type") != "image/png" {
		t.Errorf("Content-Type = %q", request.Header.Get("Content-Type"))
	}
	if request.Header.Get("Content-MD5") == "" {
		t.Error("Content-MD5 is empty")
	}
	if request.Header.Get("x-cos-forbid-overwrite") != "true" {
		t.Errorf("x-cos-forbid-overwrite = %q", request.Header.Get("x-cos-forbid-overwrite"))
	}
}

func TestNewCOSStorageRequiresHTTPSOrigin(t *testing.T) {
	for _, bucketURL := range []string{
		"http://bucket.example.com",
		"https://bucket.example.com/path",
		"https://bucket.example.com?version=1",
		"https://bucket.example.com?",
		"not-a-url",
	} {
		if _, err := NewCOSStorage(bucketURL, "secret-id", "secret-key"); err == nil {
			t.Fatalf("NewCOSStorage(%q) error = nil", bucketURL)
		}
	}
}

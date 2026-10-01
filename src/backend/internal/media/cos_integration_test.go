//go:build cosintegration

package media

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCOSStorageRoundTrip(t *testing.T) {
	required := func(key string) string {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			t.Fatalf("missing required COS integration variable %s", key)
		}
		return value
	}
	bucketURL := required("MEDIA_TEST_COS_BUCKET_URL")
	publicBase := strings.TrimRight(required("MEDIA_TEST_PUBLIC_BASE_URL"), "/")
	publicURL, err := url.Parse(publicBase)
	if err != nil || publicURL.Scheme != "https" || publicURL.Host == "" || publicURL.Path != "" || publicURL.User != nil || publicURL.RawQuery != "" || publicURL.ForceQuery || publicURL.Fragment != "" {
		t.Fatal("MEDIA_TEST_PUBLIC_BASE_URL must be an HTTPS origin without a path")
	}
	storage, err := NewCOSStorage(
		bucketURL,
		required("MEDIA_TEST_COS_SECRET_ID"),
		required("MEDIA_TEST_COS_SECRET_KEY"),
	)
	if err != nil {
		t.Fatalf("NewCOSStorage() error = %v", err)
	}

	payload, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode PNG fixture: %v", err)
	}
	randomBytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
		t.Fatalf("generate integration key: %v", err)
	}
	key := fmt.Sprintf("media/integration/%s.png", hex.EncodeToString(randomBytes))
	md5Sum := md5.Sum(payload)

	requestContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := storage.Put(
		requestContext,
		key,
		bytes.NewReader(payload),
		int64(len(payload)),
		"image/png",
		base64.StdEncoding.EncodeToString(md5Sum[:]),
	); err != nil {
		t.Fatalf("upload isolated COS object: %v", err)
	}
	uploaded := true
	t.Cleanup(func() {
		if !uploaded {
			return
		}
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := storage.Delete(cleanupContext, key); err != nil {
			t.Errorf("delete isolated COS object %s: %v", key, err)
		}
	})

	client := &http.Client{Timeout: 10 * time.Second}
	var downloaded []byte
	for attempt := 1; attempt <= 5; attempt++ {
		response, getErr := client.Get(publicBase + "/" + key)
		if getErr == nil {
			downloaded, getErr = io.ReadAll(io.LimitReader(response.Body, MaxFileSize+1))
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				getErr = fmt.Errorf("HTTPS read status = %d", response.StatusCode)
			}
		}
		if getErr == nil {
			break
		}
		if attempt == 5 {
			t.Fatalf("read isolated COS object through HTTPS: %v", getErr)
		}
		time.Sleep(time.Second)
	}
	wantChecksum := sha256.Sum256(payload)
	gotChecksum := sha256.Sum256(downloaded)
	if !bytes.Equal(downloaded, payload) || gotChecksum != wantChecksum {
		t.Fatal("HTTPS round trip changed media bytes or checksum")
	}

	cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cleanupCancel()
	if err := storage.Delete(cleanupContext, key); err != nil {
		t.Fatalf("delete isolated COS object: %v", err)
	}
	uploaded = false
}

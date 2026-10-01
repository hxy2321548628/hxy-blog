package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"hxy-blog/backend/internal/auth"
	"hxy-blog/backend/internal/media"
)

type mediaServiceStub struct {
	asset media.Asset
	err   error
	calls int
}

func (stub *mediaServiceStub) Upload(_ context.Context, file io.ReadSeeker) (media.Asset, error) {
	stub.calls++
	_, _ = io.Copy(io.Discard, file)
	return stub.asset, stub.err
}

func TestUploadMediaRequiresAccessToken(t *testing.T) {
	service := &mediaServiceStub{}
	authentication := testAuthHTTP(accessTokenVerifierStub{})
	authentication.media = service
	request := multipartRequest(t, []byte("file"))
	response := httptest.NewRecorder()

	newHandler(&postAdminStub{}, authentication).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if service.calls != 0 {
		t.Fatalf("Upload() calls = %d, want 0", service.calls)
	}
}

func TestUploadMediaReturnsCreatedAsset(t *testing.T) {
	service := &mediaServiceStub{asset: media.Asset{
		ID: 5, URL: "https://media.example.com/media/test.png",
		MIMEType: "image/png", SizeBytes: 4, Width: 2, Height: 1,
	}}
	authentication := testAuthHTTP(accessTokenVerifierStub{
		claims: auth.AccessClaims{AdminID: 1, Username: "admin"},
	})
	authentication.media = service
	request := multipartRequest(t, []byte("file"))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()

	newHandler(&postAdminStub{}, authentication).ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d; body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if service.calls != 1 || !bytes.Contains(response.Body.Bytes(), []byte(`"url":"https://media.example.com/media/test.png"`)) {
		t.Fatalf("calls = %d, body = %s", service.calls, response.Body.String())
	}
}

func TestUploadMediaMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		code       string
		result     string
	}{
		{name: "too large", err: media.ErrTooLarge, statusCode: http.StatusRequestEntityTooLarge, code: "MEDIA_TOO_LARGE", result: "too_large"},
		{name: "unsupported", err: media.ErrUnsupportedFormat, statusCode: http.StatusUnsupportedMediaType, code: "MEDIA_UNSUPPORTED", result: "validation_failed"},
		{name: "dimensions", err: media.ErrInvalidDimensions, statusCode: http.StatusUnprocessableEntity, code: "MEDIA_DIMENSIONS_INVALID", result: "validation_failed"},
		{name: "EXIF", err: media.ErrEXIFForbidden, statusCode: http.StatusUnprocessableEntity, code: "MEDIA_EXIF_FORBIDDEN", result: "validation_failed"},
		{name: "corrupt", err: media.ErrInvalidImage, statusCode: http.StatusUnprocessableEntity, code: "MEDIA_INVALID", result: "validation_failed"},
		{name: "busy", err: media.ErrBusy, statusCode: http.StatusTooManyRequests, code: "MEDIA_UPLOAD_BUSY", result: "busy"},
		{name: "storage", err: media.ErrStorageUnavailable, statusCode: http.StatusServiceUnavailable, code: "MEDIA_STORAGE_UNAVAILABLE", result: "storage_failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previousLogger) })
			service := &mediaServiceStub{err: test.err}
			request := multipartRequest(t, []byte("file"))
			response := httptest.NewRecorder()

			uploadMediaHandler(service)(testGinContext(response, request))

			if response.Code != test.statusCode || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"`+test.code+`"`)) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if !bytes.Contains(logs.Bytes(), []byte(`"result":"`+test.result+`"`)) || !bytes.Contains(logs.Bytes(), []byte(`"error_code":"`+test.code+`"`)) {
				t.Fatalf("structured log = %s", logs.String())
			}
		})
	}
}

func TestUploadMediaRejectsOversizedMultipartBody(t *testing.T) {
	service := &mediaServiceStub{}
	request := multipartRequest(t, bytes.Repeat([]byte{'x'}, int(media.MaxFileSize+1)))
	response := httptest.NewRecorder()

	uploadMediaHandler(service)(testGinContext(response, request))

	if response.Code != http.StatusRequestEntityTooLarge || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"MEDIA_TOO_LARGE"`)) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.calls != 0 {
		t.Fatalf("Upload() calls = %d, want 0", service.calls)
	}
}

func multipartRequest(t *testing.T, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "picture.png")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/admin/media", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func testGinContext(response *httptest.ResponseRecorder, request *http.Request) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = request
	return ctx
}

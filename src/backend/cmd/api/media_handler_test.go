package main

import (
	"bytes"
	"context"
	"io"
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

func TestUploadMediaMapsValidationError(t *testing.T) {
	service := &mediaServiceStub{err: media.ErrEXIFForbidden}
	request := multipartRequest(t, []byte("file"))
	response := httptest.NewRecorder()

	uploadMediaHandler(service)(testGinContext(response, request))

	if response.Code != http.StatusUnprocessableEntity || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"MEDIA_EXIF_FORBIDDEN"`)) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
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

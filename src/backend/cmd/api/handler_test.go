package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hxy-blog/backend/internal/post"
)

type postListStub struct {
	result post.ListResult
	err    error
}

func (stub postListStub) ListPublished(_ context.Context, pagination post.Pagination) (post.ListResult, error) {
	stub.result.Page = pagination.Page
	stub.result.PageSize = pagination.PageSize
	return stub.result, stub.err
}

func TestHealth(t *testing.T) {
	// httptest 在内存中构造请求与响应，测试的是完整 Handler 行为而非内部函数细节。
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{}).ServeHTTP(response, request)

	// 状态码、内容类型和响应体共同组成对 Nginx、Compose 与 CD 的健康契约。
	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if body := strings.TrimSpace(response.Body.String()); body != `{"status":"ok"}` {
		t.Fatalf("body = %q", body)
	}
}

func TestUnknownRoute(t *testing.T) {
	// 未注册 API 必须返回标准 404，不能被错误地当成 SPA 页面或健康请求。
	request := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{}).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestListPublishedPosts(t *testing.T) {
	publishedAt := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	request := httptest.NewRequest(http.MethodGet, "/api/posts?page=2&pageSize=10", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{result: post.ListResult{
		Items: []post.Summary{{Slug: "hello-world", Title: "Hello World", PublishedAt: publishedAt}},
		Total: 11,
	}}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	var body post.ListResult
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].Slug != "hello-world" {
		t.Fatalf("items = %#v", body.Items)
	}
	if body.Page != 2 || body.PageSize != 10 || body.Total != 11 {
		t.Fatalf("pagination = page %d, pageSize %d, total %d", body.Page, body.PageSize, body.Total)
	}
}

func TestListPublishedPostsRejectsInvalidPagination(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/posts?page=0&pageSize=51", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{}).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if body := strings.TrimSpace(response.Body.String()); body != `{"code":"INVALID_PAGINATION","message":"page and pageSize must be positive integers and pageSize cannot exceed 50"}` {
		t.Fatalf("body = %q", body)
	}
}

func TestListPublishedPostsHidesDatabaseErrors(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{err: errors.New("SELECT failed with private database details")}).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if body := strings.TrimSpace(response.Body.String()); body != `{"code":"INTERNAL_ERROR","message":"服务暂时不可用"}` {
		t.Fatalf("body = %q", body)
	}
}

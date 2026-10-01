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

	"hxy-blog/backend/internal/auth"
	"hxy-blog/backend/internal/post"
)

type postListStub struct {
	result         post.ListResult
	err            error
	detail         post.Detail
	detailErr      error
	categories     []post.CategorySummary
	paginationSeen *post.Pagination
}

func (stub postListStub) ListPublished(_ context.Context, pagination post.Pagination) (post.ListResult, error) {
	if stub.paginationSeen != nil {
		*stub.paginationSeen = pagination
	}
	stub.result.Page = pagination.Page
	stub.result.PageSize = pagination.PageSize
	return stub.result, stub.err
}

func (stub postListStub) GetPublishedBySlug(_ context.Context, _ string) (post.Detail, error) {
	return stub.detail, stub.detailErr
}

func (stub postListStub) ListPublishedCategories(_ context.Context) ([]post.CategorySummary, error) {
	return stub.categories, stub.err
}

type postAdminStub struct {
	postListStub
	created     post.AdminDetail
	updated     post.AdminDetail
	published   post.AdminDetail
	createCalls int
	updateCalls int
	deleteCalls int
	deleteErr   error
}

func (stub *postAdminStub) ListAdmin(_ context.Context) ([]post.AdminSummary, error) {
	return nil, nil
}

func (stub *postAdminStub) GetAdmin(_ context.Context, _ uint64) (post.AdminDetail, error) {
	return post.AdminDetail{}, post.ErrNotFound
}

func (stub *postAdminStub) CreateDraft(_ context.Context, _ post.DraftInput) (post.AdminDetail, error) {
	stub.createCalls++
	return stub.created, nil
}

func (stub *postAdminStub) Update(_ context.Context, _ uint64, _ post.DraftInput) (post.AdminDetail, error) {
	stub.updateCalls++
	return stub.updated, nil
}

func (stub *postAdminStub) Publish(_ context.Context, _ uint64) (post.AdminDetail, error) {
	return stub.published, nil
}

func (stub *postAdminStub) Delete(_ context.Context, _ uint64) error {
	stub.deleteCalls++
	return stub.deleteErr
}

func TestHealth(t *testing.T) {
	// httptest 在内存中构造请求与响应，测试的是完整 Handler 行为而非内部函数细节。
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{}, nil).ServeHTTP(response, request)

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

	newHandler(postListStub{}, nil).ServeHTTP(response, request)

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
	}}, nil).ServeHTTP(response, request)

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

func TestListPublishedPostsFiltersByCategory(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/posts?category=go&page=1&pageSize=10", nil)
	response := httptest.NewRecorder()
	var pagination post.Pagination

	newHandler(postListStub{paginationSeen: &pagination}, nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	if pagination.CategorySlug != "go" {
		t.Fatalf("category slug = %q, want go", pagination.CategorySlug)
	}
}

func TestListPublishedPostsRejectsInvalidCategory(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/posts?category=Go%20Lang", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{}, nil).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if body := strings.TrimSpace(response.Body.String()); body != `{"code":"INVALID_CATEGORY","message":"分类 slug 无效"}` {
		t.Fatalf("body = %q", body)
	}
}

func TestListPublishedCategories(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{categories: []post.CategorySummary{{
		Slug: "go", Name: "Go", PostCount: 2,
	}}}, nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(response.Body.String()); body != `{"items":[{"slug":"go","name":"Go","postCount":2}]}` {
		t.Fatalf("body = %q", body)
	}
}

func TestListPublishedPostsRejectsInvalidPagination(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/posts?page=0&pageSize=51", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{}, nil).ServeHTTP(response, request)

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

	newHandler(postListStub{err: errors.New("SELECT failed with private database details")}, nil).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if body := strings.TrimSpace(response.Body.String()); body != `{"code":"INTERNAL_ERROR","message":"服务暂时不可用"}` {
		t.Fatalf("body = %q", body)
	}
}

func TestGetPublishedPost(t *testing.T) {
	publishedAt := time.Date(2026, time.October, 1, 2, 3, 4, 0, time.UTC)
	request := httptest.NewRequest(http.MethodGet, "/api/posts/hello-world", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{detail: post.Detail{
		Slug:            "hello-world",
		Title:           "Hello World",
		ContentMarkdown: "# 正文",
		PublishedAt:     publishedAt,
	}}, nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	var body post.Detail
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Slug != "hello-world" || body.ContentMarkdown != "# 正文" {
		t.Fatalf("body = %#v", body)
	}
}

func TestGetPublishedPostReturnsNotFoundForHiddenPost(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/posts/draft-post", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{detailErr: post.ErrNotFound}, nil).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusNotFound)
	}
	if body := strings.TrimSpace(response.Body.String()); body != `{"code":"POST_NOT_FOUND","message":"文章不存在"}` {
		t.Fatalf("body = %q", body)
	}
}

func TestGetPublishedPostHidesDatabaseErrors(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/posts/hello-world", nil)
	response := httptest.NewRecorder()

	newHandler(postListStub{detailErr: errors.New("SELECT failed with private database details")}, nil).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if body := strings.TrimSpace(response.Body.String()); body != `{"code":"INTERNAL_ERROR","message":"服务暂时不可用"}` {
		t.Fatalf("body = %q", body)
	}
}

func TestCreateDraftRequiresAccessToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/admin/posts", strings.NewReader(`{"slug":"new-post","title":"新文章","contentMarkdown":"正文"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	posts := &postAdminStub{}

	newHandler(posts, testAuthHTTP(accessTokenVerifierStub{err: errors.New("missing token")})).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if posts.createCalls != 0 {
		t.Fatalf("CreateDraft() calls = %d, want 0", posts.createCalls)
	}
}

func TestCreateDraftWithAccessToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/admin/posts", strings.NewReader(`{"slug":"new-post","title":"新文章","contentMarkdown":"# 正文"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	posts := &postAdminStub{created: post.AdminDetail{
		ID: 7, Slug: "new-post", Title: "新文章", ContentMarkdown: "# 正文", Status: post.StatusDraft,
	}}

	newHandler(posts, testAuthHTTP(accessTokenVerifierStub{claims: auth.AccessClaims{AdminID: 1, Username: "admin"}})).ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d; body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if posts.createCalls != 1 {
		t.Fatalf("CreateDraft() calls = %d, want 1", posts.createCalls)
	}
}

func TestPublishDraftWithAccessToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/admin/posts/7/publish", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	publishedAt := time.Date(2026, time.October, 1, 3, 0, 0, 0, time.UTC)
	posts := &postAdminStub{published: post.AdminDetail{
		ID: 7, Slug: "new-post", Title: "新文章", ContentMarkdown: "# 正文",
		Status: post.StatusPublished, PublishedAt: &publishedAt,
	}}

	newHandler(posts, testAuthHTTP(accessTokenVerifierStub{claims: auth.AccessClaims{AdminID: 1, Username: "admin"}})).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestUpdatePublishedPostWithAccessToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/api/admin/posts/7", strings.NewReader(`{"slug":"stable-slug","title":"修改后的标题","contentMarkdown":"# 新正文"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	posts := &postAdminStub{updated: post.AdminDetail{
		ID: 7, Slug: "stable-slug", Title: "修改后的标题", ContentMarkdown: "# 新正文",
		Status: post.StatusPublished,
	}}

	newHandler(posts, testAuthHTTP(accessTokenVerifierStub{claims: auth.AccessClaims{AdminID: 1, Username: "admin"}})).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if posts.updateCalls != 1 {
		t.Fatalf("Update() calls = %d, want 1", posts.updateCalls)
	}
}

func TestDeletePostWithAccessToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodDelete, "/api/admin/posts/7", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	posts := &postAdminStub{}

	newHandler(posts, testAuthHTTP(accessTokenVerifierStub{claims: auth.AccessClaims{AdminID: 1, Username: "admin"}})).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d; body = %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if posts.deleteCalls != 1 {
		t.Fatalf("Delete() calls = %d, want 1", posts.deleteCalls)
	}
}

func TestDeletePostRequiresAccessToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodDelete, "/api/admin/posts/7", nil)
	response := httptest.NewRecorder()
	posts := &postAdminStub{}

	newHandler(posts, testAuthHTTP(accessTokenVerifierStub{err: errors.New("missing token")})).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if posts.deleteCalls != 0 {
		t.Fatalf("Delete() calls = %d, want 0", posts.deleteCalls)
	}
}

func TestDeletePostReturnsNotFound(t *testing.T) {
	request := httptest.NewRequest(http.MethodDelete, "/api/admin/posts/99", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	posts := &postAdminStub{deleteErr: post.ErrNotFound}

	newHandler(posts, testAuthHTTP(accessTokenVerifierStub{claims: auth.AccessClaims{AdminID: 1, Username: "admin"}})).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d; body = %s", response.Code, http.StatusNotFound, response.Body.String())
	}
}

func testAuthHTTP(tokens accessTokenVerifier) *authHTTPDependencies {
	return &authHTTPDependencies{
		tokens: tokens,
		config: authHandlerConfig{allowedOrigins: map[string]struct{}{}},
	}
}

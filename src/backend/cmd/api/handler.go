package main

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"hxy-blog/backend/internal/post"
)

type postListService interface {
	ListPublished(ctx context.Context, pagination post.Pagination) (post.ListResult, error)
}

// newHandler 负责集中注册 HTTP 路由。
//
// 返回 http.Handler 而不是在这里启动服务器，测试就可以用 httptest 直接调用路由，
// 不需要占用真实端口。Gin 只停留在协议边界，业务服务不依赖 Gin 类型。
func newHandler(posts postListService) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/api/health", healthHandler)
	router.GET("/api/posts", listPublishedPostsHandler(posts))
	return router
}

// healthHandler 只表达“API 进程可以处理 HTTP 请求”，不查询数据库或外部服务。
// 深层依赖故障应由独立的 readiness/业务监控表达，避免短暂依赖抖动导致容器重启。
func healthHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func listPublishedPostsHandler(posts postListService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		pagination, ok := parsePagination(ctx)
		if !ok {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"code":    "INVALID_PAGINATION",
				"message": "page and pageSize must be positive integers and pageSize cannot exceed 50",
			})
			return
		}

		result, err := posts.ListPublished(ctx.Request.Context(), pagination)
		if err != nil {
			slog.Error("list published posts failed", "error", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "服务暂时不可用",
			})
			return
		}
		ctx.JSON(http.StatusOK, result)
	}
}

func parsePagination(ctx *gin.Context) (post.Pagination, bool) {
	page, ok := positiveQueryInt(ctx, "page", post.DefaultPage)
	if !ok {
		return post.Pagination{}, false
	}
	pageSize, ok := positiveQueryInt(ctx, "pageSize", post.DefaultPageSize)
	if !ok || pageSize > post.MaxPageSize {
		return post.Pagination{}, false
	}
	return post.Pagination{Page: page, PageSize: pageSize}, true
}

func positiveQueryInt(ctx *gin.Context, key string, fallback int) (int, bool) {
	value := ctx.Query(key)
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
}

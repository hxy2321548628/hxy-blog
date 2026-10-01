package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"hxy-blog/backend/internal/post"
)

const adminPostBodyLimit = 2 << 20

type adminPostService interface {
	ListAdmin(ctx context.Context) ([]post.AdminSummary, error)
	GetAdmin(ctx context.Context, id uint64) (post.AdminDetail, error)
	CreateDraft(ctx context.Context, input post.DraftInput) (post.AdminDetail, error)
	Update(ctx context.Context, id uint64, input post.DraftInput) (post.AdminDetail, error)
	Publish(ctx context.Context, id uint64) (post.AdminDetail, error)
	Delete(ctx context.Context, id uint64) error
}

func registerAdminPostRoutes(router *gin.Engine, posts adminPostService, tokens accessTokenVerifier) {
	admin := router.Group("/api/admin")
	admin.Use(requireAccessToken(tokens))
	admin.GET("/posts", listAdminPostsHandler(posts))
	admin.GET("/posts/:id", getAdminPostHandler(posts))
	admin.POST("/posts", createDraftHandler(posts))
	admin.PUT("/posts/:id", updatePostHandler(posts))
	admin.DELETE("/posts/:id", deletePostHandler(posts))
	admin.POST("/posts/:id/publish", publishDraftHandler(posts))
}

func listAdminPostsHandler(posts adminPostService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		items, err := posts.ListAdmin(ctx.Request.Context())
		if err != nil {
			respondAdminPostError(ctx, err, "list administrator posts failed")
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"items": items})
	}
}

func getAdminPostHandler(posts adminPostService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id, ok := adminPostID(ctx)
		if !ok {
			return
		}
		detail, err := posts.GetAdmin(ctx.Request.Context(), id)
		if err != nil {
			respondAdminPostError(ctx, err, "get administrator post failed")
			return
		}
		ctx.JSON(http.StatusOK, detail)
	}
}

func createDraftHandler(posts adminPostService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		input, ok := decodeDraftInput(ctx)
		if !ok {
			return
		}
		detail, err := posts.CreateDraft(ctx.Request.Context(), input)
		if err != nil {
			respondAdminPostError(ctx, err, "create draft post failed")
			return
		}
		ctx.JSON(http.StatusCreated, detail)
	}
}

func updatePostHandler(posts adminPostService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id, ok := adminPostID(ctx)
		if !ok {
			return
		}
		input, ok := decodeDraftInput(ctx)
		if !ok {
			return
		}
		detail, err := posts.Update(ctx.Request.Context(), id, input)
		if err != nil {
			respondAdminPostError(ctx, err, "update post failed")
			return
		}
		ctx.JSON(http.StatusOK, detail)
	}
}

func deletePostHandler(posts adminPostService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id, ok := adminPostID(ctx)
		if !ok {
			return
		}
		if err := posts.Delete(ctx.Request.Context(), id); err != nil {
			respondAdminPostError(ctx, err, "delete post failed")
			return
		}
		ctx.Status(http.StatusNoContent)
	}
}

func publishDraftHandler(posts adminPostService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id, ok := adminPostID(ctx)
		if !ok {
			return
		}
		detail, err := posts.Publish(ctx.Request.Context(), id)
		if err != nil {
			respondAdminPostError(ctx, err, "publish draft post failed")
			return
		}
		ctx.JSON(http.StatusOK, detail)
	}
}

func decodeDraftInput(ctx *gin.Context) (post.DraftInput, bool) {
	var input post.DraftInput
	if err := decodeJSONBodyWithLimit(ctx, &input, adminPostBodyLimit); err != nil {
		respondError(ctx, http.StatusBadRequest, "INVALID_POST", "文章格式无效")
		return post.DraftInput{}, false
	}
	return input, true
}

func adminPostID(ctx *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondError(ctx, http.StatusBadRequest, "INVALID_POST_ID", "文章 ID 无效")
		return 0, false
	}
	return id, true
}

func respondAdminPostError(ctx *gin.Context, err error, logMessage string) {
	switch {
	case errors.Is(err, post.ErrInvalidInput):
		respondError(ctx, http.StatusBadRequest, "INVALID_POST", "slug、标题或正文不符合要求")
	case errors.Is(err, post.ErrSlugConflict):
		respondError(ctx, http.StatusConflict, "POST_SLUG_CONFLICT", "该 slug 已被使用")
	case errors.Is(err, post.ErrNotDraft):
		respondError(ctx, http.StatusConflict, "POST_NOT_DRAFT", "文章已发布，不能再次发布")
	case errors.Is(err, post.ErrNotFound):
		respondError(ctx, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")
	default:
		slog.Error(logMessage, "error", err)
		respondError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用")
	}
}

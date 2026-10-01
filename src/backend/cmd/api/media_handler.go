package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"hxy-blog/backend/internal/media"
)

const mediaRequestOverhead = 1 << 20

type mediaUploadService interface {
	Upload(ctx context.Context, file io.ReadSeeker) (media.Asset, error)
}

func registerMediaRoute(router *gin.Engine, service mediaUploadService, tokens accessTokenVerifier) {
	router.POST(
		"/api/admin/media",
		requireAccessToken(tokens),
		uploadMediaHandler(service),
	)
}

func uploadMediaHandler(service mediaUploadService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Request.Body = http.MaxBytesReader(
			ctx.Writer,
			ctx.Request.Body,
			media.MaxFileSize+mediaRequestOverhead,
		)
		if err := ctx.Request.ParseMultipartForm(1 << 20); err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				respondError(ctx, http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "图片不能超过 10 MiB")
				return
			}
			respondError(ctx, http.StatusBadRequest, "INVALID_MEDIA_REQUEST", "请选择一张图片")
			return
		}
		if ctx.Request.MultipartForm != nil {
			defer ctx.Request.MultipartForm.RemoveAll()
		}
		fileHeader, err := ctx.FormFile("file")
		if err != nil {
			respondError(ctx, http.StatusBadRequest, "INVALID_MEDIA_REQUEST", "请选择一张图片")
			return
		}
		if fileHeader.Size > media.MaxFileSize {
			respondError(ctx, http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "图片不能超过 10 MiB")
			return
		}
		file, err := fileHeader.Open()
		if err != nil {
			respondError(ctx, http.StatusUnprocessableEntity, "MEDIA_INVALID", "图片无法读取")
			return
		}
		defer file.Close()

		uploadContext, cancel := context.WithTimeout(ctx.Request.Context(), 35*time.Second)
		defer cancel()
		asset, err := service.Upload(uploadContext, file)
		if err != nil {
			respondMediaError(ctx, err)
			return
		}
		ctx.JSON(http.StatusCreated, asset)
	}
}

func respondMediaError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, media.ErrTooLarge):
		respondError(ctx, http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "图片不能超过 10 MiB")
	case errors.Is(err, media.ErrUnsupportedFormat):
		respondError(ctx, http.StatusUnsupportedMediaType, "MEDIA_UNSUPPORTED", "仅支持 JPG、PNG 和 WebP 图片")
	case errors.Is(err, media.ErrInvalidDimensions):
		respondError(ctx, http.StatusUnprocessableEntity, "MEDIA_DIMENSIONS_INVALID", "图片尺寸或总像素超出限制")
	case errors.Is(err, media.ErrEXIFForbidden):
		respondError(ctx, http.StatusUnprocessableEntity, "MEDIA_EXIF_FORBIDDEN", "图片包含 EXIF 元数据，请先在本地移除")
	case errors.Is(err, media.ErrInvalidImage):
		respondError(ctx, http.StatusUnprocessableEntity, "MEDIA_INVALID", "图片已损坏或无法解析")
	case errors.Is(err, media.ErrBusy):
		respondError(ctx, http.StatusTooManyRequests, "MEDIA_UPLOAD_BUSY", "已有图片正在上传，请稍后重试")
	case errors.Is(err, media.ErrStorageUnavailable):
		respondError(ctx, http.StatusServiceUnavailable, "MEDIA_STORAGE_UNAVAILABLE", "图片存储暂时不可用，请稍后重试")
	default:
		slog.Error("upload media failed", "error", err)
		respondError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用")
	}
}

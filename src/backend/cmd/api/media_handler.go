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
		startedAt := time.Now()
		result := "invalid_request"
		errorCode := "INVALID_MEDIA_REQUEST"
		statusCode := http.StatusBadRequest
		var sizeBytes int64
		var mediaID uint64
		defer func() {
			// 每个请求只记录一个结果事件，日志平台可按 result/error_code 聚合基础指标。
			slog.Info(
				"media upload request completed",
				"result", result,
				"error_code", errorCode,
				"status_code", statusCode,
				"media_id", mediaID,
				"size_bytes", sizeBytes,
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		}()

		ctx.Request.Body = http.MaxBytesReader(
			ctx.Writer,
			ctx.Request.Body,
			media.MaxFileSize+mediaRequestOverhead,
		)
		if err := ctx.Request.ParseMultipartForm(1 << 20); err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				result = "too_large"
				errorCode = "MEDIA_TOO_LARGE"
				statusCode = http.StatusRequestEntityTooLarge
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
		sizeBytes = fileHeader.Size
		if fileHeader.Size > media.MaxFileSize {
			result = "too_large"
			errorCode = "MEDIA_TOO_LARGE"
			statusCode = http.StatusRequestEntityTooLarge
			respondError(ctx, http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "图片不能超过 10 MiB")
			return
		}
		file, err := fileHeader.Open()
		if err != nil {
			result = "validation_failed"
			errorCode = "MEDIA_INVALID"
			statusCode = http.StatusUnprocessableEntity
			respondError(ctx, http.StatusUnprocessableEntity, "MEDIA_INVALID", "图片无法读取")
			return
		}
		defer file.Close()

		uploadContext, cancel := context.WithTimeout(ctx.Request.Context(), 35*time.Second)
		defer cancel()
		asset, err := service.Upload(uploadContext, file)
		if err != nil {
			result, errorCode, statusCode = respondMediaError(ctx, err)
			return
		}
		result = "success"
		errorCode = ""
		statusCode = http.StatusCreated
		mediaID = asset.ID
		sizeBytes = asset.SizeBytes
		ctx.JSON(http.StatusCreated, asset)
	}
}

func respondMediaError(ctx *gin.Context, err error) (string, string, int) {
	switch {
	case errors.Is(err, media.ErrTooLarge):
		respondError(ctx, http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "图片不能超过 10 MiB")
		return "too_large", "MEDIA_TOO_LARGE", http.StatusRequestEntityTooLarge
	case errors.Is(err, media.ErrUnsupportedFormat):
		respondError(ctx, http.StatusUnsupportedMediaType, "MEDIA_UNSUPPORTED", "仅支持 JPG、PNG、WebP、BMP 和 GIF 图片")
		return "validation_failed", "MEDIA_UNSUPPORTED", http.StatusUnsupportedMediaType
	case errors.Is(err, media.ErrInvalidDimensions):
		respondError(ctx, http.StatusUnprocessableEntity, "MEDIA_DIMENSIONS_INVALID", "图片尺寸或总像素超出限制")
		return "validation_failed", "MEDIA_DIMENSIONS_INVALID", http.StatusUnprocessableEntity
	case errors.Is(err, media.ErrEXIFForbidden):
		respondError(ctx, http.StatusUnprocessableEntity, "MEDIA_EXIF_FORBIDDEN", "图片包含 EXIF 元数据，请先在本地移除")
		return "validation_failed", "MEDIA_EXIF_FORBIDDEN", http.StatusUnprocessableEntity
	case errors.Is(err, media.ErrInvalidImage):
		respondError(ctx, http.StatusUnprocessableEntity, "MEDIA_INVALID", "图片已损坏或无法解析")
		return "validation_failed", "MEDIA_INVALID", http.StatusUnprocessableEntity
	case errors.Is(err, media.ErrBusy):
		respondError(ctx, http.StatusTooManyRequests, "MEDIA_UPLOAD_BUSY", "已有图片正在上传，请稍后重试")
		return "busy", "MEDIA_UPLOAD_BUSY", http.StatusTooManyRequests
	case errors.Is(err, media.ErrStorageUnavailable):
		respondError(ctx, http.StatusServiceUnavailable, "MEDIA_STORAGE_UNAVAILABLE", "图片存储暂时不可用，请稍后重试")
		return "storage_failed", "MEDIA_STORAGE_UNAVAILABLE", http.StatusServiceUnavailable
	default:
		slog.Error("upload media failed", "error", err)
		respondError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用")
		return "internal_error", "INTERNAL_ERROR", http.StatusInternalServerError
	}
}

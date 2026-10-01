package main

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hxy-blog/backend/internal/auth"
	dbconfig "hxy-blog/backend/internal/database"
	"hxy-blog/backend/internal/post"
)

func main() {
	gormDatabase, sqlDatabase, err := dbconfig.OpenApplication(os.Getenv)
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer sqlDatabase.Close()

	postService := post.NewService(post.NewRepository(gormDatabase))
	mediaService, err := buildMediaService(os.Getenv, gormDatabase)
	if err != nil {
		slog.Error("media configuration failed", "error", err)
		os.Exit(1)
	}
	authConfig, err := loadAuthConfig(os.Getenv)
	if err != nil {
		slog.Error("authentication configuration failed", "error", err)
		os.Exit(1)
	}
	tokenManager, err := auth.NewAccessTokenManager(authConfig.currentKey, authConfig.previousKey)
	if err != nil {
		slog.Error("access token initialization failed", "error", err)
		os.Exit(1)
	}
	authService, err := auth.NewService(
		context.Background(),
		auth.NewRepository(gormDatabase),
		auth.NewPasswordHasher(rand.Reader),
		tokenManager,
		rand.Reader,
	)
	if err != nil {
		slog.Error("authentication service initialization failed", "error", err)
		os.Exit(1)
	}
	authHTTP := &authHTTPDependencies{
		sessions: authService,
		tokens:   tokenManager,
		config:   authConfig.handler,
		media:    mediaService,
	}
	// 监听地址通过环境变量注入，便于同一个二进制在本机和容器中运行。
	addr := envOrDefault("HTTP_ADDR", ":8080")
	server := &http.Server{
		Addr:    addr,
		Handler: newHandler(postService, authHTTP),
		// 限制请求头读取时间，降低慢速连接长期占用服务器资源的风险。
		ReadHeaderTimeout: 5 * time.Second,
		// 空闲 Keep-Alive 连接最终会被回收，避免 2C2G 单机积累无效连接。
		IdleTimeout: 60 * time.Second,
	}

	// NotifyContext 把 Ctrl+C 与容器停止时的 SIGTERM 统一转换成 Context 取消信号。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		// Shutdown 会停止接受新请求并等待在途请求结束；10 秒后强制返回，
		// 该时间小于 Compose 的 stop_grace_period，给容器留出正常退出窗口。
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown failed", "error", err)
		}
	}()

	// slog 输出结构化键值，后续容器日志可以按 addr/error 字段检索。
	slog.Info("api server started", "addr", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("api server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

// envOrDefault 只对真正的空值使用默认值；生产所需的敏感配置不会调用该函数，
// 从而避免因为缺少密码等配置而静默回退到不安全值。
func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

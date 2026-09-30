package main

import (
	"encoding/json"
	"net/http"
)

// newHandler 负责集中注册 HTTP 路由。
//
// 返回 http.Handler 而不是在这里启动服务器，测试就可以用 httptest 直接调用路由，
// 不需要占用真实端口。Go 1.22+ 的 "GET /path" 模式同时限制方法和路径。
func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", healthHandler)
	return mux
}

// healthHandler 只表达“API 进程可以处理 HTTP 请求”，不查询数据库或外部服务。
// 深层依赖故障应由独立的 readiness/业务监控表达，避免短暂依赖抖动导致容器重启。
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	// 必须在 WriteHeader/写响应体之前设置 Header，否则 Go 会先发送默认响应头。
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// map 会被编码为紧凑 JSON；健康响应很小，写入失败时连接通常已断开，无需再响应错误。
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

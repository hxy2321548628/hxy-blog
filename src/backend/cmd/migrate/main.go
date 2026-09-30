package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/pressly/goose/v3"

	dbconfig "hxy-blog/backend/internal/database"
	"hxy-blog/backend/internal/migrations"
)

const migrationTimeout = 5 * time.Minute

func main() {
	// 把命令行和环境读取作为参数传给 run/databaseConfig，使核心校验可在测试中替换。
	if err := run(os.Args[1:], os.Getenv); err != nil {
		// 错误日志不得打印数据库密码或完整 DSN。
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string) error {
	// 只开放 up/down，明确拒绝 reset、redo 等可能在生产环境破坏数据的 Goose 命令。
	if len(args) != 1 || (args[0] != "up" && args[0] != "down") {
		return errors.New("command must be exactly one of: up, down")
	}

	config, err := dbconfig.MySQLConfig(getenv)
	if err != nil {
		return err
	}

	// sql.Open 只创建连接池句柄；真正的网络连通性由后面的 PingContext 验证。
	database, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	// 整次迁移共享同一个超时，防止锁等待让部署无限挂起。
	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	// Goose 只读取编译进二进制的 SQL，生产服务器不读取工作区文件。
	goose.SetBaseFS(migrations.Files)
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	switch args[0] {
	case "up":
		// Up 会按版本顺序执行尚未应用的迁移，重复执行时应保持幂等。
		if err := goose.UpContext(ctx, database, "."); err != nil {
			return fmt.Errorf("goose up: %w", err)
		}
	case "down":
		// Down 只回退最近一个版本，降低误操作的破坏范围。
		if err := goose.DownContext(ctx, database, "."); err != nil {
			return fmt.Errorf("goose down: %w", err)
		}
	}

	return nil
}

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	"hxy-blog/backend/internal/migrations"
)

const migrationTimeout = 5 * time.Minute

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		// 错误日志不得打印数据库密码或完整 DSN。
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string) error {
	if len(args) != 1 || (args[0] != "up" && args[0] != "down") {
		return errors.New("command must be exactly one of: up, down")
	}

	config, err := databaseConfig(getenv)
	if err != nil {
		return err
	}

	database, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

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
		if err := goose.UpContext(ctx, database, "."); err != nil {
			return fmt.Errorf("goose up: %w", err)
		}
	case "down":
		if err := goose.DownContext(ctx, database, "."); err != nil {
			return fmt.Errorf("goose down: %w", err)
		}
	}

	return nil
}

func databaseConfig(getenv func(string) string) (*mysql.Config, error) {
	required := func(key string) (string, error) {
		value := getenv(key)
		if value == "" {
			return "", fmt.Errorf("missing required environment variable %s", key)
		}
		return value, nil
	}

	host, err := required("MYSQL_HOST")
	if err != nil {
		return nil, err
	}
	port, err := required("MYSQL_PORT")
	if err != nil {
		return nil, err
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, errors.New("MYSQL_PORT must be an integer between 1 and 65535")
	}
	name, err := required("MYSQL_DATABASE")
	if err != nil {
		return nil, err
	}
	user, err := required("MYSQL_USER")
	if err != nil {
		return nil, err
	}
	password, err := required("MYSQL_PASSWORD")
	if err != nil {
		return nil, err
	}

	config := mysql.NewConfig()
	config.User = user
	config.Passwd = password
	config.Net = "tcp"
	config.Addr = net.JoinHostPort(host, port)
	config.DBName = name
	config.Collation = "utf8mb4_0900_ai_ci"
	config.ParseTime = true
	config.Timeout = 10 * time.Second
	config.ReadTimeout = 30 * time.Second
	config.WriteTimeout = 30 * time.Second
	return config, nil
}

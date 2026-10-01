package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"hxy-blog/backend/internal/auth"
	dbconfig "hxy-blog/backend/internal/database"
)

const (
	adminCommandTimeout   = 30 * time.Second
	maxAdminUsernameBytes = 64
)

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		// 错误只描述变量名和操作阶段，不打印初始密码、哈希或数据库连接串。
		slog.Error("administrator command failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string) error {
	if len(args) != 1 || args[0] != "create" {
		return errors.New("command must be exactly: create")
	}
	username, password, err := adminCredentials(getenv)
	if err != nil {
		return err
	}

	database, sqlDatabase, err := dbconfig.OpenApplication(getenv)
	if err != nil {
		return err
	}
	defer sqlDatabase.Close()

	ctx, cancel := context.WithTimeout(context.Background(), adminCommandTimeout)
	defer cancel()
	passwordHash, err := auth.NewPasswordHasher(rand.Reader).Hash(ctx, password)
	if err != nil {
		return fmt.Errorf("hash initial administrator password: %w", err)
	}
	if err := auth.NewRepository(database).CreateAdmin(ctx, username, passwordHash, time.Now().UTC()); err != nil {
		if errors.Is(err, auth.ErrAdminAlreadyExists) {
			return errors.New("administrator already exists; initial creation is one-time only")
		}
		return err
	}
	slog.Info("administrator created", "username", username)
	return nil
}

func adminCredentials(getenv func(string) string) (string, string, error) {
	username := strings.TrimSpace(getenv("ADMIN_USERNAME"))
	if username == "" || len(username) > maxAdminUsernameBytes {
		return "", "", fmt.Errorf("ADMIN_USERNAME must contain between 1 and %d bytes", maxAdminUsernameBytes)
	}
	password := getenv("ADMIN_INITIAL_PASSWORD")
	if len(password) == 0 || len(password) > auth.MaxPasswordBytes {
		return "", "", fmt.Errorf("ADMIN_INITIAL_PASSWORD must contain between 1 and %d bytes", auth.MaxPasswordBytes)
	}
	return username, password, nil
}

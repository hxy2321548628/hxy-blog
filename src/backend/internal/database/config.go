// Package database 集中管理 MySQL 连接参数，保证 API 与迁移器解释同一组环境变量。
package database

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

// MySQLConfig 验证环境输入并返回由驱动安全格式化的连接配置。
func MySQLConfig(getenv func(string) string) (*mysql.Config, error) {
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
	// DATETIME 没有时区信息，统一按 UTC 解析才不会随容器时区改变 API 时间。
	config.Loc = time.UTC
	config.Timeout = 10 * time.Second
	config.ReadTimeout = 30 * time.Second
	config.WriteTimeout = 30 * time.Second
	return config, nil
}

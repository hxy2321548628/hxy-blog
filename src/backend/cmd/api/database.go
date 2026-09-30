package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	dbconfig "hxy-blog/backend/internal/database"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openApplicationDatabase(getenv func(string) string) (*gorm.DB, *sql.DB, error) {
	config, err := dbconfig.MySQLConfig(getenv)
	if err != nil {
		return nil, nil, err
	}
	sqlDatabase, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	// 2C2G 单机不需要大连接池；显式上限防止应用抢占 MySQL 全部连接。
	sqlDatabase.SetMaxOpenConns(10)
	sqlDatabase.SetMaxIdleConns(5)
	sqlDatabase.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDatabase.PingContext(ctx); err != nil {
		sqlDatabase.Close()
		return nil, nil, fmt.Errorf("ping database: %w", err)
	}

	// SQL 日志不展开参数，避免未来的文章正文或凭据出现在容器日志中。
	gormLogger := logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
		SlowThreshold:        500 * time.Millisecond,
		LogLevel:             logger.Warn,
		ParameterizedQueries: true,
		Colorful:             false,
	})
	gormDatabase, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: sqlDatabase}), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		sqlDatabase.Close()
		return nil, nil, fmt.Errorf("initialize gorm: %w", err)
	}
	return gormDatabase, sqlDatabase, nil
}

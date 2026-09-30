package main

import (
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestDatabaseConfigPreservesCredentials(t *testing.T) {
	// 特殊字符用来证明 DSN 由驱动安全编码，而不是通过字符串拼接产生歧义。
	values := map[string]string{
		"MYSQL_HOST":     "mysql",
		"MYSQL_PORT":     "3306",
		"MYSQL_DATABASE": "hxy_blog",
		"MYSQL_USER":     "blog-user",
		"MYSQL_PASSWORD": "p@ss:/word",
	}

	config, err := databaseConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("databaseConfig() error = %v", err)
	}

	// 格式化后再解析，验证一次完整的往返转换没有改变凭据。
	parsed, err := mysql.ParseDSN(config.FormatDSN())
	if err != nil {
		t.Fatalf("ParseDSN() error = %v", err)
	}
	if parsed.User != values["MYSQL_USER"] || parsed.Passwd != values["MYSQL_PASSWORD"] {
		t.Fatal("database credentials changed during DSN formatting")
	}
	if config.Addr != "mysql:3306" || config.DBName != "hxy_blog" {
		t.Fatal("database address or name was not configured")
	}
}

func TestDatabaseConfigRejectsInvalidPort(t *testing.T) {
	// 输入边界在尝试连接前失败，错误稳定且不会产生无意义的网络请求。
	values := map[string]string{
		"MYSQL_HOST":     "mysql",
		"MYSQL_PORT":     "invalid",
		"MYSQL_DATABASE": "hxy_blog",
		"MYSQL_USER":     "hxy_blog",
		"MYSQL_PASSWORD": "secret",
	}

	_, err := databaseConfig(func(key string) string { return values[key] })
	if err == nil || err.Error() != "MYSQL_PORT must be an integer between 1 and 65535" {
		t.Fatalf("databaseConfig() error = %v", err)
	}
}

func TestRunRejectsUnsafeCommand(t *testing.T) {
	// reset 不在生产迁移器的允许列表中，避免通过部署入口清空迁移状态。
	err := run([]string{"reset"}, func(string) string { return "unused" })
	if err == nil || err.Error() != "command must be exactly one of: up, down" {
		t.Fatalf("run() error = %v", err)
	}
}

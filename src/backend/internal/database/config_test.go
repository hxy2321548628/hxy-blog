package database

import (
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestMySQLConfigPreservesCredentials(t *testing.T) {
	// 特殊字符用来证明 DSN 由驱动安全编码，而不是手工拼接。
	values := map[string]string{
		"MYSQL_HOST":     "mysql",
		"MYSQL_PORT":     "3306",
		"MYSQL_DATABASE": "hxy_blog",
		"MYSQL_USER":     "blog-user",
		"MYSQL_PASSWORD": "p@ss:/word",
	}

	config, err := MySQLConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("MySQLConfig() error = %v", err)
	}
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

func TestMySQLConfigRejectsInvalidPort(t *testing.T) {
	values := map[string]string{
		"MYSQL_HOST":     "mysql",
		"MYSQL_PORT":     "invalid",
		"MYSQL_DATABASE": "hxy_blog",
		"MYSQL_USER":     "hxy_blog",
		"MYSQL_PASSWORD": "secret",
	}

	_, err := MySQLConfig(func(key string) string { return values[key] })
	if err == nil || err.Error() != "MYSQL_PORT must be an integer between 1 and 65535" {
		t.Fatalf("MySQLConfig() error = %v", err)
	}
}

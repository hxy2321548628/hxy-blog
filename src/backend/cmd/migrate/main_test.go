package main

import (
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestDatabaseConfigPreservesCredentials(t *testing.T) {
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
	err := run([]string{"reset"}, func(string) string { return "unused" })
	if err == nil || err.Error() != "command must be exactly one of: up, down" {
		t.Fatalf("run() error = %v", err)
	}
}

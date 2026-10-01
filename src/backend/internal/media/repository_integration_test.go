//go:build integration

package media

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestRepositoryCreatesMediaMetadata(t *testing.T) {
	database := openIntegrationDatabase(t)
	transaction := database.Begin()
	if transaction.Error != nil {
		t.Fatalf("begin transaction: %v", transaction.Error)
	}
	t.Cleanup(func() { transaction.Rollback() })
	if err := transaction.Exec("DELETE FROM media_assets").Error; err != nil {
		t.Fatalf("clear media assets: %v", err)
	}

	checksum := [32]byte{1, 2, 3}
	created, err := NewRepository(transaction).Create(context.Background(), Asset{
		ObjectKey:      "media/2026/10/integration.png",
		MIMEType:       "image/png",
		SizeBytes:      1024,
		Width:          800,
		Height:         600,
		ChecksumSHA256: checksum,
		Status:         "ready",
		CreatedAt:      time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == 0 {
		t.Fatal("Create() did not return an ID")
	}

	var stored assetRecord
	if err := transaction.Table("media_assets").Where("id = ?", created.ID).Take(&stored).Error; err != nil {
		t.Fatalf("read media metadata: %v", err)
	}
	if stored.ObjectKey != created.ObjectKey || stored.MIMEType != "image/png" || string(stored.ChecksumSHA256) != string(checksum[:]) {
		t.Fatalf("stored = %#v", stored)
	}
}

func openIntegrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	required := func(key string) string {
		value := os.Getenv(key)
		if value == "" {
			t.Fatalf("missing required integration test environment variable %s", key)
		}
		return value
	}
	config := mysql.NewConfig()
	config.User = required("MYSQL_USER")
	config.Passwd = required("MYSQL_PASSWORD")
	config.Net = "tcp"
	config.Addr = fmt.Sprintf("%s:%s", required("MYSQL_HOST"), required("MYSQL_PORT"))
	config.DBName = required("MYSQL_DATABASE")
	config.ParseTime = true
	config.Loc = time.UTC
	database, err := gorm.Open(gormmysql.Open(config.FormatDSN()))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return database
}

package main

import (
	"testing"
)

func TestRunRejectsUnsafeCommand(t *testing.T) {
	// reset 不在生产迁移器的允许列表中，避免通过部署入口清空迁移状态。
	err := run([]string{"reset"}, func(string) string { return "unused" })
	if err == nil || err.Error() != "command must be exactly one of: up, down" {
		t.Fatalf("run() error = %v", err)
	}
}

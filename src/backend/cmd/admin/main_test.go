package main

import (
	"strings"
	"testing"

	"hxy-blog/backend/internal/auth"
)

func TestAdminCredentials(t *testing.T) {
	values := map[string]string{
		"ADMIN_USERNAME":         " admin ",
		"ADMIN_INITIAL_PASSWORD": "correct horse battery staple",
	}
	username, password, err := adminCredentials(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("adminCredentials() error = %v", err)
	}
	if username != "admin" || password != values["ADMIN_INITIAL_PASSWORD"] {
		t.Fatalf("credentials = %q / %q", username, password)
	}
}

func TestAdminCredentialsRejectInvalidInputWithoutEchoingPassword(t *testing.T) {
	secret := "must-not-appear-in-error"
	tests := []map[string]string{
		{"ADMIN_INITIAL_PASSWORD": secret},
		{"ADMIN_USERNAME": strings.Repeat("a", 65), "ADMIN_INITIAL_PASSWORD": secret},
		{"ADMIN_USERNAME": "admin"},
		{"ADMIN_USERNAME": "admin", "ADMIN_INITIAL_PASSWORD": strings.Repeat("x", auth.MaxPasswordBytes+1)},
	}
	for index, values := range tests {
		_, _, err := adminCredentials(func(key string) string { return values[key] })
		if err == nil {
			t.Fatalf("case %d error = nil", index)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("case %d error leaked password: %v", index, err)
		}
	}
}

func TestRunRejectsUnsupportedCommandBeforeReadingDatabaseConfig(t *testing.T) {
	if err := run([]string{"reset"}, func(string) string { return "" }); err == nil {
		t.Fatal("run() error = nil")
	}
}

// redact_test.go — 敏感字段识别与掩码替换的单元测试。
// SPDX-License-Identifier: MIT

package log

import (
	"log/slog"
	"testing"
)

func TestIsSensitiveKeyMatchesCredentialNames(t *testing.T) {
	sensitive := []string{
		"password",
		"userPassword",
		"passwd",
		"clientSecret",
		"accessToken",
		"refresh_token",
		"apiKey",
		"api-key",
		"Authorization",
		"credentials",
		"privateKey",
		"kubeconfig",
		"sessionCookie",
	}
	for _, key := range sensitive {
		if !IsSensitiveKey(key) {
			t.Errorf("IsSensitiveKey(%q) = false, want true", key)
		}
	}
}

func TestIsSensitiveKeyIgnoresHarmlessNames(t *testing.T) {
	// author 必须保持可见：wails.json 里就有 author 字段，
	// 若把 "auth" 收进敏感词表会把无害元数据一起抹掉。
	harmless := []string{
		"author",
		"authorName",
		"path",
		"version",
		"theme",
		"logFile",
		"err",
	}
	for _, key := range harmless {
		if IsSensitiveKey(key) {
			t.Errorf("IsSensitiveKey(%q) = true, want false", key)
		}
	}
}

func TestMaskSecretKeepsEmptyString(t *testing.T) {
	if got := MaskSecret(""); got != "" {
		t.Errorf("MaskSecret(\"\") = %q, want empty string", got)
	}
}

func TestMaskSecretHidesNonEmptyValue(t *testing.T) {
	if got := MaskSecret("abc123"); got != mask {
		t.Errorf("MaskSecret = %q, want %q", got, mask)
	}
}

func TestRedactAttrRecursesIntoGroups(t *testing.T) {
	group := slog.GroupValue(
		slog.String("user", "anan"),
		slog.String("password", "hunter2"),
	)
	attr := slog.Attr{Key: "session", Value: group}

	redacted := redactAttr(attr)
	if redacted.Key != "session" {
		t.Fatalf("key = %q, want %q", redacted.Key, "session")
	}

	inner := redacted.Value.Group()
	if len(inner) != 2 {
		t.Fatalf("group size = %d, want 2", len(inner))
	}
	if inner[0].Value.String() != "anan" {
		t.Errorf("user = %q, want %q", inner[0].Value.String(), "anan")
	}
	if inner[1].Value.String() != mask {
		t.Errorf("password = %q, want %q", inner[1].Value.String(), mask)
	}
}

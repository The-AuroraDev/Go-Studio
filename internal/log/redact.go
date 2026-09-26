// redact.go — 日志脱敏：按字段名识别凭据类参数，在 handler 层统一替换为掩码。
// SPDX-License-Identifier: MIT

package log

import (
	"context"
	"log/slog"
	"strings"
)

// mask 是所有敏感值的统一替换文本，不保留任何原始片段。
const mask = "***"

// sensitiveKeys 是需要脱敏的字段名片段，比较前会去掉分隔符并转小写。
// 刻意不收录 "auth" 这类过宽的片段，否则 author 一类的无害字段会被误伤。
var sensitiveKeys = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"apikey",
	"authorization",
	"credential",
	"privatekey",
	"kubeconfig",
	"cookie",
}

// IsSensitiveKey 判断一个日志字段名是否属于必须脱敏的凭据类字段。
func IsSensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	for _, candidate := range sensitiveKeys {
		if strings.Contains(normalized, candidate) {
			return true
		}
	}
	return false
}

// MaskSecret 把敏感值替换为固定掩码。空字符串保持原样，因为它不泄露任何信息。
func MaskSecret(value string) string {
	if value == "" {
		return ""
	}
	return mask
}

// redactHandler 在 slog.Handler 层做脱敏，因此所有经由本 Logger 落盘的记录
// 都被覆盖，包括第三方库直接调用 slog 写出的记录。
type redactHandler struct {
	inner slog.Handler
}

// newRedactHandler 包装一个 handler，返回带脱敏能力的等价 handler。
func newRedactHandler(inner slog.Handler) slog.Handler {
	return redactHandler{inner: inner}
}

// Enabled 透传级别判定。
func (h redactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle 复制记录并脱敏其全部属性后交给内层 handler。
func (h redactHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(redactAttr(attr))
		return true
	})
	return h.inner.Handle(ctx, redacted)
}

// WithAttrs 脱敏后附加属性。
func (h redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactAttr(attr))
	}
	return redactHandler{inner: h.inner.WithAttrs(redacted)}
}

// WithGroup 透传分组，组内属性在 Handle 时逐个脱敏。
func (h redactHandler) WithGroup(name string) slog.Handler {
	return redactHandler{inner: h.inner.WithGroup(name)}
}

// redactAttr 脱敏单个属性：命中敏感词表的值被替换为掩码，
// 嵌套组则逐项递归脱敏。
func redactAttr(attr slog.Attr) slog.Attr {
	if IsSensitiveKey(attr.Key) {
		return slog.String(attr.Key, mask)
	}

	value := attr.Value.Resolve()
	if value.Kind() != slog.KindGroup {
		return attr
	}

	group := value.Group()
	redacted := make([]slog.Attr, 0, len(group))
	for _, item := range group {
		redacted = append(redacted, redactAttr(item))
	}
	return slog.Attr{Key: attr.Key, Value: slog.GroupValue(redacted...)}
}

// normalizeKey 去掉常见分隔符并转小写，让 camelCase 与 snake_case 字段名
// 都能命中同一份敏感词表。
func normalizeKey(key string) string {
	replacer := strings.NewReplacer("_", "", "-", "", ".", "", " ", "")
	return strings.ToLower(replacer.Replace(key))
}

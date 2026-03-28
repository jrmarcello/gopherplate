package logutil

import (
	"context"
	"log/slog"
	"strings"
)

// MaskingHandler wraps an slog.Handler and masks PII fields in log attributes
// based on the configured Masker. Any log attribute whose key matches a
// sensitive field name will have its string value masked before reaching
// the wrapped handler.
type MaskingHandler struct {
	masker  *Masker
	wrapped slog.Handler
}

// NewMaskingHandler creates a handler that masks sensitive log attributes.
func NewMaskingHandler(masker *Masker, wrapped slog.Handler) *MaskingHandler {
	return &MaskingHandler{
		masker:  masker,
		wrapped: wrapped,
	}
}

// Enabled delegates to the wrapped handler.
func (h *MaskingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.wrapped.Enabled(ctx, level)
}

// Handle masks sensitive attributes in the record before delegating.
func (h *MaskingHandler) Handle(ctx context.Context, record slog.Record) error {
	masked := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)

	var attrs []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, h.maskAttr(attr))
		return true
	})
	masked.AddAttrs(attrs...)

	return h.wrapped.Handle(ctx, masked)
}

// WithAttrs masks pre-applied attributes and returns a new MaskingHandler.
func (h *MaskingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	maskedAttrs := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		maskedAttrs[i] = h.maskAttr(attr)
	}
	return &MaskingHandler{
		masker:  h.masker,
		wrapped: h.wrapped.WithAttrs(maskedAttrs),
	}
}

// WithGroup delegates to the wrapped handler.
func (h *MaskingHandler) WithGroup(name string) slog.Handler {
	return &MaskingHandler{
		masker:  h.masker,
		wrapped: h.wrapped.WithGroup(name),
	}
}

// maskAttr masks a single slog.Attr if its key matches a sensitive field.
// Handles string values directly and recurses into Group attributes.
func (h *MaskingHandler) maskAttr(attr slog.Attr) slog.Attr {
	key := attr.Key
	val := attr.Value

	// Handle group attributes recursively.
	if val.Kind() == slog.KindGroup {
		groupAttrs := val.Group()
		maskedGroupAttrs := make([]any, len(groupAttrs))
		for i, ga := range groupAttrs {
			maskedGroupAttrs[i] = h.maskAttr(ga)
		}
		return slog.Group(key, maskedGroupAttrs...)
	}

	// Check if this key is a sensitive field.
	normalizedKey := strings.ToLower(key)
	maskFn, found := h.masker.config.Fields[normalizedKey]
	if found && val.Kind() == slog.KindString {
		strVal := val.String()
		if strVal != "" {
			return slog.String(key, maskFn(strVal))
		}
	}

	return attr
}

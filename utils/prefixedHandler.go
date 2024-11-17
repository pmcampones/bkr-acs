package utils

import (
	"context"
	"fmt"
	"log/slog"
)

const (
	cyan  = "\033[0;36m"
	white = "\033[1;37m"
)

type PrefixedHandler struct {
	prefix string
	inner  slog.Handler
}

func NewPrefixedHandler(prefix string, inner slog.Handler) *PrefixedHandler {
	formatedPrefix := fmt.Sprintf("%s[%s]%s ", cyan, prefix, white)
	return &PrefixedHandler{prefix: formatedPrefix, inner: inner}
}

func (h *PrefixedHandler) Enabled(ctx context.Context, lv slog.Level) bool {
	return h.inner.Enabled(ctx, lv)
}
func (h *PrefixedHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Message = h.prefix + r.Message
	return h.inner.Handle(ctx, r)
}

func (h *PrefixedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.inner.WithAttrs(attrs)
}

func (h *PrefixedHandler) WithGroup(name string) slog.Handler {
	return h.inner.WithGroup(name)
}

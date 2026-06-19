package logctx

import (
	"context"
	"log/slog"
)

type key struct{}

func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, key{}, log)
}

func From(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(key{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

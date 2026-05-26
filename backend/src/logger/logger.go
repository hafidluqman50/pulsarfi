package logger

import (
	"log/slog"
	"os"
)

var L *slog.Logger

func init() {
	L = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(L)
}

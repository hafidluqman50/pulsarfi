package logger

import (
	"io"
	"log/slog"
	"os"
)

var L *slog.Logger

// init writes every log line to both stdout (unchanged behavior) and a
// persistent file (backend/logs/app.log) — previously stdout-only, so any
// output not explicitly redirected by whoever started the process (e.g.
// plain `go run .` in a terminal) was lost the moment that process or
// terminal ended, with no way to answer "what happened/which model got
// called yesterday" after the fact. The file is append-only across
// restarts, never truncated, so it accumulates a durable history.
func init() {
	writers := []io.Writer{os.Stdout}
	if err := os.MkdirAll("logs", 0o755); err == nil {
		if file, err := os.OpenFile("logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			writers = append(writers, file)
		}
	}
	L = slog.New(slog.NewJSONHandler(io.MultiWriter(writers...), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(L)
}

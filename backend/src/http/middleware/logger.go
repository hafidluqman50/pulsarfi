package middleware

import (
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
)

// pool: the database pool whose wait and churn counters are logged per request.
func Logger(pool *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
        stats := config.NewQueryStats()
        c.Request = c.Request.WithContext(config.WithQueryStats(c.Request.Context(), stats))
        poolBefore := pool.Stats()
		c.Next()
        poolAfter := pool.Stats()

		status := c.Writer.Status()
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		slog.Log(c.Request.Context(), level, "request",
			"status", status,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"latency", time.Since(start).String(),
			"ip", c.ClientIP(),
            "db_queries", stats.Count(),
            "db_ms", stats.Millis(),
            "db_top", strings.Join(stats.Top(4), "; "),
            "pool_wait_count", poolAfter.WaitCount-poolBefore.WaitCount,
            "pool_wait_ms", (poolAfter.WaitDuration - poolBefore.WaitDuration).Milliseconds(),
            "pool_idle_closed", poolAfter.MaxIdleClosed-poolBefore.MaxIdleClosed,
            "pool_open", poolAfter.OpenConnections,
		)
	}
}

package config

import (
    "context"
    "fmt"
    "regexp"
    "sort"
    "strings"
    "sync"
    "time"

    "gorm.io/gorm/logger"
)

type queryStat struct {
    count int
    total time.Duration
}

type QueryStats struct {
    mu      sync.Mutex
    count   int
    total   time.Duration
    byQuery map[string]*queryStat
}

type queryStatsKey struct{}

func NewQueryStats() *QueryStats {
    return &QueryStats{byQuery: map[string]*queryStat{}}
}

func WithQueryStats(ctx context.Context, stats *QueryStats) context.Context {
    return context.WithValue(ctx, queryStatsKey{}, stats)
}

func QueryStatsFrom(ctx context.Context) (*QueryStats, bool) {
    stats, ok := ctx.Value(queryStatsKey{}).(*QueryStats)
    return stats, ok
}

var queryTablePattern = regexp.MustCompile(`(?:from|into|update)\s+"?([a-z_][a-z0-9_.]*)"?`)

// statement: SQL text, read only to name the query kind and table; never logged.
func QueryKey(statement string) string {
    trimmed := strings.ToLower(strings.TrimSpace(statement))
    verb := "OTHER"
    switch {
    case strings.HasPrefix(trimmed, "select"):
        verb = "SELECT"
    case strings.HasPrefix(trimmed, "insert"):
        verb = "INSERT"
    case strings.HasPrefix(trimmed, "update"):
        verb = "UPDATE"
    case strings.HasPrefix(trimmed, "delete"):
        verb = "DELETE"
    case strings.HasPrefix(trimmed, "with"):
        verb = "WITH"
    }

    match := queryTablePattern.FindStringSubmatch(trimmed)
    if match == nil {
        return verb
    }

    return verb + " " + match[1]
}

// key: query kind and table from QueryKey.
// elapsed: time the query took.
func (stats *QueryStats) Record(key string, elapsed time.Duration) {
    stats.mu.Lock()
    defer stats.mu.Unlock()
    stats.count++
    stats.total += elapsed
    entry, ok := stats.byQuery[key]
    if !ok {
        entry = &queryStat{}
        stats.byQuery[key] = entry
    }

    entry.count++
    entry.total += elapsed
}

func (stats *QueryStats) Count() int {
    stats.mu.Lock()
    defer stats.mu.Unlock()
    return stats.count
}

func (stats *QueryStats) Millis() int64 {
    stats.mu.Lock()
    defer stats.mu.Unlock()
    return stats.total.Milliseconds()
}

// top: how many query kinds to list, longest total time first.
func (stats *QueryStats) Top(top int) []string {
    stats.mu.Lock()
    defer stats.mu.Unlock()
    keys := make([]string, 0, len(stats.byQuery))
    for key := range stats.byQuery {
        keys = append(keys, key)
    }

    sort.Slice(keys, func(left, right int) bool {
        return stats.byQuery[keys[left]].total > stats.byQuery[keys[right]].total
    })

    if len(keys) > top {
        keys = keys[:top]
    }

    lines := make([]string, 0, len(keys))
    for _, key := range keys {
        entry := stats.byQuery[key]
        lines = append(lines, fmt.Sprintf("%s x%d %dms", key, entry.count, entry.total.Milliseconds()))
    }

    return lines
}

type queryStatsLogger struct {
    logger.Interface
}

func (wrapper queryStatsLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
    if stats, ok := QueryStatsFrom(ctx); ok {
        statement, _ := fc()
        stats.Record(QueryKey(statement), time.Since(begin))
    }

    wrapper.Interface.Trace(ctx, begin, fc, err)
}

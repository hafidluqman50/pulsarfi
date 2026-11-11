package config_test

import (
    "context"
    "strings"
    "testing"
    "time"

    "github.com/horizonlabs/pulsarfi-backend/src/config"
)

func TestQueryKey_NamesTheKindAndTheTable(t *testing.T) {
    cases := map[string]string{
        `SELECT * FROM "agent_chat_messages" WHERE chat_id = 'x' ORDER BY id`:      "SELECT agent_chat_messages",
        `INSERT INTO "agent_sub_tasks" ("task_id") VALUES (1) RETURNING "id"`:       "INSERT agent_sub_tasks",
        `UPDATE "agent_tasks" SET "armed_at"='2026-01-01' WHERE id = 1`:             "UPDATE agent_tasks",
        `DELETE FROM agent_checkpoints WHERE checkpoint_id = 'a'`:                   "DELETE agent_checkpoints",
        `BEGIN`: "OTHER",
    }

    for statement, want := range cases {
        if got := config.QueryKey(statement); got != want {
            t.Fatalf("QueryKey(%q): expected %q, got %q", statement, want, got)
        }
    }
}

func TestQueryStats_CountsTimeAndListsTheSlowestKindFirst(t *testing.T) {
    ctx := config.WithQueryStats(context.Background(), config.NewQueryStats())
    stats, ok := config.QueryStatsFrom(ctx)
    if !ok {
        t.Fatal("stats were not found on the context")
    }

    stats.Record("SELECT agent_tasks", 20*time.Millisecond)
    stats.Record("INSERT agent_sub_tasks", 300*time.Millisecond)
    stats.Record("SELECT agent_tasks", 30*time.Millisecond)

    if stats.Count() != 3 || stats.Millis() != 350 {
        t.Fatalf("expected 3 queries and 350ms, got %d and %dms", stats.Count(), stats.Millis())
    }

    top := stats.Top(2)
    if len(top) != 2 || !strings.HasPrefix(top[0], "INSERT agent_sub_tasks x1 300ms") || !strings.HasPrefix(top[1], "SELECT agent_tasks x2 50ms") {
        t.Fatalf("unexpected ranking: %v", top)
    }
}

package service

import (
	"context"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/service/realtime"
)

// StartRealtimePublishers registers every topic that used to be
// frontend-polled with the realtime Hub, at the same interval the frontend
// previously polled at (docs/plans/realtime-websocket-updates.md §3). Each
// fetch runs once per interval no matter how many WebSocket clients are
// subscribed, and is skipped entirely while nobody is subscribed. Returns
// immediately; each topic runs on its own goroutine until ctx is cancelled
// (server shutdown). Two topics from the plan are deliberately not wired
// here yet: `stock-price:{ticker}` needs per-ticker scheduling driven by
// which tickers clients actually subscribe to, and `agent-task-trades:{id}`
// has no write path at all yet (AgentTradeRepository.Create has zero
// callers in this codebase) — both are open follow-ups, not silently
// dropped.
func (r *Registry) StartRealtimePublishers(ctx context.Context) {
	hub := realtime.Default()

	hub.SchedulePublish(ctx, "market-stocks", 15*time.Second, func(ctx context.Context) (any, error) {
		return r.PublicStock.ListMarketStocks(ctx)
	})
	hub.SchedulePublish(ctx, "protocol-stats", 30*time.Second, func(ctx context.Context) (any, error) {
		return r.PublicStats.GetStats(ctx)
	})
	hub.SchedulePublish(ctx, "reserves", 30*time.Second, func(ctx context.Context) (any, error) {
		return r.PublicReserve.GetReserves(ctx)
	})
	hub.SchedulePublish(ctx, "custodian-stats", 30*time.Second, func(ctx context.Context) (any, error) {
		return r.Custodian.GetStats(ctx)
	})
	hub.SchedulePublish(ctx, "custodian-requests", 15*time.Second, func(ctx context.Context) (any, error) {
		return r.Custodian.ListPendingRequests(ctx)
	})
	hub.SchedulePublish(ctx, "custodian-stocks", 30*time.Second, func(ctx context.Context) (any, error) {
		return r.Custodian.ListStocks(ctx)
	})
	hub.SchedulePublish(ctx, "custodian-wallet-verifications", 30*time.Second, func(ctx context.Context) (any, error) {
		return r.CustodianKYC.List(ctx, "")
	})
}

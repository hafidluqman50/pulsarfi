# Realtime Updates via WebSocket (Replacing Polling)

| | |
|---|---|
| **Version** | 1.3 |
| **Status** | Implemented |
| **Date Created** | 2026-09-06 |
| **Last Updated** | 2026-09-06 |

| Version | Date | Change |
|---|---|---|
| 1.3 | 2026-09-06 | **v1.2 fixed backend topic-level auth but never checked the frontend's own login/logout lifecycle — flagged live ("jangan liat backend doang yang auth, liat frontend juga").** `SiweAuthContext.tsx`'s `applySession`/`clearAuth` (sign-in/sign-out) never told the already-open socket its identity had changed: a socket that connected anonymously before login never re-authenticated after (private topics stayed silently dropped forever without a page reload), and a socket that was authenticated before logout kept flowing already-granted private data until it happened to disconnect on its own — a real stale-authorization gap, not just UX. Fixed: new `RealtimeSocket.reconnect()` (`useRealtimeSocket.ts`) closes the current connection and immediately opens a fresh one under whatever token is now in `localStorage`, re-subscribing every currently-registered topic automatically via the existing `onopen` handler; guarded a real race where a superseded socket's late `onclose` event could null out the new socket's own reference. Called from both `applySession` and `clearAuth`. `tsc`/`eslint` clean (one pre-existing, unrelated `react-hooks/set-state-in-effect` lint error in the same file confirmed present before this change too, left alone) |
| 1.2 | 2026-09-06 | **Critical gap found live: v1.1 required a valid JWT to even upgrade the WebSocket, which would have silently broken every realtime topic for `/markets` — a public, no-login page.** `market-stocks`/`protocol-stats`/`reserves` are already served unauthenticated via `GET /api/v1/public/*`; requiring a login for their realtime equivalent regresses exactly the highest-traffic, most rate-limit-prone surface back to permanent polling. Fixed: auth moved from connection-level to topic-level. `StreamHandler` now makes the `?token=` query param optional — no token connects anonymously, a token that *is* given but fails to parse is still rejected outright. New `isTopicAllowed(topic, identity)` (`authorization_service.go`) mirrors each topic's own REST equivalent's real auth requirement: `market-stocks`/`protocol-stats`/`reserves`/`agent-task-reasoning:*` are public (matching their unauthenticated or deliberately-public REST routes); `agent-task-trades:*` requires any authenticated wallet (matching `protected.GET("/tasks/:id/trades")`); `custodian-*` requires `Role == "custodian"` (matching `custodianMiddleware.Auth`). `Hub.subscribe` silently drops a disallowed topic from the request rather than closing the whole connection, since one socket multiplexes many topics. `go build`/`go vet`/`gofmt`/`tsc` all clean |
| 1.1 | 2026-09-06 | **Implemented as designed, with 3 topics deliberately left out — see §9.** Backend: `service/realtime` (`Hub`, `SchedulePublish`, last-value cache so a fresh subscriber doesn't wait a full interval), `GET /api/v1/realtime/ws` (`gorilla/websocket`, query-param JWT auth), `SubTaskRecorder.Record` now publishes `agent-task-reasoning:{id}` on every row, `Registry.StartRealtimePublishers` schedules `market-stocks`/`protocol-stats`/`reserves`/`custodian-stats`/`custodian-requests`/`custodian-stocks`/`custodian-wallet-verifications`. Frontend: `useRealtimeSocket.ts` (singleton socket, reconnect with backoff, `useRealtimeTopic`/`useRealtimeConnected`), 7 hooks across `agent`/`market`/`custodian` switched to socket-driven updates with `refetchInterval` as a disconnected-only fallback. `go build`/`go vet`/`gofmt` and `npx tsc --noEmit`/`eslint` all clean. Live end-to-end verification (real deploy, confirming Fly does not auto-stop with an open socket) still pending, per §9 |
| 1.0 | 2026-09-06 | Initial version |

**Note on scope.** This plan covers replacing every `refetchInterval`-based poll in the frontend (agent task reasoning/trades, market prices/stats, custodian stats/requests/stocks/reserves/wallet-verifications) with a single WebSocket connection. It does not cover the Executor/trade *submission* flow itself (that stays exactly as it is — `submit_trade` → on-chain `ExecuteTrade`); it only covers how the frontend *learns* that a trade/task/price changed, replacing polling with a push.

---

## 1. Problem

Live production logs (screenshot, 2026-09-06 14:55) showed a flood of `429 rate limit exceeded` responses, including on `OPTIONS` (CORS preflight) requests, across unrelated endpoints (`/agent/tasks/*/reasoning`, `/public/stats`, `/public/stock-transactions`, chat messages) within the same second.

**Root cause, found live, two compounding bugs:**

1. `middleware.RateLimit(100, time.Minute)` (`backend/src/http/middleware/ratelimit.go`, mounted globally in `backend/src/http/routes/router.go:22`) buckets by client IP only, across **every** route, including `OPTIONS` preflight. In local dev every request comes from the same IP (`::1`), so one shared 100-req/min budget is split across the entire app. A failed `OPTIONS` preflight makes the browser treat the real request as failed too (CORS), even though the real endpoint is healthy.
2. `useTaskReasoning` (5s) and `useTaskTrades` (15s) (`frontend/http/agent/hooks.ts`) poll unconditionally forever, with no check for task status, and with no cap on how many Task cards can be mounted (and therefore polling) at once. The same pattern repeats across `frontend/http/market/hooks.ts` and `frontend/http/custodian/hooks.ts` (5 different intervals: 15s/30s/60s). More tabs/cards open = more requests, all against the same shared rate-limit bucket.

Raising the rate limit or fixing the `OPTIONS` bug only patches the symptom; the underlying architecture (N clients × M hooks × their own poll interval, all hitting the backend independently) does not scale and keeps recreating this failure mode as more UI surfaces are added.

---

## 2. Approach

Replace polling with a single WebSocket connection per client, multiplexing every topic the frontend currently polls — the same shape as a managed pub/sub service (e.g. Ably): one connection, many channels/topics subscribed on top of it, not one connection per feature.

| Option | Chosen | Why |
|---|---|---|
| One WebSocket endpoint, topic-multiplexed | **Yes** | One handshake/auth/keepalive per client regardless of how many topics are open; matches the Ably-style mental model already familiar to this team; reconnect/backoff logic lives in one place |
| One WebSocket endpoint per feature area (`/ws/agent-tasks`, `/ws/market`, `/ws/custodian`) | No | Simpler per-endpoint code (topic implied by path, no subscribe/unsubscribe protocol needed), but N separate connections/reconnect-loops to write and maintain, for no correctness benefit at this app's scale |
| Server-Sent Events (SSE) | No (explicitly requested to be WebSocket) | One-directional (server→client) would have been sufficient for this specific use case (no client→server data needed beyond subscribe/unsubscribe), and reuses the pattern already used for chat streaming, but the user explicitly asked for WebSocket |

---

## 3. Backend Design

**One endpoint:** `GET /api/v1/realtime/ws` (upgrade to WebSocket, `gorilla/websocket` — new dependency).

**Protocol over the socket** (JSON text frames, client → server):
```json
{"type": "subscribe", "topics": ["agent-task-reasoning:42", "market-stocks"]}
{"type": "unsubscribe", "topics": ["agent-task-reasoning:42"]}
```
Server → client:
```json
{"type": "update", "topic": "agent-task-reasoning:42", "data": [...]}
```

**Two publish patterns, depending on the topic's data source:**

| Topic | Trigger |
|---|---|
| `agent-task-reasoning:{id}`, `agent-task-trades:{id}` | Event-driven. Published the instant `SubTaskRecorder.Record` (`backend/src/service/agent/sub_task_recorder_service.go`) writes a new row, or a Task's status changes (arm/disarm/pause/resume, `task_service.go`). No timer. |
| `market-stocks`, `stock-price:{ticker}`, `protocol-stats`, `custodian-stats`, `custodian-requests`, `custodian-stocks`, `reserves` | Timer-driven, same interval the frontend currently polls at (15s/30s/60s per topic). The backend fetches **once** per interval regardless of how many clients are subscribed, then fans out — this is what actually fixes the scaling problem, not just moving where the interval lives. |

**In-process pub/sub is sufficient:** `backend/fly.toml` runs a single machine (`min_machines_running = 0`, no multi-machine/autoscale config), so a plain Go map of `topic → []*websocket.Conn` behind a mutex needs no external broker (no Redis, no Postgres `LISTEN/NOTIFY`).

**Auth:** same JWT/session validation as the existing HTTP routes, checked once at the WebSocket upgrade (query param or header, whichever the existing auth middleware already supports for non-standard-header contexts).

---

## 4. Frontend Design

One `useRealtimeSocket()` hook, mounted once near the app root, owns the single `WebSocket` connection and a subscribe/unsubscribe registry. Existing consumer hooks (`useTaskReasoning`, `useTaskTrades`, `useStockPrice`, `useMarketStocks`, `useProtocolStats`, `useCustodianStats`, etc., in `frontend/http/agent/hooks.ts` / `frontend/http/market/hooks.ts` / `frontend/http/custodian/hooks.ts`) keep their exact current names, signatures, and React Query keys — consumer components (`PlanCard`, `TaskDetail`, dashboards) need zero changes.

Each hook's `refetchInterval` is replaced with: subscribe to its topic through `useRealtimeSocket`, and on an incoming `update` message for that topic, call `queryClient.setQueryData(queryKey, data)` instead of refetching over HTTP.

**Fallback safety net:** each hook keeps its original `refetchInterval`, but only active while the socket is disconnected (`refetchInterval: isSocketConnected ? false : <original interval>`) — so a dropped/reconnecting socket does not mean permanently stale data.

Scheme selection: `wss://` when the page itself is loaded over `https:` (production, Fly.io — required, browsers block insecure `ws://` from an `https:` page), `ws://` in local dev (`http://localhost:3000` ↔ `http://localhost:8080`, both insecure, no restriction applies).

---

## 5. Safety / Reliability

- **Keepalive.** Server sends a WebSocket ping frame every ~30s; the browser's native WebSocket implementation answers pong automatically. Prevents Fly's edge proxy (or any intermediary) from treating an idle-but-alive connection as dead and closing it.
- **Reconnect with backoff.** On disconnect, the frontend retries with increasing delay (1s, 2s, 4s, 8s, capped) rather than reconnecting immediately in a tight loop or giving up.
- **Fly.io scale-to-zero interaction (needs live verification, not just assumed from docs).** `fly.toml` has `auto_stop_machines = "stop"` + `min_machines_running = 0`. An open WebSocket connection should count as active traffic (so the machine should not stop while any client is connected), but this must be confirmed against the real deployment after this ships, not trusted blindly.

---

## 6. Impacted Files

| Layer | File | Change |
|---|---|---|
| Backend | `backend/src/service/realtime/authorization_service.go` | `[NEW]` `isTopicAllowed`, per-topic auth matching each topic's REST equivalent |
| Backend | `backend/src/service/realtime/hub_service.go` | `[NEW]` topic registry, subscribe/unsubscribe (enforces `isTopicAllowed`), fan-out, last-value cache |
| Backend | `backend/src/service/realtime/client_service.go` | `[NEW]` read/write pumps, ping keepalive |
| Backend | `backend/src/service/realtime/scheduler_service.go` | `[NEW]` `SchedulePublish` (fetch-once-fan-out-many per topic, skipped while no subscribers) |
| Backend | `backend/src/service/realtime_publishers.go` | `[NEW]` `Registry.StartRealtimePublishers`, wires the 7 timer-driven topics to their existing service methods |
| Backend | `backend/src/http/handlers/realtime/ws.go` | `[NEW]` WebSocket upgrade handler, query-param JWT auth |
| Backend | `backend/src/http/routes/realtime/router.go` | `[NEW]` mounts `GET /realtime/ws` |
| Backend | `backend/src/http/routes/router.go` | `[MODIFY]` registers the realtime route group |
| Backend | `backend/src/app/bootstrap.go` | `[MODIFY]` calls `StartRealtimePublishers` alongside the other background loops |
| Backend | `backend/src/service/agent/sub_task_recorder_service.go` | `[MODIFY]` `Record` publishes `agent-task-reasoning:{id}` |
| Backend | `backend/go.mod` | `[MODIFY]` `gorilla/websocket` now a direct dependency |
| Frontend | `frontend/http/realtime/useRealtimeSocket.ts` | `[NEW]` single socket owner, subscribe registry, reconnect/backoff, `useRealtimeTopic`/`useRealtimeConnected` |
| Frontend | `frontend/http/agent/hooks.ts` | `[MODIFY]` `useTaskReasoning`/`useTaskTrades` read from socket, fallback interval |
| Frontend | `frontend/http/market/hooks.ts` | `[MODIFY]` `useMarketStocks`/`useProtocolStats` same |
| Frontend | `frontend/http/custodian/hooks.ts` | `[MODIFY]` all 5 read hooks same |

---

## 7. Open Questions

- Exact auth mechanism at WebSocket upgrade time — resolved during implementation: JWT as a `?token=` query param (browsers cannot set custom headers on a WebSocket upgrade request), validated with the same `auth.ParseAccessToken` the existing HTTP middleware already uses.
- Whether `custodian-requests`/`custodian-stocks`/wallet-verification topics need per-custodian scoping — resolved during implementation by matching current behavior exactly: the underlying `CustodianService`/`KYCService` methods take no per-custodian filter today (their existing REST endpoints already return the same global data to any authenticated custodian), so the realtime topics broadcast the same way. Revisit if per-custodian scoping is ever added to the REST endpoints themselves.

---

## 8. Verification Plan

- **Automated:** none for the WebSocket transport itself (hard to unit-test a live socket loop meaningfully); `go build ./...`, `go vet ./...`, `gofmt -l`, `npx tsc --noEmit`, and `eslint` all run clean on every changed file. Existing tests for the underlying data-fetching (`test/public/portfolio_chart_service_test.go`, etc.) stay as the correctness check for the data itself.
- **Manual, verified live (2026-09-06, separate instance on port 8081, real Supabase DB, never the user's own running server on 8080):** a raw Go WebSocket client connected anonymously (no token) and subscribed to `market-stocks`/`protocol-stats`/`reserves`/`custodian-stats`. Within 40s: 3 real `market-stocks` updates (real tickers/prices/sparklines from the DB), at least one real `protocol-stats` and `reserves` update each — confirming the scheduler actually re-fires on its own interval, not just once. `custodian-stats` never arrived even once — confirming `isTopicAllowed` genuinely blocks a custodian-only topic for an anonymous connection, not just in code review. A request with a deliberately invalid `?token=` was rejected with `401` before the WebSocket upgrade completed, confirmed via raw HTTP headers (curl). IDX being closed (weekend) means the underlying price values themselves don't change tick-to-tick, but that only affects the *data*, not the push mechanism being tested here.
- **Manual, still pending:** the event-driven `agent-task-reasoning` topic (needs a real chat turn through the LLM pipeline, not just a raw socket client); the full frontend flow in an actual browser (login/logout triggering `reconnect()`, `/markets` working with no wallet connected at all); killing the backend process mid-session and confirming the frontend reconnects and catches up; deploying to Fly.io and confirming the machine does not auto-stop while a socket is connected.

---

## 9. Follow-Ups Not Covered By This Pass

| Topic | Why deferred |
|---|---|
| `stock-price:{ticker}` | Per-ticker, needs the scheduler to know which tickers clients currently have subscribed and poll only those — a different shape than the fixed, global topics wired here. `useStockPrice`/`useStockHistory` still poll as before. |
| `stock-transactions:{wallet}` | Per-wallet, same shape of gap as above. `useStockTransactions` still polls as before. |
| `agent-task-trades:{id}` | Wired end-to-end on both backend (`SubTaskRecorder` topic naming convention) and frontend (`useTaskTrades`), but nothing publishes to it yet: `AgentTradeRepository.Create` has zero callers anywhere in this codebase — a pre-existing gap (trades are never written at all today), not something introduced or silently patched over by this change. Whoever closes that gap gets live trade updates for free by publishing to `agent-task-trades:{id}` at the same place. |

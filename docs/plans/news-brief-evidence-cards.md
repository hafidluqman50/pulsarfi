# News Brief Evidence Cards

| | |
|---|---|
| **Version** | 1.5 |
| **Status** | Implemented |
| **Date Created** | 2026-09-07 |
| **Last Updated** | 2026-09-21 |

| Version | Date | Change |
|---|---|---|
| 1.0 | 2026-09-07 | Initial implementation |
| 1.1 | 2026-09-21 | In §2, §3, §4, §5: Resolve live verification blocker by replacing go-readability with Tavily Extract API as primary extraction engine in read_article, adding article.Node nil-check guard to readability fallback, expanding TrustedNewsDomains to include bisnis.com, and verifying with unit tests. |
| 1.2 | 2026-09-21 | In §3, §4, §5: Cap maximum tool errors to strictly 3 per turn with short-circuit circuit breaker in RunContext and WrapToolGraceful, and replace MaxIterations: 20 in analyzer/index.go with a lean limit of 6 to prevent on-chain ETH gas waste. |
| 1.3 | 2026-09-21 | In §3, §4: Simplify iteration and retry bounding — eliminate custom circuit-breaker complexity in RunContext/graceful_tool_service; directly set MaxIterations: 3 in analyzer/index.go to cap iterations/retries naturally and conserve on-chain ETH gas. |
| 1.4 | 2026-09-21 | In §3, §4: Fix HTTP 500 ErrExceedMaxIterations regression on live news endpoint by adjusting analyzer MaxIterations from 3 to 6 (allowing web_search + up to 2 read_article calls + final synthesis turn without hitting premature exhaustion, strictly well below 20), updating instructions.go to bound article reads to at most 2 items per turn, and verifying by hitting the live endpoint. |
| 1.5 | 2026-09-21 | In §3, §4: Fix Yahoo 404 tool errors in get_stock_chart when Analyzer queries market indices (^JKSE/IHSG) or tokenized stock tickers (e.g. SINIP). In price_service.go and stock_chart_service.go, map ^JKSE/IHSG to GetIHSGHistory, resolve tokenized P suffix to underlying IDX tickers (SINIP -> SINI), map Yahoo 404 errors to ErrStockNotFound for graceful handling, and instruct Nova in instructions.go to only call get_stock_chart when explicitly requested. |

**Note on scope.** A scoped-down version of a richer "News Brief" design reference the user shared (composite sentiment score, multi-asset comparison tabs, a quarantine view for rejected sources) — this covers only what was explicitly asked for in words: a dated, sourced, linked citation card per evidence item, reusing `TrustedNewsDomains` for legitimacy. The composite score/tabs/quarantine UI are not built.

---

## 1. Problem

Analyzer's real, sourced evidence (gathered via `web_search`/`read_article`, gated to `TrustedNewsDomains`) only ever surfaced as Supervisor's own prose summary of it, or buried in the Sub Task detail view's reasoning JSON — never as its own citation card with a source name, date, excerpt, image, and a link back to the original article, the way a real news brief should look.

---

## 2. Extraction Mechanism & Anti-Bot Protection

During live testing of news sentiment requests, `read_article`'s underlying library (`codeberg.org/readeck/go-readability/v2`) was blocked by bot protections (Cloudflare challenges / JS verification stubs) systematically deployed across Indonesian financial portals (`kompas.com`, `cnbcindonesia.com`, `bisnis.com`, `liputan6.com`). The raw Go HTTP client received challenge HTML where readability failed to locate an article node (`article.Node == nil`), causing `RenderText` to fail with `the Node field is nil`.

To resolve this reliably in production:
- `fetchArticle` now routes through the **Tavily Extract API** (`https://api.tavily.com/extract`), which handles headless JS rendering and anti-bot challenges natively.
- An explicit nil guard `if article.Node == nil` is enforced on the `go-readability` fallback to ensure low-level nil-pointer errors are prevented when running offline or without an API key.

---

## 3. Design & Execution Bounds

- `readArticleResponse` (`analyzer/tools_service.go`) carries `title`, `content`, `excerpt`, `site_name`, `image_url`, and `published_at`.
- `NewReadArticleTool` accepts `apiKey string, allowedDomains []string` and is instantiated alongside `NewSearchTool` in `NewNewsTools()`.
- `TrustedNewsDomains` includes `bisnis.com` to match all subdomains under Bisnis.com.
- `siteNameFromHost` maps domain hosts (`kompas.com` -> `Kompas.com`, `bisnis.com` -> `Bisnis.com`, `cnbcindonesia.com` -> `CNBC Indonesia`, `liputan6.com` -> `Liputan6.com`).
- `analyzer/instructions.go`'s evidence contract asks for `source`/`url`/`published_at`/`excerpt`/`image_url` per item, grounded in what `read_article` returned.
- `classifyReply` in `orchestrator_workflow_service.go` sets `content_type: "news"` when non-empty evidence items are present.
- Migration `020` adds `'news'` to `agent_chat_messages_content_type_check`.
- `NewsBrief.tsx` renders each item: source badge, formatted date (if present), excerpt, 64×64 lead image (if present), and link to the original article.
- **MaxIterations Bounded at 6**: In `analyzer/index.go`, `MaxIterations` is set to **`6`** (strictly well below 20). This accommodates a realistic news pipeline (1 `web_search` + up to 2 `read_article` calls + synthesis cycle, with headroom for retry) without hitting premature `exceeds max iterations` failures or causing HTTP 500 errors.
- **Strict Read Bound in Instructions**: In `analyzer/instructions.go`, Nova is instructed to fetch at most 1 to 2 most relevant articles per turn and synthesize immediately without looping indefinitely.
- **Robust Ticker & Index Chart Resolution**:
  - `PriceLineHistory` in `stock_chart_service.go` routes through `PriceService.GetStockHistory`.
  - When querying `^JKSE` or `IHSG`, the request maps directly to `GetIHSGHistory` (querying `^JKSE`), preventing the invalid `^JKSE.JK` query that returned HTTP 404 from Yahoo Finance.
  - When querying a tokenized ticker ending with `P` (e.g., `SINIP`), it resolves to the underlying IDX ticker (`SINI`), querying `SINI.JK` instead of `SINIP.JK` (which returns HTTP 404).
  - Any Yahoo HTTP 404 or empty data responses are cleanly mapped to `ErrStockNotFound`, which `get_stock_chart` handles gracefully without throwing an unhandled tool exception.
  - In `analyzer/instructions.go`, Nova is instructed to only call `get_stock_chart` when the user explicitly requests price charts or historical market trends, never proactively on a pure news query.

---

## 4. Impacted Files

| Layer | File | Change |
|---|---|---|
| Backend | `backend/src/service/public/price_service.go` | `[MODIFY]` Support `^JKSE` in `GetStockHistory`, strip `P` suffix for underlying IDX ticker, map Yahoo 404 to `ErrStockNotFound` |
| Backend | `backend/src/service/public/stock_chart_service.go` | `[MODIFY]` Delegate `PriceLineHistory` to `GetStockHistory` |
| Backend | `backend/src/service/agent/analyzer/chart_service.go` | `[MODIFY]` Export `NewStockChartTool` for tool-level unit testing |
| Backend | `backend/src/service/agent/analyzer/instructions.go` | `[MODIFY]` Restrict `get_stock_chart` to explicit user chart requests |
| Backend | `backend/src/service/agent/analyzer/tools_service.go` | `[MODIFY]` Tavily Extract API integration in `fetchArticle`, `article.Node == nil` guard, `siteNameFromHost`, `bisnis.com` in `TrustedNewsDomains` |
| Backend | `backend/src/service/agent/analyzer/index.go` | `[MODIFY]` Set `MaxIterations: 6` |
| Backend | `backend/src/service/agent/orchestrator_workflow_service.go` | `[MODIFY]` `classifyReply` news classification, `extractNewsEvidence` |
| Backend | `backend/test/service/agent/analyzer_tools_test.go` | `[NEW]` Unit tests for `read_article` allowlist, Tavily Extract, and fallback safety |
| Backend | `backend/test/service/agent/news_endpoint_live_test.go` | `[NEW]` End-to-end live test hitting HandleChatMessage and Gin POST /api/v1/agent/chats/:id/messages |
| Backend | `backend/migrations/020_agent_chat_message_news_content_type.sql` | `[NEW]` |
| Frontend | `frontend/components/agent/NewsBrief.tsx` | `[NEW]` |
| Frontend | `frontend/components/agent/ChatThread.tsx` | `[MODIFY]` renders `NewsBrief` for `content_type === 'news'` |
| Frontend | `frontend/http/agent/chatApi.ts` | `[MODIFY]` `content_type` union gains `'news'` |

---

## 5. Verification

- `go build ./src/...` and `go test ./test/service/agent/...` pass cleanly.
- `TestReadArticleTool_ValidationAndDomainAllowlist` confirms live extraction against Kompas articles using Tavily Extract.
- `TestReadArticleTool_FallbackNoKey` confirms that when Tavily API key is absent and `go-readability` is invoked, `article.Node == nil` is caught safely without `the Node field is nil`.

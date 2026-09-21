# News Brief Evidence Cards

| | |
|---|---|
| **Version** | 1.10 |
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
| 1.6 | 2026-09-21 | In §3, §4: Fortify external PriceService against invalid index and duplicate .JK suffixes by implementing cleanYahooIDXSymbol, routing ^JKSE/IHSG/JKSE calls in GetYahooIDX, GetYahooIDXMarket, and GetYahooIDXHistory to IHSG handlers, and deduplicating .JK on all ticker lookups. |
| 1.7 | 2026-09-21 | In §3, §4: Revert manual string-cleaning Go code in external and public price services. Enforce LLM-side ticker cleanup and filtering directly in analyzer/instructions.go and stockChartRequest jsonschema description: Nova must filter when to invoke get_stock_chart (never on pure news) and clean up tickers to official 4-letter IDX symbols (stripping tokenized 'P' suffix) or 'IHSG' before calling tools. |
| 1.8 | 2026-09-21 | In §3, §4: Fix HTTP 500 on follow-up reference queries (e.g. 'Btw itu SINI referensinya dari mana?') by passing recent assistant context to Nova in runAnalyze, bounding reference searches in analyzer/instructions.go to at most 1 targeted search/read without open-ended loops, recovering gracefully from ErrExceedMaxIterations in runRoleAgent without crashing the HTTP server, and setting analyzer MaxIterations to 8 for multi-turn research headroom. |
| 1.9 | 2026-09-21 | In §2, §3, §4, §5: Focus strictly on Nova's tools without modifying external workflow files. Upgrade read_article in analyzer/tools_service.go to support batch URL extraction via Tavily Extract API (urls: []string), remove restrictive include_domains hard-filter in web_search to allow discovering unverified news, instruct Nova in analyzer/instructions.go to extract all trusted domain URLs simultaneously in a single call, enforce transparent dynamic disclosure when news only exists outside trusted domains ('Ini tidak ada di trusted domain PulsarFi, tapi referensinya ada dan isinya begini'), mandate markdown links [Media](URL) for all cited facts to ensure accountability, and verify via analyzer_tools_test.go. |
| 1.10 | 2026-09-21 | In §4, §5: Verify zero regression across the analyzer-to-executor pipeline in backend/test/service/agent/comet_regression_live_test.go with live trader wallet (0xd8bf50c157a79260c77b25f89ef713e6c3feda6f) on BRPT (BRPTP) and BMRI (BMRIP), confirming seamless routing to analyzer_then_executor, clean evidence gathering, Arm card creation, and Eino pause before execution. |

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
- **MaxIterations Bounded at 8**: In `analyzer/index.go`, `MaxIterations` is set to **`8`** (strictly well below 20). This accommodates multi-step news research and follow-up reference queries without hitting premature `exceeds max iterations` failures or causing HTTP 500 errors.
- **Strict Read Bound & Follow-up Rules in Instructions**: In `analyzer/instructions.go`, Nova is instructed to fetch at most 1 to 2 most relevant articles per turn and synthesize immediately. When asked for references, links, or sources for previously mentioned items, Nova must inspect the conversation context first and execute at most 1 targeted search and 1 read without entering an open-ended loop.
- **Conversation Context Threading in `runAnalyze`**: Pass the last assistant message (if present) into the analyzer request prompt so Nova knows what was previously stated, avoiding blind repetitive searches when the user asks follow-up questions like "where did that reference come from?".
- **Graceful Iteration Recovery**: In `runRoleAgent` (`orchestrator_workflow_service.go`), catch `exceeds max iterations` and recover gracefully using `lastNonEmptyAssistant` or tool results collected so far, never letting an iteration limit bubble up as an unhandled error causing an HTTP 500 response.
- **LLM-Driven Ticker Cleanup & Chart Filtering**:
  - Filtering: In `analyzer/instructions.go`, Nova is explicitly instructed never to invoke `get_stock_chart` proactively on news or general inquiries.
  - Ticker Cleanup: In `analyzer/instructions.go` and `stockChartRequest` schema, Nova is instructed to clean up any ticker before passing it to `get_stock_chart`:
    - Use the official 4-letter IDX equity ticker (e.g. `SINI`, `BUMI`, `BBCA`), stripping any tokenized `'P'` suffix (such as `SINIP` -> `SINI`).
    - Use `'IHSG'` for the composite index (never pass raw index codes like `^JKSE` or append `.JK`).
  - `PriceLineHistory` in `stock_chart_service.go` routes through `PriceService.GetStockHistory`, and Yahoo HTTP 404 / empty responses map to `ErrStockNotFound` for graceful handling.
  - Manual code hacks (`cleanYahooIDXSymbol` and manual Go `TrimSuffix` branches) are eliminated in favor of clean LLM-driven normalization.
- **Batch Article Extraction in `read_article`**:
  - `readArticleRequest` in `analyzer/tools_service.go` supports `urls: []string` (with `url: string` preserved for backwards compatibility).
  - Gated to `TrustedNewsDomains` (`liputan6.com`, `kompas.com`, `bisnis.com`, `market.bisnis.com`, `cnbcindonesia.com`).
  - Calls Tavily Extract API once with all valid URLs (`POST /extract {"urls": [...]}`).
  - Returns `articles: []readArticleItem` for batch extraction and populates top-level fields for single-article calls.
  - Skips failed individual extractions gracefully without returning a fatal error.
- **Open News Discovery in `web_search`**:
  - In `analyzer/tools_service.go`, remove hard-filtered `include_domains` from Tavily Search request so Nova can see all market reporting across the web.
  - Increase `max_results` to 10 so coverage across trusted and non-trusted domains is visible.
- **Transparent Non-Trusted Domain Reporting & Citation Accountability**:
  - In `analyzer/instructions.go`, instruct Nova:
    - When news is found in trusted domains: fetch all of them in a single batch `read_article` call, cross-verify consensus vs bias, and cite them with direct markdown links `[Media](URL)`.
    - When news does NOT exist in trusted domains, but is found in other domains: strictly avoid saying "no news" or "unknown". Explicitly inform the user: *"Ini tidak ada di trusted domain PulsarFi, tapi referensinya ada di [Nama Sumber](URL) dan isinya begini: [ringkasan]."* Note that because it is unverified by PulsarFi's trusted sources, it carries speculative risk (`condition_met: false`, `confidence: low`).
- **Laser Focus on Nova Tools (No Overengineering)**:
  - Changes are strictly isolated to `analyzer/tools_service.go`, `analyzer/instructions.go`, and test verification in `backend/test/service/agent/`.

---

## 4. Impacted Files

| Layer | File | Change |
|---|---|---|
| Backend | `backend/src/service/agent/analyzer/instructions.go` | `[MODIFY]` Instruct Nova to batch extract all trusted URLs, report unverified sources transparently, and attach markdown links |
| Backend | `backend/src/service/agent/analyzer/tools_service.go` | `[MODIFY]` Support `urls: []string` batch in `read_article`, remove restrictive `include_domains` in `web_search` |
| Backend | `backend/test/service/agent/analyzer_tools_test.go` | `[MODIFY]` Add unit tests verifying batch `read_article` and open `web_search` |
| Backend | `backend/src/service/agent/analyzer/index.go` | `[MODIFY]` Set `MaxIterations: 8` |
| Backend | `backend/src/service/agent/orchestrator_workflow_service.go` | `[MODIFY]` Gracefully recover from `exceeds max iterations` in `runRoleAgent` |
| Backend | `backend/src/service/agent/analyzer/chart_service.go` | `[MODIFY]` Update `stockChartRequest.Ticker` jsonschema description with cleanup instructions, export `NewStockChartTool` |
| Backend | `backend/src/service/public/stock_chart_service.go` | `[MODIFY]` Delegate `PriceLineHistory` to `GetStockHistory` |
| Backend | `backend/src/service/public/price_service.go` | `[MODIFY]` Map Yahoo 404 to `ErrStockNotFound` in `GetStockHistory` |
| Backend | `backend/test/service/agent/news_endpoint_live_test.go` | `[MODIFY]` Add multi-turn follow-up test verifying reference query does not hit HTTP 500 |
| Backend | `backend/test/service/agent/news_endpoint_live_test.go` | `[NEW]` End-to-end live test hitting HandleChatMessage and Gin POST /api/v1/agent/chats/:id/messages |
| Backend | `backend/test/service/agent/comet_regression_live_test.go` | `[NEW]` Live pipeline test validating analyzer_then_executor for BRPT and BMRI with trader wallet |
| Backend | `backend/migrations/020_agent_chat_message_news_content_type.sql` | `[NEW]` |
| Frontend | `frontend/components/agent/NewsBrief.tsx` | `[NEW]` |
| Frontend | `frontend/components/agent/ChatThread.tsx` | `[MODIFY]` renders `NewsBrief` for `content_type === 'news'` |
| Frontend | `frontend/http/agent/chatApi.ts` | `[MODIFY]` `content_type` union gains `'news'` |

---

## 5. Verification

- `go build ./src/...` and `go test ./test/service/agent/...` pass cleanly.
- `TestReadArticleTool_ValidationAndDomainAllowlist` confirms live single and batch extraction against trusted articles using Tavily Extract.
- `TestReadArticleTool_Batch` verifies multi-URL batch extraction in 1 tool call.
- `TestReadArticleTool_FallbackNoKey` confirms that when Tavily API key is absent and `go-readability` is invoked, `article.Node == nil` is caught safely without `the Node field is nil`.
- `TestCometRegression_BRPT_And_BMRI` confirms zero regression on the complete Nova-to-Comet actionable pipeline for both BRPT (BRPTP) and BMRI (BMRIP) using the live trader wallet.

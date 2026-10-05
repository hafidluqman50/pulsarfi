# News Brief Evidence Cards

| | |
|---|---|
| **Version** | 1.18 |
| **Status** | In Review |
| **Date Created** | 2026-09-07 |
| **Last Updated** | 2026-10-05 |

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
| 1.11 | 2026-09-21 | In §3, §4: Re-anchor web_search to native Tavily include_domains with tool-level auto-fallback for unlisted news; strip bloated domain rules and hardcoded disclaimer templates from analyzer/instructions.go; delete dead code wrapper fetchArticle in analyzer/tools_service.go; isolate iteration exhaustion recovery strictly to Analyzer in orchestrator_workflow_service.go; eliminate raw JSON tool dump on missing assistant replies. |
| 1.12 | 2026-09-21 | In §3, §4, §5: Eliminate redundant root-level fields (title, content, excerpt, etc.) from readArticleResponse in analyzer/tools_service.go to stop wasteful duplication of articles[0] data and prevent LLM context token inflation. readArticleResponse now cleanly and exclusively returns articles: []readArticleItem; refactor fetchSingleReadability to return readArticleItem directly; update unit tests in analyzer_tools_test.go. |
| 1.13 | 2026-09-21 | In §3, §4, §5: Fix fatal classification bug where NewsBrief card never rendered due to extractNewsEvidence trying to json.Unmarshal Nova's markdown chat prose. Refactor classifyReply to extract news evidence directly from turn.AnalyzerToolCalls (read_article and web_search), populating uiProps and setting content_type: "news" deterministically, which persists to agent_chat_messages and activates <NewsBrief /> in frontend. |
| 1.14 | 2026-10-05 | **`web_search` no longer asks Tavily for images.** Measured directly against `api.tavily.com/search` with the tool's exact request: with `include_images: true` three of ten requests took over 10 seconds (up to 18.3s, close to the tool's 20s client timeout, and one 40s worst case when the empty-result fallback search runs after it), without it four of four finished in 1.5 to 3.5 seconds. The occasional timeout surfaced as `web_search: request failed ... context deadline exceeded`, softened by `WrapToolGraceful` but costing Nova a news source. Decision (maintainer): no images from search. `executeTavilySearch` stops sending `include_images` and stops mapping the `images` array onto results. News card images are unaffected where it matters: `read_article` still returns `image_url` per article (§3), and the markdown-image fallback in `extractNewsEvidenceFromToolCalls` still reads images embedded in a search snippet; only the search-level image list is gone, so a card built purely from `web_search` results may show no picture. |
| 1.15 | 2026-10-05 | **Verification of v1.14, with the numbers.** Paired, interleaved comparison of the same five queries against Tavily with the tool's real request: without `include_images` 2.3, 1.2, 1.4, 5.1, 3.7s (mean 2.7s, max 5.1s); with it 4.7, 11.8, 10.3, 5.7, 7.8s (mean 8.1s, max 11.8s), and the 11.8s request returned 0 results, which is exactly what makes the tool run its second, fallback search. The real `web_search` tool after the change returned 10 results with no search-level image in five of five calls. Its latency was 6.5 to 10.7s in one run and 0.5 to 3.4s in a run minutes later, so Tavily's latency also swings with the time of day independently of this change; removing images removes the part of the variance the tool controls, not all of it. Not done, still the maintainer's call: one retry on timeout and a longer client timeout (currently 20s, `analyzer/tools_service.go`). |
| 1.16 | 2026-10-05 | **Nova's prompt now carries the trusted news domains, generated from the one list in code, with strict rules (maintainer decision, option 1).** Finding: `TrustedNewsDomains` (`analyzer/tools_service.go`) gated `read_article` and filtered `web_search`, but Nova's prompt named no domain at all (the comment claiming a copy "hardcoded into instructions.go" was stale), so live runs showed Nova repeatedly trying `ajaib.co.id`, `kontan`, `infobanknews`, each refused with "not on the trusted domain allowlist" and surfaced as a failed step. Change: `analyzer.New` builds Nova's instruction as the base prompt plus a "Trusted News Sources" section produced by one function from `TrustedNewsDomains`, so the prompt and the code gate can never disagree and no domain is written twice. The section says: read only URLs on those domains and check each before it goes into the `urls` array; a result whose title starts with the external-source marker, or whose domain is not on the list, is external and unverified, never passed to `read_article`, quotable as a snippet only, with the user told so in their language; never invent or guess a URL; if the trusted domains returned nothing useful, say so instead of searching for other domains. The marker (`[Sumber Eksternal]`, added by the empty-result fallback search) becomes one exported constant used by both the tool and the prompt. The code gate in `read_article` is unchanged and stays the last line of defence, because article text is untrusted content. Test: the generated section lists every configured domain and the marker. |
| 1.17 | 2026-10-05 | **Verification of v1.16, and the one failure it did not remove.** Live, two news requests through the real chat flow: zero refusals "not on the trusted domain allowlist" (earlier runs showed four in a single turn), so Nova now stays on the trusted domains. One `read_article` failure per request remained: `failed to extract readable content from provided URLs`. All four URLs involved were real `bisnis.com` articles from search results (`market.bisnis.com`, `premium.bisnis.com`), on the allowlist, and extracting each one by hand against `api.tavily.com/extract` returned `Failed to fetch url` every time, so the cause is that Tavily's fetcher cannot read bisnis.com, not the prompt or the gate. Open decision for the maintainer: remove `bisnis.com` and `market.bisnis.com` from `TrustedNewsDomains` (one edit, the prompt follows automatically, but search then stops returning bisnis.com results too), or keep them and accept a failed step whenever Nova picks one. Tests: `TestTrustedSources_PromptIsGeneratedFromTheList` (offline) and `TestLiveAgentAsTool_NovaStaysOnTrustedDomains` (live, opt-in). |
| 1.18 | 2026-10-05 | **`TrustedNewsDomains` changed (maintainer decision): `bisnis.com` and `market.bisnis.com` removed, `bloomberg.com` and `msci.com` added.** The new list is `liputan6.com`, `kompas.com`, `cnbcindonesia.com`, `bloomberg.com`, `msci.com`; Nova's prompt and the `read_article` gate both follow it automatically (v1.16). Checked against Tavily before the change, search then extract of the first three results per domain: `msci.com` extracts in full (index factsheets and documents, 6,600 to 21,000 characters), `bloomberg.com` extracts but only a teaser of 543 to 733 characters because of the paywall, so Nova can cite it but not read the article body. Not added, only noted: `bloombergtechnoz.com`, Bloomberg's Indonesian-language arm, extracts in full (about 14,000 characters) and covers IDX news; it is a different domain from `bloomberg.com`, so it needs its own entry if wanted. Side effects: §3 and §4 above still name `bisnis.com`/`market.bisnis.com` and are historical; `agent-role-architecture.md` §6a names the old four domains in a corroboration rule that no code implements (grep finds no use of it), so nothing depends on the count; the host display-name mapping for `bisnis.com` in the code is left alone and harmless. |

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
  - `readArticleRequest` in `analyzer/tools_service.go` supports `urls: []string` (with `url: string` preserved for single URL calls).
  - Gated to `TrustedNewsDomains` (`liputan6.com`, `kompas.com`, `bisnis.com`, `market.bisnis.com`, `cnbcindonesia.com`).
  - Calls Tavily Extract API once with all valid URLs (`POST /extract {"urls": [...]}`).
  - Returns `articles: []readArticleItem` exclusively — eliminating redundant root-level fields (`title`, `content`, etc.) and preventing wasteful double-serialization of `articles[0]` in LLM context.
  - Skips failed individual extractions gracefully without returning a fatal error.
  - Removed dead code wrapper `fetchArticle` — single and batch extraction logic is unified directly inside `fetchArticles`.
- **Native Trusted Domain Search with Tool-Level Auto-Fallback in `web_search`**:
  - In `analyzer/tools_service.go`, `web_search` passes `include_domains: TrustedNewsDomains` natively to Tavily Search API.
  - If trusted domain search returns 0 results, the Go tool automatically falls back to a general search without `include_domains`, marking the external results cleanly in Go (e.g. prefixing snippet/title with `[Sumber Publik / Di Luar Trusted Domain PulsarFi]`).
  - Zero prompt bloat: LLM does not need to parse, filter, or triage domain URLs manually.
- **Clean Nova Instructions (Zero Prompt Bloat & Zero Static Disclaimers)**:
  - In `analyzer/instructions.go`, all verbose domain allowlist rules and static disclaimer string templates (`Ini tidak ada di trusted domain PulsarFi...`) are completely removed.
  - Nova focuses strictly on analytical verification: querying tools, batch reading trusted articles, detecting consensus vs bias, providing direct markdown citations `[Media](URL)`, and determining whether the trigger condition is met.
- **Deterministic Tool-Call Evidence Extraction in `classifyReply`**:
  - `classifyReply` in `orchestrator_workflow_service.go` replaces the broken `extractNewsEvidence(turn.AnalyzerReply)` (which wrongly attempted to `json.Unmarshal` Nova's natural-language markdown prose) with deterministic inspection of `turn.AnalyzerToolCalls`.
  - Scans `turn.AnalyzerToolCalls` for `read_article` tool results:
    - Parses `readArticleResponse.Articles` directly from `tc.Result`.
    - Maps `SiteName` -> `newsEvidenceItem.Source`, `URL` -> `newsEvidenceItem.URL`, `PublishedAt` -> `newsEvidenceItem.PublishedAt`, `Excerpt` -> `newsEvidenceItem.Excerpt`, and `ImageURL` -> `newsEvidenceItem.ImageURL`.
  - Provides a secondary fallback for `web_search`: if `read_article` was not called or yielded no articles, inspects `web_search` results, parsing `Source` from URL host and `Excerpt` from snippet content.
  - Deduplicates articles across tool calls by URL.
  - Returns `content_type: "news"` and marshaled `uiProps` containing the evidence array whenever `len(evidence) > 0`.
  - This directly feeds `persistSupervisorReply`, writing `content_type = 'news'` and `ui_props` into PostgreSQL `agent_chat_messages`, emitting the `final` WebSocket event, and activating `<NewsBrief uiProps={message.ui_props} />` on the frontend.

---

## 4. Impacted Files

| Layer | File | Change |
|---|---|---|
| Backend | `backend/src/service/agent/analyzer/instructions.go` | `[MODIFY]` Strip bloated domain allowlist rules and static disclaimer templates; maintain clean analytical instructions |
| Backend | `backend/src/service/agent/analyzer/tools_service.go` | `[MODIFY]` Native Tavily `include_domains` with tool-level auto-fallback; remove dead wrapper `fetchArticle`; eliminate root-level duplication in `readArticleResponse` |
| Backend | `backend/src/service/agent/orchestrator_workflow_service.go` | `[MODIFY]` Scope max-iterations recovery to analyzer only; eliminate raw tool JSON payload leakage; extract news evidence directly from `turn.AnalyzerToolCalls` in `classifyReply` and map to `newsEvidenceItem` with `content_type: "news"` |
| Backend | `backend/test/service/agent/analyzer_tools_test.go` | `[MODIFY]` Update unit tests for native trusted search and batch extraction |
| Backend | `backend/src/service/agent/analyzer/index.go` | `[MODIFY]` Set `MaxIterations: 8` |
| Backend | `backend/src/service/agent/analyzer/chart_service.go` | `[MODIFY]` Update `stockChartRequest.Ticker` jsonschema description with cleanup instructions, export `NewStockChartTool` |
| Backend | `backend/src/service/public/stock_chart_service.go` | `[MODIFY]` Delegate `PriceLineHistory` to `GetStockHistory` |
| Backend | `backend/src/service/public/price_service.go` | `[MODIFY]` Map Yahoo 404 to `ErrStockNotFound` in `GetStockHistory` |
| Backend | `backend/test/service/agent/news_endpoint_live_test.go` | `[MODIFY]` Assert `card.ContentType == "news"` and verify `uiProps` evidence payload structure |
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
- `TestSearchTool_TrustedDomainsAndFallback` verifies that `web_search` leverages native `include_domains` and executes graceful fallback if trusted results are empty.
- `TestCometRegression_BRPT_And_BMRI` confirms zero regression on the complete Nova-to-Comet actionable pipeline for both BRPT (BRPTP) and BMRI (BMRIP) using the live trader wallet.
- `TestLiveNewsQueryEndpoint` in `news_endpoint_live_test.go` confirms `card.ContentType == "news"` and `card.UIProps` contains the parsed evidence array.


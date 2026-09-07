# News Brief Evidence Cards

| | |
|---|---|
| **Version** | 1.0 |
| **Status** | Implemented |
| **Date Created** | 2026-09-07 |
| **Last Updated** | 2026-09-07 |

**Note on scope.** A scoped-down version of a richer "News Brief" design reference the user shared (composite sentiment score, multi-asset comparison tabs, a quarantine view for rejected sources) — this covers only what was explicitly asked for in words: a dated, sourced, linked citation card per evidence item, reusing `TrustedNewsDomains` for legitimacy. The composite score/tabs/quarantine UI are not built.

---

## 1. Problem

Analyzer's real, sourced evidence (gathered via `web_search`/`read_article`, gated to `TrustedNewsDomains`) only ever surfaced as Supervisor's own prose summary of it, or buried in the Sub Task detail view's reasoning JSON — never as its own citation card with a source name, date, excerpt, image, and a link back to the original article, the way a real news brief should look.

---

## 2. What Was Actually Available Already

`read_article`'s underlying library (`codeberg.org/readeck/go-readability/v2`) already parses `Excerpt()`, `SiteName()`, `ImageURL()`, and `PublishedTime()` from an article page's own metadata — none of this was being read before, only `Title()` and the rendered body text. Tavily's own search results carry no publish-date field at all (confirmed against its API docs), so the date has to come from the article page itself via `read_article`, not the search step.

---

## 3. Design

- `readArticleResponse` (`analyzer/tools_service.go`) gained `excerpt`, `site_name`, `image_url`, `published_at` (RFC3339, omitted entirely when the page provides none — never guessed).
- `analyzer/instructions.go`'s evidence contract now asks for `source`/`url`/`published_at`/`excerpt`/`image_url` per item (was just `source`/`summary`), grounded explicitly in what `read_article` returned.
- `buildWorkflowCard` (`task_service.go`) gained a second classification pass: if `analyzer_agent`'s own conclusion JSON has a non-empty `evidence` array, the reply becomes `content_type: "news"` with that array as `ui_props` — checked only after the existing chart check, so a request that legitimately produced both stays a chart card.
- New `agent_chat_messages_content_type_check` migration (`020`) adds `'news'` to the allowed values.
- New `NewsBrief.tsx` renders each item: source badge, formatted date (only if present), excerpt, a 64×64 lead image (only if present), and a link to the original article.

---

## 4. Impacted Files

| Layer | File | Change |
|---|---|---|
| Backend | `backend/src/service/agent/analyzer/tools_service.go` | `[MODIFY]` `readArticleResponse` gains excerpt/site_name/image_url/published_at |
| Backend | `backend/src/service/agent/analyzer/instructions.go` | `[MODIFY]` evidence contract wording |
| Backend | `backend/src/service/agent/task_service.go` | `[MODIFY]` `buildWorkflowCard` news classification, `NewsEvidenceItem`/`extractNewsEvidence` |
| Backend | `backend/migrations/020_agent_chat_message_news_content_type.sql` | `[NEW]` |
| Frontend | `frontend/components/agent/NewsBrief.tsx` | `[NEW]` |
| Frontend | `frontend/components/agent/ChatThread.tsx` | `[MODIFY]` renders `NewsBrief` for `content_type === 'news'` |
| Frontend | `frontend/http/agent/chatApi.ts` | `[MODIFY]` `content_type` union gains `'news'` |

---

## 5. Verification

- `go build`/`go vet`/`gofmt` and `tsc`/`eslint` all clean.
- Migration `020` applied to the live Supabase DB, confirmed via `pg_get_constraintdef`.
- **Not yet verified live:** an actual sentiment/trigger question that exercises `web_search` (now Tavily-backed, API key just configured) end-to-end through to a rendered `NewsBrief` card — pending a real test.

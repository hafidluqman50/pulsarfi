# Dynamic Model Tier / Reasoning-Effort Routing

| | |
|---|---|
| **Version** | 2.0 |
| **Status** | Draft |
| **Date Created** | 2026-09-06 |
| **Last Updated** | 2026-09-07 |

| Version | Date | Change |
|---|---|---|
| 2.0 | 2026-09-07 | **The escalation decision itself — what actually sets `Depth`/picks the model — is redesigned from pure LLM text-interpretation into two independent, code-computed signals. Not implemented yet, design only; see §5.** Traced live: `req.Depth` (`supervisor/tools_service.go`) was populated directly from Quasar's own tool-call JSON with zero Go-level validation — unpredictable, and explicitly flagged as indefensible in front of hackathon judges/VCs ("Ini akan jadi pertanyaan dari juri atau VC"). Considered switching to Flash-only and dropping tiering entirely; researched live (§2 restated below with the newer, more relevant benchmark) and reversed course — DeepSeek V4 Flash matches or beats Pro on generic agentic tool-use (Terminal Bench 2.1: Flash 82.7 vs Pro-Preview 72.1), but on **ICBCBench specifically (financial deep research, the closest real proxy to what Analyzer does)**, Pro produced the strongest open-agentic-framework result; SimpleQA-Verified factual recall still shows Pro ~2x ahead. For a trading agent, ICBCBench is the more relevant signal than generic tool-use parity — Pro genuinely needs to run more often than a pure cost-minimization design would like, "demi menang dan demi matang". §5 replaces the free-text `Depth` field with (1) a capability-count complexity score Quasar enumerates but code thresholds, and (2) a fully code-driven, model-invisible re-verification gate on actionable trigger conclusions |
| 1.3 | 2026-09-07 | **v1.2 never actually changed which model ran — found live against real DeepSeek billing: a "quick" chart-only request still billed at Pro rates.** `reasoningEffort` (v1.2) only overrides the reasoning-effort parameter *within* whichever model was already configured; Analyzer's underlying model itself was still hardcoded to `external.ModelPro` at construction time in `agent_registry.go`, completely unconditionally — `Depth` never touched it. Fixed properly this time: Analyzer is now built twice (`analyzer.New` called twice, once per model — cheap, since the tools it builds are stateless closures over already-shared readers/services), `analyzerAgentQuick` (Flash) and `analyzerAgentDeep` (Pro). `supervisor.New`/`newAnalyzerTool` now take both, and pick which one actually runs per-call based on `req.Depth` — the reasoning-effort override from v1.2 stays too, applied on top of whichever model gets picked (so "deep" is Pro *and* max effort, not one or the other). `go build`/`go vet`/`gofmt` clean. Live cost verification (confirming a "quick" chart request now actually bills at Flash rates) still pending |
| 1.2 | 2026-09-06 | **Implemented as designed in v1.1.** `RunAgentWithTrace`/`RunAgentWithHistory` (`llm_service.go`) gained a `reasoningEffort string` parameter — empty leaves the model's own default untouched (every existing Supervisor-level call site passes `""`, unaffected); non-empty applies via `adk.WithChatModelOptions([]model.Option{einoopenai.WithReasoningEffort(...)})`. `routeRequest` (`supervisor/tools_service.go`) gained `Depth` ("quick"/"deep"), read by `newAnalyzerTool` via a new `reasoningEffortFor(depth)` mapping ("deep" → "max", otherwise no override). `newExecutorTool` ignores `Depth` and always requests "max" — any Executor call means an action might actually be taken, inherently the highest-stakes case regardless of what led there. `supervisor/instructions.go` gained a dedicated section reinforcing "quick" as the default and naming the exact two cases that warrant "deep", since getting this wrong (defaulting to "deep") silently reintroduces the same blanket-Pro-cost problem this whole feature exists to fix. `go build`/`go vet`/`gofmt` clean. Live cost/behavior verification (confirming a chart request actually runs cheaper/faster at default effort, and an actionable trigger genuinely gets "max") still pending |
| 1.1 | 2026-09-06 | **Option 1 confirmed feasible against eino's actual source, mechanism designed.** `adk.Agent.Run` (`= TypedAgent[*schema.Message].Run`) accepts `...AgentRunOption`; `adk.WithChatModelOptions([]model.Option{...})` is one, and `TypedChatModelAgent.Run` (`chatmodel.go:1677-1678`) forwards it into the actual model call via `compose.WithChatModelOption(...)` — confirmed by reading the library, not assumed. `eino-ext/libs/acl/openai`'s `ReasoningEffortLevel` (`option.go:28`) is a plain `string` type, so `"max"` works even without eino-ext's own Low/Medium/High constants, matching DeepSeek's own documented low/high/max range. Design: `RunAgentWithTrace`/`RunAgentWithHistory` (`llm_service.go`) gain a `reasoningEffort string` parameter, passed to `a.Run` via `adk.WithChatModelOptions` only when non-empty (empty = today's default, unchanged). Supervisor decides the level per call: `routeRequest` (`supervisor/tools_service.go`, already used for `analyzer_agent`/`executor_agent`) gains a `Depth` field — `"quick"` (default: chart/portfolio lookups, general sentiment reading) or `"deep"` (explicit user request for deep/quant analysis, or an actionable trigger whose conclusion could authorize a real trade — the "hallucination-sensitive" case §2 identifies) |
| 1.0 | 2026-09-06 | Initial version, research only |

**Note on scope.** This plan covers how much reasoning weight (model tier and/or reasoning effort) a given Analyzer/Executor call actually gets, decided per-request instead of hardcoded. It does not cover the routing logic between Supervisor/Analyzer/Executor themselves (unchanged) or the tools each role has access to.

---

## 1. Problem

`agent_registry.go` hardcodes Analyzer and Executor to `external.ModelPro` ("costlier, reserved for the rarer... analysis call") for every single call, regardless of what the request actually needs. Live testing this session showed every chart/portfolio lookup — a pure data-fetch-and-narrate task with no judgment risk — pays full Pro-tier cost, the same as a real trigger-condition evaluation that could authorize an actual trade. Flagged live: "Jangan apa apa ke pro, ini butuh bahasan yang dalem kalau misal butuh pro" and "kalau sentiment masa flash gak cukup?" — questioning whether the Pro-always assumption is even justified.

---

## 2. Research Findings (DeepSeek V4 Flash vs Pro, Aug 2026 release)

| Benchmark | Flash | Pro | Gap |
|---|---|---|---|
| MMLU-Pro (general reasoning) | 83.0–86.2 | 82.9–87.5 | Nearly identical |
| Codeforces (complex multi-step logic) | 2816–3052 | 2919–3206 | Small |
| SimpleQA-Verified (factual accuracy / hallucination resistance) | 23.1–34.1 | 45.0–57.9 | **~2x, Pro clearly ahead** |

Vendor's own guidance: **"Flash@max ≈ Pro@high" on reasoning** — the real gap is hallucination resistance (SimpleQA), not general reasoning capability. Official recommendation reserves Pro for "hallucination-sensitive enterprise stacks, deep agentic loops, and frontier reasoning," not blanket use. Flash output is ~3.1x cheaper than Pro.

**Implication for this app:** a chart/portfolio lookup has no hallucination-sensitive judgment call at all (the tool returns real data, the model just narrates it) — Flash is not just adequate, it's the benchmark-appropriate choice. A trigger-condition evaluation that could authorize a real trade is exactly the "hallucination-sensitive" case the vendor's own guidance calls out for Pro. General sentiment reading sits in between and, per the near-identical MMLU-Pro/Codeforces numbers, is likely fine on Flash, especially at a higher reasoning-effort setting.

---

## 3. Two Implementation Paths Considered

| Option | Mechanism | Status |
|---|---|---|
| **1 (chosen, implementing)** | Extend `RunAgentWithTrace`/`RunAgentWithHistory` to accept a per-call `model.Option` (`einoopenai.WithReasoningEffort`), so Supervisor's own routing tools can pick low/medium/high(/max) effort per request without needing separate agent instances | Confirmed end-to-end: `eino-ext/components/model/openai` exposes `WithReasoningEffort` as a `model.Option`, and `adk.Agent.Run`'s `AgentRunOption`s (`adk.WithChatModelOptions`) are confirmed by reading `chatmodel.go` to actually reach the underlying model call. See §2 changelog v1.1. |
| 2 (fallback if 1 doesn't pan out) | Build multiple pre-configured Analyzer agent instances at boot (e.g. `analyzerQuick` at Flash+Medium, `analyzerDeep` at Flash+High or full Pro), Supervisor picks which one to call — same pattern it already uses to pick between `analyzer_agent`/`executor_agent` | Not started; architecturally certain to work (no unverified internals), but coarser-grained (agent-instance level, not per-call) and needs N agent instances instead of 1 |

---

## 4. Open Questions

- ~~Does `adk.ChatModelAgent`/`RunAgentWithTrace` support injecting a per-call `model.Option`...~~ Resolved v1.1/v1.3: yes, and model-instance switching (not just reasoning-effort override) was needed to actually change billing.
- ~~What concretely triggers "this needs more reasoning weight"...~~ Resolved, see §5 — no longer an explicit user phrase or a free-text model guess.
- Does DeepSeek's real API accept an effort value beyond eino-ext's exposed Low/Medium/High constants (the vendor benchmark also mentions "max")? Not yet checked against the raw API.

---

## 5. Escalation Mechanism v2 — Measurable, Not Pure Text Interpretation

**Architectural home.** This gate is now also recorded in `agent-role-architecture.md` §6a as a fifth code-enforced check, the same class as that document's corroboration/persistence/quarantine/market-hours gates — this section stays the mechanical detail (exact struct fields, threshold function), §6a states its place in the architecture.

**Problem with v1.3.** `routeRequest.Depth` (`"quick"`/`"deep"`) is a plain string Quasar's own tool call fills in, per a jsonschema description telling it when to pick each — but nothing in Go ever checks or computes it. Confirmed by reading `newAnalyzerTool`: `req.Depth` flows straight from the model's own JSON into `if req.Depth == "deep"`, no validation, no fallback logic. This is unpredictable (Quasar can misjudge either direction) and, more importantly, indefensible: "why did this request cost more" has no answer besides "the model decided to write 'deep' here", which does not hold up to a judge or investor asking how cost/model-tier decisions are actually made.

**Design goal, stated explicitly by the user:** text interpretation is fine and unavoidable for an LLM agent — the *escalation* decision specifically needs a real, presentable, code-computed signal behind it, not eliminated free-text judgment altogether.

**Two independent signals, OR'd together — either one escalates to deep:**

| # | Signal | Measures | Computed by |
|---|---|---|---|
| 1 | **Capability count** | Does this request need to synthesize 2+ distinct analysis lenses together (sentiment, technical, fundamental) — the genuine "financial deep research" case ICBCBench rewards Pro for | Code counts a list Quasar enumerates; Quasar never states a depth or a score directly |
| 2 | **Actionable + condition_met** | Is this conclusion about to authorize a real trade — the hallucination-sensitive case SimpleQA-Verified flags Pro for | Code only; Analyzer/Quasar never asked to decide this at all |

**Signal 1 — capability count.** `routeRequest.Depth` is replaced by `Capabilities []string`, an enum of exactly `"sentiment"`, `"technical"`, `"fundamental"` — Quasar lists which lenses the request genuinely needs (still a text-interpretation step, but now an auditable, countable list instead of a raw depth claim). A new pure function, `depthForCapabilities(capabilities []string) string`, returns `"deep"` when `len(capabilities) >= 2`, else `"quick"` — code decides the threshold, not the model.

Deliberately excluded from this score: time-range breadth (e.g. "sejak Januari") and ticker count (e.g. "BBCA dan BBRI"). Both were considered and rejected — they measure *tool-call volume* (already bounded by `MaxIterations: 20`), not *reasoning depth*. Folding them into the same score caused single-lens requests with a wide date range or multiple tickers to falsely trigger deep, diluting the signal without actually needing Pro's better synthesis.

**Signal 2 — actionable + condition_met, fully code-driven.** `RunContext` gains a new field, `IsActionable bool`, set once inside `newCreateTaskTool` alongside `rc.TaskID` (no extra DB round-trip — it is already in `createTaskRequest.IsActionable` at that point). After a quick-tier Analyzer run returns its conclusion, if `rc.IsActionable` is true, the conclusion string is defensively parsed as JSON (mirroring the frontend's own `StructuredOrProse` pattern — Analyzer's reply is prose-or-JSON depending on how it chose to answer, never enforced strictly by the tool schema). If it parses and `condition_met == true`, the exact same request is silently re-run against the deep-tier model at max effort before Supervisor ever sees an answer — a transparent re-verification, not something Quasar has to ask for or even know happened. A conclusion that fails to parse, or parses with `condition_met == false`, never escalates via this path — the failure mode is "stays quick", never a false escalation.

**Defensibility.** Both signals are countable and loggable per request — a demo can show, for a real request, "capability_count: 3, escalated: true" or "is_actionable: true, condition_met: true, re-verified on Pro" as an actual trace, not a claim about what the model probably meant.

**Not covered here, deferred:** the Nova↔Quasar review/confirm loop for standing/recurring instructions, a scheduler for `TaskService.Evaluate` (currently zero callers), and Executor risk-profile/drawdown instructions — these are a separate, larger architectural piece the user has flagged as "yang bestnya" but explicitly not scoped into this document.

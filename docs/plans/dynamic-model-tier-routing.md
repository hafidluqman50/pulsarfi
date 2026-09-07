# Dynamic Model Tier / Reasoning-Effort Routing

| | |
|---|---|
| **Version** | 1.2 |
| **Status** | Implemented |
| **Date Created** | 2026-09-06 |
| **Last Updated** | 2026-09-06 |

| Version | Date | Change |
|---|---|---|
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

- Does `adk.ChatModelAgent`/`RunAgentWithTrace` support injecting a per-call `model.Option`, or is reasoning effort only settable at `ChatModelConfig` construction time? Being verified now against eino's actual source.
- What concretely triggers "this needs more reasoning weight" — an explicit user phrase ("analisa mendalam", "pake quant"), a request classification Supervisor itself makes, or something else? Not yet decided.
- Does DeepSeek's real API accept an effort value beyond eino-ext's exposed Low/Medium/High constants (the vendor benchmark also mentions "max")? Not yet checked against the raw API.

# BUSINESS.md & Docusaurus — V4 Sync and AI Agent Documentation

| | |
|---|---|
| **Version** | 1.0 |
| **Status** | Draft |
| **Date Created** | 2026-09-27 |
| **Last Updated** | 2026-09-27 |

| Version | Date | Change |
|---|---|---|
| 1.0 | 2026-09-27 | Initial version |

---

## 1. Problem Statement

BUSINESS.md and 3 of the 11 public Docusaurus pages (`overview.md`, `app-flow.md`, `judge-demo-access.md`) still describe the pre-cutover Uniswap V2 architecture as current, despite the V4 cutover (PR #23/#27) already being live and reflected in `architecture.md`, `protocol-design.md`, and `business-flow.md`. BUSINESS.md additionally names specific real institutions (Mirae Asset, Mandiri Sekuritas, Ajaib, Growin) as current custodians, contradicting its own Roadmap table which says V1 custodian seats are internal-team-only. Separately, the entire AI trading-agent product surface (Quasar/Nova/Comet, Task/TradePermission/Sub Task/Trade, `AgentTaskManager.sol`) — a substantial, partly-production part of the codebase — is undocumented in both BUSINESS.md and the public docs site, so a reader relying on either would not know the feature exists.

## 2. Definition of Done

- No page under `docs/content/` or `BUSINESS.md` references Uniswap V2 as the current trading venue, `PulsarProtocol.swap` (nonexistent function), or an "AMM as Uniswap V2 Pool" label.
- BUSINESS.md's custodian description matches its own Roadmap (V1 = internal team) and does not name specific unaffiliated financial institutions.
- BUSINESS.md's Roadmap no longer lists already-shipped Uniswap V4 hooks as a future item.
- BUSINESS.md contains a new section documenting the AI trading agent as a product differentiator, explicitly stating it is not a separate fee mechanism.
- A new Docusaurus page documents the AI trading agent (roles, Task/TradePermission/Sub Task/Trade concept, on-chain guardrails, scalp-production/swing-deferred status) and is reachable from the sidebar and from `overview.md`'s reading order.

## 3. Feature Description

Two kinds of change, applied together:

1. **Factual sync (no new content, just correcting stale claims):** `overview.md`, `app-flow.md`, `judge-demo-access.md`, and BUSINESS.md's fee/mint/roadmap sections get their V2 references replaced with the V4 wording already established in `business-flow.md`/`architecture.md`. BUSINESS.md's custodian paragraph is reworded to drop named institutions and match its own Roadmap.
2. **New content:** a business-level "AI Trading Agent" section is added to BUSINESS.md, and a matching (more detailed) Docusaurus page is added under the **Product** sidebar category, both describing the Quasar/Nova/Comet roles, the Task → TradePermission → Sub Task → Trade lifecycle at a product (not Solidity-struct) level, on-chain guardrails (`totalBudget`, `maxAmountPerTrade`, `cooldownInterval`), and current status (scalp production-hardened per `docs/plans/onchain-scalp-regression-matrix.md`; swing deferred per `docs/plans/swing-trade-auto-exit-and-monitoring.md`).

No code, contract, or backend/frontend changes — documentation only.

## 4. Impacted Files

| File | Change |
|---|---|
| `BUSINESS.md` | `[MODIFY]` — custodian wording, V4 fee/mint terminology, roadmap correction, new AI agent section |
| `docs/content/overview.md` | `[MODIFY]` — V4 wording, new bullet + user group + reading-order link for AI agent |
| `docs/content/app-flow.md` | `[MODIFY]` — `swap` → `swapV4` |
| `docs/content/judge-demo-access.md` | `[MODIFY]` — sequence diagram label V2 → V4 |
| `docs/content/ai-trading-agent.md` | `[NEW]` — AI trading agent product page |
| `docs/sidebars.ts` | `[MODIFY]` — add `ai-trading-agent` to Product category |

## 5. Verification Plan

**Automated tests:** none applicable (docs-only change); run `npm run build` inside `docs/` to confirm Docusaurus still builds (new page registered correctly, no broken internal links).

**Manual verification:**
- Grep `docs/content/` and `BUSINESS.md` for `Uniswap V2`, `PulsarProtocol.swap` (without `V4`), and `Uniswap V2 Pool` — expect zero matches.
- Confirm BUSINESS.md's custodian paragraph no longer names Mirae Asset/Mandiri Sekuritas/Ajaib/Growin.
- Confirm new AI agent page renders in local `docusaurus start` and is linked from the sidebar and `overview.md`.

## 6. Decisions Requiring Review

> [!IMPORTANT]
> **Custodian wording change removes named real institutions entirely (per your answer), rather than reframing them as a V2 target list.** If judges/investors expect to see concrete example names for credibility, this trades that off for internal consistency with the Roadmap.

## 7. Open Questions

None — scope and wording confirmed in chat prior to this plan.

---

## Riwayat Prompt

### v1.0 — 2026-09-27
> Request to read pulsarfi fully and align BUSINESS.md and the Docusaurus docs; branch to be created from main.

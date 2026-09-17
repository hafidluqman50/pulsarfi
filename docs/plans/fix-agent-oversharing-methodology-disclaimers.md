# Fix: Agents Volunteer Unprompted Methodology and Routing Disclaimers

| | |
|---|---|
| **Version** | 1.1 |
| **Status** | Implemented |
| **Date Created** | 2026-09-17 |
| **Last Updated** | 2026-09-17 |

| Version | Date | Change |
|---|---|---|
| 1.1 | 2026-09-17 | Extended the same zero-tolerance principle to a second, related pattern flagged live: Comet narrating its own internal routing decision to the user (e.g. "karena ini perintah langsung dari Supervisor, saya eksekusi tanpa menunggu analisis Nova") — restating a choice the user already made themselves as if it were a justification, and exposing internal role names as process commentary. New bullet added to the same `GlobalInstructions` section, explicit that concrete transaction facts (wallet position, order validity, size executed) are never covered by this rule — only the meta-commentary about which internal role handled the request and why. |
| 1.0 | 2026-09-17 | Initial draft, implemented immediately per direct instruction. |

## Problem Statement

### Root Cause Analysis

Flagged live: asked to check liquidity/slippage/gas, an agent's reply opened with an unprompted paragraph confessing its own tool's limitations ("tool yang saya punya cuma membaca harga spot dari pool, dan itu valuasi linear yang tidak memodelkan kedalaman likuiditas, slippage, atau price impact... Saya juga tidak punya pembacaan estimasi biaya gas terpisah..."). `executor/instructions.go`'s Objective section (line 14) already tells Comet internally that `get_spot_price` "values linearly and does NOT model slippage or price impact" — correct as internal reasoning guidance for sizing decisions, but nothing stopped the model from repeating that internal caveat verbatim to the user as a lengthy self-critical disclaimer instead of just stating the number and acting on it.

This is a Voice/tone problem, not specific to one agent — `GlobalInstructions` (`instructions.go`), shared across Quasar/Nova/Comet, had no rule against volunteering this kind of methodology confession.

**v1.1 found a second instance of the same underlying pattern**: Comet's reply to a direct executor-only order included "Karena ini perintah langsung dari Supervisor, saya langsung mengeksekusi penuh sesuai instruksi tanpa menunggu analisis Nova" — narrating its own internal routing decision (why Nova was skipped) instead of just reporting the trade. The `consult_nova` choice being explained back was something the user themselves already answered during intake; restating it as a justification is redundant, and naming "Supervisor"/"analisis Nova" exposes internal architecture as if it were relevant trading information.

## Definition of Done

- [x] No agent volunteers an unprompted paragraph about what its own tools/data/calculations do not account for.
- [x] An agent still explains a specific limitation if the user directly asks about that exact thing (this is not a ban on honesty when asked — only on volunteering it unprompted).
- [x] No agent narrates its own internal routing decision (which role handled the request, why another role was or was not consulted) back to the user.
- [x] Concrete transaction facts (wallet position, order validity window, size executed) are always still stated — only the process meta-commentary is banned.
- [x] Applies uniformly to Quasar, Nova, and Comet (added to `GlobalInstructions`, not one role's prompt).

## Feature Description

Two zero-tolerance bullets added to `GlobalInstructions`'s "# System Context" section: (1) agents state the number/result and move on, never volunteering a disclaimer about spot-price-only valuation, missing slippage/liquidity-depth modeling, or missing gas estimates unless the user explicitly asked about that specific limitation; (2) agents never explain which internal role handled a request or why another role was skipped, and never restate a choice the user already made as if it were a justification — while still always stating the concrete facts that describe or justify the action itself.

## Impacted Files

| File | Change |
|---|---|
| `backend/src/service/agent/instructions.go` | `[MODIFY]` Two zero-tolerance bullets in `GlobalInstructions`: unprompted methodology disclaimers (v1.0), unprompted internal-routing narration (v1.1) |

## Verification Plan

**Automated tests**: none — prompt wording, not testable via unit test.

**Manual verification**: ask an agent to check liquidity/slippage/gas for a trade and confirm the reply states the figure plainly without a self-critical methodology paragraph. Separately, directly ask "does this account for slippage?" and confirm the agent still answers that specific question honestly when asked. Trigger an executor-only order (consult_nova: no) and confirm the reply reports the trade facts (position, validity, size) without explaining that Nova was skipped or naming "Supervisor".

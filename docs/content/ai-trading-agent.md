---
id: ai-trading-agent
title: AI Trading Agent
sidebar_label: AI Trading Agent
slug: /ai-trading-agent
---

PulsarFi ships a three-role AI agent system that lets a user delegate spot
trades in natural-language chat instead of manually building each swap. It is
a UX and execution layer on top of the existing protocol — every trade the
agent places still settles through the same `swapV4` path, pays the same
protocol fee (`swapFeeBps`), and is subject to the same on-chain rules as a
manual trade described in [Market & Revenue Model](./business-flow). The agent
does not introduce a new fee mechanism.

## Roles

| Role | Job |
| --- | --- |
| Quasar (Supervisor) | Conversational entry point. Routes the request, asks clarifying questions, and auto-settles direct trades. |
| Nova (Analyzer) | Technical analysis, news synthesis, stock charts, and portfolio snapshots that inform the trade decision. |
| Comet (Executor) | On-chain execution — sizing calculation and spot swaps via `AgentTaskManager`. |

A single user instruction can pass through one, two, or all three roles
depending on what it asks for. A request for "what's the latest news on
BUMIP" only needs Quasar and Nova; a request to actually trade also involves
Comet.

## Task, TradePermission, Sub Task, Trade

The domain model deliberately keeps identity, budget, and execution as
separate concepts, so that informational activity (routing, reading news,
concluding there's nothing to do) is represented on-chain just as much as an
actual trade is:

| Concept | What it is |
| --- | --- |
| Task | An identity and lifecycle status for one user instruction — never a money amount. Carries a short, human-readable summary of what was asked. |
| TradePermission | A budget ceiling and expiry, granted separately from Task creation, only once Executor has concluded a trade is actually warranted for that Task. |
| Sub Task | One step recorded by whichever agent performed it (Supervisor routing, Analyzer's gathering/conclusion, Executor's decide) — the readable trail of how a Task was handled. |
| Trade | An actual execution against a Task — ticker, direction, amount, and a readable summary of why it happened. Zero, one, or many can exist per Task. |

This means a Task is never locked to one ticker or one side up front: budget
and trade details only get attached once the agent has actually decided a
trade is warranted.

## On-chain guardrails

`AgentTaskManager` enforces the limits a user grants, independent of anything
the agent itself decides:

- `totalBudget` — the overall ceiling a Task's `TradePermission` cannot exceed.
- `maxAmountPerTrade` — a per-tranche ceiling, used for DCA-style or standing
  orders so a single execution can never consume the whole budget at once.
- `cooldownInterval` — the minimum time between executions against the same
  Task.

The real custody boundary is the user's own ERC-20 `approve()` to
`AgentTaskManager`'s address — independent of any of the above. Every
execution checks both the recorded `TradePermission` and the live on-chain
allowance, and only proceeds if the requested amount fits under both. The
agent can never move more than the user explicitly authorized, and never
holds a standing balance beyond the lifetime of a single execution call.

## Status

| Trade shape | Status |
| --- | --- |
| Scalp (short-horizon, immediate execution) | Production-hardened — validated end-to-end against live Postgres and live Arbitrum Sepolia contracts. |
| Swing / longer-horizon (auto-exit, standing price guardrails) | Deferred. Still in design; today it has no on-chain behavior distinguishing it from scalp beyond a longer expiry clock. |

## Where this fits in the product

The agent surface sits alongside the manual swap UI described in
[App & Operator Flow](./app-flow) — same wallet, same `PulsarProtocol` proxy,
same fee split described in [Fee distribution](./business-flow#fee-distribution).
It is an additional way to reach the same trade, not a parallel product.

# Fix: Sell Allowance Token Address Missing in Trigger Description

| | |
|---|---|
| **Version** | 1.1 |
| **Status** | Implemented |
| **Date Created** | 2026-09-16 |
| **Last Updated** | 2026-09-16 |

| Version | Date | Change |
|---|---|---|
| 1.0 | 2026-09-16 | Initial draft |
| 1.1 | 2026-09-16 | Status updated to Implemented — `resolveStockContractAddress` helper + `buildTriggerDescription` param change shipped in `orchestrator_workflow_service.go` |

---

## Problem Statement

When a user prompts a **sell** trade, the ERC-20 `approve` step for the stock token is silently skipped, causing the on-chain `executeTrade` call to revert with `allowance = 0`.

**Root cause:** `buildTriggerDescription` (in `orchestrator_workflow_service.go`) writes `stock_ticker` and `token_symbol` for sell tasks, but **never writes `stock_contract_address` or `token_address`**. The frontend's `extractTaskTokenDetails` in `PlanCard.tsx` reads `parsed.token_address || parsed.stock_contract_address` to determine which ERC-20 to approve — because neither field exists in the DB row, both evaluate to `undefined`.

By contrast, **buy** is unaffected: the IDRX address is hardcoded from `NEXT_PUBLIC_IDRX_ADDRESS` and never reads from `trigger_description`.

### Failure chain

```
User: "Jual BRPTP 20 token"
  → buildTriggerDescription writes: side, resolved_ticker, stock_ticker, token_symbol, sell_amount
  → DB trigger_description: NO "stock_contract_address", NO "token_address"
  → PlanCard.extractTaskTokenDetails: parsed.token_address = undefined, parsed.stock_contract_address = undefined
  → ArmPanel: effectiveTokenAddress = undefined
  → needsApproval = isActionable && !!effectiveTokenAddress && ... = false
  → Approve step SKIPPED
  → executeTrade on-chain → REVERT (stock token allowance = 0)
```

---

## Definition of Done

1. `trigger_description` for sell tasks includes `"stock_contract_address": "<0x...>"`.
2. `ArmPanel` receives a defined `effectiveTokenAddress` for sell tasks.
3. `needsApproval = true` for sell tasks that have a valid token address.
4. The ERC-20 `approve` step fires for the stock token (not IDRX) on sell.
5. Buy flow is completely unaffected.

---

## Feature Description

### Fix location: backend — `buildTriggerDescription` in `orchestrator_workflow_service.go`

`Orchestrator` already carries a `Stocks StockCatalog` field with `FindByTickerOrIdxTicker`. The fix adds a single DB lookup (stock catalog — read-only, no fund movement risk) inside `buildTriggerDescription` when `side == "sell"` and `ResolvedTicker` is set, then writes the result as `"stock_contract_address"` into the trigger map.

`buildTriggerDescription` is a pure helper today (no IO). To keep the signature minimal, the lookup is lifted one level up into `commitTask` and `updateTask` — both callers already hold the Orchestrator pointer (`o.Stocks`) and a `context.Context`. The resolved address is passed into `buildTriggerDescription` as an optional extra arg.

**No frontend changes required.** `PlanCard.extractTaskTokenDetails` already reads `parsed.stock_contract_address` — once the backend writes it, the field is available and `effectiveTokenAddress` resolves correctly.

---

## Impacted Files

| File | Change |
|---|---|
| `backend/src/service/agent/orchestrator_workflow_service.go` | `buildTriggerDescription` accepts new `stockContractAddress string` param; `commitTask` and `updateTask` do stock lookup before calling it |

---

## UI/UX Changes

None. The approve step for the stock token will now appear in MetaMask for sell tasks, exactly mirroring the IDRX approve for buy tasks.

---

## Flowchart

```
commitTask / updateTask
  │
  ├─ side == "sell" && ResolvedTicker != ""
  │     └─ o.Stocks.FindByTickerOrIdxTicker(ctx, ResolvedTicker)
  │           ├─ found && ContractAddress != nil → stockContractAddress = *ContractAddress
  │           └─ not found / nil               → stockContractAddress = ""
  │
  └─ buildTriggerDescription(turn, decision, card, side, stockContractAddress)
        ├─ existing fields unchanged
        └─ if stockContractAddress != "" → triggerMap["stock_contract_address"] = stockContractAddress
```

---

## Verification Plan

1. Prompt a sell task (e.g. "Jual BRPTP 20 token scalp") and arm it.
2. Query DB: `SELECT trigger_description FROM agent_tasks WHERE ... ORDER BY id DESC LIMIT 1` — confirm `stock_contract_address` field is present and equals the PulsarStock contract address.
3. Confirm ArmPanel shows the approve step with the stock token address (not IDRX).
4. Confirm MetaMask popup fires an ERC-20 `approve` for the stock token.
5. Confirm `executeTrade` succeeds on-chain after approve.
6. Run a buy task end-to-end — confirm IDRX approve and execution still work unchanged.

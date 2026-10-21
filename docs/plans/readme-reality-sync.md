# README Reality Sync

| | |
|---|---|
| **Version** | 1.0 |
| **Status** | Draft |
| **Date Created** | 2026-10-20 |
| **Last Updated** | 2026-10-20 |

| Version | Date | Change |
|---|---|---|
| 1.0 | 2026-10-20 | Initial draft. |

## 1. Problem Statement

Both READMEs describe a protocol that no longer exists. They say swaps go
through a Uniswap **V2 Router**, describe `swap(...)`, `redeem(...)` and a
single-step custodian redeem, list `IDRX.sol` at the contract root, and tell a
deployer to provide `UNISWAP_V2_ROUTER`. The code on `main` is V4-only
(`PoolManager` + `PulsarSwapHook`), the swap entry point is `swapV4`, redeem is
a three-step request/approve/execute flow with KYC enforced only at redemption,
the IDRX mock lives in `src/mocks/`, and the deploy script takes
`PRIVATE_KEY`, `TREASURY`, `WETH` and `CUSTODIAN_1..5`.

The backend README is silent about the agent system that now runs the product:
Quasar/Nova/Comet on CloudWeGo Eino, the on-chain `AgentTaskManager`
guardrails, and the foldered `service/agent` layout. A reader following the
current README would deploy the wrong venue and look for files that moved.

## 2. Definition of Done

1. Every contract name, function name, parameter list, env var and file path
   mentioned in either README exists on `main` as described.
2. No claim about Uniswap V2 as the live venue remains (V2 helpers, if
   mentioned, are labelled legacy).
3. The agent system is described: the three roles, `AgentTaskManager`
   guardrails, and where the backend code lives.
4. Token naming follows `AGENTS.md` §5: tickers are ALL-CAPS with a `P` suffix.
5. No new files beyond this plan, `README.md`, `smart-contract/README.md`.

## 3. Feature Description

| Current claim | Reality on `main` |
|---|---|
| Uniswap V2 Router | Uniswap V4 `PoolManager` + `PulsarSwapHook` (`src/v4/`) |
| `swap(ticker, amountIn, amountOutMin, buyStock)` | `swapV4(ticker, amountIn, minOut, buyStock)` |
| `redeem(ticker, user, amount, attestationHash)` | `requestRedeem(ticker, tokenAmount)` → `approveRedeem(requestId)` → `executeRedeem(requestId)` |
| `executeMint(proposalId)` | `executeMint(proposalId, sqrtPriceX96)` — lazy `PulsarStock` deploy + V4 pool seed |
| `IDRX.sol` | `src/mocks/IDRX.sol` (+ `IDRXFaucet.sol`) |
| `UNISWAP_V2_ROUTER` | `PRIVATE_KEY`, `TREASURY`, `WETH`, `CUSTODIAN_1..5`; `V4Cutover.s.sol` calls `configureV4(poolManager, hook)` |
| No agent system | `AgentTaskManager` + Quasar/Nova/Comet (Eino) + `service/agent/{orchestrator,chat,task,...}` |
| "pStock" generically | `PulsarStock` contracts; tickers `BUMIP`, `ENRGP`, `BRPTP`, … |

## 4. Impacted Files

- `README.md`
- `smart-contract/README.md`
- `docs/plans/readme-reality-sync.md` (this document)

No Go, Solidity or frontend code changes.

## 5. Code Shape

Documentation only; there is no code shape. Every statement added is backed by
a file on `main`:

| Statement | Verified in |
|---|---|
| `swapV4(string,uint256,uint256,bool)` | `smart-contract/src/PulsarProtocol.sol:437` |
| `requestMint`, `executeMint(uint256,uint160)` | `PulsarProtocol.sol:227,267` |
| `requestRedeem`, `approveRedeem`, `executeRedeem` | `PulsarProtocol.sol:301,150,330` |
| `configureV4(address,address)` | `script/V4Cutover.s.sol:110` |
| Hook fee | `src/v4/PulsarSwapHook.sol` (`_getHookFee`, `handleHookFees`) |
| `createTask`, `grantTradePermission`, `recordSubTasks`, `executeTrade`, `cancelTask` | `src/AgentTaskManager.sol` |
| Deploy env | `script/Deploy.s.sol:12-22` |
| Eino | `backend/go.mod:10` (`cloudwego/eino v0.9.15`) |
| Agent folders | `backend/src/service/agent/{orchestrator,chat,task,subtask,checkpoint,supervisor,analyzer,executor}` |
| Chain settings | `backend/src/config/onchain.go`, `backend/src/service/external/onchain_service.go` |
| Next.js version | `frontend/package.json` (`next 16.2.6`) |

## 6. UI/UX Changes (Lo-Fi)

None.

## 7. Flowchart

```text
Mint   requestMint -> approveMint x3/5 -> executeMint(proposalId, sqrtPriceX96)
                                            -> _ensureStock (lazy PulsarStock)
                                            -> _provideMintToV4 (pool + liquidity)

Swap   swapV4(ticker, amountIn, minOut, buyStock)
         -> PoolManager V4 -> PulsarSwapHook (swapFeeBps in IDRX)

Redeem requestRedeem -> approveRedeem x3/5 -> executeRedeem
         (KYC checked here; holding and swapping are permissionless)
```

## 8. Verification Plan

1. Re-run the greps in §5 against `main`; every path/line must resolve.
2. `forge build` in `smart-contract/` (the README's contract names must compile).
3. `cd backend && go build ./...` (the README's paths must exist).
4. Check every relative link in both READMEs resolves in the tree.
5. Manual read: a new contributor can deploy contracts, run the backend and
   frontend, and know where the agent code lives without opening the code.

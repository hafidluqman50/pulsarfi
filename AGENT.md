# PulsarFi — Agent Strategy & Context Guide

## 1. Project Core Philosophy

PulsarFi is an Asset-Backed Tokenization Platform (RWA) on Arbitrum Sepolia that tokenizes Indonesian IDX equities (BUMIP, ENRGP, etc.) into 1:1 on-chain receipts paired against IDRX liquidity pools.

**Fundamental Rules:**

- **Asset-Backed, NOT Synthetic.** Every token minted represents actual shares held 1:1 in custody at KSEI. Price discovery comes from AMM mechanics and arbitrage, not oracles.
- **USDC Compliance Model.** Anyone can hold and swap tokens permissionlessly (no upfront KYC). KYC is enforced only at the Redemption Gateway when converting tokens back to physical securities.

---

## 2. Token Naming Convention

All asset tokens use ALL-CAPS with a `P` suffix indicating Pulsar origin.

- Valid: `BUMIP`, `ENRGP`, `BRPTP`, `PTROP`, `BBRIP`, `BMRIP`, `BBCAP`, `BDMNP`
- Forbidden: `pBUMI`, `pENRG`, `PBUMI`, `PENRG`

---

## 3. Architecture

### Smart Contracts (`/smart-contract`)
- **`PulsarProtocol.sol`** — UUPS upgradeable proxy, single entry point for all protocol operations. Its own compiled bytecode exceeds the EIP-170 24,576-byte limit once V4 mint/redeem/fee/circuit-breaker logic is included, so ~20 less-hot-path functions are relocated into `PulsarProtocolOps` (see below) — fully transparent to every caller, same function names/params/events/access control.
  - Mint 3/5 multisig: `requestMint` → `approveMint` → `executeMint` (only requester executes) OR `rejectMint` → `executeRejectMint` (first rejecter executes). All new mints go to the ticker's Uniswap V4 pool — no `OperatorWallet`/OTC destination exists.
  - Liquidity pool minting uses custodian-funded IDRX. The protocol must not mint IDRX internally; it pulls IDRX with ERC20 `transferFrom`, so the requester/funder must approve the protocol first. On a ticker's first mint, `executeMint` also creates its V4 pool and seeds full-range liquidity.
  - Redeem 3/5 multisig: `requestRedeem(ticker, tokenAmount, userAddress)` — custodian only, KYC checked on userAddress, quotes the stock's IDRX value from the V4 pool's own spot price → `approveRedeem` → `executeRedeem` OR `rejectRedeem` → `executeReject`
  - `swapV4(ticker, amountIn, amountOutMin, buyStock)` — permissionless, no auth required, executes against the ticker's Uniswap V4 pool. No fee-cutting logic lives here: the `swapFeeBps` protocol fee is enforced entirely by `PulsarSwapHook` at the **pool level**, in IDRX, so it applies regardless of entry point (protocol, aggregator, or Uniswap's own UI) — see §4a.
  - `approveKYC(userAddress)` / `revokeKYC(userAddress)` — admin only (delegated to `PulsarProtocolOps`)
  - Circuit breakers: `pause()`/`unpause()` (blocks `executeMint`/`swapV4`/`addV4Liquidity`), `pauseHook()`/`unpauseHook()` (halts all V4 swaps pool-wide, any entry point), `emergencyWithdrawV4(ticker)` (pauses the hook and pulls the ticker's entire V4 liquidity back to the protocol — incident-only, delegated).
- **`PulsarProtocolOps.sol`** — delegatecall target for the ~20 functions relocated to fit `PulsarProtocol` under EIP-170: mint/redeem approve-reject-execute, `migrateV2ToV4`, `emergencyWithdrawV4`, `distributeFees`, KYC, admin setters. Never called directly with real effect — its own storage trie is permanently empty (never `initialize()`d), so every real check it runs fails closed for a caller bypassing the proxy. Both contracts inherit `PulsarProtocolStorage`, an abstract base declaring every state variable once, guaranteeing identical storage slot layout by construction.
- **`v4/PulsarSwapHook.sol`** — Uniswap V4 hook, live and immutable. Enforces the `swapFeeBps` protocol fee at the pool level on every swap against a registered pool, always in IDRX (skimmed from input on buy, from output on sell). This is a fee-enforcement hook only — it has no KYC logic.
- **`PulsarStock.sol`** — Independent ERC20 per stock, owned by PulsarProtocol. Deployed lazily on first `executeMint`.
- **`IDRX.sol`** — Mock stablecoin (2 decimals) for Arbitrum Sepolia. Production/mainnet must configure the real IDRX token and rely on user/custodian balances + allowances.
- **AMM**: Uniswap V4 (official `PoolManager`) is the live, sole swap/liquidity venue — this is the current deployed state, not a future upgrade. Uniswap V2 (Factory/Router) is legacy: kept only so the one-time `migrateV2ToV4` runbook step can pull liquidity out of tickers that still had V2 history at cutover time (BUMIP, ENRGP). A fresh (e.g. mainnet) launch with no V2 history would never need V2 at all. If V2 does need redeploying for a migration, use official build artifacts in `smart-contract/script/artifacts`, not a Foundry recompile of V2 core/periphery — the official router hardcodes the pair init code hash, and recompiled pair bytecode can make `pairFor()` point to the wrong address.

### Current Arbitrum Sepolia Deployment

| Contract | Address | Verification |
|---|---:|---|
| `PulsarProtocol` proxy | `0x204488318C0E75978B3c851382Aa83f3065a8f5A` | Verified (`ERC1967Proxy`) |
| `PulsarProtocol` implementation | `0x529Cf96BD1Ea34C8B9fA931aBbaD70812A4ea10e` | Verified — V4-native |
| `PulsarProtocolOps` | `0xF6C640e11D6507bfEa8B1E15972790ba74b0Fef4` | Verified — delegatecall target, not a proxy |
| `PulsarSwapHook` | `0x793bAC5e3CE0e42E719883b1C7038a8743dc00Cc` | Verified — immutable V4 hook |
| Uniswap V4 `PoolManager` | `0xFB3e0C6F74eB1a21CC1Da29aeC80D2Dfe6C9a317` | Official Uniswap deployment, not PulsarFi's own |
| `IDRX` mock | `0x03b53A71C5517907006EAb512A31C1eD5a56Ae64` | Verified |
| `IDRXFaucet` | `0x286954bE9b8a2B52f2A61432Fa448C5287e4dDEA` | Verified |
| `UniswapV2Factory` | `0x4254378E95dBD9816a1a18428A81B4E1fBe5C296` | Legacy — only relevant for `migrateV2ToV4` on tickers with V2 history |
| `UniswapV2Router02` | `0xFEf655B2A0742134242711b80899d0b543A74223` | Legacy — same as above |
| WETH | `0x980B62Da83eFf3D4576C647993b0c1D7faf17c73` | External dependency (V2-only) |

All 8 currently listed tickers (`BUMIP`, `ENRGP`, `BRPTP`, `PTROP`, `BBRIP`, `BMRIP`, `BBCAP`, `BDMNP`) have `isV4Migrated(ticker) == true` on-chain — confirmed live via `cast call`, not assumed from docs.

The matching `.env` keys are `PULSAR_PROTOCOL_PROXY`, `PULSAR_PROTOCOL_IMPL`, `PULSAR_PROTOCOL_OPS`, `IDRX`, `UNISWAP_V2_FACTORY`, `UNISWAP_V2_ROUTER`, and `WETH`. Keep these in sync with `frontend/.env.local` (`NEXT_PUBLIC_PULSAR_PROTOCOL_ADDRESS`, `NEXT_PUBLIC_IDRX_ADDRESS`).
`PulsarStock` tokens are deployed lazily on first successful `executeMint`; verify each stock token address after deployment.

### Uniswap V2 — Legacy, Migration-Only

V2 is no longer part of the live swap/mint path. It is only ever redeployed to support `migrateV2ToV4` against a ticker that still has V2-era liquidity (BUMIP, ENRGP at cutover time). If that's ever needed again, deploy V2 through `script/OfficialUniswapV2.s.sol`, which reads official build artifacts from `script/artifacts`:

```bash
DEPLOY_UNISWAP_V2=true forge script script/Upgrade.s.sol:UpgradeScript --rpc-url "$RPC_URL" --broadcast
```

Do not deploy V2 by recompiling `lib/v2-core` and `lib/v2-periphery` with Foundry. The official `UniswapV2Router02` uses `UniswapV2Library.pairFor()`, and that library hardcodes the pair init code hash. If factory pair bytecode does not match that hash, `factory.createPair()` can succeed while router `addLiquidity()` calls the wrong deterministic pair address.

Post-deploy checks:

```bash
cast call $UNISWAP_V2_ROUTER "factory()(address)" --rpc-url "$RPC_URL"
cast call $UNISWAP_V2_FACTORY "feeToSetter()(address)" --rpc-url "$RPC_URL"
```

For the live V4 path, verify instead:

```bash
cast call $PULSAR_PROTOCOL_PROXY "poolManager()(address)" --rpc-url "$RPC_URL"
cast call $PULSAR_PROTOCOL_PROXY "swapHook()(address)" --rpc-url "$RPC_URL"
cast call $PULSAR_PROTOCOL_PROXY "opsContract()(address)" --rpc-url "$RPC_URL"
cast call $PULSAR_PROTOCOL_PROXY "isV4Migrated(string)(bool)" "BUMIP" --rpc-url "$RPC_URL"
```

### Backend (`/backend`)
- Go + Gin + GORM + PostgreSQL
- Structure: `src/{app,auth,config,http,logger,model,repository,service}`
- **Auth**: SIWE (Sign-In with Ethereum, EIP-4361) for ALL users — custodians and retail alike. No email/password.
  - `GET /api/v1/auth/nonce?address=0x...` → nonce (5-min TTL, one-time use)
  - `POST /api/v1/auth/verify` → `{ address, message, signature, nonce }` → JWT
  - JWT payload: `{ wallet_address, role: "custodian"|"user" }`
  - Custodian role determined by wallet lookup in `custodians` table at verify time.
- **Response format** (all endpoints):
  ```json
  { "status_code": 200, "message": "...", "data": {...} }
  ```
- **Swap recording**: Frontend hits SC via wagmi → after tx confirmed (`useWaitForTransactionReceipt`) → frontend hits `POST /api/v1/public/stock-transactions`. No indexer, no WebSocket, no polling. Idempotent via `tx_hash` UNIQUE constraint.

### Frontend (`/frontend`)
- Next.js + TailwindCSS + RainbowKit (wallet connection + SIWE)
- `/swap` — permissionless swap view
- `/custodian` — multisig mint dashboard, KYC management, pending proposals
- Sonner for all toast/notification UI

### Frontend Styling Rules

- Use Tailwind utility classes for component styling. Do not add new React inline `style` props for normal layout, spacing, typography, color, borders, or shadows.
- Preserve existing visuals 1:1 when refactoring styles. Prefer arbitrary Tailwind values such as `px-[24px]`, `mt-[28px]`, and `grid-cols-[...]` when exact pixel parity matters.
- Existing global helper classes such as `display`, `mono`, `eyebrow`, `hairline`, `card`, `btn`, and responsive table/layout classes may stay in place; compose Tailwind around them instead of rewriting behavior.

---

## 3a. AI Trading Agent

Chat-first, self-custodied AI agent — see `docs/plans/agent-role-architecture.md` and `docs/plans/agent-task-manager-rebuild.md`/`agent-task-manager-code-implementation.md` for the full design. Ground-up rebuild, not an evolution of the old Trader/Fund Manager split: **Task** (any request needing data fetched/searched, informational or actionable — a greeting is not a Task), **Sub Task** (one hash-chained step, written by whichever role actually did it), **Trade** (an actual fill, zero-to-many per Task, never pre-locked to ticker/direction).

Three roles, Supervisor is the mandatory entry point for every chat message:

- **Supervisor** — recognizes a new Task (`create_task` tool) and routes to Analyzer and/or Executor.
- **Analyzer** — gathers news/technical evidence for a trigger condition, *and* answers portfolio/chart questions (`get_portfolio_snapshot`, a closed 5-lens catalog, prompt-injection-hardened: lens is enum-validated server-side, chart data is always fetched fresh, never LLM-supplied).
- **Executor** — decides sell/buy/hold and submits on-chain when it decides to act (`submit_trade`; ticker/side/amount are call-time, never pre-locked to the Task).

**Smart contract** — `smart-contract/src/AgentTaskManager.sol`, `AGENT_ROLE`-gated `createTask`/`grantTradePermission`/`recordSubTasks`/`executeTrade`, owner-or-agent `cancelTask`. Custody: owner `approve()`s the contract directly, agent wallet never holds funds. **Deployed on Arbitrum Sepolia at `0x15080823e6d91DfE37593Fb4CE91E08bb294B01f`** (`smart-contract/script/DeployAgentTaskManager.s.sol`), `AGENT_ROLE` granted to `AGENT_WALLET`; fork-tested (`smart-contract/test/AgentTaskManagerFork.t.sol`, 17/17 passing against the live `PulsarProtocol` proxy).

**Backend pipeline** (`backend/src/service/agent/`), `github.com/cloudwego/eino` (`adk` agent framework, agents as callable tools rather than a fixed graph). `analyzer/`, `executor/`, `supervisor/` are each an agent's own package (own `index.go`, `instructions.go`, `tools_service.go`); every non-`index.go`/`instructions.go` file ends `_service.go`. `backend/src/onchain/agenttaskmanager/` is the real on-chain Go client (abigen-generated binding + a hand-written signer using `AGENT_WALLET_PRIVATE_KEY`) — `CreateTask`/`GrantTradePermission`/`RecordSubTasks`/`CancelTask`/`TradePermissionRemaining` are real; `ExecuteTrade` is not yet (the `TaskExecutor` interface is missing subTaskId/token/minimumOutputAmount/summary, `executor.StubTaskExecutor` still stands in for it). `backend/src/service/agent/task_service.go` is the HTTP↔Supervisor boundary: `HandleChatMessage` (chat intake), `ArmTask`/`DisarmTask`/`PauseTask`/`ResumeTask` (Task lifecycle), `Evaluate` (scheduler-driven re-check tick for an armed Task — not yet wired to an actual scheduler).

**API** (`/api/v1/agent`, see `backend/src/http/routes/agent/router.go`):

| Endpoint | Auth | Purpose |
|---|---|---|
| `POST /agent/chats` | wallet owner | Open a new chat thread |
| `GET /agent/chats` | wallet owner | List own chats |
| `GET /agent/chats/:id/messages` | wallet owner | Chat transcript |
| `POST /agent/chats/:id/messages` | wallet owner | Send a prompt — Supervisor may open a Task (`create_task`) as a side effect |
| `GET /agent/tasks` | wallet owner | List own Tasks |
| `POST /agent/tasks/:id/arm` | wallet owner | `createTask` + (if actionable) `grantTradePermission` on-chain |
| `POST /agent/tasks/:id/disarm` | wallet owner | `cancelTask` on-chain (frontend separately zeroes the ERC20 allowance) |
| `POST /agent/tasks/:id/pause`, `/resume` | wallet owner | Off-chain only — skips/resumes the evaluation tick, no on-chain call |
| `GET /agent/tasks/:id/trades` | wallet owner | On-chain Trade ledger for the Task |
| `GET /agent/tasks/:id/reasoning` | public, no auth | Full Sub Task hash chain — anyone can recompute and verify against the on-chain `reasoningHash` |

**Known open items**: the scheduler that periodically calls `Evaluate` for armed Tasks does not exist yet; chart rendering (echarts + a lens→option mapping) is not wired on the frontend (placeholder only); a multi-tool-call chat turn sometimes returns a blank final reply (reasoning chain is still correct) — root cause not yet found.

---

## 4. Custodian Flows (3/5 Multisig)

### Mint
```
Custodian A  →  requestMint(ticker, stockName, idxTicker, tokenAmount, idrxAmount, attestationHash, destination)
Custodian B/C → approveMint(proposalId)  OR  rejectMint(proposalId)
Custodian A  →  executeMint(proposalId)      ← only requester, after approvalCount >= 3
                 └─ _ensureStock → deploy PulsarStock if first mint
                 └─ _mint → PulsarStock.mint(to, amount)
                 └─ if LiquidityPool → fund IDRX by allowance/transferFrom → _provideToPool → addLiquidity
First rejecter → executeRejectMint(proposalId) ← after rejectCount >= 3
```

### Redeem
```
Custodian    →  requestRedeem(ticker, tokenAmount, userAddress)
                 └─ checks kycApproved[userAddress]
                 └─ locks user tokens + IDRX fee in contract
Custodian    →  approveRedeem(requestId)  OR  rejectRedeem(requestId)
First approver → executeRedeem(requestId)  ← after approvalCount >= 3, burns tokens, fee joins accumulatedFees
First rejecter → executeReject(requestId)  ← after rejectCount >= 3, returns tokens + fee to user
```

### KYC (prerequisite for redeem)
```
User contacts operator off-chain (phone/email)
Custodian → inputs wallet address in dashboard → approveKYC(userAddress) on-chain
```

---

## 4a. Fee Accounting & Distribution

- `swapFeeBps` — protocol fee enforced by `PulsarSwapHook` at the pool level on every `swapV4()`, always in IDRX (from `amountIn` when buying, from output when selling), regardless of entry point. Separate from Uniswap's own 0.3% AMM fee, which stays in pool reserves untouched (protocol-owned liquidity, not revenue).
- `accumulatedFees` — internal counter of *confirmed* protocol revenue (swap fee + executed redeem fee). Redeem fee only joins this counter at `executeRedeem`, never at `requestRedeem` time, so pending escrow that might still need refunding via `executeReject` is never at risk of being swept.
- `distributeFees()` — permissionless, callable by anyone once `accumulatedFees >= minimumDistributionThreshold`. Splits 30% to `treasury`, 70% equally among custodians that have ever called `requestMint`/`approveMint` (tracked in `_activeCustodians`).
- See `BUSINESS.md` for the full rationale and recommended default values.

---

## 5. Database Tables

| Table | Purpose |
|---|---|
| `custodians` | 5 multisig operator participants |
| `stocks` | Listed tokens; `contract_address` null until first executeMint |
| `mint_proposals` | Mirror of on-chain mint proposals |
| `mint_attestations` | Unified approve+reject votes per custodian per proposal (`type`: approve/reject) |
| `redeem_proposals` | Mirror of on-chain redeem requests; `user_address` = beneficiary |
| `redeem_attestations` | Unified approve+reject votes per custodian per redeem |
| `wallet_verifications` | KYC records managed by operator; `type`: retail/institution |
| `stock_transactions` | Swap events: side buy/sell, idrx_amount, stock_amount, protocol_fee_idrx (NUMERIC 78,0) |
| `stock_attestations` | Proof of reserves per stock (operator-level, not per-custodian) |
| `agent_tasks`, `agent_sub_tasks`, `agent_trades`, `agent_chats`, `agent_chat_messages` | AI Trading Agent (§3a) — Task/Sub Task/Trade, chat threads. `agent_sub_tasks.reasoning`/`.output` are exact bytes whose keccak256 is the on-chain `reasoningHash`/`outputHash`, stored verbatim and never re-serialized |

---

## 6. Key Constraints

- IDRX decimals = 2 (matches real IDRX on Base/other chains)
- PulsarStock decimals = 18
- Amounts stored in DB as `NUMERIC(78,0)` — raw on-chain uint256
- `stocks(ticker)` is UNIQUE, not PK — all FK relations use `stocks(id)`
- Nonce store is in-memory (sufficient for hackathon MVP)
- IDRX not deployed on Arbitrum Sepolia by default — use `IDRX.sol` mock
- Liquidity pool proposals require IDRX funding before execution. New LP requests fund during `requestMint` after an IDRX approval. Existing LP proposals can be topped up with `fundMintLiquidity(proposalId, amount)` before `executeMint`.
- Do not edit files under `smart-contract/lib/**`. Treat Uniswap dependencies as read-only; fixes belong in project scripts/contracts or dependency pinning.

---

## 7. AMM Versioning

- **Live now**: Uniswap V4 (official `PoolManager`) is the sole swap/liquidity venue, on all currently listed tickers — confirmed on-chain, not a roadmap item. `PulsarSwapHook` enforces the protocol swap fee at the pool level on every swap regardless of entry point. It is a fee-enforcement hook only, not a KYC hook.
- **Legacy**: Uniswap V2 (Factory/Router) is no longer part of the live path. Kept only for the one-time `migrateV2ToV4` runbook step on tickers that had V2 history at cutover (BUMIP, ENRGP). A fresh deployment with no V2 history never needs it.

---

## 8. Project Structure

```
pulsarfi/
├── AGENT.md, BUSINESS.md, README.md
├── docs/
├── smart-contract/              Foundry — Solidity contracts, tests, deploy scripts
│   ├── src/
│   │   ├── PulsarProtocol.sol
│   │   ├── PulsarProtocolOps.sol
│   │   ├── PulsarProtocolStorage.sol
│   │   ├── PulsarStock.sol
│   │   ├── AgentTaskManager.sol      Trade model (§3a) — deployed, see §3a
│   │   ├── v4/PulsarSwapHook.sol      live V4 fee-enforcement hook, not a future upgrade
│   │   ├── interfaces/
│   │   ├── helpers/                   legacy Uniswap V2 compile helpers (migration-only)
│   │   └── mocks/
│   ├── test/
│   │   └── AgentTaskManagerFork.t.sol   fork tests vs. the live proxy
│   └── script/                        deploy/upgrade scripts + official Uniswap V2 artifacts (legacy, migration-only)
│
├── backend/                     Go + Gin + GORM + PostgreSQL
│   ├── migrations/               001..015_*.sql
│   └── src/
│       ├── app/                   bootstrap, gin engine wiring
│       ├── auth/                  SIWE, JWT, nonce store
│       ├── config/
│       ├── http/
│       │   ├── handlers/{agent,auth,custodian,public}/
│       │   ├── middleware/{custodian,user}/
│       │   ├── request/, response/
│       │   └── routes/{agent,custodian,public}/
│       ├── model/                 AgentTask, Stock, MintProposal, RedeemProposal, ...
│       ├── repository/
│       ├── service/
│       │   ├── agent/               AI Trading Agent (§3a) — Task/Sub Task/Trade, chat-first
│       │   │   ├── state_service.go       TradeIntent, TradeSide
│       │   │   ├── run_context_service.go RunContext (per-run data threaded through nested tool calls)
│       │   │   ├── sub_task_recorder_service.go  hash-chained agent_sub_tasks writer
│       │   │   ├── hashchain_service.go   GenesisHash/DecisionHash
│       │   │   ├── llm_service.go         InvokeAgentStructured/RunAgentWithTrace over adk.Agent.Run
│       │   │   ├── task_service.go        HandleChatMessage/ArmTask/DisarmTask/PauseTask/ResumeTask/Evaluate
│       │   │   ├── subtask_retry_service.go  retries unconfirmed recordSubTasks batches
│       │   │   ├── supervisor/        entry point for every chat message — create_task, routes to analyzer/executor
│       │   │   ├── analyzer/          evidence-gathering + get_portfolio_snapshot (chart lens catalog)
│       │   │   └── executor/          decides sell/buy/hold, submit_trade
│       │   ├── auth/, custodian/, public/, indexer/
│       │   └── external/            DeepSeek (Eino ChatModel), price/email/storage/stream
│       ├── onchain/agenttaskmanager/  real on-chain Go client (abigen binding + signer)
│       └── logger/
│   └── test/                     unit tests live here, never colocated with src/ (no agent/ tests yet)
│
└── frontend/                    Next.js + Tailwind + RainbowKit
    ├── /swap                     permissionless swap view
    └── /custodian                multisig mint dashboard, KYC management, pending proposals
```

---

## 9. Planning Mode Guidelines

When asked to create an implementation plan (Planning Mode), follow these rules strictly:

1. **No code during Planning Mode.** Do not write or modify any code, run any state-changing command, or otherwise alter the repository until the user has explicitly approved the plan or given explicit instructions to start implementing. The only output during this phase is the plan document itself.
2. **Version every plan document.** State the document version at the top. Any change to the document, however small, must bump the version — and any other plan document covering a correlated/dependent part of the same feature under development must be updated to stay consistent with it.
3. **English only.** Plan documents are written entirely in English, regardless of the language used in conversation while producing them.
4. **Problem Statement.** What the problem is, and why it needs solving.
5. **Definition of Done.** The concrete, checkable conditions under which this feature is considered complete.
6. **Feature Description.** What is being built or changed.
7. **Impacted Files.** Every file expected to be created or modified.
8. **UI/UX Changes (Lo-Fi).** A low-fidelity description/wireframe of any UI/UX change. State "N/A" explicitly when the feature has no UI/UX surface.
9. **Flowchart.** A flowchart (e.g. Mermaid) of the feature's flow.
10. **Verification Plan.** How the feature will be verified once implemented — automated tests and manual checks.

### Plan Document Header

Every plan document opens with:

```markdown
# <Feature Name>

| | |
|---|---|
| **Version** | 1.0 |
| **Status** | Draft |
| **Date Created** | 2026-08-28 |
| **Last Updated** | 2026-08-28 |
```

Plans are saved under `docs/plans/<feature-name-kebab-case>.md`.

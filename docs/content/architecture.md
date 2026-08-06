---
id: architecture
title: Architecture
sidebar_label: Architecture
slug: /architecture
---

PulsarFi is split into three implementation surfaces: smart contracts, backend,
and frontend. Each surface has a different trust level and a different reason to
exist.

## High-level system

```text
+------------------+
| User / Custodian |
| Wallet           |
+--------+---------+
         |
         | signs transactions and SIWE messages
         v
+------------------+        HTTPS         +----------------------+
| Frontend         +--------------------->| Backend API          |
| Next.js + wagmi  |                      | Go + Gin + GORM      |
+--------+---------+                      +----------+-----------+
         |                                           |
         | on-chain calls                            v
         v                                +----------------------+
+------------------+                      | PostgreSQL           |
| PulsarProtocol   |                      | operational mirror   |
| UUPS proxy       |                      +----------------------+
+--------+---------+
         |
         +-- deploys / controls --> PulsarStock ERC-20 contracts
         |
         +-- delegatecall (rare ops) --> PulsarProtocolOps
         |
         +-- swaps / liquidity ---> Uniswap V4 PoolManager + PulsarSwapHook
         |
         +-- pulls / transfers ---> IDRX mock token
```

The smart contract is the source of truth for token movement, mint execution,
redeem locking, KYC state, and swap execution. The backend is an operational
mirror for UX, dashboards, queues, and history. The frontend coordinates wallet
transactions and records successful receipts.

## Two-contract split: PulsarProtocol + PulsarProtocolOps

`PulsarProtocol` is the only contract the proxy ever points to, and the only
address the frontend, backend, or any external caller ever needs to know. Once
this session's V4 cutover landed, its compiled bytecode exceeded the EIP-170
24,576-byte deploy limit. Rather than cut functionality, the ~20 least-hot-path
functions (mint/redeem approve-reject-execute, `migrateV2ToV4`,
`emergencyWithdrawV4`, `distributeFees`, KYC, admin setters) were moved into a
separate `PulsarProtocolOps` contract, called via `delegatecall` from thin
one-line dispatchers on `PulsarProtocol`.

```text
caller --> PulsarProtocol.approveMint(id)
             |
             | delegatecall (same storage, same msg.sender, same balances)
             v
           PulsarProtocolOps.approveMint(id)   <- real logic runs here
```

This is fully transparent to every caller: same function name, same
parameters, same events, same access control — the only observable difference
is one extra `DELEGATECALL` of gas on the relocated functions. Hot-path
functions (`executeMint`, `swapV4`, `requestRedeem`, `collectV4Fees`, `pause`,
`pauseHook`) stay directly on `PulsarProtocol` at full gas efficiency.

Both contracts inherit an abstract `PulsarProtocolStorage` base that declares
every state variable once, so the compiler — not a hand count — guarantees
identical storage slot layout between them. `PulsarProtocolOps` is never used
standalone: it holds no meaningful state of its own (its own storage trie is
permanently empty, since it's never `initialize()`d), and a direct call to it
bypassing the proxy harmlessly fails every check it runs (roles, pool
existence, ticker lookups all read empty storage). See [Protocol
Design](/docs/protocol-design#the-ops-split) for the full function list.

## Design principle: backend follows chain

The backend does not mint tokens, approve KYC on-chain, or execute swaps. It
records what happened after the frontend observes a confirmed transaction
receipt.

This creates a deliberate ordering:

```text
wallet signs -> contract executes -> receipt confirms -> frontend records -> backend displays
```

If the backend is unavailable, on-chain execution still works. If the chain
transaction fails, the backend should not create the final state.

## Component responsibilities

| Component | Technology | Responsibility |
| --- | --- | --- |
| Frontend | Next.js, wagmi, RainbowKit | Wallet connection, SIWE auth, transaction simulation, approvals, transaction submission, UX state. |
| Backend API | Go, Gin, GORM | SIWE verification, JWT issuance, proposal mirrors, transaction history, KYC records, stats, reserve snapshots, on-chain V4 price reads. |
| Database | PostgreSQL | Durable operational state for UI and dashboards. |
| PulsarProtocol | Solidity, UUPS | Mint proposals, redeem requests, KYC mapping, `swapV4` entrypoint, `collectV4Fees`, protocol configuration. |
| PulsarProtocolOps | Solidity, delegatecall target | Mint/redeem approve-reject-execute, `migrateV2ToV4`, `emergencyWithdrawV4`, `distributeFees`, KYC, admin setters — relocated to fit PulsarProtocol under EIP-170. |
| PulsarSwapHook | Solidity, Uniswap V4 hook | Enforces the protocol swap fee at the pool level, in IDRX, on every swap against a registered pool — regardless of entry point (protocol, aggregator, or Uniswap's own UI). |
| PulsarStock | Solidity ERC-20 | One token contract per listed pStock. Mint and burn controlled by PulsarProtocol. |
| IDRX mock | Solidity ERC-20 | Testnet IDRX with 2 decimals. |
| IDRXFaucet | Solidity | Public testnet drip for reviewers and testers. |
| Uniswap V4 | Official PoolManager | IDRX-pStock liquidity pools and swap execution, flash-accounted via `unlock`/`unlockCallback`. |
| Uniswap V2 | Factory and Router (legacy) | No longer part of the live path. Kept only so `migrateV2ToV4` can pull liquidity from tickers that still had V2 history at cutover time (BUMIP, ENRGP). A fresh mainnet launch would never need this. |
| External storage | S3-compatible | Private KYC document storage. |
| Market data service | Backend external service | IDX and on-chain V4 pool price aggregation for UI. |

## Trust boundaries

The system has several boundaries that should not be blurred:

| Boundary | Rule |
| --- | --- |
| Wallet to frontend | The wallet owns keys. The frontend can request signatures, not sign for users. |
| Frontend to backend | Backend accepts records, but sensitive routes require SIWE JWT. |
| Frontend to chain | All token movements are direct wallet transactions. |
| Backend to chain | Backend reads and mirrors; it does not act as a transaction executor. |
| Custodian to protocol | Custodians can vote, but one custodian cannot complete threshold operations alone. |
| Admin to protocol | Admin can configure dependencies and upgrades, but mint/redeem operations remain role-gated. |
| PulsarProtocol to PulsarProtocolOps | Ops only ever executes with real effect via delegatecall from the proxy; a direct call to its own address fails closed (empty storage, no roles granted). |

## Runtime environments

### Local development

```text
frontend:       npm run dev
backend:        go run main.go
smart-contract: forge build / forge test
docs:           npm run start
database:       PostgreSQL via DATABASE_URL
```

### Testnet deployment

```text
network:        Arbitrum Sepolia
token unit:     IDRX uses 2 decimals
pStock unit:    pStocks use 18 decimals
AMM:            Uniswap V4 (official PoolManager) + PulsarSwapHook fee hook
auth:           SIWE + JWT
storage:        optional S3-compatible private bucket for KYC statements
```

## Data synchronization model

The frontend is responsible for calling backend record endpoints after a
transaction is confirmed. This keeps the deployed system lightweight while still
preserving an auditable mirror of confirmed transactions.

Example swap sync:

```text
1. User submits swapV4 transaction.
2. Protocol emits V4Swapped; PulsarSwapHook emits BuySideFeeTaken (buy) or HookFee (sell).
3. Frontend waits for receipt.
4. Frontend parses event logs (matched by signature, no hook address needed).
5. Frontend sends tx_hash, wallet, side, amounts, exact protocol fee, and block_number to backend.
6. Backend inserts stock_transactions using tx_hash as idempotency key.
```

Example mint sync:

```text
1. Custodian submits requestMint on-chain.
2. Frontend records mint proposal in backend.
3. Custodians approve/reject on-chain.
4. Frontend records each vote in backend.
5. Requester executes mint on-chain, passing a canonical sqrtPriceX96 for the
   ticker's first V4 pool (ignored on later mints for the same ticker).
6. Frontend reads deployed stock address and records execution.
7. Backend updates stock contract address and reserve snapshot.
```

## Why no indexer

An indexer would be stronger for large-scale reconciliation, but it adds
operational overhead. Transaction receipt parsing from the frontend is enough for
the current deployment while keeping the system simple.

The current design still leaves room for an indexer later because the backend
tables already model proposals, attestations, transactions, and reserve records
as explicit events.

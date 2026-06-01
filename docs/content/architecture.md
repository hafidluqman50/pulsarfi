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
| UUPS contract    |                      +----------------------+
+--------+---------+
         |
         +-- deploys / controls --> PulsarStock ERC-20 contracts
         |
         +-- swaps / liquidity ---> Uniswap V2 Router + Factory
         |
         +-- pulls / transfers ---> IDRX mock token
```

The smart contract is the source of truth for token movement, mint execution,
redeem locking, KYC state, and swap execution. The backend is an operational
mirror for UX, dashboards, queues, and history. The frontend coordinates wallet
transactions and records successful receipts.

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
| Backend API | Go, Gin, GORM | SIWE verification, JWT issuance, proposal mirrors, transaction history, KYC records, stats, reserve snapshots. |
| Database | PostgreSQL | Durable operational state for UI and dashboards. |
| PulsarProtocol | Solidity, UUPS | Mint proposals, redeem requests, KYC mapping, swap entrypoint, protocol configuration. |
| PulsarStock | Solidity ERC-20 | One token contract per listed pStock. Mint and burn controlled by PulsarProtocol. |
| IDRX mock | Solidity ERC-20 | Testnet IDRX with 2 decimals. |
| IDRXFaucet | Solidity | Public testnet drip for reviewers and testers. |
| Uniswap V2 | Factory and Router | IDRX-pStock liquidity pools and swap execution. |
| External storage | S3-compatible | Private KYC document storage. |
| Market data service | Backend external service | IDX and on-chain price aggregation for UI. |

## Trust boundaries

The system has several boundaries that should not be blurred:

| Boundary | Rule |
| --- | --- |
| Wallet to frontend | The wallet owns keys. The frontend can request signatures, not sign for users. |
| Frontend to backend | Backend accepts records, but sensitive routes require SIWE JWT. |
| Frontend to chain | All token movements are direct wallet transactions. |
| Backend to chain | Backend reads and mirrors; it does not act as a transaction executor in the MVP. |
| Custodian to protocol | Custodians can vote, but one custodian cannot complete threshold operations alone. |
| Admin to protocol | Admin can configure dependencies and upgrades, but mint/redeem operations remain role-gated. |

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
AMM:            custom Uniswap V2 deployment
auth:           SIWE + JWT
storage:        optional S3-compatible private bucket for KYC statements
```

## Data synchronization model

The frontend is responsible for calling backend record endpoints after a
transaction is confirmed. This is used instead of an indexer for the MVP.

Example swap sync:

```text
1. User submits swap transaction.
2. Protocol emits TokensSwapped.
3. Frontend waits for receipt.
4. Frontend parses event logs.
5. Frontend sends tx_hash, wallet, side, amounts, and block_number to backend.
6. Backend inserts stock_transactions using tx_hash as idempotency key.
```

Example mint sync:

```text
1. Custodian submits requestMint on-chain.
2. Frontend records mint proposal in backend.
3. Custodians approve/reject on-chain.
4. Frontend records each vote in backend.
5. Requester executes mint on-chain.
6. Frontend reads deployed stock address and records execution.
7. Backend updates stock contract address and reserve snapshot.
```

## Why no indexer in the MVP

An indexer would be stronger for production-grade reconciliation, but it adds
operational overhead. For the hackathon MVP, transaction receipt parsing from the
frontend is enough to demonstrate the full flow while keeping the system simple.

The current design still leaves room for an indexer later because the backend
tables already model proposals, attestations, transactions, and reserve records
as explicit events.

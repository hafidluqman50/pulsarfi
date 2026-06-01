---
id: data-api
title: Data & API Design
sidebar_label: Data & API
slug: /data-api
---

The backend exists to make the app usable. It stores operational records that are
awkward to query directly from contracts in a product UI: queues, KYC records,
approval history, portfolio history, reserve snapshots, and dashboard stats.

## API principles

- The chain remains the execution layer.
- The backend records confirmed actions.
- Sensitive custodian routes require SIWE-based JWT auth.
- Public market routes do not require auth.
- Response envelopes are consistent across endpoints.
- Transaction hashes are used for idempotency where applicable.

All API responses follow:

```json
{
  "status_code": 200,
  "message": "ok",
  "data": {}
}
```

## Authentication

Authentication uses SIWE for both normal users and custodians.

```text
GET  /api/v1/auth/nonce?address=0x...
POST /api/v1/auth/verify
```

Verification flow:

1. User connects a wallet.
2. Frontend asks the backend for a nonce.
3. Frontend builds a SIWE message.
4. User signs the message.
5. Backend recovers the signer address.
6. Backend consumes the nonce.
7. Backend checks whether the wallet exists in `custodians`.
8. Backend issues JWT with role `user` or `custodian`.

The nonce store is in-memory for the MVP. That is acceptable for a hackathon
deployment but should be moved to shared storage for multi-instance production.

## Public API surface

| Endpoint | Purpose |
| --- | --- |
| `GET /api/v1/public/stocks` | List market-ready pStocks. |
| `GET /api/v1/public/prices/:ticker` | Fetch stock or pool price data. |
| `GET /api/v1/public/stats` | Protocol-level public stats. |
| `GET /api/v1/public/reserves` | Latest proof-of-reserves records. |
| `GET /api/v1/public/stock-transactions` | Wallet transaction history. |
| `POST /api/v1/public/stock-transactions` | Record a confirmed swap. |
| `POST /api/v1/public/wallet-verifications` | Submit wallet verification intent. |
| `POST /api/v1/public/redeem-requests` | Record a confirmed redeem request. |
| `GET /api/v1/public/redeem-requests` | List redeem requests for a wallet. |

## Custodian API surface

| Endpoint | Purpose |
| --- | --- |
| `GET /api/v1/custodian/stats` | Dashboard metrics. |
| `GET /api/v1/custodian/requests` | Unified queue for mint and redeem proposals. |
| `POST /api/v1/custodian/mint-proposals` | Record a mint proposal after on-chain request. |
| `POST /api/v1/custodian/mint-proposals/approve` | Record mint approval. |
| `POST /api/v1/custodian/mint-proposals/reject` | Record mint rejection. |
| `POST /api/v1/custodian/mint-proposals/execute` | Record mint execution and stock address. |
| `POST /api/v1/custodian/mint-proposals/execute-reject` | Record mint rejection execution. |
| `POST /api/v1/custodian/redeem-proposals/approve` | Record redeem approval. |
| `POST /api/v1/custodian/redeem-proposals/reject` | Record redeem rejection. |
| `POST /api/v1/custodian/redeem-proposals/execute` | Record redeem execution. |
| `POST /api/v1/custodian/redeem-proposals/execute-reject` | Record redeem rejection execution. |
| `POST /api/v1/custodian/wallet-verifications` | Create an approved KYC record with document reference. |
| `GET /api/v1/custodian/wallet-verifications` | List KYC records. |
| `GET /api/v1/custodian/attestations` | List reserve attestations. |
| `POST /api/v1/custodian/attestations` | Submit reserve attestation. |

## Database model

| Table | Product meaning |
| --- | --- |
| `custodians` | Wallets allowed to access custodian role and dashboard operations. |
| `stocks` | pStock listings and deployed contract addresses. |
| `mint_proposals` | Backend mirror of on-chain mint proposals. |
| `mint_attestations` | Approval and rejection votes for mint proposals. |
| `redeem_proposals` | Backend mirror of on-chain redeem requests. |
| `redeem_attestations` | Approval and rejection votes for redeem requests. |
| `wallet_verifications` | KYC records and document references. |
| `stock_transactions` | User swap and portfolio activity history. |
| `stock_attestations` | Proof-of-reserves snapshots. |

## Idempotency

Swap records use `tx_hash` as a unique key. This matters because frontend calls
can be retried after a timeout or page refresh. A duplicate record should not
double-count user activity.

For proposal and execution flows, `on_chain_id` is the anchor back to contract
state.

## Derived data

The backend computes or aggregates:

- protocol stats;
- 24-hour mint volume;
- pending request counts;
- reserve coverage status;
- latest attestation per stock;
- market stock list with prices and sparklines;
- wallet transaction history.

The frontend computes display-only views such as portfolio allocation, estimated
Pnl, user balances, and chart ranges.

---
id: business-flow
title: Business Flow
sidebar_label: Business Flow
slug: /business-flow
---

The PulsarFi business process connects off-chain securities custody with
on-chain receipt-token liquidity. The system is intentionally designed as a
two-world workflow: the off-chain world proves that shares exist, while the
on-chain world makes the receipt liquid and transferable.

## Actors

| Actor | Responsibility |
| --- | --- |
| Custodian | Holds real IDX shares, initiates or votes on mint/redeem proposals, approves KYC, maintains reserve records. |
| Trader | Buys and sells pStocks through IDRX pools. |
| Redeemer | KYC-approved user who wants to convert pStock into an off-chain securities process. |
| Protocol admin | Configures protocol dependencies and fee parameters. |
| Backend operator | Runs API, database, optional storage, and operational dashboards. |

## Asset lifecycle

```text
off-chain shares
    -> custodian attestation
    -> mint proposal
    -> 3-of-5 approval
    -> pStock mint
    -> IDRX-pStock liquidity pool
    -> user trading
    -> KYC-gated redemption
    -> pStock burn
    -> off-chain settlement
```

## Mint flow

Minting is used when custodians want to bring new backed supply on-chain.

### 1. Off-chain preparation

Before a mint is requested, the custodian prepares:

- stock ticker and pStock ticker;
- number of underlying shares represented by the mint;
- IDRX amount to pair with the new pStock supply;
- attestation hash or supporting proof reference;
- requester wallet that will fund IDRX liquidity.

### 2. Mint proposal

The requester submits `requestMint`. The proposal stores:

| Field | Meaning |
| --- | --- |
| `ticker` | pStock symbol such as `BUMIP`. |
| `stockName` | ERC-20 name for the token. |
| `idxTicker` | Underlying IDX ticker such as `BUMI`. |
| `tokenAmount` | Raw pStock amount with 18 decimals. |
| `idrxAmount` | Raw IDRX amount with 2 decimals. |
| `attestationHash` | Linkable proof hash for custody evidence. |
| `requester` | Custodian wallet that can execute after approval. |

The requester automatically becomes the first approver. Two more approvals are
required before execution.

### 3. Approval or rejection

Other custodians review the proposal. They can approve or reject it. Votes are
stored both on-chain and mirrored in the backend for dashboard visibility.

Mint proposal states:

| State | Description |
| --- | --- |
| `pending` | Proposal exists and is waiting for threshold approval or rejection. |
| `executed` | Proposal passed approval threshold and minted supply into liquidity. |
| `rejected` | Proposal passed rejection threshold and was closed without minting. |

### 4. Execution and liquidity

Only the original requester can execute an approved mint. This matters because
the requester must fund the IDRX side of liquidity.

Execution does four things:

1. Deploys the stock token if it does not exist yet.
2. Mints pStock to the protocol.
3. Pulls IDRX from the requester.
4. Adds both assets to the Uniswap V2 pool.

The business outcome is that new pStock supply is not released without matching
pool liquidity.

## Trading flow

Trading is intentionally simple:

```text
user wallet -> approve token -> protocol swap -> Uniswap V2 pool -> output token to user
```

No KYC is required for trading. The product uses a stablecoin-like compliance
model: the token can circulate freely, but regulated off-chain delivery remains
controlled.

IDRX is the base trading currency because the underlying IDX market is priced in
Rupiah. This avoids forcing users to reason about USD/IDR conversion every time
they compare pool prices to IDX reference prices.

Price behavior:

- The pool price can deviate from IDX prices.
- Arbitrage is expected to pull the pool price back toward fair value.
- The UI can show both IDX-derived market price and pool price.
- Slippage controls protect users from poor execution.

## Redemption flow

Redemption is used when a user wants to exit from pStock into the off-chain
securities process.

### 1. KYC approval

The user contacts the operator or custodian. The custodian verifies the user
off-chain, then calls `approveKYC(userAddress)` on-chain. The backend stores the
KYC record and optional signed statement document reference.

### 2. Redeem request

The user submits a redeem request from the portfolio page. The protocol checks
KYC, locks the pStock, and locks the IDRX fee if a redeem fee is active.

### 3. Custodian decision

Custodians approve or reject the request.

If approved:

- locked pStock is burned;
- redeem fee is sent to treasury;
- custodian proceeds with off-chain settlement.

If rejected:

- locked pStock is returned to the user;
- locked fee is returned to the user;
- the request is closed.

## Proof-of-reserves flow

Reserve records are used to explain whether off-chain custodian holdings match
on-chain supply.

The backend stores:

- custodian holdings;
- on-chain supply;
- stock ID;
- attestation hash;
- attestation timestamp.

Reserve snapshots are updated after mint and redeem execution. Custodians can
also submit manual attestations from the console.

## Revenue and fees

The MVP supports two fee concepts:

| Fee | Mechanism | Recipient |
| --- | --- | --- |
| Swap fee | Standard Uniswap V2 pool fee. | Accumulates inside pool reserves. |
| Redeem fee | Optional fee charged in IDRX at redeem request time. | Treasury on successful redeem. |

There is no user mint fee in the current model. Custodians already fund IDRX
liquidity during mint execution.

## Operational controls

The business process depends on these controls:

- custodian threshold prevents unilateral minting;
- pending mint flag prevents overlapping mint proposals for the same ticker;
- KYC mapping gates redemption;
- backend idempotency prevents duplicate transaction records;
- reserve snapshots create an audit trail for supply changes;
- testnet faucet lets reviewers bootstrap IDRX without manual distribution.

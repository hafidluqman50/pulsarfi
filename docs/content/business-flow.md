---
id: business-flow
title: Market & Revenue Model
sidebar_label: Market & Revenue Model
slug: /business-flow
---

PulsarFi connects off-chain securities custody with on-chain receipt-token
liquidity. The system is intentionally designed as a two-world workflow: the
off-chain world proves that shares exist, while the on-chain world makes the
receipt liquid, transferable, and available outside traditional exchange hours.

The economic model is built around one constraint: PulsarFi must stay
**spot-backed**. Custodians cannot mint pStock unless the corresponding IDX
equity exposure is held or verified off-chain. This keeps pStock as an
asset-backed receipt, not an unbacked synthetic derivative.

## Actors

| Actor | Responsibility |
| --- | --- |
| Custodian | Holds real IDX shares, initiates or votes on mint/redeem proposals, approves KYC, maintains reserve records. |
| Trader | Buys and sells pStocks through IDRX pools. |
| Redeemer | KYC-approved user who wants to convert pStock into an off-chain securities process. |
| Protocol admin | Configures protocol dependencies and fee parameters. |
| Backend operator | Runs API, database, optional storage, and operational dashboards. |

## Spot-backed lifecycle

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

## Market exit problem

The product is designed around a practical market problem: Indonesian equities
do not trade 24/7, while macro and micro risk can move outside local exchange
hours.

```text
IDX market closes
    -> global macro or issuer-specific news happens
    -> user cannot exit through the exchange overnight
    -> pre-open can gap down
    -> lower auto-rejection / downside price limit can block executable sells
    -> broker cash settlement remains tied to the securities settlement cycle
```

PulsarFi creates a secondary on-chain exit path through pStock/IDRX liquidity.
The off-chain share remains custodied, but the receipt can trade against IDRX
when the traditional exchange path is unavailable, delayed, or constrained by
price bands.

## Custodian-maintained soft peg

PulsarFi uses a custodian-maintained soft peg to IDX reference prices. The pool
price can move away from the IDX reference price because it is an AMM market, but
custodians are responsible for keeping the system spot-backed and economically
anchored.

When pStock trades at a premium:

```text
custodian verifies or acquires underlying IDX exposure
    -> requests backed mint
    -> receives threshold approval
    -> executes mint with IDRX liquidity
    -> adds supply/liquidity near reference value
```

When pStock trades at a discount:

```text
market maker or approved actor buys discounted pStock
    -> KYC-gated redeem path locks pStock
    -> custodians approve redemption
    -> pStock is burned
    -> off-chain settlement process handles the underlying exit
```

This is why the peg is not purely algorithmic. A bot cannot safely mint unbacked
supply. Custodian inventory and reserve attestations are part of the peg
maintenance process.

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
- Custodian-operated market making is expected to pull the pool price back toward fair value.
- The UI can show both IDX-derived market price and pool price.
- Slippage controls protect users from poor execution.

Every trade pays two fees, always denominated in IDRX: Uniswap's native 0.3%
(stays in the pool as protocol-owned liquidity) and an explicit `swapFeeBps`
protocol fee (realized immediately toward the treasury/custodian split — see
[Fee distribution](#fee-distribution)).

## Fee distribution

PulsarFi deliberately keeps two kinds of fee growth separate:

- **AMM LP fee (0.3%, Uniswap V2 native)** — never withdrawn. It compounds
  inside each pStock/IDRX pool's reserves. Since `PulsarProtocol` is the sole
  LP on every pair, this growth is 100% protocol-owned, but its value is
  realized indirectly: deeper pools mean less slippage, more volume, and a
  stronger buffer during redemption runs. Extracting it would mean removing
  liquidity from the very pool that backs the peg, so it stays untouched.
- **Protocol fee (`swapFeeBps` + `redeemFeeBps`)** — realized immediately in
  IDRX and tracked in `accumulatedFees`. This is the actual revenue that gets
  distributed to the treasury and custodians.

Distribution economics, once `accumulatedFees` reaches
`minimumDistributionThreshold`:

| Recipient | Allocation | Rationale |
| --- | ---: | --- |
| Protocol treasury | 30% | Platform operations, monitoring, and settlement coordination. |
| Active custodians | 70% | Custody, reserve availability, approval operations, and peg maintenance. |

With five active custodians, the custodian allocation is split equally:

```text
70% custodian pool / 5 custodians = 14% per custodian
```

The equal split is simple and reflects that each custodian participates in the
approval committee and standby settlement process. A future allocation policy can
weight the custodian share by reserve contribution, liquidity contribution, or
operational uptime, but the core principle remains the same: custodians earn
protocol fee revenue because they keep pStock spot-backed.

`distributeFees()` is permissionless — anyone, including a custodian collecting
its own share, can trigger it once the threshold is met. The threshold exists
only to keep gas cost from exceeding the payout on a distribution — not as a
business lever.

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
- locked redeem fee joins `accumulatedFees` (confirmed revenue, distributed later via `distributeFees()`);
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

PulsarFi has two revenue paths, both realized through `accumulatedFees` and
`distributeFees()`:

| Revenue path | Mechanism | Recipient |
| --- | --- | --- |
| Protocol swap fee | `swapFeeBps` on every buy/sell, denominated in IDRX. | 70% active custodians, 30% protocol treasury. |
| Redeem fee | IDRX fee charged when a user exits into off-chain settlement. | 70% active custodians, 30% protocol treasury. |

The AMM's own 0.3% LP fee is a separate, non-distributed growth path — see
[Fee distribution](#fee-distribution).

The redeem fee is an exit-platform fee. It compensates the protocol treasury for
the coordination cost of moving from an on-chain receipt into the off-chain
securities settlement process.

## Operational controls

The business process depends on these controls:

- custodian threshold prevents unilateral minting;
- pending mint flag prevents overlapping mint proposals for the same ticker;
- KYC mapping gates redemption;
- backend idempotency prevents duplicate transaction records;
- reserve snapshots create an audit trail for supply changes;
- testnet faucet lets reviewers bootstrap IDRX without manual distribution.

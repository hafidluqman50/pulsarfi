---
id: protocol-design
title: Protocol Design
sidebar_label: Protocol Design
slug: /protocol-design
---

`PulsarProtocol` is the single on-chain entrypoint for PulsarFi. It is UUPS
upgradeable and owns every `PulsarStock` token contract it deploys.

## Roles

| Role | Capability |
| --- | --- |
| `DEFAULT_ADMIN_ROLE` | Configure treasury, router, IDRX address, redeem fee, swap fee, distribution threshold, and authorize upgrades. |
| `CUSTODIAN_ROLE` | Create mint proposals, vote on mint/redeem, execute threshold operations, approve or revoke KYC. |
| User wallet | Swap, request redeem if KYC-approved, transfer ERC-20 tokens, call `distributeFees()` (permissionless). |

The custodian role is operational. The admin role is configurational. Keeping
those responsibilities separate makes protocol operations easier to reason
about.

## Core state

| State | Purpose |
| --- | --- |
| `stocks[ticker]` | Maps pStock ticker to deployed ERC-20 address. |
| `kycApproved[user]` | Determines whether a user can request redemption. |
| `proposals[id]` | Stores mint proposal parameters and vote counters. |
| `redeemRequests[id]` | Stores redeem requests, locked token amount, fee, and vote counters. |
| `hasApproved` / `hasRejected*` | Prevents duplicate votes. |
| `hasPendingRequest[ticker]` | Prevents overlapping mint proposals per ticker. |
| `swapFeeBps` | Protocol fee taken on every `swap()`, denominated in IDRX. |
| `accumulatedFees` | Confirmed protocol revenue (swap fee + executed redeem fee) awaiting distribution. |
| `minimumDistributionThreshold` | Minimum `accumulatedFees` balance before `distributeFees()` can run. |
| `isActiveCustodian[custodian]` | Marks a custodian eligible for fee distribution once they've requested/approved a mint. |

## Mint lifecycle

Minting is a 3-of-5 custodian process. New supply is minted into the protocol and
paired with IDRX liquidity.

```text
requestMint
  input: ticker, stockName, idxTicker, tokenAmount, idrxAmount, attestationHash
  effects:
    - creates proposal
    - marks requester as first approver
    - sets pending flag for ticker

approveMint
  effects:
    - adds approval vote
    - increments approvalCount

rejectMint
  effects:
    - adds rejection vote
    - sets first rejecter as rejectInitiator

executeMint
  requirements:
    - caller is proposal requester
    - approvalCount >= 3
    - proposal not executed
    - requester has approved enough IDRX
  effects:
    - deploys PulsarStock if needed
    - mints pStock to protocol
    - pulls IDRX from requester
    - adds pStock and IDRX to Uniswap V2
    - clears pending flag

executeRejectMint
  requirements:
    - caller is first rejecter
    - rejectCount >= 3
  effects:
    - marks proposal rejected
    - refunds legacy pre-funded IDRX if present
    - clears pending flag
```

## Redemption lifecycle

Redemption moves a user from permissionless on-chain holding into an off-chain
securities process. It is therefore KYC-gated.

```text
requestRedeem
  requirements:
    - caller has KYC approval
    - ticker exists
    - caller has pStock allowance
    - caller has IDRX allowance if redeem fee is active
  effects:
    - pulls pStock into protocol
    - pulls IDRX fee if configured
    - creates redeem request

approveRedeem
  effects:
    - adds approval vote
    - sets first approver as approveInitiator

rejectRedeem
  effects:
    - adds rejection vote
    - sets first rejecter as rejectInitiator

executeRedeem
  requirements:
    - caller is first approver
    - approvalCount >= 3
  effects:
    - burns locked pStock
    - adds locked fee to accumulatedFees (confirmed revenue, not yet distributed)
    - marks request approved and processed

executeReject
  requirements:
    - caller is first rejecter
    - rejectCount >= 3
  effects:
    - returns locked pStock to user
    - returns IDRX fee to user
    - marks request rejected and processed
```

## Swap design

`swap(ticker, amountIn, amountOutMin, buyStock)` provides a protocol-level
wrapper around Uniswap V2 swaps, plus an explicit protocol fee (`swapFeeBps`)
kept separate from Uniswap's own 0.3% AMM fee.

The AMM fee is never withdrawn — it compounds inside the Uniswap V2 pair's
reserves as protocol-owned liquidity (`PulsarProtocol` is the sole LP on every
pair). The protocol fee, by contrast, is realized immediately in IDRX and
tracked in `accumulatedFees` for later distribution. See
[Market & Revenue Model](/docs/business-flow) for the rationale.

When `buyStock` is true:

```text
tokenIn  = IDRX
tokenOut = pStock

effects:
  - pulls amountIn of IDRX from the user
  - if swapFeeBps > 0: cuts fee from amountIn before it reaches the router
  - swaps the remainder through Uniswap V2, output sent directly to the user
  - fee joins accumulatedFees; emits SwapFeeCollected
```

When `buyStock` is false:

```text
tokenIn  = pStock
tokenOut = IDRX

effects:
  - pulls amountIn of pStock from the user
  - swaps the full amount through Uniswap V2, output routed to the protocol
  - if swapFeeBps > 0: cuts fee from the IDRX output, forwards the remainder to the user
  - fee joins accumulatedFees; emits SwapFeeCollected
```

The user approves the input token to `PulsarProtocol` in both directions.

## Fee distribution lifecycle

```text
distributeFees
  requirements:
    - accumulatedFees >= minimumDistributionThreshold
    - at least one active custodian exists
  effects:
    - resets accumulatedFees to 0
    - transfers 30% (+ integer-division remainder) to treasury
    - transfers 70% split equally across all active custodians
    - emits FeesDistributed
```

Callable by anyone — including a custodian collecting its own share — so
distribution never depends on the admin remembering to trigger it.

## Token units

| Token | Decimals | Notes |
| --- | ---: | --- |
| IDRX mock | 2 | Mirrors the real IDRX decimal model used elsewhere. |
| pStock | 18 | Standard ERC-20 precision for tokenized share receipts. |

All backend raw amounts are stored as integer strings or `NUMERIC(78,0)` so no
precision is lost.

## Protocol invariants

The protocol should preserve these invariants:

- Only `PulsarProtocol` can mint or burn `PulsarStock`.
- A mint proposal cannot execute before threshold approval.
- Only the original mint requester can execute an approved mint.
- Only the first rejecter can execute a threshold mint rejection.
- A redeem request cannot be created by a non-KYC wallet.
- Redeem execution burns locked user tokens.
- Redeem rejection returns locked user tokens.
- New mint supply goes through liquidity provisioning.
- `tx_hash` should be unique in backend records for idempotency.
- Redeem fee only joins `accumulatedFees` at `executeRedeem` (confirmed), never at `requestRedeem` — so a distribution can never sweep funds still owed back to a user whose redeem is later rejected.
- The AMM's 0.3% fee is never withdrawn from pool reserves; only `accumulatedFees` (protocol swap fee + executed redeem fee) is ever distributed.

## Upgrade and router dependency

The protocol is UUPS upgradeable. The current router dependency is Uniswap V2,
deployed from official build artifacts stored in the smart contract project. This
matters because Uniswap V2 router pair address calculation depends on the pair
init code hash. Recompiling the pair with different bytecode can make the router
calculate the wrong deterministic pair address.

Do not deploy Uniswap V2 by recompiling dependency contracts through Foundry.
Use the official artifact deployment scripts already included in the repo.

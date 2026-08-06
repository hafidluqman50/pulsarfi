---
id: protocol-design
title: Protocol Design
sidebar_label: Protocol Design
slug: /protocol-design
---

`PulsarProtocol` is the single on-chain entrypoint for PulsarFi. It is UUPS
upgradeable and owns every `PulsarStock` token contract it deploys. Roughly 20
of its functions actually execute in a separate `PulsarProtocolOps` contract
via `delegatecall` — see [the ops split](#the-ops-split) below — but that is an
internal deploy-size detail, invisible to every caller.

## Roles

| Role | Capability |
| --- | --- |
| `DEFAULT_ADMIN_ROLE` | Configure treasury, router, IDRX address, redeem fee, swap fee, distribution threshold, `opsContract`, V4 pool manager/hook, and authorize upgrades. Unpauses both circuit breakers. |
| `CUSTODIAN_ROLE` | Create mint proposals, vote on mint/redeem, execute threshold operations, approve or revoke KYC, trigger both circuit breakers, run the one-time `migrateV2ToV4`. |
| User wallet | `swapV4`, request redeem if KYC-approved, transfer ERC-20 tokens, call `distributeFees()` or `collectV4Fees()` (both permissionless). |

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
| `swapFeeBps` | Protocol fee taken on every `swapV4`, enforced at the pool level by `PulsarSwapHook` — applies regardless of entry point. |
| `accumulatedFees` | Confirmed protocol revenue (swap fee + executed redeem fee) awaiting distribution. |
| `minimumDistributionThreshold` | Minimum `accumulatedFees` balance before `distributeFees()` can run. |
| `isActiveCustodian[custodian]` | Marks a custodian eligible for fee distribution once they've requested/approved a mint. |
| `poolManager` / `swapHook` | The configured Uniswap V4 `PoolManager` and `PulsarSwapHook` addresses. |
| `poolKeys[ticker]` | The V4 `PoolKey` (currencies, fee tier, tick spacing, hook) for a ticker's pool. |
| `isV4Migrated[ticker]` | Whether swaps for this ticker route to a live V4 pool. |
| `opsContract` | The deployed `PulsarProtocolOps` address, invoked via `delegatecall`. |

## Mint lifecycle

Minting is a 3-of-5 custodian process. New supply is minted into the protocol
and paired with V4 liquidity, creating the ticker's pool on its first mint.

```text
requestMint
  input: ticker, stockName, idxTicker, tokenAmount, idrxAmount, attestationHash
  effects:
    - creates proposal
    - marks requester as first approver
    - sets pending flag for ticker

approveMint  (delegated to PulsarProtocolOps)
  effects:
    - adds approval vote
    - increments approvalCount

rejectMint  (delegated to PulsarProtocolOps)
  effects:
    - adds rejection vote
    - sets first rejecter as rejectInitiator

executeMint
  input: proposalId, sqrtPriceX96
  requirements:
    - caller is proposal requester
    - approvalCount >= 3
    - proposal not executed
    - requester has approved enough IDRX
  effects:
    - deploys PulsarStock if needed
    - mints pStock to protocol
    - pulls IDRX from requester
    - on first mint for the ticker: creates the V4 pool at sqrtPriceX96
      (canonical, orientation-independent — see "Mint price parameter" below)
    - seeds full-range V4 liquidity with the minted pStock + pulled IDRX
    - clears pending flag

executeRejectMint  (delegated to PulsarProtocolOps)
  requirements:
    - caller is first rejecter
    - rejectCount >= 3
  effects:
    - marks proposal rejected
    - refunds legacy pre-funded IDRX if present
    - clears pending flag
```

### Mint price parameter

`sqrtPriceX96` is `sqrt(IDRX_raw per 1 stock_raw) * 2^96` — computed off-chain
from the proposal's stored `idrxAmount`/`tokenAmount` — and is
orientation-independent: the caller does not need to know the (lazily
deployed) stock's address to compute it, since the contract flips it to the
pool's actual currency ordering (`currency0`/`currency1`, sorted by address)
on-chain via `FullMath`, with no on-chain square root. It is only read on the
first mint for a ticker; later mints ignore it and just add to the existing
pool.

## Redemption lifecycle

Redemption moves a user from permissionless on-chain holding into an off-chain
securities process. It is therefore KYC-gated.

```text
requestRedeem
  requirements:
    - caller has KYC approval
    - ticker exists and has a live V4 pool
    - caller has pStock allowance
    - caller has IDRX allowance if redeem fee is active
  effects:
    - quotes the stock's IDRX value from the V4 pool's own spot price
      (StateLibrary.getSlot0 + FullMath — same math the pool itself uses)
    - pulls pStock into protocol
    - pulls IDRX fee if configured
    - creates redeem request

approveRedeem  (delegated to PulsarProtocolOps)
  effects:
    - adds approval vote
    - sets first approver as approveInitiator

rejectRedeem  (delegated to PulsarProtocolOps)
  effects:
    - adds rejection vote
    - sets first rejecter as rejectInitiator

executeRedeem  (delegated to PulsarProtocolOps)
  requirements:
    - caller is first approver
    - approvalCount >= 3
  effects:
    - burns locked pStock
    - adds locked fee to accumulatedFees (confirmed revenue, not yet distributed)
    - marks request approved and processed

executeReject  (delegated to PulsarProtocolOps)
  requirements:
    - caller is first rejecter
    - rejectCount >= 3
  effects:
    - returns locked pStock to user
    - returns IDRX fee to user
    - marks request rejected and processed
```

The redeem fee quote (`quoteStockToIdrx`) is also exposed as a public view for
off-chain price feeds and UI quotes, so on-chain and off-chain never drift.

## Swap design

`swapV4(ticker, amountIn, minOut, buyStock)` is the only swap entrypoint. There
is no protocol-level fee-cutting logic in `swapV4` itself — the protocol fee is
enforced entirely by `PulsarSwapHook` at the **pool level**, which means it
applies no matter how a swap reaches the pool: through `PulsarProtocol`,
through an aggregator, or directly through Uniswap's own UI. This closes a gap
that existed under the old V2 design, where the fee only applied to swaps
routed through the protocol's own `swap()` wrapper.

The fee is always denominated in IDRX, in both directions:

```text
buyStock = true   (IDRX in, pStock out)
  - IDRX is the swap INPUT
  - hook skims the fee off the input in _beforeSwap, before the core swap runs
  - emits BuySideFeeTaken

buyStock = false  (pStock in, IDRX out)
  - IDRX is the swap OUTPUT
  - hook skims the fee off the output in _afterSwap (OZ's audited BaseHookFee path)
  - emits HookFee
```

Both sides accrue as ERC-6909 IDRX claims held by the hook — not moved
per-swap, for gas efficiency. `collectV4Fees()` (permissionless) sweeps those
claims to the protocol, redeems them for real IDRX, and books the exact
collected amount into `accumulatedFees`:

```text
collectV4Fees
  effects:
    - calls PulsarSwapHook.handleHookFees to sweep IDRX claims to the protocol
    - redeems the claim for real IDRX (burn claim, take real tokens — nets to zero in the unlock)
    - measures the actual balance delta (not a return value) and adds it to accumulatedFees
    - emits V4FeesCollected
```

The pool's own AMM fee (0.3% LP fee, configurable per pool) is never
withdrawn — it compounds inside the pool as protocol-owned liquidity, since
`PulsarProtocol` is the sole LP on every pStock/IDRX pool. See [Market &
Revenue Model](/docs/business-flow) for the economic rationale.

The user approves the input token to `PulsarProtocol` in both directions, same
as before.

## Fee distribution lifecycle

```text
distributeFees  (delegated to PulsarProtocolOps)
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

## Circuit breakers

Three escalating levers, each with a distinct blast radius:

| Lever | Gate | Effect |
| --- | --- | --- |
| `pause()` / `unpause()` | Custodian pauses; admin unpauses. | Blocks `executeMint`, `swapV4`, `addV4Liquidity` (protocol entry points only). Reject/refund/withdrawal paths stay open so funds are never trapped. |
| `pauseHook()` / `unpauseHook()` | Custodian pauses; admin unpauses. | Pool-level breaker: halts **all** V4 swaps for every pool, through any entry point, without touching liquidity. Narrower than `emergencyWithdrawV4`. |
| `emergencyWithdrawV4(ticker)` (delegated) | Custodian only. | Pauses the hook **and** pulls the ticker's entire V4 liquidity back to the protocol. Heaviest lever, incident-only. |

`pause()` alone cannot stop a swap that reaches the pool through a path other
than the protocol (an aggregator, or Uniswap's own UI) — that is exactly what
`pauseHook`/`emergencyWithdrawV4` are for, since they act on the hook itself,
which every pool for every ticker shares.

## The ops split

`PulsarProtocol`'s compiled bytecode exceeded the EIP-170 24,576-byte limit
once the V4 cutover's mint/redeem/fee/circuit-breaker logic was added. Rather
than remove any capability, ~20 less-frequently-called functions were moved
into a separately deployed `PulsarProtocolOps` contract, invoked via
`delegatecall` from a one-line dispatcher on `PulsarProtocol`:

```solidity
function approveMint(uint256 proposalId) external {
    _delegateToOps();
}
```

`_delegateToOps()` forwards the entire original `msg.data` unmodified — no
`abi.encodeCall` re-encoding, which is cheaper and was the difference between
fitting and not fitting under the size limit. Because `delegatecall` preserves
storage context, `PulsarProtocolOps`'s code runs against `PulsarProtocol`'s
own storage, balances, and `msg.sender` — from every caller's perspective, the
function behaves identically to a normal call.

Functions moved to `PulsarProtocolOps`:

- Mint: `fundMintLiquidity` (deprecated), `approveMint`, `rejectMint`, `executeRejectMint`
- Redeem: `approveRedeem`, `rejectRedeem`, `executeReject`, `executeRedeem`
- V4 liquidity: `addV4Liquidity`, `migrateV2ToV4`, `emergencyWithdrawV4`
- Fees: `distributeFees`
- KYC: `approveKYC`, `revokeKYC`
- Admin: `setTreasury`, `setRedeemFeeBps`, `setSwapFeeBps`, `setMinimumDistributionThreshold`, `setRouter`, `setIDRX`

Functions that stay directly on `PulsarProtocol` (hot path, full gas
efficiency, no delegatecall overhead): `requestMint`, `executeMint`,
`requestRedeem`, `swapV4`, `collectV4Fees`, `configureV4`, `setOpsContract`,
`pause`/`unpause`/`pauseHook`/`unpauseHook`, `unlockCallback`.

`unlockCallback` deserves a special note: `poolManager.unlock()` routes its
callback to `address(this)` — the proxy — regardless of whether the code that
called `unlock()` was executing directly on `PulsarProtocol` or via
delegatecall from `PulsarProtocolOps`. This means `PulsarProtocolOps`'s
`addV4Liquidity`/`migrateV2ToV4`/`emergencyWithdrawV4` reuse
`PulsarProtocol`'s existing liquidity-management dispatch for free, with zero
duplicated logic. Only `_createV4Pool` (a synchronous call with no callback
routing) needed its own copy in `PulsarProtocolOps` — a cost that only counts
against `PulsarProtocolOps`'s own size budget, which has an order of magnitude
more headroom.

Both contracts inherit `PulsarProtocolStorage`, an abstract base declaring
every state variable — never deployed on its own, purely to guarantee
identical slot layout by construction rather than by hand-counting.
`PulsarProtocolOps` is never called directly with any effect: its own storage
trie is permanently empty (it is never `initialize()`d), so every real check
it runs — role checks, ticker/pool lookups — reads empty storage and fails
closed for anyone bypassing the proxy.

## Token units

| Token | Decimals | Notes |
| --- | ---: | --- |
| IDRX mock | 2 | Mirrors the real IDRX decimal model used elsewhere. |
| pStock | 18 | Standard ERC-20 precision for tokenized share receipts. |

All backend raw amounts are stored as integer strings or `NUMERIC(78,0)` so no
precision is lost. The V4 pool's own price already encodes the IDRX(2)/
stock(18) decimal gap — no extra scaling is needed when reading `getSlot0`.

## Protocol invariants

The protocol should preserve these invariants (the fee/solvency ones below are
enforced by a stateful Foundry invariant suite that fuzzes buy/sell/setFee/
collect/distribute sequences):

- Only `PulsarProtocol` can mint or burn `PulsarStock`.
- A mint proposal cannot execute before threshold approval.
- Only the original mint requester can execute an approved mint.
- Only the first rejecter can execute a threshold mint rejection.
- A redeem request cannot be created by a non-KYC wallet.
- Redeem execution burns locked user tokens.
- Redeem rejection returns locked user tokens.
- New mint supply goes through V4 liquidity provisioning.
- `tx_hash` should be unique in backend records for idempotency.
- Redeem fee only joins `accumulatedFees` at `executeRedeem` (confirmed), never at `requestRedeem` — so a distribution can never sweep funds still owed back to a user whose redeem is later rejected.
- The protocol's IDRX balance must always cover `accumulatedFees` (solvency).
- Every raw IDRX unit the hook has ever taken as a fee is accounted for exactly once: uncollected in the hook's ERC-6909 claim, collected but undistributed in `accumulatedFees`, or already paid out via `distributeFees` (fee conservation).
- A direct call to `PulsarProtocolOps` bypassing the proxy must fail closed on every real check (empty storage, no roles granted there).

## Upgrade and dependencies

The protocol is UUPS upgradeable. Its live AMM dependency is the official
Uniswap V4 `PoolManager`, configured via `configureV4`, plus the deployed
`PulsarSwapHook` for pool-level fee enforcement.

The V2 `router` dependency is kept only for the one-time `migrateV2ToV4`
runbook step, used to pull liquidity out of tickers (BUMIP, ENRGP) that
already had V2 history at the time of the V4 cutover. It is deployed from
official Uniswap V2 build artifacts stored in the smart contract project —
this matters because the V2 router's pair address calculation depends on the
pair init code hash, so recompiling the pair with different bytecode would
make the router compute the wrong deterministic pair address. Do not deploy
Uniswap V2 by recompiling dependency contracts through Foundry; use the
official artifact deployment scripts already included in the repo. A fresh
(e.g. mainnet) launch with no V2 history would never need this path at all.

# PulsarFi — Business Model

## Custodian Model

Custodians are **independent licensed financial institutions** (e.g. Mirae Asset, Mandiri Sekuritas, Ajaib, Growin) — not a centralized operator.

By signing the custodian agreement and joining the multisig, they commit to the fee structure below. Mint = agreement accepted.

Trustless by design: custodians are competitors. Colluding to fraudulently mint requires 3/5 agreement across rival firms — economically irrational.

---

## Revenue Streams

### B2C — User-Facing

| Source | Mechanism | Recipient |
|---|---|---|
| AMM LP fee | 0.3% (Uniswap V2 native) on every buy/sell | Pool reserves — protocol-owned liquidity, not withdrawn |
| Protocol swap fee | `swapFeeBps` on every buy/sell, denominated in IDRX | `accumulatedFees` (in-contract) |
| Redeem fee | `redeemFeeBps` basis points on every redemption | `accumulatedFees` (in-contract) |

### B2B — Custodian-Facing

Custodians earn a share of `accumulatedFees` via `distributeFees()` — see below.

---

## Two Kinds of "Fee Growth" — Kept Separate on Purpose

The 0.3% AMM fee and the protocol fee are **not the same thing**, and mixing them was the original design flaw:

- **AMM LP fee (0.3%)**: never withdrawn. It compounds inside each Uniswap V2 pair's reserves. Since `PulsarProtocol` is the sole LP on every pair (`addLiquidity(..., to: address(this), ...)`), this growth is 100% protocol-owned — but its value is realized *indirectly*: deeper pools mean less slippage, more trading volume, and a stronger peg buffer during redemption runs. Extracting it would mean removing liquidity from the very pool that backs the peg, so it is intentionally left untouched.
- **Protocol swap fee + redeem fee**: realized *immediately* and *explicitly*, following the same pattern already used by `redeemFeeBps` — a bps cut taken directly in IDRX, never entangled with AMM reserves. This is the actual treasury/custodian revenue.

## Protocol Swap Fee (`swapFeeBps`)

Always denominated in IDRX regardless of trade direction:
- Buying stock: cut from `amountIn` before it reaches the router.
- Selling stock: cut from the IDRX output after the swap, before it's sent to the user.

Set by admin via `setSwapFeeBps(uint256 feeBps)` — max 10% (1000 bps).

---

## Fee Distribution (`distributeFees`)

Confirmed fees (protocol swap fee + executed redeem fee) accumulate in `accumulatedFees`, tracked separately from any IDRX still locked as pending redeem escrow (so a distribution can never accidentally sweep funds owed back to a user whose redeem is later rejected).

`distributeFees()` is **permissionless** — anyone can call it, including a custodian collecting its own share — once `accumulatedFees` reaches `minimumDistributionThreshold`:

```
accumulatedFees (once ≥ minimumDistributionThreshold)
├── 30% → treasury
└── 70% → active custodians, equal split
            Active = has ever approved/requested a mint proposal
            e.g. 5 active custodians = 14% each
```

### Distribution Threshold

`minimumDistributionThreshold` (admin-set via `setMinimumDistributionThreshold`) exists purely to avoid dust distributions where gas cost exceeds the payout — not as a business lever. Starting point: ~0.1–0.2% of pool TVL, tuned after real volume data is available.

---

## Mint — All to Liquidity Pool

All mints go directly to the Uniswap V2 liquidity pool. There is no OTC / OperatorWallet destination.

**Why:** An OTC mint path creates tokens without proportionally funding the pool. If the OTC recipient sells into the pool, IDRX drains and price depegs. Since the pool is the only on-chain liquidity venue, every new token supply must be matched by IDRX liquidity.

Institutional buyers wanting large positions should swap from the pool (accepting normal slippage) or exit via `requestRedeem`. Protocol-owned liquidity grows with each new mint, deepening the pool over time.

## Mint Fee — None

Custodians already spend IDRX to fund liquidity at `executeMint`. Adding a protocol fee on top increases their cost and disincentivizes onboarding new custodians.

Custodian compensation comes from `distributeFees()` above.

---

## Redeem Fee — Exit Fee Model

Users pay a fee in IDRX upon redemption, locked at `requestRedeem` time. Fee joins `accumulatedFees` on `executeRedeem` (confirmed only — never before, to avoid touching funds still owed back to the user); returned to user on `executeReject`.

Rate is set by admin via `setRedeemFeeBps(uint256 feeBps)` — max 10% (1000 bps). Default 0 until admin configures it.

Rationale:

- Zero friction to enter the ecosystem (no mint fee for users buying from pool)
- Fee on exit aligns protocol incentives with keeping liquidity in the ecosystem
- Analogous to exit fee in mutual funds / early-redemption penalty
- Treasury accumulates IDRX that can fund operations or be redistributed

---

## Recommended Defaults

| Parameter | Default | Why |
|---|---|---|
| `swapFeeBps` | 20 bps (0.2%) | Total swap cost (0.3% AMM + 0.2% protocol) lands near typical Indonesian brokerage fees (~0.15–0.4%) — competitive, not free. |
| `redeemFeeBps` | 50 bps (0.5%) | Light exit fee, in line with mutual-fund early-redemption penalties (0.5–2%), well under the 10% cap. |
| `minimumDistributionThreshold` | ~0.1% of pool TVL | Just enough to keep `distributeFees()` calls from costing more gas than they pay out. |
| Treasury / custodian split | 30% / 70% | Majority to custodians, who carry the custody/attestation risk. |

All four are admin-adjustable post-deploy — these are conservative starting points, not fixed policy.

---

## Roadmap

| Phase | Model |
|---|---|
| V1 (now) | PulsarFi as trusted operator, custodian = internal team |
| V2 | Onboard licensed broker-dealers as independent custodians, off-chain SLA + on-chain multisig |
| V3 | On-chain proof of reserve, decentralized KYC (zkKYC / Polygon ID), Uniswap V4 hooks |

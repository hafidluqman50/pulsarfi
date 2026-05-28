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
| Swap fee | 0.3% LP fee on every buy/sell via Uniswap V2 | Accumulates in pool |
| Redeem fee | `redeemFeeBps` basis points on every redemption | Treasury |

### B2B — Custodian-Facing

LP fees accumulate as pool reserve growth (Uniswap V2 does not separate fees from reserves). Extraction is via `collectFees()` — admin-triggered, subject to minimum reserve constraint per pool.

---

## Fee Distribution (`collectFees`)

```
Total collected IDRX
├── 30% → Treasury (PulsarFi operational)
└── 70% → Active custodians, equal split
            Active = has approved at least 1 mint proposal
            e.g. 5 active custodians = 14% each
```

### Minimum Reserve Constraint

`collectFees` may only be called if pool reserve after withdrawal remains above the per-ticker minimum. This prevents pool collapse.

Minimum is set per ticker by admin via `setPoolMinimumReserve(string ticker, uint256 minimumIdrx)`.

---

## Mint — All to Liquidity Pool

All mints go directly to the Uniswap V2 liquidity pool. There is no OTC / OperatorWallet destination.

**Why:** An OTC mint path creates tokens without proportionally funding the pool. If the OTC recipient sells into the pool, IDRX drains and price depegs. Since the pool is the only on-chain liquidity venue, every new token supply must be matched by IDRX liquidity.

Institutional buyers wanting large positions should swap from the pool (accepting normal slippage) or exit via `requestRedeem`. Protocol-owned liquidity grows with each new mint, deepening the pool over time.

## Mint Fee — None

Custodians already spend IDRX to fund liquidity at `executeMint`. Adding a protocol fee on top increases their cost and disincentivizes onboarding new custodians.

Custodian compensation comes from LP fee distribution above.

---

## Redeem Fee — Exit Fee Model

Users pay a fee in IDRX upon redemption, locked at `requestRedeem` time. Fee goes to `treasury` on `executeRedeem`; returned to user on `executeReject`.

Rate is set by admin via `setRedeemFeeBps(uint256 feeBps)` — max 10% (1000 bps). Default 0 until admin configures it.

Rationale:

- Zero friction to enter the ecosystem (no mint fee for users buying from pool)
- Fee on exit aligns protocol incentives with keeping liquidity in the ecosystem
- Analogous to exit fee in mutual funds / early-redemption penalty
- Treasury accumulates IDRX that can fund operations or be redistributed

---

## Roadmap

| Phase | Model |
|---|---|
| V1 (now) | PulsarFi as trusted operator, custodian = internal team |
| V2 | Onboard licensed broker-dealers as independent custodians, off-chain SLA + on-chain multisig |
| V3 | On-chain proof of reserve, decentralized KYC (zkKYC / Polygon ID), Uniswap V4 hooks |

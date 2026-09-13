# Market Stats Data Gaps

| | |
|---|---|
| **Version** | 1.1 |
| **Status** | Implemented |
| **Date Created** | 2026-09-07 |
| **Last Updated** | 2026-09-07 |

**Note on scope.** Three unrelated symptoms reported together on the stock detail page and home page: Pool Price empty for every stock, Total Supply/IDX Mkt Cap empty for BRPTP specifically, and 24h Volume showing 0. Investigated together, fixed separately — they had three different root causes.

| Version | Date | Change |
|---|---|---|
| 1.1 | 2026-09-07 | **§2's "custodian needs to submit attestations" conclusion was half right — checked live before accepting it: attestations are purely an off-chain DB record (confirmed no on-chain attestation event/registry exists at all, the only indexer in the repo watches `Transfer` only), but the real mint data those attestations should reflect was already sitting in `mint_proposals`, just never mirrored.** Every one of the 6 stocks with zero `stock_attestations` rows (BBRIP, BRPTP, PTROP, BMRIP, BBCAP, BDMNP) had exactly one `executed` mint proposal each (all minted 2026-08-19, 10 tokens each) and zero redeems — confirmed against the existing BUMIP/ENRGP attestations' own pattern (each attestation = running total of executed mints minus executed redeems, e.g. BUMIP's 3rd attestation, 99990 tokens, is exactly its 100000-token cumulative mint total minus its one 10-token executed redeem) before trusting the derivation. Backfilled: `INSERT INTO stock_attestations (stock_id, custodian_holdings, on_chain_supply, attestation_hash, attested_at) SELECT stock_id, token_amount, token_amount, attestation_hash, executed_at FROM mint_proposals WHERE stock_id IN (5,9,10,11,12,13) AND status = 'executed'` — real hashes and timestamps from the actual executed mints, nothing fabricated. Verified live: `GET /public/reserves` now returns real `on_chain_supply` for all 8 tokenized stocks, not just 2 |

---

## 1. Pool Price empty for every stock — real bug, fixed

`GET /public/prices/:ticker` (`GetOnchainPriceV4`, `external/price_service.go`) reads an on-chain Uniswap V4 quote via `os.Getenv("PULSAR_PROTOCOL")` — this env var was **missing entirely** from `backend/.env` (only documented in `.env.example`), so every call went out with an empty contract address and always failed silently into a 500, leaving `poolPrice` `undefined` on the frontend regardless of which stock.

**Fix.** Added `PULSAR_PROTOCOL=0x204488318C0E75978B3c851382Aa83f3065a8f5A` to `backend/.env` — cross-checked against three independent sources that all agree on this exact address: `backend/.env.example`, `frontend/.env.local`'s `NEXT_PUBLIC_PULSAR_PROTOCOL_ADDRESS`, and `smart-contract/.env`'s `PULSAR_PROTOCOL_PROXY` (the proxy address, not the implementation address, which is what calls should always go through). Verified live (separate instance, port 8081/8082/8083, never the user's own server on 8080): `GET /public/prices/BUMI` and `/BRPT` both now return `"source":"onchain-v4"` with real prices.

---

## 2. Total Supply / IDX Mkt Cap empty for BRPTP — data gap, not a bug

Both fields come from `stock_attestations` (`reserve_service.go`) — a custodian must manually submit an attestation row per stock; this is not a live on-chain supply query. DB check: BUMIP has 3 attestation rows, BRPTP has zero, even though both have a `contract_address` set. Nothing to fix in code — a custodian needs to submit BRPTP's attestation for these fields to populate, the same as BUMIP's already did.

---

## 3. 24h Volume showing 0 — two stacked causes, both fixed

The obvious cause (this demo's real transaction history is sparse — weeks between trades, so a strict 24h window is legitimately empty most of the time) turned out to not be the only one. `StockTransactionRepository.ComputeStats`'s raw SQL aliases a column `volume_24h`, scanned into a Go struct field `Volume24h` with no explicit `gorm` column tag — GORM's default naming strategy maps `Volume24h` to `volume24h` (no underscore before the digits), not `volume_24h`, so `Scan` never matched the column and this field silently stayed `0` regardless of what the query actually computed, independent of the time window entirely. Found by removing the 24h window first (per the user's own ask, since a demo's own sparse activity means even a 30-day window would sit at the same boundary the current data is right at) and observing the result was still `0` — a real query returning a real non-zero sum would have surfaced this immediately, which is what exposed the deeper bug.

**Fix.** (a) `ComputeStats`'s SQL no longer filters by time at all — sums all-time buy/sell volume (field/column name kept as `volume_24h`/`Volume24h` to avoid an unrelated rename, only the window changed); frontend label changed from "24h Volume" to "Total Volume" (`SwapView.tsx`). (b) Added `gorm:"column:volume_24h"` to `StatsRow.Volume24h` so `Scan` actually populates it. Verified live: `GET /public/stats` now returns `volume_24h: 8938865.07`, matching a hand-computed sum against the real `stock_transactions` rows (11 buys + 1 sell), not `0`.

---

## 4. Verification

- `go build`/`go vet`/`gofmt` and `tsc`/`eslint` all clean.
- All three backend fixes verified live against real data via a separate local instance (ports 8081–8083, never the user's own running server on 8080), never guessed from reading code alone.
- Not touched: `CustodianView.tsx`'s "24h Mint Volume" (`mint_volume_24h_idrx`) — a different metric (custodian mint/burn events, not swap volume), out of scope for this pass.

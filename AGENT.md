# PulsarFi — Agent Strategy & Context Guide

## 1. Project Core Philosophy

PulsarFi is an Asset-Backed Tokenization Platform (RWA) on Arbitrum Sepolia that tokenizes Indonesian IDX equities (BUMIP, ENRGP, etc.) into 1:1 on-chain receipts paired against IDRX liquidity pools.

**Fundamental Rules:**

- **Asset-Backed, NOT Synthetic.** Every token minted represents actual shares held 1:1 in custody at KSEI. Price discovery comes from AMM mechanics and arbitrage, not oracles.
- **USDC Compliance Model.** Anyone can hold and swap tokens permissionlessly (no upfront KYC). KYC is enforced only at the Redemption Gateway when converting tokens back to physical securities.

---

## 2. Token Naming Convention

All asset tokens use ALL-CAPS with a `P` suffix indicating Pulsar origin.

- Valid: `BUMIP`, `ENRGP`, `KIJAP`, `TLKMP`, `BBRIP`, `GOTOP`, `ASIIP`, `UNVRP`
- Forbidden: `pBUMI`, `pENRG`, `PBUMI`, `PENRG`

---

## 3. Architecture

### Smart Contracts (`/smart-contract`)
- **`PulsarProtocol.sol`** — UUPS upgradeable proxy, single entry point for all protocol operations.
  - On-chain 3/5 multisig: `requestMint` → `approveMint` → `executeMint` (only requester can execute after threshold)
  - `MintDestination`: `OperatorWallet` or `LiquidityPool`
  - `swap(ticker, amountIn, amountOutMin, buyStock)` — permissionless, no auth required
  - `redeem(ticker, user, amount, attestationHash)` — custodian only
  - `approveKYC` / `revokeKYC` — admin only
- **`PulsarStock.sol`** — Independent ERC20 per stock, owned by PulsarProtocol. Deployed lazily on first `executeMint`.
- **`IDRX.sol`** — Mock stablecoin (2 decimals), deployed alongside protocol. Ownership transferred to PulsarProtocol so `_provideToPool` can mint.
- **Router**: Uniswap V2 (custom deploy on Arbitrum Sepolia — not available by default).
- **Upgrade path**: UUPS → Uniswap V4 with `beforeSwap` KYC hooks post-MVP.

### Backend (`/backend`)
- Go + Gin + GORM + PostgreSQL
- Structure: `src/{app,auth,config,http,logger,model,repository,service}`
- **Auth**: SIWE (Sign-In with Ethereum, EIP-4361) for ALL users — custodians and retail alike. No email/password.
  - `GET /api/v1/auth/nonce?address=0x...` → nonce (5-min TTL, one-time use)
  - `POST /api/v1/auth/verify` → `{ address, message, signature, nonce }` → JWT
  - JWT payload: `{ wallet_address, role: "custodian"|"user" }`
  - Custodian role determined by wallet lookup in `custodians` table at verify time.
- **Response format** (all endpoints):
  ```json
  { "status_code": 200, "message": "...", "data": {...} }
  ```
- **Event indexing**: WebSocket subscription (not polling) to Alchemy to sync on-chain events into DB. Avoids burning Alchemy CU budget.

### Frontend (`/frontend`)
- Next.js + TailwindCSS + RainbowKit (wallet connection + SIWE)
- `/swap` — permissionless swap view
- `/custodian` — multisig mint dashboard, KYC management, pending proposals
- Sonner for all toast/notification UI

---

## 4. Custodian Mint Flow (3/5 Multisig)

```
Custodian A  →  requestMint(ticker, stockName, idxTicker, tokenAmount, idrxAmount, attestationHash, destination)
                 └─ creates proposal, approvalCount = 1, hasPendingRequest[ticker] = true

Custodian B/C →  approveMint(proposalId)
                 └─ increments approvalCount

Custodian A  →  executeMint(proposalId)   ← only requester, only after approvalCount >= 3
                 └─ _ensureStock → deploy PulsarStock if first mint
                 └─ _mint → PulsarStock.mint(to, amount)
                 └─ if LiquidityPool → _provideToPool → IDRX.mint + addLiquidity
```

---

## 5. Database Tables

| Table | Purpose |
|---|---|
| `custodians` | 5 multisig participants (wallet_address, name, email for notifications) |
| `stocks` | Listed tokens; `contract_address` null until first executeMint |
| `mint_proposals` | Mirror of on-chain proposals |
| `mint_approvals` | One row per custodian approval event |
| `wallet_verifications` | KYC records, type: retail/institution, PDF in Supabase Storage |
| `stock_transactions` | Swap events: side buy/sell, idrx_amount, stock_amount (NUMERIC 78,0) |

---

## 6. Key Constraints

- IDRX decimals = 2 (matches real IDRX on Base/other chains)
- PulsarStock decimals = 18
- Amounts stored in DB as `NUMERIC(78,0)` — raw on-chain uint256
- `stocks(ticker)` is UNIQUE, not PK — all FK relations use `stocks(id)`
- Nonce store is in-memory (sufficient for hackathon MVP)
- IDRX not deployed on Arbitrum Sepolia by default — use `IDRX.sol` mock

---

## 7. Router Versioning

- **MVP**: Uniswap V2, custom-deployed on Arbitrum Sepolia
- **Production**: Uniswap V4 with `beforeSwap` KYC hooks via Horizon Labs Identity Registry

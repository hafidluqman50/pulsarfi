# PulsarFi

Asset-Backed Tokenization Platform for Indonesian IDX equities on Arbitrum Sepolia.

Tokenizes stocks like BUMIP, ENRGP into 1:1 on-chain receipts paired against IDRX liquidity pools via Uniswap V2. Built for the Arbitrum Open House London Hackathon.

---

## Architecture

```
smart-contract/   Solidity (Foundry) — PulsarProtocol, PulsarStock, IDRX mock
backend/          Go + Gin + GORM + PostgreSQL
frontend/         Next.js + RainbowKit + TailwindCSS
```

**Smart Contracts**
- `PulsarProtocol` — UUPS upgradeable proxy, 3/5 on-chain multisig mint pipeline
- `PulsarStock` — ERC20 per stock, owned by PulsarProtocol, deployed lazily
- `IDRX` — Mock stablecoin (2 decimals), ownership transferred to protocol at deploy

**Auth** — SIWE (EIP-4361) for everyone. Wallet connects via RainbowKit → signs message → JWT issued with `role: custodian|user`.

---

## Quick Start

### Smart Contracts

```bash
cd smart-contract
cp .env.example .env   # fill PRIVATE_KEY, UNISWAP_V2_ROUTER, CUSTODIAN_1..5

forge build
forge test

forge script script/Deploy.s.sol --rpc-url $ARB_SEPOLIA_RPC --broadcast --verify
```

### Backend

```bash
cd backend
cp .env.example .env   # fill DATABASE_URL, JWT_SECRET, etc.

# run migration
psql $DATABASE_URL -f migrations/001_initial_schema.sql

go run main.go
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

---

## API

All responses follow:
```json
{ "status_code": 200, "message": "...", "data": {} }
```

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/v1/auth/nonce?address=0x...` | — | Issue SIWE nonce |
| POST | `/api/v1/auth/verify` | — | Verify signature, get JWT |
| GET | `/api/v1/public/stocks` | — | List all listed stocks |
| POST | `/api/v1/custodian/...` | JWT (custodian) | Mint proposals, KYC management |

---

## Token Naming

All tokens: ALL-CAPS + `P` suffix — `BUMIP`, `ENRGP`, `KIJAP`, `TLKMP`, `BBRIP`, `GOTOP`, `ASIIP`, `UNVRP`.

---

## Roadmap

- **MVP** — Uniswap V2, SIWE auth, 3/5 multisig, KYC at redemption
- **v2** — Uniswap V4 with `beforeSwap` KYC hooks, mainnet IDRX bridge

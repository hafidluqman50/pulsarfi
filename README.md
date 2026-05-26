# PulsarFi

Asset-Backed Tokenization Platform for Indonesian IDX equities on Arbitrum Sepolia.

Tokenizes stocks like BUMIP, ENRGP into 1:1 on-chain receipts paired against IDRX liquidity pools via Uniswap V2. Built for the Arbitrum Open House London Hackathon.

---

## Engineering Ownership

This repository uses AI-assisted development workflows, but the code is developer-owned. Architecture, integration, debugging, smart contract reasoning, and security-critical decisions are handled manually by the developer.

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
- `IDRX` — Mock stablecoin (2 decimals) for Arbitrum Sepolia. LP minting uses custodian-funded IDRX via ERC20 allowance, so the protocol is not coupled to minting mock IDRX.
- `UniswapV2Factory` / `UniswapV2Router02` — deployed from official Uniswap build artifacts in `smart-contract/script/artifacts`. This keeps the router's hardcoded pair init code hash aligned with the factory pair bytecode.

**Auth** — SIWE (EIP-4361) for everyone. Wallet connects via RainbowKit → signs message → JWT issued with `role: custodian|user`.

---

## Arbitrum Sepolia Deployment

| Contract | Address | Status |
|---|---:|---|
| PulsarProtocol proxy | `0x204488318C0E75978B3c851382Aa83f3065a8f5A` | Verified |
| PulsarProtocol implementation | `0x37b032989A095b882a25D2BFf36ca37d79f6Df6F` | Verified |
| IDRX mock | `0x03b53A71C5517907006EAb512A31C1eD5a56Ae64` | Verified |
| UniswapV2Factory | `0x4254378E95dBD9816a1a18428A81B4E1fBe5C296` | Verified |
| UniswapV2Router02 | `0xFEf655B2A0742134242711b80899d0b543A74223` | Verified |
| WETH | `0x980B62Da83eFf3D4576C647993b0c1D7faf17c73` | External |

Keep `smart-contract/.env` and `frontend/.env.local` aligned with these addresses.
`PulsarStock` tokens are deployed lazily on first successful `executeMint`; verify each stock token address after it exists.

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

For upgrades, `script/Upgrade.s.sol` can deploy official Uniswap V2 artifacts, a testnet IDRX mock, upgrade the UUPS implementation, and update protocol config:

```bash
DEPLOY_UNISWAP_V2=true \
DEPLOY_IDRX_MOCK=true \
MINT_IDRX_TO=0x... \
MINT_IDRX_AMOUNT=880000000 \
forge script script/Upgrade.s.sol:UpgradeScript \
  --rpc-url $RPC_URL \
  --broadcast \
  --with-gas-price 60000000 \
  --priority-gas-price 10000000
```

Liquidity pool minting requires IDRX allowance because the protocol pulls IDRX with `transferFrom` at execution time. The flow is: `requestMint` (no IDRX needed) → 3/5 custodian approvals → requester approves IDRX → `executeMint` (protocol pulls IDRX and adds liquidity in one transaction). `fundMintLiquidity` exists for storage layout compatibility but is no longer part of the normal flow.

### Deploying The Uniswap V2 Router

Use the official Uniswap V2 build artifacts already stored in `smart-contract/script/artifacts`:

- `UniswapV2Factory.json`
- `UniswapV2Router02.json`

The deployment scripts call `_deployOfficialUniswapV2Factory()` and `_deployOfficialUniswapV2Router()` from `script/OfficialUniswapV2.s.sol`. Use these scripts instead of `deployCode("UniswapV2Factory.sol:UniswapV2Factory", ...)`.

Deploy or replace the router during an upgrade:

```bash
cd smart-contract
set -a && source .env && set +a

DEPLOY_UNISWAP_V2=true \
forge script script/Upgrade.s.sol:UpgradeScript \
  --rpc-url "$RPC_URL" \
  --broadcast \
  --with-gas-price 60000000 \
  --priority-gas-price 10000000
```

Why this must not be wrong: `UniswapV2Router02` uses `UniswapV2Library.pairFor()`, which computes pair addresses from a hardcoded pair init code hash. If the factory deploys pair bytecode from a different build, the factory can create the real pair successfully but the router will calculate another pair address and call a non-contract address during `addLiquidity`. The visible failure is an `executeMint` revert inside liquidity provisioning even though requester, approvals, and balances are valid.

After deploy, verify:

```bash
cast call $PULSAR_PROTOCOL_PROXY "router()(address)" --rpc-url "$RPC_URL"
cast call $UNISWAP_V2_ROUTER "factory()(address)" --rpc-url "$RPC_URL"
cast call $UNISWAP_V2_FACTORY "feeToSetter()(address)" --rpc-url "$RPC_URL"
```

The router must point to the same factory stored in `.env`, and both contracts must be verified on Arbiscan.

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

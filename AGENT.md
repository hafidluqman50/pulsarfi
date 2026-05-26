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
  - Mint 3/5 multisig: `requestMint` → `approveMint` → `executeMint` (only requester executes) OR `rejectMint` → `executeRejectMint` (first rejecter executes)
  - `MintDestination`: `OperatorWallet` (institutional) or `LiquidityPool`
  - Liquidity pool minting uses custodian-funded IDRX. The protocol must not mint IDRX internally; it pulls IDRX with ERC20 `transferFrom`, so the requester/funder must approve the protocol first.
  - Redeem 3/5 multisig: `requestRedeem(ticker, tokenAmount, userAddress)` — custodian only, KYC checked on userAddress → `approveRedeem` → `executeRedeem` OR `rejectRedeem` → `executeReject`
  - `swap(ticker, amountIn, amountOutMin, buyStock)` — permissionless, no auth required
  - `approveKYC(userAddress)` / `revokeKYC(userAddress)` — admin only
- **`PulsarStock.sol`** — Independent ERC20 per stock, owned by PulsarProtocol. Deployed lazily on first `executeMint`.
- **`IDRX.sol`** — Mock stablecoin (2 decimals) for Arbitrum Sepolia. Production/mainnet must configure the real IDRX token and rely on user/custodian balances + allowances.
- **Router**: Uniswap V2 (custom deploy on Arbitrum Sepolia — not available by default). Deploy Uniswap V2 from official build artifacts in `smart-contract/script/artifacts`, not by recompiling V2 core/periphery through Foundry. The official router hardcodes the pair init code hash; recompiling pair bytecode can make `pairFor()` point to the wrong address.
- **Upgrade path**: UUPS → Uniswap V4 with `beforeSwap` KYC hooks post-MVP.

### Current Arbitrum Sepolia Deployment

| Contract | Address | Verification |
|---|---:|---|
| `PulsarProtocol` proxy | `0x204488318C0E75978B3c851382Aa83f3065a8f5A` | Verified (`ERC1967Proxy`) |
| `PulsarProtocol` implementation | `0xB3185EB1d15D107915e6ecC019c519525545287A` | Verified |
| `IDRX` mock | `0x03b53A71C5517907006EAb512A31C1eD5a56Ae64` | Verified |
| `UniswapV2Factory` | `0x4254378E95dBD9816a1a18428A81B4E1fBe5C296` | Verified |
| `UniswapV2Router02` | `0xFEf655B2A0742134242711b80899d0b543A74223` | Verified |
| WETH | `0x980B62Da83eFf3D4576C647993b0c1D7faf17c73` | External dependency |

The matching `.env` keys are `PULSAR_PROTOCOL_PROXY`, `PULSAR_PROTOCOL_IMPL`, `IDRX`, `UNISWAP_V2_FACTORY`, `UNISWAP_V2_ROUTER`, and `WETH`. Keep these in sync with `frontend/.env.local` (`NEXT_PUBLIC_PULSAR_PROTOCOL_ADDRESS`, `NEXT_PUBLIC_IDRX_ADDRESS`).
`PulsarStock` tokens are deployed lazily on first successful `executeMint`; verify each stock token address after deployment.

### Uniswap V2 Deployment Rule

Deploy Uniswap V2 through `script/OfficialUniswapV2.s.sol`, which reads official build artifacts from `script/artifacts`:

```bash
DEPLOY_UNISWAP_V2=true forge script script/Upgrade.s.sol:UpgradeScript --rpc-url "$RPC_URL" --broadcast
```

Do not deploy V2 by recompiling `lib/v2-core` and `lib/v2-periphery` with Foundry. The official `UniswapV2Router02` uses `UniswapV2Library.pairFor()`, and that library hardcodes the pair init code hash. If factory pair bytecode does not match that hash, `factory.createPair()` can succeed while router `addLiquidity()` calls the wrong deterministic pair address. The symptom is `executeMint` reverting during liquidity provisioning with requester, threshold, and balances already valid.

Post-deploy checks:

```bash
cast call $PULSAR_PROTOCOL_PROXY "router()(address)" --rpc-url "$RPC_URL"
cast call $UNISWAP_V2_ROUTER "factory()(address)" --rpc-url "$RPC_URL"
cast call $UNISWAP_V2_FACTORY "feeToSetter()(address)" --rpc-url "$RPC_URL"
```

The router's factory must equal `UNISWAP_V2_FACTORY`, the protocol router must equal `UNISWAP_V2_ROUTER`, and both contracts must be verified.

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
- **Swap recording**: Frontend hits SC via wagmi → after tx confirmed (`useWaitForTransactionReceipt`) → frontend hits `POST /api/v1/public/stock-transactions`. No indexer, no WebSocket, no polling. Idempotent via `tx_hash` UNIQUE constraint.

### Frontend (`/frontend`)
- Next.js + TailwindCSS + RainbowKit (wallet connection + SIWE)
- `/swap` — permissionless swap view
- `/custodian` — multisig mint dashboard, KYC management, pending proposals
- Sonner for all toast/notification UI

---

## 4. Custodian Flows (3/5 Multisig)

### Mint
```
Custodian A  →  requestMint(ticker, stockName, idxTicker, tokenAmount, idrxAmount, attestationHash, destination)
Custodian B/C → approveMint(proposalId)  OR  rejectMint(proposalId)
Custodian A  →  executeMint(proposalId)      ← only requester, after approvalCount >= 3
                 └─ _ensureStock → deploy PulsarStock if first mint
                 └─ _mint → PulsarStock.mint(to, amount)
                 └─ if LiquidityPool → fund IDRX by allowance/transferFrom → _provideToPool → addLiquidity
First rejecter → executeRejectMint(proposalId) ← after rejectCount >= 3
```

### Redeem
```
Custodian    →  requestRedeem(ticker, tokenAmount, userAddress)
                 └─ checks kycApproved[userAddress]
                 └─ locks user tokens + IDRX fee in contract
Custodian    →  approveRedeem(requestId)  OR  rejectRedeem(requestId)
First approver → executeRedeem(requestId)  ← after approvalCount >= 3, burns tokens + fee to treasury
First rejecter → executeReject(requestId)  ← after rejectCount >= 3, returns tokens + fee to user
```

### KYC (prerequisite for redeem)
```
User contacts operator off-chain (phone/email)
Custodian → inputs wallet address in dashboard → approveKYC(userAddress) on-chain
```

---

## 5. Database Tables

| Table | Purpose |
|---|---|
| `custodians` | 5 multisig operator participants |
| `stocks` | Listed tokens; `contract_address` null until first executeMint |
| `mint_proposals` | Mirror of on-chain mint proposals |
| `mint_attestations` | Unified approve+reject votes per custodian per proposal (`type`: approve/reject) |
| `redeem_proposals` | Mirror of on-chain redeem requests; `user_address` = beneficiary |
| `redeem_attestations` | Unified approve+reject votes per custodian per redeem |
| `wallet_verifications` | KYC records managed by operator; `type`: retail/institution |
| `stock_transactions` | Swap events: side buy/sell, idrx_amount, stock_amount (NUMERIC 78,0) |
| `stock_attestations` | Proof of reserves per stock (operator-level, not per-custodian) |

---

## 6. Key Constraints

- IDRX decimals = 2 (matches real IDRX on Base/other chains)
- PulsarStock decimals = 18
- Amounts stored in DB as `NUMERIC(78,0)` — raw on-chain uint256
- `stocks(ticker)` is UNIQUE, not PK — all FK relations use `stocks(id)`
- Nonce store is in-memory (sufficient for hackathon MVP)
- IDRX not deployed on Arbitrum Sepolia by default — use `IDRX.sol` mock
- Liquidity pool proposals require IDRX funding before execution. New LP requests fund during `requestMint` after an IDRX approval. Existing LP proposals can be topped up with `fundMintLiquidity(proposalId, amount)` before `executeMint`.
- Do not edit files under `smart-contract/lib/**`. Treat Uniswap dependencies as read-only; fixes belong in project scripts/contracts or dependency pinning.

---

## 7. Router Versioning

- **MVP**: Uniswap V2, custom-deployed on Arbitrum Sepolia
- **Production**: Uniswap V4 with `beforeSwap` KYC hooks via Horizon Labs Identity Registry

# PulsarFi Smart Contracts

Solidity contracts for the PulsarFi RWA tokenization protocol, built with Foundry.

---

## Contracts

### `PulsarProtocol.sol`
UUPS upgradeable proxy. Single entry point for all protocol operations.

- **Roles**: `DEFAULT_ADMIN_ROLE` (admin), `CUSTODIAN_ROLE` (5 custodians)
- **Multisig mint** (threshold 3/5):
  - `requestMint(...)` — custodian submits proposal, auto-approves as first signer
  - `approveMint(proposalId)` — other custodians approve
  - `executeMint(proposalId)` — requester executes after threshold met
- **`swap(ticker, amountIn, amountOutMin, buyStock)`** — permissionless IDRX ↔ PulsarStock
- **`redeem(ticker, user, amount, attestationHash)`** — custodian burns tokens on physical settlement
- **`approveKYC` / `revokeKYC`** — admin gates redemption

### `PulsarStock.sol`
ERC20 token per IDX equity. Owned by `PulsarProtocol`. Deployed lazily on first `executeMint`.
- 18 decimals
- `mint` / `burn` restricted to owner (PulsarProtocol)

### `IDRX.sol`
Mock IDR stablecoin for Arbitrum Sepolia testnet (real IDRX not deployed on Arbitrum).
- 2 decimals (matches real IDRX on Base/other chains)
- Ownership transferred to PulsarProtocol at deploy so `_provideToPool` can mint

---

## Setup

```bash
# Install dependencies
forge install

# Build
forge build

# Test
forge test -vvv

# Format
forge fmt
```

## Deploy

```bash
cp .env.example .env
# Fill: PRIVATE_KEY, UNISWAP_V2_ROUTER, CUSTODIAN_1..5

forge script script/Deploy.s.sol \
  --rpc-url $ARB_SEPOLIA_RPC \
  --broadcast \
  --verify
```

Deploy script:
1. Deploys `IDRX` mock with deployer as owner
2. Deploys `PulsarProtocol` implementation + ERC1967 proxy
3. Transfers `IDRX` ownership to the proxy

`PulsarStock` contracts are deployed lazily — first `executeMint` for a given ticker triggers `_ensureStock`.

## Environment Variables

| Variable | Description |
|---|---|
| `PRIVATE_KEY` | Deployer private key |
| `UNISWAP_V2_ROUTER` | V2 Router address (custom deploy on Arbitrum Sepolia) |
| `CUSTODIAN_1` .. `CUSTODIAN_5` | Custodian wallet addresses |

## Notes

- Uniswap V2 is not available by default on Arbitrum Sepolia — deploy Factory + Router manually
- UUPS proxy enables future migration to Uniswap V4 hooks without redeploying stocks

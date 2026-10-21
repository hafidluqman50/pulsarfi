# PulsarFi Smart Contracts

Solidity contracts for the PulsarFi RWA tokenization protocol, built with Foundry on Arbitrum Sepolia.

---

## Contracts

### `PulsarProtocol.sol`
UUPS upgradeable proxy and the single entry point for protocol operations. Functions that would push the contract past the EIP-170 size limit are `delegatecall`-ed to `PulsarProtocolOps`.

- **Roles**: `DEFAULT_ADMIN_ROLE` (admin), `CUSTODIAN_ROLE` (5 custodians)
- **Mint (threshold 3/5)**:
  - `requestMint(ticker, stockName, idxTicker, tokenAmount, idrxAmount, attestationHash)`
  - `approveMint(proposalId)`
  - `executeMint(proposalId, sqrtPriceX96)` — deploy that ticker's `PulsarStock` lazily, mint the backing, pull IDRX, and seed the Uniswap V4 pool
- **Redeem (threshold 3/5)**:
  - `requestRedeem(ticker, tokenAmount)`
  - `approveRedeem(requestId)`
  - `executeRedeem(requestId)` — KYC is checked here, at physical settlement
- **Swap**: `swapV4(ticker, amountIn, minOut, buyStock)` — permissionless spot swap against the ticker's V4 pool
- **Fees/admin**: `setTreasury`, `setSwapFeeBps`, `setRedeemFeeBps`, `distributeFees`, `setOpsContract`
- **KYC**: `approveKYC(wallet)` / `revokeKYC(wallet)` (custodian-gated, redemption only)
- **Pause**: protocol and hook can be paused independently

### `PulsarProtocolOps.sol`
`delegatecall` target for the less-hot-path functions moved out to comply with EIP-170. Not a proxy; referenced by address via `PulsarProtocol.setOpsContract()`.

Covers mint/redeem approve-reject-execute, `distributeFees`, KYC, admin setters, and the V4 migration/emergency paths.

### `v4/PulsarSwapHook.sol`
Uniswap V4 hook that enforces the protocol fee (`swapFeeBps`) in IDRX at the pool level. One hook instance serves every ticker's pool; the address is read from `PulsarProtocol.poolKeys(ticker).hooks`.

### `PulsarStock.sol`
ERC-20 per IDX equity, owned by `PulsarProtocol`, deployed lazily on the first `executeMint` for that ticker.
- 18 decimals
- `mint` / `burn` restricted to the protocol
- Ticker naming: ALL-CAPS with a `P` suffix (`BUMIP`, `ENRGP`, `BRPTP`, …)

### `AgentTaskManager.sol`
On-chain manager for self-custodial agent trades. The backend signs with the `AGENT_ROLE` wallet, but the guardrails are enforced here, not in the backend.

- `createTask(owner, isActionable, summary, promptHash)`
- `grantTradePermission(taskId, totalBudget, duration, maxAmountPerTrade, cooldownInterval)`
- `recordSubTasks(taskId, records)` — hash-chained reasoning steps
- `executeTrade(...)` — only within the granted permission
- `cancelTask(taskId)`, `isActive(taskId)`
- Decimals are decoupled: IDRX amounts in 2-decimal base units, stock amounts in 18-decimal base units

### `mocks/IDRX.sol` and `mocks/IDRXFaucet.sol`
Testnet IDR stablecoin (2 decimals, matching real IDRX) and a faucet for local/demo use. On mainnet, configure the protocol with the real IDRX address.

> The `interfaces/IUniswapV2*`, `helpers/UniswapV2*Compile.sol` and `script/OfficialUniswapV2.s.sol` files remain for the historical deploy path. The live venue is Uniswap V4; `swapV4` never routes through the V2 router.

---

## Setup

```bash
forge install
forge build
forge test -vvv
forge fmt
```

## Deploy

```bash
cp .env.example .env
# Fill: PRIVATE_KEY, TREASURY, WETH, CUSTODIAN_1..5
forge script script/Deploy.s.sol --rpc-url $RPC_URL --broadcast --verify
```

`Deploy.s.sol` deploys the official V2 factory/router bytecode (kept as the legacy router argument), the IDRX mock, the `PulsarProtocol` implementation and its ERC1967 proxy, and initializes the proxy with the custodians.

Then, in order:

```bash
# Configure the live venue: PoolManager + PulsarSwapHook.
forge script script/V4Cutover.s.sol --rpc-url $RPC_URL --broadcast

# AgentTaskManager + AGENT_ROLE for the backend wallet.
forge script script/DeployAgentTaskManager.s.sol --rpc-url $RPC_URL --broadcast

# Optional: IDRX faucet.
forge script script/DeployFaucet.s.sol --rpc-url $RPC_URL --broadcast
```

`PulsarStock` contracts are deployed lazily — the first `executeMint` for a ticker triggers `_ensureStock` and seeds that ticker's V4 pool.

## Environment Variables

| Variable | Description |
|---|---|
| `PRIVATE_KEY` | Deployer key (also used by the V4 cutover and agent deploy scripts) |
| `RPC_URL` | Arbitrum Sepolia RPC endpoint |
| `ARBISCAN_API_KEY` | Contract verification |
| `TREASURY` | Protocol fee recipient |
| `WETH` | Wrapped ETH on Arbitrum Sepolia |
| `CUSTODIAN_1` .. `CUSTODIAN_5` | Custodian wallet addresses (3/5 threshold) |
| `CUSTODIAN_1_PRIVATE_KEY` | Custodian key used by `V4Cutover.s.sol` |
| `AGENT_WALLET` | Backend wallet granted `AGENT_ROLE` |
| `PULSAR_PROTOCOL_PROXY` | Deployed proxy address |
| `PULSAR_PROTOCOL_OPS` | Delegatecall target |
| `PULSAR_SWAP_HOOK` | V4 hook address |
| `IDRX` / `IDRX_FAUCET` | Testnet token and faucet |
| `AGENT_TASK_MANAGER` | Agent manager address (redeployed, not upgraded, when its ABI changes) |

## Notes

- Every token is backed 1:1 by shares held in custody at KSEI; price discovery comes from the AMM, not an oracle.
- Holding and swapping are permissionless; KYC applies only at redemption.
- The hook fee is charged in IDRX at the pool level, so `swapV4` returns the post-fee amount.
- UUPS keeps the proxy address stable across implementation upgrades; only `PulsarStock` deployments are per-ticker.

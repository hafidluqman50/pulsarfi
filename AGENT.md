PulsarFi - Agent Strategy & Context Guide
1. Project Core Philosophy & Business Logic
PulsarFi is an Asset-Backed Tokenization Platform (RWA) on Arbitrum Sepolia that brings traditional asset exposure (specifically Indonesian IDX equities like BUMIP and ENRGP) to a global DeFi audience using IDRX as the core fiat-stable matching currency.

Fundamental Rules:

• Asset-Backed, NOT Synthetic: Every token minted (`BUMIP`, `ENRGP`) represents actual physical underlying shares legally held and locked 1:1 in custody. Price discovery is driven by direct market mechanics and arbitrage, not oracles.

• The USDC Compliance Model: Anyone globally can hold and trade tokens in the AMM DEX pools completely permissionless (without upfront KYC) to capture price exposure and economics sharing. However, strict Anti-Money Laundering (AML) and Identity Matching (KYC) are enforced at the Redemption Gateway when converting digital tokens back into physical securities via traditional brokers (e.g., Stockbit/Ajaib).

---

2. Token Naming Convention
To mirror institutional asset listings and avoid confusing RWA entity tokens with yield-bearing instruments (like stETH), all asset tokens must use All-Caps with a 'P' suffix indicating Pulsar.

• Valid Ticketing Examples: `BUMIP`, `ENRGP`

• Forbidden Patterns: `pBUMI`, `pENRG`, `PBUMI`, `PENRG`

---

3. Core Functional Operations & Backend Triggers (Golang to Web3)
All operations are unified via a REST API backend written in Golang, interfacing with PostgreSQL for state queuing and Foundry/Ethers RPC for smart contracts.

A. The B2B Custodian Order (Mint to Client Wallet)

Used when an external institution or authorized entity completes off-chain custody settlement and requests initial primary token issuance.

• Endpoint: `POST /api/custodian/mint`

• Payload:

```

{

  "ticker": "ENRGP",

  "amount": "1000000",

  "target_wallet": "0xClientWalletAddress..."

}

```

• Flow: Enters PostgreSQL as `PENDING` $\rightarrow$ Validated on the admin portal (`/custodian`) $\rightarrow$ Executed by Golang backend using Custodian private key calling the `mint(to, amount)` function on the token contract.

B. The Internal Market Maker Initialize (Open AMM Pool)

Used when PulsarFi initializes direct platform-driven market liquidity on the Uniswap V2 Router.

• Endpoint: `POST /api/amm/initialize`

• Payload:

```

{

  "ticker": "BUMIP",

  "initial_token_supply": "10000",

  "initial_idrx_liquidity": "100000000"

}

```

• Flow Sequential:

  1. Backend calls contract `mint()` to send tokens to the internal automated arbitrage/bot wallet.

  2. Sends contract `approve()` granting maximum allowance to the Uniswap V2 Router address.

  3. Triggers `addLiquidity()` on the Uniswap Router with the defined token and `IDRX` amount.

  4. Automatically boots up the background Goroutine tracking live on-chain prices against live reference data for programmatic rebalancing.

---

4. UI Layout Routing Guidelines
Maintain consistent screen architectures when building Frontend blocks in Next.js:

• `/swap` : Permissionless consumer swap view handling standard Router operations.

• `/custodian` : Administrative operation control deck highlighting pending database queues, state changes, and primary token minting controls. Use Sonner library exclusively for all system notifications and confirmations.

• Code validation error formats must remain strictly in English for international validator/hackathon standards.

---

5. Router Versioning & Evolution Strategy (MVP vs Production)
• MVP Implementation (Current): Strictly use Uniswap V2 Router on Arbitrum Sepolia. It is lightweight, mathematically predictable ($x \cdot y = k$), and avoids tick/singleton complexities, allowing rapid 20-day delivery of core Golang/Kustodian infrastructure.

• Production Roadmap: Explicitly target Uniswap V4 with `beforeSwap` KYC Hooks. This will dynamically gate liquidity pools using Horizon Labs' Identity Registry, ensuring automated protocol-level compliance. However, this requires significant architectural changes and will be a major milestone post-MVP launch.
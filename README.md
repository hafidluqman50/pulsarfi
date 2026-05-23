PulsarFi - Asset-Backed Indonesian Equity (IDX) Tokenization Infrastructure
PulsarFi is an institutional-grade Real-World Asset (RWA) tokenization protocol built for the Arbitrum Buildathon. The platform unlocks global liquidity for the Indonesian Stock Exchange (IDX) by allowing public companies and institutions to tokenize traditional equities (e.g., `BUMIP`, `ENRGP`) into 1:1 asset-backed cryptographic receipts paired natively against `IDRX` liquidity pools.



---



🏛️ The B2B Tokenization Architecture & Narration
PulsarFi operates on a B2B Tokenization-as-a-Service (TaaS) framework, ensuring complete regulatory alignment and mitigating asset-owner liability:



1. Asset Custody & Locking: Pitted directly into the Indonesian traditional finance infrastructure, issuers (emiten) legally transfer and lock physical stock certificates into the Horizon Labs Custodian Account at KSEI (Kustodian Sentral Efek Indonesia).



2. Fiat Settlement Leg: Large-scale primary issuance and off-chain market making settlements are processed securely via the dedicated Bank Mandiri IDR Settlement Vault.



3. Primary Issuance: Once assets are audited in custody, the platform mints 1:1 backed tokens (`BUMIP`, `ENRGP`) directly to the issuer's or institutional market maker's secure Web3 wallet.



---



⚖️ The Guardrail Framework (USDC Compliance Model)
To bridge strict Indonesian regulatory bodies (OJK/BI) with permissionless DeFi rails, PulsarFi utilizes a Chokepoint Enforcement Architecture:



• Trading Layer (Permissionless Exposure): Modeled exactly after Circle's USDC, anyone globally can hold, transfer, and swap tokens on Arbitrum DEXs without upfront platform-level KYC. This grants international investors frictionless price exposure and economic utility sharing to high-performing Indonesian equities.



• Redemption Gateway (Strict Guardrails): The loop closes at the `/custodian` portal. To realize physical delivery (mutating digital tokens back into underlying stocks in TradFi brokerages like Stockbit or Ajaib), users must pass rigorous AML/KYC checks, providing valid SID (Single Investor Identification) numbers. The Golang backend validates identity parity, programmatically rejecting mismatched accounts to prevent fraud or illicit asset leakage.



---



🛠️ Tech Stack & Core Mechanisms
• Frontend: Next.js (TailwindCSS, Radix UI, managed with Sonner for robust transaction status reporting). Code validation and error messaging are written strictly in English for global validator standards.



• Backend Engine: Golang REST API acting as the central processing unit interfacing with a PostgreSQL state ledger.



• Smart Contracts: Solidity contracts engineered via Foundry, deployed on Arbitrum Sepolia.



• Automated Market Stabilization: Rather than deploying external third-party market makers, PulsarFi runs internal Golang Arbitrage Bot Workers that programmatically monitor order-book disparities between the IDX live feed and Uniswap pools, maintaining a strict peg through automatic rebalancing.



---



🚀 Execution Quick Start
1. Smart Contract Deployment (Foundry)



```



cd foundry



forge script script/DeployPulsarFi.s.sol --rpc-url $ARBITRUM_SEPOLIA_RPC --broadcast --verify



```



2. Backend Rest Engine (Golang)



```



cd backend



go mod tidy



go run cmd/migrate/main.go



go run cmd/api/main.go



```



3. Frontend App (Next.js)



```



cd frontend



npm install



npm run dev



```



---



📋 Protocol Core Endpoints
• `POST /api/custodian/mint` : Handles B2B corporate primary issuance into target institution vaults upon off-chain KSEI verification.



• `POST /api/amm/initialize` : Triggers platform-driven seed liquidity allocation (`Token + IDRX`) onto the Automated Market Maker router.



---



🚀 Technical Roadmap: Uniswap V2 to V4 Hooks
• Phase 1 (Buildathon MVP): Powered by a streamlined Uniswap V2 implementation to prove backend orchestration, T+0 settlement speeds, and zero-oracle internal arbitrage bot stability.



• Phase 2 (Production Launch): Graduating to Uniswap V4. Harnessing a customized `beforeSwap` compliance hook hooked directly to the Horizon Labs Identity Registry to programmatically block unverified or blacklisted wallets at the pool level.
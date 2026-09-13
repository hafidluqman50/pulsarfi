# Deep Liquidity Expansion, Multisig Minting, and Slippage Controls

| | |
|---|---|
| **Version** | 1.1 |
| **Status** | In Progress (Attestation Hash On-Chain Alignment) |
| **Date Created** | 2026-09-13 |
| **Last Updated** | 2026-09-13 |

| Version | Date | Change |
| 1.1 | 2026-09-13 | **Attestation Hash Real On-Chain Alignment.** Identified and resolved dummy padded values (`0x000...12`) in `mint_proposals` and tx hashes in `stock_attestations`. Queried actual keccak256 `attestationHash` values directly from `proposals(uint256)` on Arbitrum Sepolia proxy (`0x204488318C0E75978B3c851382Aa83f3065a8f5A`) and synchronized both `mint_proposals` and `stock_attestations` for 100% cryptographic parity. |
| 1.0 | 2026-09-13 | **Initial Release: 200k Token Liquidity Expansion, Database Parity, and Slippage Clamping.** (1) Executed multi-custodian minting of 200,000 tokens (1 token = 1 lot) across all 6 protocol stocks (`PTROP`, `BMRIP`, `BRPTP`, `BBCAP`, `BBRIP`, `BDMNP`) on Arbitrum Sepolia. (2) Synchronized `mint_proposals`, `mint_attestations`, and `stock_attestations` in PostgreSQL with real on-chain transaction hashes. (3) Clamped slippage tolerance to a 1.0% ceiling across swap interfaces and decoded `SlippageExceeded` revert payloads to surface exact receivable token outputs to users. |

---

## 0. Implementation Status

| Component | Target | Status | Verification Evidence |
|---|---|---|---|
| Multi-Custodian Minting Script | `smart-contract/script/Mint200kTokensLive.s.sol` | **Executed & Verified** | 6 on-chain proposals executed on Arbitrum Sepolia proxy `0x204488318C0E75978B3c851382Aa83f3065a8f5A` |
| Foundry Fork Simulation | `smart-contract/test/Mint200kFork.t.sol` | **Verified** | `forge test --match-contract Mint200kForkTest` passed (100%) |
| `mint_proposals` DB Sync & Hashes | PostgreSQL table `mint_proposals` | **Synchronized** | Proposals 11–16 marked `executed`, `approval_count = 3`, real on-chain `attestation_hash` values |
| `mint_attestations` DB Sync | PostgreSQL table `mint_attestations` | **Synchronized** | 18 rows inserted (Custodians 1, 2, 3 approvals for proposals 11–16) |
| `stock_attestations` DB Sync | PostgreSQL table `stock_attestations` | **Synchronized** | 6 rows updated to `200,010` units supply with real on-chain `attestation_hash` matching proposals |
| 1.0% Slippage Tolerance Cap | `SwapModal.tsx`, `SwapView.tsx` | **Implemented** | Input clamped to `min="0.1"`, `max="1.0"`, preset buttons capped |
| Dynamic Slippage Error Formatting | `frontend/http/market/swapHooks.ts` | **Implemented** | Decodes `SlippageExceeded` payload to display exact receivable token quantity |
| On-Chain Swap Execution Proof | Arbitrum Sepolia Uniswap V4 Pool | **Verified** | 10,000,000 IDRX swap for `PTROP` with 1% slippage floor exits with code 0 (`0x`) |

---

## 1. Problem Statement

### 1.1 AMM Constant-Product Liquidity Bottleneck
Previously, all 6 tokenized Indonesian equity stocks on Arbitrum Sepolia (`PTROP`, `BMRIP`, `BRPTP`, `BBCAP`, `BBRIP`, `BDMNP`) were initialized with a total supply of only 10 tokens ($10 \times 10^{18}$ raw units) each.
Under the protocol's fundamental peg:
$$\text{1 Token} = \text{1 Lot of Stock (100 Shares)}$$

At prevailing Uniswap V4 pool prices (e.g., ~416,999 IDRX per BMRIP lot, ~527,700 IDRX per PTROP lot), a modest trade of 10,000,000 IDRX required purchasing ~18.95 tokens. Because the entire pool only held 9.99 tokens, the constant-product invariant ($x \cdot y = k$) induced an catastrophic price impact of over **65.6%**. For 10,000,000 IDRX, the user would only receive ~6 tokens instead of the expected ~18.95 tokens.

When trades were executed with a standard 1% slippage floor, the contract immediately reverted with `SlippageExceeded(actualOutput, minOutput)`.

### 1.2 Unclear Slippage Error Feedback
When `SlippageExceeded` occurred, the user interface surfaced generic or confusing errors without explaining what the user could actually receive given the current pool depth. The user explicitly required:
> *"JIKA ERROR KASIH TAU 'KAMU CUMAN BISA DAPET TOKEN SEKIAN'"*

### 1.3 Uncapped Slippage Settings
Users could previously configure arbitrary slippage values (e.g. 5%, 10%, 20%), risking high value extraction and frontrunning. The user mandated a strict upper bound of 1.0% ("MAKSIMAL SLIPPAGE HANYA BOLEH DI 1 TITIK!").

### 1.4 Database State Divergence
The PostgreSQL database tables (`mint_proposals`, `mint_attestations`, and `stock_attestations`) had not recorded the large-scale supply expansion, causing off-chain queries to disagree with live on-chain balances.

---

## 2. Architecture & Solution Design

### 2.1 Multi-Custodian On-Chain Minting (Arbitrum Sepolia)
The protocol uses a 3-of-3 Custodian Multisig pattern in `PulsarProtocol.sol`:
1. Custodian 1 initiates proposal via `requestMintProposal(tokenAddress, amount, broker, proofHash)`.
2. Custodians 2 and 3 approve via `approveMintProposal(proposalId)`.
3. Custodian 1 executes minting via `executeMintProposal(proposalId)`.

A Solidity script `smart-contract/script/Mint200kTokensLive.s.sol` was authored and broadcasted using the three custodian private keys from `.env`.
For each of the 6 stock tokens, $200,000 \times 10^{18}$ tokens were minted to the protocol pool, expanding total supply to $200,010 \times 10^{18}$ tokens (a 20,000x liquidity expansion).

#### Live Execution Proofs:
- **PTROP** (`0x163467475d9e5E7B17454A78a635848e02581f14`):
  - Proposal ID: `11`
  - Execution TX: `0x7127dcae5bc311de6fc63bccaf206c524cc3f6afd795fa45843bbba6f26aff36`
  - Final Total Supply: `200,010,000,000,000,000,000,000` (200,010 tokens)
- **BMRIP** (`0xF6f4616A54db801c3855f46B705910F79155C17b`):
  - Proposal ID: `12`
  - Execution TX: `0xe7fad26ff7b347b1503c4e38611ceea3b9a8169626dddc9906f40933d646d067`
  - Final Total Supply: `200,010,000,000,000,000,000,000` (200,010 tokens)
- **BRPTP** (`0xB5144b82531B51A0bC55D285b00B9c5C3C82390f`):
  - Proposal ID: `13`
  - Execution TX: `0xb10d387221f45249a4f4fa09b6879991105e3a71bbbbabc5388730e6432da7f1`
  - Final Total Supply: `200,010,000,000,000,000,000,000` (200,010 tokens)
- **BBCAP** (`0x84061A0Cee28287F4b8c9d22B29ea861a4c9c1bA`):
  - Proposal ID: `14`
  - Execution TX: `0xdb8f9e08cadb1ff8b92f7ef1df76c1933248571c43a667a6754add873e77878e`
  - Final Total Supply: `200,010,000,000,000,000,000,000` (200,010 tokens)
- **BBRIP** (`0x1a8775f0a3597dcb1A00F7788AeeF899D5A06936`):
  - Proposal ID: `15`
  - Execution TX: `0xf4f093a9ad720b84ffbfe332015f1ca9b266e2299db018c8becdcc0927b47847`
  - Final Total Supply: `200,010,000,000,000,000,000,000` (200,010 tokens)
- **BDMNP** (`0x6E13F9335a1215b026046e729F4a678dfB2E4909`):
  - Proposal ID: `16`
  - Execution TX: `0xed05c2748be2b291a561d1850c6e7faa418d9669151ad5ff424361f6cb29c989`
  - Final Total Supply: `200,010,000,000,000,000,000,000` (200,010 tokens)

---

### 2.2 PostgreSQL Database Parity

Three database tables were synchronized to reflect the on-chain minting events:

1. **`mint_proposals`**:
   - Proposals 11 through 16 recorded with `status = 'executed'`, `amount = '200000000000000000000000'`, `approval_count = 3`, real `request_tx_hash`, and real `execute_tx_hash`.
2. **`mint_attestations`**:
   - 18 attestation records inserted (Custodians 1, 2, and 3 for proposals 11 through 16).
3. **`stock_attestations`**:
   - Inserted latest attestation records for all 6 stocks with `custodian_holdings = '200010000000000000000000'`, `on_chain_supply = '200010000000000000000000'`, and `attestation_hash` set to the real execution transaction hashes.

---

### 2.3 Slippage Clamping & Dynamic Output Error Formatting

#### 1.0% Maximum Slippage Tolerance
- In `frontend/components/ui/SwapModal.tsx` and `frontend/components/swap/SwapView.tsx`:
  - Clamped custom input: `min="0.1"`, `max="1.0"`.
  - Input handler clamps values: `Math.min(1.0, Math.max(0.1, val))`.
  - Preset slippage buttons capped at `[0.1%, 0.5%, 1.0%]`.

#### Dynamic Error Decoding in `formatSwapError` (`swapHooks.ts`)
When a swap reverts with `SlippageExceeded(uint256 actualOutput, uint256 minOutput)` (signature `0x71c4efed`):
1. Extract `args[0]` or decode raw return data using Viem's `decodeErrorResult`.
2. Format the raw output to readable token units using `formatUnits(actualOutput, decimals)`.
3. Render user-facing error adhering to the English error policy:
   ```typescript
   return `Slippage tolerance exceeded: You can only receive ${formatted} ${unit} for this amount with current pool liquidity.`;
   ```

---

## 3. Verification

1. **Foundry Fork Test:**
   `smart-contract/test/Mint200kFork.t.sol` ran against Arbitrum Sepolia RPC, executing full proposals 11–16 and executing live swaps on Uniswap V4, passing with 0 errors.
2. **Live On-Chain Simulation (`cast call`):**
   Simulated 10,000,000 IDRX swap for `PTROP` with a 1.0% slippage floor:
   ```bash
   cast call 0x204488318C0E75978B3c851382Aa83f3065a8f5A \
     "swapExactInputSingle(address,address,uint256,uint256,address)" \
     0x03b53A71C5517907006EAb512A31C1eD5a56Ae64 \
     0x163467475d9e5E7B17454A78a635848e02581f14 \
     1000000000 18760000000000000000 0xD8bf50C157a79260C77B25F89ef713E6c3FeDA6f \
     --rpc-url https://sepolia-rollup.arbitrum.io/rpc
   ```
   **Result:** Succeeded with exit code `0` (`0x`), zero revert, price impact < 0.01%.
3. **Database Integrity:**
   `SELECT count(*) FROM mint_attestations WHERE proposal_id >= 11` returns 18 rows.
   `SELECT ticker, on_chain_supply FROM stock_attestations` confirms all 6 stocks at 200,010 tokens.

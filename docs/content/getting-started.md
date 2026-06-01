---
id: getting-started
title: Getting Started
sidebar_label: Get Started
slug: /getting-started
---

This guide targets the Arbitrum Sepolia deployment. It is split by user type
because traders, redeemers, custodians, and local developers need different
starting points.

## Network requirements

| Item | Value |
| --- | --- |
| Network | Arbitrum Sepolia |
| Chain ID | `421614` |
| Hex chain ID | `0x66eee` |
| Gas token | Sepolia ETH bridged to Arbitrum Sepolia |
| Stable token | Mock IDRX |
| Explorer | `https://sepolia.arbiscan.io` |

You need a browser wallet such as MetaMask or Rabby.

## Reviewer quick path

Use this path when you only want to try the product quickly.

1. Open the PulsarFi frontend.
2. Connect a wallet on Arbitrum Sepolia.
3. Sign the SIWE message.
4. Open the contracts and faucet page in these docs.
5. Click **Mint 100,000 IDRX**.
6. Return to the app and open swap.
7. Swap IDRX into a deployed pStock.
8. Open portfolio and confirm the position appears.

If the selected pStock has no pool yet, the swap UI should show that the pool is
unavailable. Use a token that has already been minted and listed in the market
view.

## Trader flow

1. Connect wallet.
2. Sign in with SIWE.
3. Check IDRX balance.
4. Pick an input and output token.
5. Enter amount.
6. Review expected output, slippage, and route.
7. Approve input token if prompted.
8. Confirm swap.
9. Wait for transaction receipt.
10. Review the resulting balance in portfolio.

## Redemption flow

Redemption requires KYC. Without KYC, the on-chain transaction will revert with
`KYCRequired`.

1. Contact the operator or custodian for verification.
2. Custodian approves your wallet on-chain.
3. Open portfolio.
4. Pick the pStock position.
5. Click redeem.
6. Enter amount.
7. Approve pStock if prompted.
8. Approve IDRX fee if redeem fee is active.
9. Submit redeem request.
10. Wait for custodian multisig approval and execution.

## Custodian flow

Custodians must use a wallet that exists in the backend `custodians` table and
has `CUSTODIAN_ROLE` on-chain.

### Mint

1. Connect custodian wallet.
2. Sign in with SIWE.
3. Open custodian console.
4. Fill ticker, stock name, IDX ticker, token amount, and IDRX amount.
5. Submit mint proposal.
6. Wait for at least two additional custodian approvals.
7. Ensure requester has enough IDRX balance.
8. Ensure requester has approved IDRX to `PulsarProtocol`.
9. Execute mint.
10. Confirm stock contract address and reserve snapshot are recorded.

### Redeem

1. Open the request queue.
2. Review redeem request details.
3. Approve or reject.
4. Wait for threshold.
5. First approver executes approved redeem, or first rejecter executes rejection.
6. Confirm backend status and reserve record.

### KYC

1. Verify user identity off-chain.
2. Open KYC registry.
3. Enter wallet address, user type, name, email, and signed statement document.
4. Submit KYC approval.
5. Confirm `approveKYC` transaction.
6. Confirm backend record status is approved.

## Local development

### Frontend

```bash
cd frontend
nvm use default
npm install
npm run dev
```

### Backend

```bash
cd backend
go run main.go
```

Required backend environment:

```text
DATABASE_URL=postgres://...
JWT_SECRET=...
LISTEN_ADDR=:8080
```

Optional backend environment for KYC document storage and email:

```text
SUPABASE_S3_ENDPOINT=...
SUPABASE_S3_ACCESS_KEY=...
SUPABASE_S3_SECRET_KEY=...
SUPABASE_KYC_BUCKET=...
RESEND_API_KEY=...
```

### Smart contracts

```bash
cd smart-contract
forge build
forge test
```

### Docs

```bash
cd docs
nvm use default
npm install
npm run start
```

## Common issues

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Wallet cannot swap | Wrong network or no token balance. | Switch to Arbitrum Sepolia and claim IDRX. |
| Swap simulation reverts | Pool missing, slippage too tight, or allowance issue. | Pick a deployed token, adjust slippage, approve token. |
| Redeem request reverts | Wallet is not KYC-approved. | Ask custodian to approve KYC. |
| Execute mint fails | Requester is not caller or IDRX allowance is missing. | Switch to requester wallet and approve IDRX. |
| Custodian console unauthorized | Wallet is not in custodian table or JWT role is user. | Use registered custodian wallet and sign in again. |

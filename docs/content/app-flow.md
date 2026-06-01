---
id: app-flow
title: App Flow
sidebar_label: App Flow
slug: /app-flow
---

The PulsarFi frontend is a wallet-first application. It does not hide blockchain
operations behind a custodial account. Users connect a wallet, sign SIWE for app
authentication, and submit transactions directly from their wallet.

## Application surfaces

| Surface | Primary user | Purpose |
| --- | --- | --- |
| Home / swap | Trader | Buy and sell pStocks against IDRX. |
| Stocks | Trader | Browse market-ready pStocks, prices, and pool state. |
| Stock detail | Trader | Inspect one token, its price history, and trading actions. |
| Portfolio | Holder | Review balances, PnL, allocation, activity, transfer, and redeem. |
| Custodian console | Custodian | Run mint, redeem, KYC, queue, and reserve operations. |

## Authentication flow

```text
wallet connect
    -> backend nonce
    -> SIWE message
    -> wallet signature
    -> backend verification
    -> JWT with role
    -> frontend role-aware UI
```

The wallet address determines whether the user is a custodian. If the address is
found in the `custodians` table, the JWT role is `custodian`; otherwise it is
`user`.

## Swap flow

The swap UI performs several steps before the final transaction:

1. Load market-ready stocks from the backend.
2. Read wallet balances from IDRX and pStock contracts.
3. Build a quote from token metadata, amount, and slippage.
4. Check input token allowance.
5. Submit approval if allowance is too low.
6. Simulate `PulsarProtocol.swap`.
7. Submit the swap transaction.
8. Wait for receipt.
9. Parse `TokensSwapped`.
10. Record the transaction in the backend.
11. Invalidate market and portfolio queries.

Failure states the UI should handle:

| Failure | User-facing meaning |
| --- | --- |
| Wallet not connected | User must connect before trading. |
| Pool unavailable | The pStock has not been deployed or has no usable pool. |
| Insufficient balance | User does not have enough input token. |
| Insufficient allowance | App requests an approval transaction. |
| Slippage exceeded | Swap simulation or execution reverts. |
| User rejected | Wallet transaction was cancelled. |

## Portfolio flow

The portfolio view combines on-chain balances and backend activity.

Inputs:

- IDRX balance;
- pStock balances;
- market stock list;
- wallet transaction history;
- current pool and IDX-derived prices.

Outputs:

- total net worth;
- cash and pStock split;
- position list;
- unrealized PnL estimate;
- allocation chart;
- recent activity;
- transfer and redeem actions.

The portfolio is a computed view. It should not be treated as the ledger of
record. The ledger of record is still the blockchain plus backend transaction
history.

## Redemption flow in the app

```text
portfolio position
    -> redeem modal
    -> approve pStock if needed
    -> approve IDRX fee if needed
    -> requestRedeem transaction
    -> parse RedeemRequested event
    -> backend record
    -> custodian queue
```

A non-KYC wallet can still hold and trade pStock, but `requestRedeem` will revert
with `KYCRequired`.

## Custodian mint flow

The custodian console turns a complex multisig process into an operational
pipeline.

1. Custodian fills the mint order form.
2. Frontend computes raw pStock and IDRX amounts.
3. Frontend builds an attestation hash.
4. Custodian submits `requestMint`.
5. Backend records the proposal.
6. Other custodians approve or reject from the request queue.
7. Requester prepares IDRX allowance.
8. Requester executes mint.
9. Frontend reads the deployed stock contract address.
10. Backend marks proposal executed and creates reserve snapshot.

## Custodian redeem flow

1. User submits redeem request from portfolio.
2. Backend displays the request in custodian queue.
3. Custodians review user address, ticker, amount, fee, and request hash.
4. Custodians approve or reject.
5. First approver can execute after threshold approval.
6. First rejecter can execute rejection after threshold rejection.
7. Backend marks final state and updates reserve snapshot if executed.

## KYC flow

The KYC UI is part of the custodian console because KYC is an operational action,
not a user self-serve toggle.

1. User submits identity information off-chain or through an operator process.
2. Custodian verifies documents.
3. Custodian calls `approveKYC` on-chain.
4. Frontend uploads signed statement document to private storage.
5. Backend stores wallet verification status and document reference.
6. The wallet can now call `requestRedeem`.

## Real-time console feedback

The backend includes an SSE stream for custodian console events. It is used to
show operational logs such as proposal records, reserve updates, and attestation
submissions. This is product feedback only; it is not the source of execution
truth.

## UX design constraints

The app is designed around these UX rules:

- wallet actions are explicit;
- approvals are separate from execution when ERC-20 allowance is required;
- transaction hash is shown after successful submission;
- backend state is refreshed after confirmed receipts;
- custodian queues prioritize status, threshold counts, initiators, and next action;
- users are not blocked from trading just because they are not KYC-approved.

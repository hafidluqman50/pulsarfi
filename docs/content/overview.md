---
id: overview
title: Overview
sidebar_label: Overview
slug: /overview
---

PulsarFi is an asset-backed tokenization protocol for Indonesian public equities.
The current deployment runs on Arbitrum Sepolia and represents selected
IDX-listed shares as ERC-20 receipt tokens called pStocks. Examples include
`BUMIP`, `ENRGP`, `BBCAP`, `BMRIP`, and `BBRIP`.

The product is built around a simple premise: a user should be able to hold and
trade exposure to Indonesian equities on-chain, while the real shares remain
accounted for by a custodian and redemption back into the off-chain securities
system remains compliance-gated.

## What PulsarFi is

PulsarFi is:

- an RWA receipt-token system for IDX equities;
- a 3-of-5 custodian-controlled mint and redemption workflow;
- an IDRX-denominated trading venue using Uniswap V2 pools;
- a wallet-first app for swaps, portfolio tracking, redemption, and custodian operations;
- a testnet deployment that demonstrates the full lifecycle from custody to on-chain liquidity.

PulsarFi is not:

- a synthetic equity protocol;
- a CFD or oracle-settled derivative;
- a centralized ledger pretending to be on-chain;
- a permissioned trading app where every swap requires KYC.

## Product thesis

Traditional equity access is operationally heavy. Crypto market structure is
fast, composable, and global. PulsarFi connects those two systems without
removing the part that matters for real-world assets: custody and redemption
must remain accountable.

The resulting product model is:

| Layer | Responsibility | User impact |
| --- | --- | --- |
| Custody | Holds the real IDX shares and signs operational attestations. | Token supply should correspond to real assets. |
| Protocol | Enforces mint, redeem, KYC, and swap rules on-chain. | Critical state transitions are transparent. |
| Liquidity | Uses IDRX-pStock pools for trading. | Users get immediate market access. |
| Backend | Mirrors confirmed on-chain actions and stores operational records. | The UI can show queues, history, KYC records, and dashboards. |
| Frontend | Coordinates wallet actions and backend recording. | Users get a guided product experience. |

## Core user groups

### Retail or testnet trader

The trader connects a wallet, claims testnet IDRX, swaps IDRX into pStocks, and
tracks positions in the portfolio page. No KYC is required for holding or
swapping. KYC becomes relevant only if the trader wants to redeem into the
off-chain securities process.

### Custodian operator

The custodian operator manages mint proposals, approvals, rejections, execution,
KYC approvals, proof-of-reserves records, and redemption requests. The console is
designed for repeated operational use, not for one-off demos.

### Protocol administrator

The administrator configures protocol-level dependencies such as router address,
IDRX address, treasury, and redeem fee. This role is intentionally narrower than
custodian operations.

## Key guarantees

PulsarFi is designed around the following guarantees:

- pStock supply is created only through the protocol.
- Each mint requires custodian authorization.
- New mint supply is paired with IDRX liquidity.
- Swaps are permissionless.
- Redemption requires KYC and custodian approval.
- Backend records follow on-chain events, not the other way around.
- Custodian dashboards derive approval counts from stored attestations.
- IDRX is the canonical settlement asset because the product is Rupiah-native.

## Current scope

The current implementation targets Arbitrum Sepolia. It uses a mock IDRX token,
a custom Uniswap V2 deployment, and a faucet for testnet liquidity. The design is
structured so production deployments can replace mock IDRX, strengthen
proof-of-reserves automation, and migrate liquidity controls to a more advanced
AMM model.

## Reading order

If you are new to the system, read these pages in order:

1. [Why PulsarFi](./problem)
2. [IDRX Settlement](./why-idrx)
3. [Market & Revenue Model](./business-flow)
4. [Architecture](./architecture)
5. [Protocol Design](./protocol-design)
6. [App & Operator Flow](./app-flow)
7. [Getting Started](./getting-started)

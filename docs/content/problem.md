---
id: problem
title: Why This Exists
sidebar_label: Problem
slug: /problem
---

Indonesian public equities are large, liquid, and familiar to local investors,
but they do not naturally fit crypto-native workflows. PulsarFi exists to make
IDX equity exposure programmable without pretending that compliance and custody
do not matter.

## Market problem

IDX equity access is still tied to traditional market structure:

- trading happens within exchange hours;
- brokerage onboarding is jurisdiction-specific;
- settlement is slower than token settlement;
- portfolio positions are trapped inside broker interfaces;
- assets are not composable with DeFi rails;
- international crypto-native users cannot easily interact with Indonesian equities.

This creates a gap between two markets:

| Traditional IDX market | Crypto-native market |
| --- | --- |
| Strong real-world asset base | 24/7 global accessibility |
| Regulated custody and settlement | Fast self-custody transfers |
| Broker-centric UX | Wallet-centric UX |
| Limited composability | Programmable asset flows |
| Compliance built into account access | Compliance usually built into gateways |

PulsarFi is designed to bridge these strengths rather than replace one side with
the other.

## Product problem

Most tokenized asset products fall into one of two weak patterns:

1. They become too permissioned, making every interaction feel like a traditional
   brokerage wrapped in a wallet.
2. They become too synthetic, giving users a token that tracks price but does not
   represent an accountable off-chain asset position.

PulsarFi avoids both extremes:

- trading and holding are permissionless;
- redemption is compliance-gated;
- minting is custodian-controlled;
- supply creation is tied to custody records;
- liquidity is created directly on-chain.

## Compliance problem

KYC is important when a user exits from an on-chain receipt into an off-chain
regulated asset. However, requiring KYC before a user can hold or swap a receipt
token makes the product harder to use and reduces liquidity.

PulsarFi uses a gateway model:

| Action | KYC required? | Reason |
| --- | --- | --- |
| Hold pStock | No | Holding an ERC-20 receipt is permissionless in the MVP. |
| Transfer pStock | No | ERC-20 transferability is preserved. |
| Swap pStock and IDRX | No | Trading happens through the pool. |
| Request redemption | Yes | The user is entering the off-chain securities delivery process. |
| Approve KYC | Custodian only | Custodians control redemption eligibility. |

This keeps the entry path open while enforcing compliance at the point where it
matters.

## Currency problem

The underlying assets are Indonesian equities, and Indonesian equities are
Rupiah-denominated. If the protocol used USDC as the default pool asset, users
would need to reason about two things at once: stock exposure and USD/IDR FX.

PulsarFi uses IDRX to keep the product locally coherent:

- IDX reference prices are in Rupiah.
- User portfolio value can be shown in Rupiah terms.
- Custodian reporting and redemption accounting are naturally IDR-based.
- The product creates real demand for Rupiah stablecoin adoption.
- Pool prices can be compared to IDX reference prices without a USD conversion step.

The choice is strategic. PulsarFi is not only bringing Indonesian equities
on-chain; it is also giving Rupiah stablecoin rails a concrete financial use
case.

## Liquidity problem

If a protocol mints receipt tokens directly to an operator wallet without
matching liquidity, the first seller can drain the pool and break the expected
relationship between the receipt and the underlying asset. For this reason, the
MVP design sends new mints into the liquidity pool with IDRX funding.

The result is a stricter minting rule:

```text
new pStock supply must be paired with IDRX liquidity
```

This is why mint proposals include both `tokenAmount` and `idrxAmount`.

## Trust model

PulsarFi does not claim to remove all trust. It makes trust explicit.

| Party | Trusted for | Not trusted for |
| --- | --- | --- |
| Custodians | Holding and attesting off-chain shares. | Unilaterally minting supply. |
| Protocol admin | Configuration and upgrades. | Bypassing custodian mint thresholds. |
| Backend | Displaying operational records. | Executing token transfers or minting. |
| User wallet | Signing transactions. | Receiving redemption without KYC. |
| AMM pool | Executing swaps. | Guaranteeing exact IDX price at all times. |

The system reduces unilateral risk by requiring custodian threshold approval for
mint and redemption execution.

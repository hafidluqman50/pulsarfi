---
id: problem
title: Why PulsarFi
sidebar_label: Why PulsarFi
slug: /problem
---

Indonesian public equities are large, liquid, and familiar to local investors,
but they do not naturally fit crypto-native workflows. PulsarFi exists to make
IDX equity exposure programmable without pretending that compliance and custody
do not matter.

## Market context

IDX equity access is still tied to traditional market structure:

- trading happens within exchange hours;
- investors cannot exit while the market is closed;
- overnight macro shocks can reprice risk before the local market opens;
- pre-opening gaps can push stocks directly into downside price limits;
- brokerage onboarding is jurisdiction-specific;
- exchange settlement and withdrawable cash follow securities settlement timing;
- portfolio positions are trapped inside broker interfaces;
- assets are not composable with DeFi rails;
- international crypto-native users cannot easily interact with Indonesian equities.

In Indonesian market language, **ARB** usually refers to **Auto Rejection
Bawah**, where sell orders below the exchange's lower price boundary are
automatically rejected. For international readers, this is closest to a
single-stock **downside price limit** or **limit-down style price band**. It is
not exactly the same as the U.S. Limit Up-Limit Down mechanism, but the economic
effect is similar: when price protection bands are hit, users may not be able to
exit at the price or time they want.

This matters most around market close and pre-open:

```text
global macro shock after IDX close
    -> user cannot sell the underlying stock overnight
    -> pre-open reprices the stock lower
    -> downside price limit / ARB can reject lower sell orders
    -> broker exit is delayed even before T+2 cash settlement
```

This creates a gap between two markets:

| Traditional IDX market | Crypto-native market |
| --- | --- |
| Strong real-world asset base | 24/7 global accessibility |
| Regulated custody and T+2-style settlement | Fast self-custody transfers |
| Broker-centric UX | Wallet-centric UX |
| Limited composability | Programmable asset flows |
| Compliance built into account access | Compliance usually built into gateways |

PulsarFi is designed to bridge these strengths rather than replace one side with
the other.

## Product thesis

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
| Hold pStock | No | Holding an ERC-20 receipt is permissionless. |
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

## Exit liquidity problem

Traditional equity settlement creates two separate exit frictions:

1. **Trading exit**: the investor needs an open market and executable price.
2. **Cash exit**: after the trade, cash availability follows the securities
   settlement cycle.

IDX-listed equities use exchange settlement timing rather than instant wallet
settlement. In practice, this means a user can be exposed to overnight risk,
pre-open repricing, downside price limits, and delayed cash availability.

## Profit is not withdrawable cash

In a brokerage account, selling a profitable stock position does not mean the
user can withdraw the cash immediately. The trade can show realized profit in
the broker interface, but regular-market cash availability still follows the
securities settlement cycle.

For IDX regular-market equities, settlement is commonly T+2: the transaction is
settled two exchange days after the trade date. That creates a practical capital
lock:

```text
profitable stock sale
    -> profit appears in broker account
    -> cash is not withdrawable on the same day
    -> user waits for T+2-style settlement
    -> capital cannot be redeployed or withdrawn instantly
```

This is one of the core reasons PulsarFi uses an IDRX-denominated on-chain
receipt market. A user exiting a pStock position receives wallet-settled IDRX
from the pool instead of waiting for brokerage cash settlement.

PulsarFi does not claim to remove the underlying market's custody requirements.
Instead, it creates an on-chain secondary exit path: a user can swap pStock
against IDRX liquidity even when the traditional brokerage exit path is slower
or temporarily constrained.

References:

- [IDX trading hours and settlement mechanism](https://www.idx.id/en/products-services/trading-hours-and-mechanism/)
- [KPEI / IDClear equity settlement overview](https://www.idclear.co.id/en/segmentation/equities/settlement)
- [KSEI transaction settlement services](https://web.ksei.co.id/services/types/transaction-settlement?setLocale=en-US)

## Liquidity integrity problem

If a protocol mints receipt tokens directly to an operator wallet without
matching liquidity, the first seller can drain the pool and break the expected
relationship between the receipt and the underlying asset. For this reason,
PulsarFi sends new mints into the liquidity pool with IDRX funding.

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

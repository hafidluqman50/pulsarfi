---
id: why-idrx
title: Why IDRX
sidebar_label: Why IDRX
slug: /why-idrx
---

PulsarFi uses IDRX as the settlement and liquidity currency because the product
is built around Indonesian equities. The goal is not only to tokenize IDX
exposure, but also to make Rupiah-denominated stablecoin rails more useful in
real financial workflows.

## Short answer

PulsarFi uses IDRX instead of USDC because the underlying assets are Indonesian
stocks, priced in Rupiah, traded by a market that thinks in Rupiah, and redeemed
through an Indonesian securities context.

Using IDRX keeps the product native to the asset market it represents.

## Strategic reason

The broader thesis is Rupiah adoption.

Stablecoins become useful when they are attached to real use cases. If every
on-chain financial product defaults to USD, local-currency stablecoins remain
secondary and speculative. PulsarFi gives IDRX a concrete use case:

```text
Rupiah stablecoin -> Indonesian equity liquidity -> portfolio settlement -> redemption gateway
```

This creates a more natural loop for Indonesian users, custodians, and market
participants.

## Product fit

IDX stocks are quoted in Indonesian Rupiah. Using IDRX means the unit of account
matches the real-world market.

| Product question | With IDRX | With USDC |
| --- | --- | --- |
| What currency is the underlying stock quoted in? | Same currency: Rupiah. | Different currency: USD. |
| Does the user need to mentally convert price? | No, price remains IDR-native. | Yes, user thinks in USD/IDR FX. |
| Does the pool reflect local settlement context? | Yes, IDRX-pStock. | Partially, but FX is embedded. |
| Is the product helping local stablecoin adoption? | Yes. | No, it reinforces USD dominance. |
| Is redemption accounting simpler? | Yes, fee and local notional stay Rupiah-denominated. | Less direct because the off-chain asset is IDR-based. |

## Why not USDC

USDC is more liquid globally, but it is not always the best currency for a
local-market RWA product.

USDC would introduce several mismatches:

- IDX assets are priced in IDR, not USD.
- Indonesian users usually reason about stock prices in Rupiah.
- Redemption and custody operations are naturally Rupiah-denominated.
- A USDC pool embeds USD/IDR FX exposure into the product.
- It makes the system look like Indonesian equities are being dollarized instead
  of brought on-chain in their native market context.

USDC is excellent for global crypto settlement. PulsarFi's first market is
different: it is Indonesian equity exposure. For that market, IDRX is the more
coherent base asset.

## FX risk and price clarity

If the pool used USDC, the user would not only be trading stock exposure. They
would also be implicitly exposed to USD/IDR conversion.

Example with USDC:

```text
IDX reference price: BBRI quoted in IDR
USDC pool price:    BBRIP quoted in USD
User comparison:    must convert USD -> IDR before judging peg or slippage
```

Example with IDRX:

```text
IDX reference price: BBRI quoted in IDR
IDRX pool price:    BBRIP quoted in IDRX
User comparison:    direct local-currency comparison
```

This makes the UI easier to understand and makes arbitrage reasoning cleaner.

## Local adoption thesis

PulsarFi is also a distribution surface for Rupiah stablecoin usage.

IDRX gains practical demand when users need it to:

- enter pStock positions;
- exit pStock positions;
- fund liquidity for new mint proposals;
- pay redemption fees;
- account for portfolio value in Rupiah terms.

That is more meaningful than holding a stablecoin with no specific local use
case.

## Custodian and operator alignment

Custodians operate in an Indonesian market context. Their real-world references
are IDR-denominated:

- share prices;
- notional value;
- settlement reporting;
- local client expectations;
- redemption fee accounting;
- reserve and custody reconciliation.

Using IDRX keeps on-chain operations aligned with those references.

## Tradeoffs

Choosing IDRX is not free.

| Tradeoff | Impact | Why acceptable for MVP |
| --- | --- | --- |
| Lower global liquidity than USDC | Pools may start thinner. | The product is local-market-first and uses faucet/testnet liquidity for MVP. |
| More dependency on IDRX availability | Production needs real IDRX deployment or bridge support. | The architecture isolates IDRX address configuration. |
| Less familiar for global DeFi users | Some users may prefer USD-denominated assets. | The docs and UI explain IDRX as the Rupiah settlement asset. |

The tradeoff is intentional. PulsarFi optimizes for Indonesian equity market
coherence before global stablecoin liquidity.

## Future path

USDC can still be useful later as a secondary route or aggregator input. A future
version could support:

- USDC -> IDRX -> pStock routing;
- multi-stable routing;
- FX-aware quotes;
- deeper liquidity incentives;
- institutional rails that bridge IDR and USD liquidity.

But the canonical pool and accounting currency should remain IDRX for the core
Indonesian equity product.

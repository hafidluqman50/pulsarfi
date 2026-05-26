import { parseUnits, type Address } from 'viem';
import type { MarketStock } from '@/http/market/priceApi';
import { fmtIDRX, fmtNum, PStock, PSTOCKS, Token } from '@/lib/data';

export type MarketToken = PStock & { contractAddress?: Address };

export interface SwapQuote {
  inputAmount: number;
  rate: number;
  grossOutputAmount: number;
  outputAmount: number;
  minReceived: number;
  amountIn: bigint;
  amountOutMin: bigint;
  rateSummary: string;
}

export function toMarketToken(stock: MarketStock): MarketToken {
  const fallback = PSTOCKS.find(item => item.ticker === stock.ticker);
  return {
    ticker: stock.ticker,
    name: stock.stock_name,
    sector: stock.sector ?? fallback?.sector ?? 'Other',
    price: stock.pool_price || 0,
    change24h: stock.change_24h ?? fallback?.change24h ?? 0,
    supply: fallback?.supply ?? 0,
    ipo: stock.idx_ticker || fallback?.ipo || stock.ticker,
    contractAddress: stock.contract_address as Address | undefined,
    isStable: false,
  };
}

export function tokenDecimals(token: Token): number {
  return token.isStable ? 2 : 18;
}

export function tokenAddress(token: Token, idrxAddress?: Address): Address | undefined {
  if (token.isStable) return idrxAddress;
  return (token as MarketToken).contractAddress;
}

export function stockTickerForSwap(input: Token, output: Token): string {
  return input.isStable ? output.ticker : input.ticker;
}

export function unitsFromNumber(value: number, decimals: number): bigint {
  return parseUnits(value.toFixed(Math.min(decimals, 6)), decimals);
}

export function unitsFromDecimalInput(value: string, decimals: number): bigint {
  const [whole = '0', fraction = ''] = (value || '0').split('.');
  const normalized = fraction
    ? `${whole || '0'}.${fraction.slice(0, decimals)}`
    : whole || '0';
  return parseUnits(normalized, decimals);
}

export function buildSwapQuote(input: Token, output: Token, rawAmount: string, slippage: number): SwapQuote {
  const inputAmount = parseFloat(rawAmount) || 0;
  const rate = output.price > 0 ? input.price / output.price : 0;
  const grossOutputAmount = inputAmount * rate;
  const outputAmount = grossOutputAmount * (1 - 0.003);
  const minReceived = outputAmount * (1 - slippage / 100);
  const rateSummary = output.price > 0 && input.isStable && !output.isStable
    ? `1 ${output.ticker} = ${fmtIDRX(output.price)}`
    : `1 ${input.ticker} = ${rate.toFixed(4)} ${output.ticker}`;

  return {
    inputAmount,
    rate,
    grossOutputAmount,
    outputAmount,
    minReceived,
    amountIn: unitsFromDecimalInput(rawAmount, tokenDecimals(input)),
    amountOutMin: unitsFromNumber(minReceived, tokenDecimals(output)),
    rateSummary,
  };
}

export function swapToastDescription(input: Token, output: Token, quote: SwapQuote): string {
  return `${fmtNum(quote.inputAmount)} ${input.ticker} -> ${fmtNum(quote.outputAmount, 2)} ${output.ticker}`;
}

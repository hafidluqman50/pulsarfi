import { useMemo } from 'react';
import { formatUnits, type Address } from 'viem';
import { useAccount, useReadContracts } from 'wagmi';
import { IDRX_ABI } from '@/lib/abi/idrx_abi';
import { PULSAR_STOCK_ABI } from '@/lib/abi/pulsar_stock_abi';
import type { Balances } from '@/lib/data';
import { MarketToken, toMarketToken } from '@/lib/swap';
import { useMarketStocks } from './hooks';

export function useMarketTokens(): MarketToken[] {
  const { data: marketStocks = [] } = useMarketStocks();
  return useMemo(() => marketStocks.map(toMarketToken), [marketStocks]);
}

export function useWalletTokenBalances(tokens: MarketToken[]): Balances {
  const { address } = useAccount();
  const idrxAddress = process.env.NEXT_PUBLIC_IDRX_ADDRESS as Address | undefined;
  const tokenContracts = useMemo(() => {
    if (!address) return [];
    return tokens
      .filter(token => token.contractAddress)
      .flatMap(token => ([
        {
          address: token.contractAddress!,
          abi: PULSAR_STOCK_ABI,
          functionName: 'balanceOf',
          args: [address],
        },
        {
          address: token.contractAddress!,
          abi: PULSAR_STOCK_ABI,
          functionName: 'decimals',
        },
      ]));
  }, [address, tokens]);

  const { data: balanceReads } = useReadContracts({
    contracts: [
      ...(idrxAddress && address ? [
        {
          address: idrxAddress,
          abi: IDRX_ABI,
          functionName: 'balanceOf',
          args: [address],
        },
        {
          address: idrxAddress,
          abi: IDRX_ABI,
          functionName: 'decimals',
        },
      ] : []),
      ...(address ? tokenContracts : []),
    ],
    query: {
      enabled: Boolean(address && idrxAddress),
      refetchInterval: 15_000,
    },
  });

  return useMemo(() => {
    if (!balanceReads || !address || !idrxAddress) return {};

    const next: Balances = {};
    const idrxBalance = balanceReads[0]?.result;
    const idrxDecimals = balanceReads[1]?.result;
    if (typeof idrxBalance === 'bigint' && typeof idrxDecimals === 'number') {
      next.IDRX = Number(formatUnits(idrxBalance, idrxDecimals));
    }

    let readIndex = 2;
    for (const token of tokens.filter(item => item.contractAddress)) {
      const tokenBalance = balanceReads[readIndex]?.result;
      const tokenDecimals = balanceReads[readIndex + 1]?.result;
      if (typeof tokenBalance === 'bigint' && typeof tokenDecimals === 'number') {
        next[token.ticker] = Number(formatUnits(tokenBalance, tokenDecimals));
      }
      readIndex += 2;
    }

    return next;
  }, [address, balanceReads, idrxAddress, tokens]);
}

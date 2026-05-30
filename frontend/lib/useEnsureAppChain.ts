'use client';

import { useAccount, useSwitchChain } from 'wagmi';
import { appChainId } from '@/lib/wagmi';

export function useEnsureAppChain() {
  const { chainId: connectedChainId } = useAccount();
  const { switchChainAsync } = useSwitchChain();

  return async () => {
    if (connectedChainId !== appChainId) {
      await switchChainAsync({ chainId: appChainId });
    }
  };
}

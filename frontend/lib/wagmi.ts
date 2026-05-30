'use client';

import { getDefaultConfig } from '@rainbow-me/rainbowkit';
import { arbitrumSepolia } from 'wagmi/chains';
import { http } from 'wagmi';

export const config = getDefaultConfig({
  appName: 'PulsarFi',
  projectId: process.env.NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID || 'pulsarfi-dev',
  chains: [arbitrumSepolia],
  transports: {
    [arbitrumSepolia.id]: http(process.env.NEXT_PUBLIC_RPC_URL),
  },
  ssr: true,
});

export const appChain = config.chains[0];
export const appChainId = appChain.id;

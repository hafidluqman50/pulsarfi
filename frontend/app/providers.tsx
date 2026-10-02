'use client';

import { RainbowKitProvider, lightTheme } from '@rainbow-me/rainbowkit';
import { WagmiProvider } from 'wagmi';
import { QueryClientProvider, QueryClient } from '@tanstack/react-query';
import { config } from '@/lib/wagmi';
import { Toaster } from 'sonner';
import { SiweAuthProvider } from '@/contexts/SiweAuthContext';

import '@rainbow-me/rainbowkit/styles.css';

const queryClient = new QueryClient();

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <WagmiProvider config={config}>
      <QueryClientProvider client={queryClient}>
        <RainbowKitProvider
          theme={lightTheme({
            accentColor: '#c8102e',
            accentColorForeground: 'white',
            borderRadius: 'none',
            fontStack: 'system',
          })}
          locale="en-US"
        >
          <SiweAuthProvider>
            {children}
          </SiweAuthProvider>
          <Toaster
            position="bottom-right"
            style={{ '--width': '380px' } as React.CSSProperties}
            icons={{
              success: <span className="toast-glyph" style={{ color: 'var(--positive)' }}>✓</span>,
              error: <span className="toast-glyph" style={{ color: 'var(--merah)' }}>!</span>,
              info: <span className="toast-glyph">i</span>,
              loading: <span className="toast-spinner" />,
            }}
            toastOptions={{
              unstyled: true,
              classNames: { toast: 'paper-toast' },
            }}
          />
        </RainbowKitProvider>
      </QueryClientProvider>
    </WagmiProvider>
  );
}

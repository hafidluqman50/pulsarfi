'use client';

import { createContext, useContext, useEffect, useRef, useState } from 'react';
import { useAccount, useDisconnect, useSignMessage } from 'wagmi';
import { toast } from 'sonner';
import { buildSiweMessage, fetchNonce, verifySignature } from '@/http/auth/siweApi';

const ACCESS_TOKEN_KEY = 'access_token';

interface SiweAuthState {
  isAuthenticated: boolean;
  role: 'user' | 'custodian' | null;
  signIn: () => Promise<void>;
  signOut: () => void;
}

const SiweAuthContext = createContext<SiweAuthState>({
  isAuthenticated: false,
  role: null,
  signIn: async () => {},
  signOut: () => {},
});

interface TokenClaims {
  wallet_address?: string;
  sub?: string;
  role?: string;
  exp?: number;
}

interface StoredSession {
  token: string;
  walletAddress: string;
  role: 'user' | 'custodian' | null;
}

export function useSiweAuth() {
  return useContext(SiweAuthContext);
}

function decodeToken(token: string): TokenClaims | null {
  try {
    const payload = token.split('.')[1];
    if (!payload) return null;
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
    const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), '=');
    return JSON.parse(atob(padded)) as TokenClaims;
  } catch {
    return null;
  }
}

function parseRole(claims: TokenClaims): 'user' | 'custodian' | null {
  return claims.role === 'user' || claims.role === 'custodian' ? claims.role : null;
}

function isExpired(claims: TokenClaims) {
  return typeof claims.exp === 'number' && claims.exp * 1000 <= Date.now();
}

function readStoredSession(): StoredSession | null {
  if (typeof window === 'undefined') return null;

  const token = localStorage.getItem(ACCESS_TOKEN_KEY);
  if (!token) return null;

  const claims = decodeToken(token);
  const walletAddress = (claims?.wallet_address ?? claims?.sub)?.toLowerCase();
  if (!claims || !walletAddress || isExpired(claims)) return null;

  return {
    token,
    walletAddress,
    role: parseRole(claims),
  };
}

function hasStoredToken() {
  return typeof window !== 'undefined' && Boolean(localStorage.getItem(ACCESS_TOKEN_KEY));
}

export function SiweAuthProvider({ children }: { children: React.ReactNode }) {
  const { address, isConnected } = useAccount();
  const { disconnect } = useDisconnect();
  const { signMessageAsync } = useSignMessage();

  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [role, setRole] = useState<'user' | 'custodian' | null>(null);

  // track which address we've authed for so we don't double-trigger
  const authedAddressRef = useRef<string | null>(null);
  const signingInRef = useRef(false);

  function clearAuth() {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
    setIsAuthenticated(false);
    setRole(null);
    authedAddressRef.current = null;
  }

  function applySession(session: StoredSession) {
    localStorage.setItem(ACCESS_TOKEN_KEY, session.token);
    setIsAuthenticated(true);
    setRole(session.role);
    authedAddressRef.current = session.walletAddress;
  }

  function applyToken(token: string, fallbackAddress: string) {
    const claims = decodeToken(token);
    const walletAddress = (claims?.wallet_address ?? claims?.sub ?? fallbackAddress).toLowerCase();
    applySession({
      token,
      walletAddress,
      role: claims ? parseRole(claims) : null,
    });
  }

  function deferApplySession(session: StoredSession) {
    window.setTimeout(() => applySession(session), 0);
  }

  function deferClearAuth() {
    window.setTimeout(() => clearAuth(), 0);
  }

  // restore from localStorage on mount
  useEffect(() => {
    const session = readStoredSession();
    if (session) deferApplySession(session);
    else if (hasStoredToken()) deferClearAuth();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // keep the persisted SIWE session when the same wallet reconnects or the app refreshes
  useEffect(() => {
    if (!isConnected || !address) return;

    const walletAddress = address.toLowerCase();
    const session = readStoredSession();

    if (session?.walletAddress === walletAddress) {
      deferApplySession(session);
      return;
    }

    if (session || (authedAddressRef.current && authedAddressRef.current !== walletAddress)) {
      deferClearAuth();
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isConnected, address]);

  // auto-trigger SIWE only when there is no valid stored session for this wallet
  useEffect(() => {
    if (!isConnected || !address) return;

    const walletAddress = address.toLowerCase();
    const session = readStoredSession();

    if (session?.walletAddress === walletAddress) {
      deferApplySession(session);
      return;
    }

    if (session || hasStoredToken()) {
      deferClearAuth();
    }

    if (signingInRef.current) return;

    // slight delay so RainbowKit modal closes first
    const timer = setTimeout(() => signIn(), 400);
    return () => clearTimeout(timer);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isConnected, address]);

  async function signIn() {
    if (!address || signingInRef.current) return;
    signingInRef.current = true;
    try {
      const nonce = await fetchNonce(address);
      const message = buildSiweMessage(address, nonce);
      const signature = await signMessageAsync({ message });
      const token = await verifySignature(address, message, signature, nonce);
      applyToken(token, address);
      toast.success('Signed in', { description: `Wallet verified · ${address.slice(0, 6)}…${address.slice(-4)}` });
    } catch (error: unknown) {
      const msg = error instanceof Error ? error.message : 'Sign-in failed';
      // user rejected the signature — disconnect so UI resets cleanly
      if (msg.toLowerCase().includes('user rejected') || msg.toLowerCase().includes('denied')) {
        toast.info('Sign-in cancelled', { description: 'Wallet disconnected' });
        disconnect();
      } else {
        toast.error('Sign-in failed', { description: msg });
      }
    } finally {
      signingInRef.current = false;
    }
  }

  function signOut() {
    clearAuth();
    disconnect();
  }

  return (
    <SiweAuthContext.Provider value={{ isAuthenticated, role, signIn, signOut }}>
      {children}
    </SiweAuthContext.Provider>
  );
}

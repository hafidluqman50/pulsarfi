'use client';

import { createContext, useContext, useEffect, useRef, useState } from 'react';
import { useAccount, useDisconnect, useSignMessage } from 'wagmi';
import { toast } from 'sonner';
import { buildSiweMessage, fetchNonce, verifySignature } from '@/http/auth/siweApi';

const ACCESS_TOKEN_KEY = 'access_token';

export type SignInPhase = 'idle' | 'requesting-nonce' | 'awaiting-signature' | 'verifying';

interface SiweAuthState {
  isAuthenticated: boolean;
  role: 'user' | 'custodian' | null;
  signInPhase: SignInPhase;
  signIn: () => Promise<void>;
  signOut: () => void;
}

const SiweAuthContext = createContext<SiweAuthState>({
  isAuthenticated: false,
  role: null,
  signInPhase: 'idle',
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

let globalInFlightAddress: string | null = null;

export function SiweAuthProvider({ children }: { children: React.ReactNode }) {
  const { address, isConnected } = useAccount();
  const { disconnect } = useDisconnect();
  const { signMessageAsync } = useSignMessage();

  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [role, setRole] = useState<'user' | 'custodian' | null>(null);
  const [signInPhase, setSignInPhase] = useState<SignInPhase>('idle');

  // track which address we've authed for so we don't double-trigger
  const authedAddressRef = useRef<string | null>(null);
  // synchronous re-entrancy guard
  const signingInRef = useRef(false);

  function clearAuth() {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
    setIsAuthenticated(false);
    setRole(null);
    authedAddressRef.current = null;
    globalInFlightAddress = null;
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

  // restore session or auto-trigger SIWE when wallet connects
  useEffect(() => {
    if (!isConnected || !address) {
      if (authedAddressRef.current !== null || isAuthenticated) {
        clearAuth();
      }
      return;
    }

    const walletAddress = address.toLowerCase();

    // Already authed in memory for this exact wallet address
    if (authedAddressRef.current === walletAddress && isAuthenticated) {
      return;
    }

    // Check stored session in localStorage
    const session = readStoredSession();
    if (session?.walletAddress === walletAddress) {
      applySession(session);
      return;
    }

    // If stored session is for another address or expired, clear it
    if (session || hasStoredToken()) {
      clearAuth();
    }

    // Don't trigger if already signing in globally or in ref
    if (signingInRef.current || globalInFlightAddress === walletAddress) return;

    // Lock immediately so no other effect/timer can trigger
    globalInFlightAddress = walletAddress;

    // Slight delay so RainbowKit modal closes first
    const timer = setTimeout(() => {
      signIn();
    }, 400);

    return () => {
      clearTimeout(timer);
      if (!signingInRef.current && globalInFlightAddress === walletAddress) {
        globalInFlightAddress = null;
      }
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isConnected, address, isAuthenticated]);

  async function signIn() {
    if (!address) {
      globalInFlightAddress = null;
      return;
    }
    const targetAddress = address.toLowerCase();

    // Skip if already authed for this address
    if (authedAddressRef.current === targetAddress && isAuthenticated) {
      globalInFlightAddress = null;
      return;
    }

    if (signingInRef.current) return;

    signingInRef.current = true;
    globalInFlightAddress = targetAddress;

    try {
      setSignInPhase('requesting-nonce');
      const nonce = await fetchNonce(targetAddress);
      const message = buildSiweMessage(targetAddress, nonce);

      setSignInPhase('awaiting-signature');
      const signature = await signMessageAsync({ message });

      setSignInPhase('verifying');
      const token = await verifySignature(targetAddress, message, signature, nonce);

      if (address?.toLowerCase() === targetAddress) {
        applyToken(token, targetAddress);
        toast.success('Signed in', { description: `Wallet verified · ${targetAddress.slice(0, 6)}…${targetAddress.slice(-4)}` });
      }
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
      globalInFlightAddress = null;
      setSignInPhase('idle');
    }
  }

  function signOut() {
    clearAuth();
    disconnect();
  }

  return (
    <SiweAuthContext.Provider value={{ isAuthenticated, role, signInPhase, signIn, signOut }}>
      {children}
    </SiweAuthContext.Provider>
  );
}

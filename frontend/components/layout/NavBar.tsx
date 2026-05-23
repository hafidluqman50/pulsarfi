'use client';

import { ConnectButton } from '@rainbow-me/rainbowkit';
import Image from 'next/image';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { Wordmark } from '@/components/ui/Wordmark';
import { shortAddr } from '@/lib/data';

const NAVIGATION_ITEMS = [
  { href: '/home',      label: 'Home'      },
  { href: '/portfolio', label: 'Portfolio' },
  { href: '/custodian', label: 'Custodian' },
  { href: '/stocks',    label: 'Markets'   },
] as const;

export function NavBar() {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const pathname = usePathname();

  const isRouteActive = (href: string) =>
    href === '/stocks' ? pathname.startsWith('/stocks') : pathname === href;

  return (
    <div
      className="hairline-strong"
      style={{ position: 'sticky', top: 0, zIndex: 100, background: 'var(--canvas)', padding: '14px 24px' }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16 }}>

        {/* Left: hamburger (mobile) + navigation tabs */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 16, flex: 1, minWidth: 0 }}>
          <button
            className="only-mobile"
            onClick={() => setIsMobileMenuOpen(isOpen => !isOpen)}
            aria-label="Toggle navigation menu"
            style={{ appearance: 'none', border: '1px solid var(--ink)', background: 'var(--canvas)', padding: '8px', cursor: 'pointer', color: 'var(--ink)', lineHeight: 0 }}
          >
            <Icon name={isMobileMenuOpen ? 'x' : 'menu'} size={18} />
          </button>

          <nav className="nav-tabs-desktop" style={{ display: 'flex', gap: 28 }}>
            {NAVIGATION_ITEMS.map(item => (
              <Link
                key={item.href}
                href={item.href}
                className={`tab ${isRouteActive(item.href) ? 'active' : ''}`}
                style={{ textDecoration: 'none', color: 'inherit' }}
              >
                {item.label}
              </Link>
            ))}
          </nav>
        </div>

        {/* Center: Wordmark */}
        <div style={{ textAlign: 'center' }}>
          <Link href="/home" style={{ textDecoration: 'none', color: 'inherit' }}>
            {/*<Wordmark size={28} />*/}
            <Image src="/logo.png" alt="PulsarFi" width={200} height={55} style={{ objectFit: 'contain' }} />
          </Link>
        </div>

        {/* Right: wallet connect */}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12, alignItems: 'center', flex: 1 }}>
          <span className="eyebrow nav-wallet-ens" style={{ color: 'var(--body)' }}>EN · IDR/USD 16,142</span>
          <ConnectButton.Custom>
            {({ account, chain, openConnectModal, openAccountModal, mounted }) => {
              if (!mounted) return null;
              const isWalletConnected = account && chain;
              if (!isWalletConnected) {
                return (
                  <button className="btn btn-merah" onClick={openConnectModal} style={{ padding: '10px 14px', fontSize: 13 }}>
                    Connect Wallet
                  </button>
                );
              }
              return (
                <button
                  className="btn btn-outline"
                  onClick={openAccountModal}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: 10, padding: '10px 14px' }}
                >
                  <span style={{ width: 8, height: 8, background: '#1f7a4b', display: 'inline-block' }} />
                  <span className="mono" style={{ fontSize: 12 }}>{shortAddr(account.address)}</span>
                </button>
              );
            }}
          </ConnectButton.Custom>
        </div>
      </div>

      {/* Mobile slide-down menu */}
      {isMobileMenuOpen && (
        <div className="mobile-menu only-mobile">
          {NAVIGATION_ITEMS.map(item => (
            <Link
              key={item.href}
              href={item.href}
              className={`mm-tab ${isRouteActive(item.href) ? 'active' : ''}`}
              style={{ textDecoration: 'none' }}
              onClick={() => setIsMobileMenuOpen(false)}
            >
              {item.label}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

'use client';

import { ConnectButton } from '@rainbow-me/rainbowkit';
import Image from 'next/image';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { shortAddr } from '@/lib/data';
import { useSiweAuth } from '@/contexts/SiweAuthContext';

const NAVIGATION_ITEMS = [
  { href: '/home',      label: 'Home'      },
  { href: '/portfolio', label: 'Portfolio' },
  { href: '/custodian', label: 'Custodian' },
  { href: '/stocks',    label: 'Markets'   },
] as const;

export function NavBar() {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const pathname = usePathname();
  const { isAuthenticated, role, signOut } = useSiweAuth();

  const isRouteActive = (href: string) =>
    href === '/stocks' ? pathname.startsWith('/stocks') : pathname === href;

  return (
    <div className="hairline-strong sticky top-[0px] z-[100] bg-[var(--canvas)] px-[24px] py-[14px]">
      <div className="relative flex items-center justify-between gap-[16px]">

        {/* Left: hamburger + logo mobile + nav tabs desktop */}
        <div className="flex items-center gap-[16px] flex-1 min-w-0">
          <button
            type="button"
            className="only-mobile appearance-none border border-[var(--ink)] bg-[var(--canvas)] p-[8px] cursor-pointer text-[var(--ink)] leading-none"
            onClick={() => setIsMobileMenuOpen(isOpen => !isOpen)}
            aria-label="Toggle navigation menu"
          >
            <Icon name={isMobileMenuOpen ? 'x' : 'menu'} size={18} />
          </button>

          <Link href="/home" className="only-mobile no-underline">
            <Image src="/logo-only-nobg.png" alt="PulsarFi" width={40} height={40} className="nav-logo-mobile w-auto block" />
          </Link>

          <nav className="nav-tabs-desktop flex gap-[28px]">
            {NAVIGATION_ITEMS.map(item => (
              <Link
                key={item.href}
                href={item.href}
                className={`tab no-underline text-inherit ${isRouteActive(item.href) ? 'active' : ''}`}
              >
                {item.label}
              </Link>
            ))}
          </nav>
        </div>

        {/* Center: Logo desktop — absolute so it doesn't expand navbar height */}
        <div className="nav-logo-center only-desktop">
          <Link href="/home" className="no-underline">
            <Image src="/logo-nobg.png" alt="PulsarFi" width={200} height={70} className="nav-logo-img w-auto block" />
          </Link>
        </div>

        {/* Right: wallet connect */}
        <div className="flex justify-end gap-[12px] items-center flex-1">
          <span className="eyebrow nav-wallet-ens text-[var(--body)]">EN · IDR/USD 16,142</span>
          <ConnectButton.Custom>
            {({ account, chain, openConnectModal, openAccountModal, mounted }) => {
              if (!mounted) return null;
              const isWalletConnected = account && chain;
              if (!isWalletConnected) {
                return (
                  <button type="button" className="btn btn-merah px-[14px] py-[10px] text-[13px]" onClick={openConnectModal}>
                    Connect Wallet
                  </button>
                );
              }
              return (
                <div className="inline-flex items-center gap-[8px]">
                  {role === 'custodian' && (
                    <span className="eyebrow text-[10px] px-[8px] py-[4px] bg-[var(--merah)] text-white">
                      CUSTODIAN
                    </span>
                  )}
                  <button
                    type="button"
                    className="btn btn-outline inline-flex items-center gap-[10px] px-[14px] py-[10px]"
                    onClick={isAuthenticated ? signOut : openAccountModal}
                  >
                    <span className={`w-[8px] h-[8px] inline-block ${isAuthenticated ? 'bg-[#1f7a4b]' : 'bg-[var(--body)]'}`} />
                    <span className="mono text-[12px]">{shortAddr(account.address)}</span>
                  </button>
                </div>
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
              className={`mm-tab no-underline ${isRouteActive(item.href) ? 'active' : ''}`}
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

'use client';

import { useState } from 'react';
import { type Address } from 'viem';
import { fmtNum, fmtIDRX } from '@/lib/data';

export type RedeemToken = {
  ticker: string;
  name: string;
  price: number;
  address: Address;
};

interface RedeemModalProps {
  token: RedeemToken;
  balance: number;
  onClose: () => void;
  onSubmit: (opts: { token: RedeemToken; amount: string }) => void;
  busy: boolean;
}

export function RedeemModal({ token, balance, onClose, onSubmit, busy }: RedeemModalProps) {
  const [amt, setAmt] = useState('');
  const num = parseFloat(amt) || 0;
  const ok  = num > 0 && num <= balance && !busy;

  return (
    <div className="overlay fixed inset-[0] z-[200] flex items-center justify-center bg-[rgba(22,17,14,0.32)] p-[16px]" onClick={onClose}>
      <div className="rise paper-sheaf w-[440px] max-w-full" onClick={e => e.stopPropagation()}>
        <div className="sheet">

          <div className="flex items-center justify-between border-b border-[var(--hairline)] px-[20px] py-[16px]">
            <div className="display !text-[22px] !font-[500] !leading-[normal]">Redeem {token.ticker}</div>
            <button
              className="h-[30px] w-[30px] cursor-pointer appearance-none border border-[var(--hairline)] bg-[var(--putih)] text-[13px] text-[var(--ink)]"
              onClick={onClose}
              aria-label="Close"
            >
              ✕
            </button>
          </div>

          <div className="flex flex-col gap-[14px] p-[20px]">

            <div className="relative -rotate-[0.4deg] border border-[#e8d9b0] bg-[#fbf6e8] p-[14px] shadow-[0_8px_14px_-10px_rgba(22,17,14,0.3)]">
              <div className="mb-[4px] text-[12px] font-[600] uppercase leading-[normal] tracking-[0.08em]">KYC required</div>
              <div className="text-[13px] leading-[1.55] text-[var(--body)]">
                Redemption converts your tokens back to physical IDX shares settled via KSEI.
                Your wallet must be KYC-verified first.{' '}
                <span className="font-[500] text-[var(--ink)]">
                  Send your identity &amp; broker documents to{' '}
                  <span className="mono">kyc@pulsarfi.xyz</span>
                </span>{' '}
                to get verified.
              </div>
            </div>

            <div>
              <div className="mb-[6px] flex justify-between">
                <div className="text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">Token amount to redeem</div>
                <span className="mono text-[12px] leading-[normal] text-[var(--body)]">Balance {fmtNum(balance, 4)}</span>
              </div>
              <div className="relative">
                <input
                  className={`input mono !bg-[var(--canvas)] !p-[12px] !pr-[60px] !text-[15px] !font-[500] ${amt ? '!border-[var(--ink)]' : ''}`}
                  placeholder="0.00"
                  inputMode="decimal"
                  value={amt}
                  onChange={e => setAmt(e.target.value.replace(/[^0-9.]/g, ''))}
                />
                <button
                  onClick={() => setAmt(balance.toString())}
                  className="absolute right-[6px] top-1/2 -translate-y-1/2 cursor-pointer appearance-none border border-[var(--hairline-strong)] bg-[var(--canvas-soft)] px-[8px] py-[4px] text-[11px] font-[600] [font-family:var(--font-inter,_Inter,_sans-serif)]"
                >
                  MAX
                </button>
              </div>
              {num > balance && (
                <div className="mt-[6px] text-[12px] text-[var(--merah)]">Exceeds available balance</div>
              )}
            </div>

            {num > 0 && (
              <div className="flex flex-col gap-[6px] border-t border-[var(--hairline)] pt-[14px] text-[13px]">
                <div className="flex items-baseline justify-between gap-[12px]">
                  <span className="text-[var(--body)]">Est. value</span>
                  <span className="mono text-right">{fmtIDRX(num * token.price)}</span>
                </div>
                <div className="flex items-baseline justify-between gap-[12px]">
                  <span className="text-[var(--body)]">Settlement</span>
                  <span className="mono text-right text-[var(--body)]">Via KSEI · 2–3 business days</span>
                </div>
                <div className="flex items-baseline justify-between gap-[12px]">
                  <span className="text-[var(--body)]">Network</span>
                  <span className="mono text-right">Arbitrum Sepolia</span>
                </div>
              </div>
            )}

            <button
              className="btn btn-primary !w-full !p-[14px]"
              disabled={!ok}
              onClick={() => onSubmit({ token, amount: amt })}
            >
              {busy ? 'Submitting...' : `Redeem ${token.ticker}`}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

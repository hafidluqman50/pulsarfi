'use client';

import { useState } from 'react';
import { type Address } from 'viem';
import { fmtNum, fmtIDRX } from '@/lib/data';
import { Icon } from '@/components/ui/Icon';

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
    <div className="overlay fixed inset-[0] z-[200] flex items-center justify-center bg-[rgba(22,17,14,0.45)] p-[16px]" onClick={onClose}>
      <div className="modal w-[440px] max-w-full border border-[var(--ink)] bg-[var(--putih)] shadow-[8px_8px_0_0_rgba(22,17,14,0.10)]" onClick={e => e.stopPropagation()}>

        {/* Header */}
        <div className="hairline flex items-center justify-between px-[20px] py-[16px]">
          <div className="display !text-[22px]">Redeem {token.ticker}</div>
          <button className="btn-ghost btn !p-[4px]" onClick={onClose}><Icon name="x" /></button>
        </div>

        <div className="flex flex-col gap-[14px] p-[20px]">

          {/* KYC notice */}
          <div className="border border-[var(--hairline-strong)] bg-[var(--canvas-soft)] p-[14px]">
            <div className="mb-[4px] text-[12px] font-semibold uppercase tracking-[0.08em]">KYC required</div>
            <div className="text-[13px] leading-[1.55] text-[var(--body)]">
              Redemption converts your tokens back to physical IDX shares settled via KSEI.
              Your wallet must be KYC-verified first.{' '}
              <span className="font-medium text-[var(--ink)]">
                Send your identity &amp; broker documents to{' '}
                <span className="mono">kyc@pulsarfi.xyz</span>
              </span>{' '}
              to get verified.
            </div>
          </div>

          {/* Amount input */}
          <div>
            <div className="mb-[6px] flex justify-between">
              <div className="eyebrow !text-[var(--body)]">Token amount to redeem</div>
              <span className="mono text-[12px] text-[var(--body)]">Balance {fmtNum(balance, 4)}</span>
            </div>
            <div className="relative">
              <input
                className="input mono !pr-[60px]"
                placeholder="0.00"
                inputMode="decimal"
                value={amt}
                onChange={e => setAmt(e.target.value.replace(/[^0-9.]/g, ''))}
              />
              <button
                onClick={() => setAmt(balance.toString())}
                className="absolute right-[6px] top-1/2 -translate-y-1/2 cursor-pointer appearance-none border border-[var(--hairline-strong)] bg-[var(--canvas)] px-[8px] py-[4px] text-[11px] font-semibold [font-family:var(--font-inter,_Inter,_sans-serif)]"
              >
                MAX
              </button>
            </div>
            {num > balance && (
              <div className="mt-[6px] text-[12px] text-[var(--negative)]">Exceeds available balance</div>
            )}
          </div>

          {/* Summary */}
          {num > 0 && (
            <div className="hairline-top flex flex-col gap-[6px] pt-[14px] text-[13px]">
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
  );
}

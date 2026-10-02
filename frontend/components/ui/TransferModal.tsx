'use client';

import { useState } from 'react';
import { type Address } from 'viem';
import { fmtNum } from '@/lib/data';
import { DetailRow } from '@/components/swap/SwapView';

export type TransferToken = {
  ticker: string;
  name: string;
  price: number;
  address: Address;
  isStable: boolean;
};

interface TransferModalProps {
  token: TransferToken;
  balance: number;
  onClose: () => void;
  onSubmit: (opts: { token: TransferToken; to: Address; amount: string }) => void;
  busy: boolean;
}

export function TransferModal({ token, balance, onClose, onSubmit, busy }: TransferModalProps) {
  const [to, setTo]   = useState('');
  const [amt, setAmt] = useState('');
  const num = parseFloat(amt) || 0;
  const ok  = /^0x[a-fA-F0-9]{40}$/.test(to) && num > 0 && num <= balance && !busy;

  return (
    <div className="overlay fixed inset-[0] z-[200] flex items-center justify-center bg-[rgba(22,17,14,0.32)] p-[16px]" onClick={onClose}>
      <div className="rise paper-sheaf w-[440px] max-w-full" onClick={e => e.stopPropagation()}>
        <div className="sheet">
          <div className="flex items-center justify-between border-b border-[var(--hairline)] px-[20px] py-[16px]">
            <div className="display !text-[22px] !font-[500] !leading-[normal]">Send {token.ticker}</div>
            <button
              className="h-[30px] w-[30px] cursor-pointer appearance-none border border-[var(--hairline)] bg-[var(--putih)] text-[13px] text-[var(--ink)]"
              onClick={onClose}
              aria-label="Close"
            >
              ✕
            </button>
          </div>
          <div className="flex flex-col gap-[14px] p-[20px]">
            <div>
              <div className="mb-[6px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">Recipient address</div>
              <input
                className={`input mono !bg-[var(--canvas)] !p-[12px] !text-[14px] !font-[500] ${to ? '!border-[var(--ink)]' : ''}`}
                placeholder="0x… or .arb name"
                value={to}
                onChange={e => setTo(e.target.value)}
              />
            </div>
            <div>
              <div className="mb-[6px] flex justify-between">
                <div className="text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">Amount</div>
                <span className="mono text-[12px] leading-[normal] text-[var(--body)]">Balance {fmtNum(balance, 4)}</span>
              </div>
              <div className="relative">
                <input
                  className={`input mono !bg-[var(--canvas)] !p-[12px] !pr-[60px] !text-[15px] !font-[500] ${amt ? '!border-[var(--ink)]' : ''}`}
                  placeholder="0.00"
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
              {num > balance && <div className="mt-[6px] text-[12px] text-[var(--merah)]">Exceeds available balance</div>}
            </div>
            <div className="flex flex-col gap-[6px] border-t border-[var(--hairline)] pt-[14px] text-[13px]">
              <DetailRow k="Network"     v="Arbitrum Sepolia" />
              <DetailRow k="Network fee" v="~$0.12" />
            </div>
            {!token.isStable && (
              <div className="border border-[var(--hairline)] bg-[var(--canvas-soft)] px-[12px] py-[10px] text-[12px] leading-[1.45] text-[var(--body)]">
                Cost basis is tracked accurately for PulsarFi swaps. The recipient&apos;s average buy and P&amp;L may be estimated from the current IDX reference price.
              </div>
            )}
            <button
              className="btn btn-primary !w-full !p-[14px]"
              disabled={!ok}
              onClick={() => onSubmit({ token, to: to as Address, amount: amt })}
            >
              {busy ? 'Sending...' : `Send ${token.ticker}`}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

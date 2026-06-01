'use client';

import { useState } from 'react';
import { type Address } from 'viem';
import { fmtNum } from '@/lib/data';
import { DetailRow } from '@/components/swap/SwapView';
import { Icon } from '@/components/ui/Icon';

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
    <div className="overlay fixed inset-[0] z-[200] flex items-center justify-center bg-[rgba(22,17,14,0.45)] p-[16px]" onClick={onClose}>
      <div className="modal w-[440px] max-w-full border border-[var(--ink)] bg-[var(--putih)] shadow-[8px_8px_0_0_rgba(22,17,14,0.10)]" onClick={e => e.stopPropagation()}>
        <div className="hairline flex items-center justify-between px-[20px] py-[16px]">
          <div className="display !text-[22px]">Send {token.ticker}</div>
          <button className="btn-ghost btn !p-[4px]" onClick={onClose}><Icon name="x" /></button>
        </div>
        <div className="flex flex-col gap-[14px] p-[20px]">
          <div>
            <div className="eyebrow mb-[6px] !text-[var(--body)]">Recipient address</div>
            <input className="input mono" placeholder="0x… or .arb name" value={to} onChange={e => setTo(e.target.value)} />
          </div>
          <div>
            <div className="mb-[6px] flex justify-between">
              <div className="eyebrow !text-[var(--body)]">Amount</div>
              <span className="mono text-[12px] text-[var(--body)]">Balance {fmtNum(balance, 4)}</span>
            </div>
            <div className="relative">
              <input
                className="input mono !pr-[60px]"
                placeholder="0.00"
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
            {num > balance && <div className="mt-[6px] text-[12px] text-[var(--negative)]">Exceeds available balance</div>}
          </div>
          <div className="hairline-top flex flex-col gap-[6px] pt-[14px] text-[13px]">
            <DetailRow k="Network"     v="Arbitrum Sepolia" />
            <DetailRow k="Network fee" v="~$0.12" />
          </div>
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
  );
}

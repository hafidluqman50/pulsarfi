'use client';

import React from 'react';
import { createPortal } from 'react-dom';
import type { AttestorInfo, CustodianRequest } from '@/http/custodian/custodianApi';
import { relativeAge } from './utils';

interface AttestorsModalProps {
  request: CustodianRequest;
  onClose: () => void;
}

export function AttestorsModal({ request, onClose }: AttestorsModalProps): React.ReactPortal | null {
  if (typeof document === 'undefined') return null;
  return createPortal(
    <div
      onClick={onClose}
      className="fixed inset-[0] z-[9999] flex items-center justify-center bg-[rgba(0,0,0,0.55)] backdrop-blur-[2px]"
    >
      <div
        onClick={event => event.stopPropagation()}
        className="mx-[16px] w-full max-w-[460px] bg-[var(--putih)] shadow-[0_32px_80px_rgba(0,0,0,0.28)]"
      >
        <div className="flex items-start justify-between border-b border-[var(--hairline)] px-[24px] py-[20px]">
          <div>
            <div className="eyebrow mb-[4px] !text-[var(--body)]">Multisig attestors</div>
            <div className="text-[15px] font-bold text-[var(--ink)] [font-family:var(--font-inter,_Inter,_sans-serif)]">REQ-{request.on_chain_id} · {request.ticker}</div>
          </div>
          <button onClick={onClose} className="mt-[2px] cursor-pointer border-0 bg-transparent pb-[0] pl-[16px] pr-[0] pt-[0] text-[22px] leading-none text-[var(--body)]">&times;</button>
        </div>

        <div className="flex gap-[20px] border-b border-[var(--hairline)] bg-[var(--canvas-soft)] px-[24px] py-[10px]">
          <span className="mono text-[12px] text-[var(--body)]">
            <span className="font-bold text-[#16a34a]">{request.approval_count}</span> approve
          </span>
          <span className="mono text-[12px] text-[var(--body)]">
            <span className="font-bold text-[#ef4444]">{request.reject_count}</span> reject
          </span>
          <span className="mono ml-auto text-[12px] text-[var(--body)]">threshold 3/5</span>
        </div>

        <div className="max-h-[300px] overflow-y-auto">
          {(!request.attestors || request.attestors.length === 0) ? (
            <div className="px-[24px] py-[28px] text-center text-[13px] text-[var(--body)]">No attestations yet</div>
          ) : (
            request.attestors.map((attestor: AttestorInfo, i: number) => (
              <div key={i} className={`flex items-center justify-between px-[24px] py-[14px] ${i < request.attestors!.length - 1 ? "border-b border-[var(--hairline)]" : ""}`}>
                <div>
                  <div className="text-[13px] font-semibold text-[var(--ink)] [font-family:var(--font-inter,_Inter,_sans-serif)]">{attestor.name}</div>
                  <div className="mono mt-[3px] text-[11px] text-[var(--body)]">
                    {attestor.wallet_address.slice(0, 6)}…{attestor.wallet_address.slice(-4)}
                  </div>
                </div>
                <div className="flex items-center gap-[14px]">
                  <span className={`eyebrow ${attestor.type === 'approve' ? "!text-[#16a34a]" : "!text-[#ef4444]"}`}>{attestor.type}</span>
                  <span className="mono text-[11px] text-[var(--body)]">{relativeAge(attestor.attested_at ?? '')}</span>
                </div>
              </div>
            ))
          )}
        </div>

        <div className="border-t border-[var(--hairline)] px-[24px] py-[16px]">
          <button onClick={onClose} className="btn btn-primary !w-full !p-[12px] !text-[11px] !font-bold !uppercase !tracking-[0.08em]">
            Close
          </button>
        </div>
      </div>
    </div>,
    document.body
  );
}

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
  const approvals = request.approval_count;
  return createPortal(
    <div
      onClick={onClose}
      className="overlay fixed inset-[0] z-[9999] flex items-center justify-center bg-[rgba(22,17,14,0.32)] p-[16px]"
    >
      <div
        onClick={event => event.stopPropagation()}
        className="rise paper-sheaf w-full max-w-[460px]"
      >
        <div className="sheet">
          <div className="flex items-start justify-between border-b border-[var(--hairline)] px-[24px] py-[20px]">
            <div>
              <div className="mb-[4px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">Multisig attestors</div>
              <div className="display !text-[20px] !font-[500] !leading-[normal]">REQ-{request.on_chain_id} · {request.ticker}</div>
            </div>
            <button
              onClick={onClose}
              aria-label="Close"
              className="h-[30px] w-[30px] cursor-pointer appearance-none border border-[var(--hairline)] bg-[var(--putih)] text-[13px] text-[var(--ink)]"
            >
              ✕
            </button>
          </div>

          <div className="mono flex gap-[20px] border-b border-[var(--hairline)] bg-[var(--canvas-soft)] px-[24px] py-[10px] text-[12px] leading-[normal] text-[var(--body)]">
            <span><b className="text-[var(--positive)]">{request.approval_count}</b> approve</span>
            <span><b className="text-[var(--merah)]">{request.reject_count}</b> reject</span>
            <span className="ml-auto">threshold 3/5</span>
          </div>

          <div className="flex gap-[6px] border-b border-[var(--hairline)] px-[24px] py-[14px]">
            {Array.from({ length: 5 }, (_, segmentIndex) => {
              const isApproved = segmentIndex < approvals;
              const isNextNeeded = !isApproved && segmentIndex === approvals && approvals < 3;
              return (
                <span
                  key={segmentIndex}
                  className={`h-[8px] flex-1 ${isApproved ? "bg-[var(--positive)]" : isNextNeeded ? "bg-[var(--hairline)] outline outline-1 -outline-offset-1 outline-dashed outline-[var(--positive)]" : "bg-[var(--canvas-soft)]"}`}
                />
              );
            })}
          </div>

          <div className="max-h-[300px] overflow-y-auto">
            {(!request.attestors || request.attestors.length === 0) ? (
              <div className="px-[24px] py-[28px] text-center text-[13px] text-[var(--body)]">No attestations yet</div>
            ) : (
              request.attestors.map((attestor: AttestorInfo, i: number) => (
                <div key={i} className={`flex items-center justify-between px-[24px] py-[14px] ${i < request.attestors!.length - 1 ? "border-b border-[var(--hairline)]" : ""}`}>
                  <div>
                    <div className="text-[13px] font-[600] leading-[normal] text-[var(--ink)]">{attestor.name}</div>
                    <div className="mono mt-[3px] text-[11px] leading-[normal] text-[var(--body)]">
                      {attestor.wallet_address.slice(0, 6)}…{attestor.wallet_address.slice(-4)}
                    </div>
                  </div>
                  <div className="flex items-center gap-[14px]">
                    <span className={`text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] ${attestor.type === 'approve' ? "text-[var(--positive)]" : "text-[var(--merah)]"}`}>{attestor.type}</span>
                    <span className="mono text-[11px] leading-[normal] text-[var(--body)]">{relativeAge(attestor.attested_at ?? '')}</span>
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="border-t border-[var(--hairline)] px-[24px] py-[16px]">
            <button onClick={onClose} className="w-full cursor-pointer appearance-none border-0 bg-[var(--ink)] p-[12px] text-[11px] font-[700] uppercase tracking-[0.08em] text-[var(--putih)]">
              Close
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body
  );
}

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
      style={{ position: 'fixed', inset: 0, zIndex: 9999, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(0,0,0,0.55)', backdropFilter: 'blur(2px)' }}
    >
      <div
        onClick={event => event.stopPropagation()}
        style={{ background: 'var(--putih)', width: '100%', maxWidth: 460, margin: '0 16px', boxShadow: '0 32px 80px rgba(0,0,0,0.28)' }}
      >
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', padding: '20px 24px', borderBottom: '1px solid var(--hairline)' }}>
          <div>
            <div className="eyebrow" style={{ color: 'var(--body)', marginBottom: 4 }}>Multisig attestors</div>
            <div style={{ fontSize: 15, fontWeight: 700, fontFamily: 'Inter', color: 'var(--ink)' }}>REQ-{request.on_chain_id} · {request.ticker}</div>
          </div>
          <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: 22, color: 'var(--body)', lineHeight: 1, padding: '0 0 0 16px', marginTop: 2 }}>&times;</button>
        </div>

        <div style={{ display: 'flex', gap: 20, padding: '10px 24px', background: 'var(--canvas-soft)', borderBottom: '1px solid var(--hairline)' }}>
          <span className="mono" style={{ fontSize: 12, color: 'var(--body)' }}>
            <span style={{ color: '#16a34a', fontWeight: 700 }}>{request.approval_count}</span> approve
          </span>
          <span className="mono" style={{ fontSize: 12, color: 'var(--body)' }}>
            <span style={{ color: '#ef4444', fontWeight: 700 }}>{request.reject_count}</span> reject
          </span>
          <span className="mono" style={{ fontSize: 12, color: 'var(--body)', marginLeft: 'auto' }}>threshold 3/5</span>
        </div>

        <div style={{ maxHeight: 300, overflowY: 'auto' }}>
          {(!request.attestors || request.attestors.length === 0) ? (
            <div style={{ textAlign: 'center', color: 'var(--body)', fontSize: 13, padding: '28px 24px' }}>No attestations yet</div>
          ) : (
            request.attestors.map((attestor: AttestorInfo, i: number) => (
              <div key={i} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 24px', borderBottom: i < request.attestors!.length - 1 ? '1px solid var(--hairline)' : 'none' }}>
                <div>
                  <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)', fontFamily: 'Inter' }}>{attestor.name}</div>
                  <div className="mono" style={{ fontSize: 11, color: 'var(--body)', marginTop: 3 }}>
                    {attestor.wallet_address.slice(0, 6)}…{attestor.wallet_address.slice(-4)}
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                  <span className="eyebrow" style={{ color: attestor.type === 'approve' ? '#16a34a' : '#ef4444' }}>{attestor.type}</span>
                  <span className="mono" style={{ fontSize: 11, color: 'var(--body)' }}>{relativeAge(attestor.attested_at ?? '')}</span>
                </div>
              </div>
            ))
          )}
        </div>

        <div style={{ padding: '16px 24px', borderTop: '1px solid var(--hairline)' }}>
          <button onClick={onClose} className="btn btn-primary" style={{ width: '100%', padding: 12, fontSize: 11, fontWeight: 700, letterSpacing: '0.08em', textTransform: 'uppercase' }}>
            Close
          </button>
        </div>
      </div>
    </div>,
    document.body
  );
}

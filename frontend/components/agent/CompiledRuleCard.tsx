'use client';

import { useState } from 'react';

type CompiledRuleCardProps = {
  lockedSummary: string;
  bands: { label: string; value: string }[];
};

// Pixel-matched to Agent Chat.dc.html's "locked" summary + "The rule"
// collapsible card. Always renders "fully autonomous, no signature per
// fill" — this is not configurable per Task, so it's a literal, not a
// prop, to make it impossible for a future edit to accidentally introduce
// per-fill wording.
export function CompiledRuleCard({ lockedSummary, bands }: CompiledRuleCardProps) {
  const [open, setOpen] = useState(true);

  return (
    <>
      <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--ink)', background: 'var(--putih)', padding: '13px 15px' }}>
        <div style={{ fontSize: 14.5, lineHeight: 1.55 }}>{lockedSummary}</div>
      </div>

      <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
        <button
          onClick={() => setOpen((v) => !v)}
          style={{ width: '100%', appearance: 'none', border: 0, cursor: 'pointer', background: 'var(--canvas-soft)', padding: '11px 13px', textAlign: 'left', display: 'flex', alignItems: 'center', gap: 9 }}
        >
          <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>The rule</span>
          <span style={{ marginLeft: 'auto', fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)' }}>{open ? 'hide' : 'show'}</span>
        </button>
        {open && (
          <div>
            {bands.map((band) => (
              <div key={band.label} style={{ borderTop: '1px solid var(--hairline)', padding: 13 }}>
                <div style={{ font: '600 9px/1.3 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase', color: 'var(--body)', marginBottom: 3 }}>{band.label}</div>
                <div style={{ fontFamily: 'var(--font-mono)', fontSize: 12, lineHeight: 1.45 }}>{band.value}</div>
              </div>
            ))}
            <div style={{ borderTop: '1px solid var(--hairline)', padding: 13 }}>
              <div style={{ font: '700 9.5px/1 var(--font-sans)', letterSpacing: '.16em', textTransform: 'uppercase', color: 'var(--merah)', marginBottom: 9 }}>Governance</div>
              <div style={{ padding: '6px 0', borderTop: '1px solid var(--canvas-soft)' }}>
                <div style={{ font: '600 9px/1.3 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase', color: 'var(--body)', marginBottom: 3 }}>Approval</div>
                <div style={{ fontFamily: 'var(--font-mono)', fontSize: 12, lineHeight: 1.45 }}>fully autonomous, no signature per fill</div>
              </div>
              <div style={{ padding: '6px 0', borderTop: '1px solid var(--canvas-soft)' }}>
                <div style={{ font: '600 9px/1.3 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase', color: 'var(--body)', marginBottom: 3 }}>Your gate</div>
                <div style={{ fontFamily: 'var(--font-mono)', fontSize: 12, lineHeight: 1.45 }}>creating this Task — asked, never assumed</div>
              </div>
            </div>
          </div>
        )}
      </div>
    </>
  );
}

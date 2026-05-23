'use client';

import { Wordmark } from '@/components/ui/Wordmark';

export function Footer() {
  return (
    <div style={{ background: "var(--ink)", color: "var(--putih)", padding: "48px 24px 32px", marginTop: 48 }}>
      <div className="footer-grid" style={{ display: "grid", gridTemplateColumns: "2fr 1fr 1fr 1fr", gap: 32, alignItems: "start" }}>
        <div>
          <Wordmark size={28} />
          <div style={{ marginTop: 12, color: "rgba(255,255,255,0.65)", fontSize: 13, maxWidth: 380, lineHeight: 1.6 }}>
            A 24/7 automated market maker for 1:1 tokenized Indonesian equities. Settle in seconds, not days. Built on Arbitrum, custody-secured by Horizon Labs.
          </div>
        </div>
        <div>
          <div className="eyebrow" style={{ color: "rgba(255,255,255,0.7)", marginBottom: 12 }}>Protocol</div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6, fontSize: 13 }}>
            <span>Whitepaper</span><span>Tokenomics</span><span>Audits</span><span>GitHub</span>
          </div>
        </div>
        <div>
          <div className="eyebrow" style={{ color: "rgba(255,255,255,0.7)", marginBottom: 12 }}>Bridge</div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6, fontSize: 13 }}>
            <span>Custodian Reports</span><span>Peg Status</span><span>Mint / Burn</span><span>Reserves</span>
          </div>
        </div>
        <div>
          <div className="eyebrow" style={{ color: "rgba(255,255,255,0.7)", marginBottom: 12 }}>Legal</div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6, fontSize: 13 }}>
            <span>Terms</span><span>Risk Disclosures</span><span>Geographic Restrictions</span>
          </div>
        </div>
      </div>
      <div style={{ marginTop: 36, paddingTop: 20, borderTop: "1px solid rgba(255,255,255,0.18)", display: "flex", justifyContent: "space-between", color: "rgba(255,255,255,0.55)", fontSize: 12, flexWrap: "wrap", gap: 8 }}>
        <span>© 2026 Horizon Labs · Arbitrum Buildathon</span>
        <span>v0.4.1 — Sepolia Testnet</span>
      </div>
    </div>
  );
}

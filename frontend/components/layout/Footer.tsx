'use client';

import { Wordmark } from '@/components/ui/Wordmark';

export function Footer() {
  return (
    <div className="bg-[var(--ink)] text-[var(--putih)] px-[24px] pt-[48px] pb-[32px] mt-[48px]">
      <div className="footer-grid grid grid-cols-[2fr_1fr_1fr_1fr] gap-[32px] items-start">
        <div>
          <Wordmark size={28} />
          <div className="mt-[12px] text-[rgba(255,255,255,0.65)] text-[13px] max-w-[380px] leading-[1.6]">
            A 24/7 automated market maker for 1:1 tokenized Indonesian equities. Settle in seconds, not days. Built on Arbitrum, custody-secured by Horizon Labs.
          </div>
        </div>
        <div>
          <div className="eyebrow text-[rgba(255,255,255,0.7)] mb-[12px]">Protocol</div>
          <div className="flex flex-col gap-[6px] text-[13px]">
            <span>Whitepaper</span><span>Tokenomics</span><span>Audits</span><span>GitHub</span>
          </div>
        </div>
        <div>
          <div className="eyebrow text-[rgba(255,255,255,0.7)] mb-[12px]">Bridge</div>
          <div className="flex flex-col gap-[6px] text-[13px]">
            <span>Custodian Reports</span><span>Peg Status</span><span>Mint / Burn</span><span>Reserves</span>
          </div>
        </div>
        <div>
          <div className="eyebrow text-[rgba(255,255,255,0.7)] mb-[12px]">Legal</div>
          <div className="flex flex-col gap-[6px] text-[13px]">
            <span>Terms</span><span>Risk Disclosures</span><span>Geographic Restrictions</span>
          </div>
        </div>
      </div>
      <div className="mt-[36px] pt-[20px] border-t border-[rgba(255,255,255,0.18)] flex justify-between text-[rgba(255,255,255,0.55)] text-[12px] flex-wrap gap-[8px]">
        <span>© 2026 Horizon Labs · Arbitrum Buildathon</span>
        <span>v0.4.1 — Sepolia Testnet</span>
      </div>
    </div>
  );
}

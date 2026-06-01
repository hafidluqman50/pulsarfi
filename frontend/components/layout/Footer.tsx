'use client';

import Image from 'next/image';

export function Footer() {
  return (
    <div className="bg-[var(--ink)] text-[var(--putih)] px-[24px] pt-[48px] pb-[32px] mt-[48px]">
      <div className="footer-grid grid grid-cols-[2fr_1fr_1fr_1fr] gap-[32px] items-start">
        <div>
          <div className="flex items-center gap-[0px]">
            <Image src="/logo-only-nobg.png" alt="" width={52} height={30} className="block" style={{ marginRight: -8 }} />
            <span className="display" style={{ fontSize: 28, fontWeight: 500, letterSpacing: '-0.025em', color: 'inherit' }}>
              PulsarF<span style={{ position: 'relative', display: 'inline-block' }}>
                i
                <span style={{ position: 'absolute', top: 2, left: '50%', transform: 'translateX(-50%)', width: 4, height: 4, borderRadius: '50%', background: 'var(--merah)', display: 'block' }} />
              </span>
            </span>
          </div>
          <div className="mt-[12px] text-[rgba(255,255,255,0.65)] text-[13px] max-w-[380px] leading-[1.6]">
            A 24/7 automated market maker for 1:1 tokenized Indonesian equities. Settle in seconds, not days. Built on Arbitrum, custody-secured by licensed custodians.
          </div>
        </div>
        <div>
          <div className="eyebrow !text-[var(--putih)] font-bold mb-[12px]">Protocol</div>
          <div className="flex flex-col gap-[6px] text-[13px]">
            <span>Whitepaper</span><span>Tokenomics</span><span>Audits</span><span>GitHub</span>
          </div>
        </div>
        <div>
          <div className="eyebrow !text-[var(--putih)] font-bold mb-[12px]">Bridge</div>
          <div className="flex flex-col gap-[6px] text-[13px]">
            <span>Custodian Reports</span><span>Peg Status</span><span>Mint / Burn</span><span>Reserves</span>
          </div>
        </div>
        <div>
          <div className="eyebrow !text-[var(--putih)] font-bold mb-[12px]">Legal</div>
          <div className="flex flex-col gap-[6px] text-[13px]">
            <span>Terms</span><span>Risk Disclosures</span><span>Geographic Restrictions</span>
          </div>
        </div>
      </div>
      <div className="mt-[36px] pt-[20px] border-t border-[rgba(255,255,255,0.18)] flex justify-between text-[rgba(255,255,255,0.55)] text-[12px] flex-wrap gap-[8px]">
        <span>© 2026 PulsarFi · Arbitrum Buildathon</span>
        <span>v0.4.1 | Sepolia Testnet</span>
      </div>
    </div>
  );
}

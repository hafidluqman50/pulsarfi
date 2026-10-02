'use client';

import type { ReserveEntry } from '@/http/custodian/custodianApi';
import { formatRawToken, relativeAge } from '@/components/custodian/utils';
import { ISO_PALETTES, IsoBar } from '@/components/ui/IsoBar';
import { rawTokenToNumber } from '@/lib/stockDetail';
import { useViewportWidth } from '@/lib/useViewportWidth';

const MOBILE_BREAKPOINT = 720;
const MAX_BAR_HEIGHT = 130;
const MIN_BAR_HEIGHT = 6;
const BAR_FOOTPRINT = 60;

const CUSTODY_PALETTE = ISO_PALETTES[1];
const SUPPLY_PALETTE = ISO_PALETTES[0];

function barHeight(value: number, largest: number) {
  if (largest <= 0) return MIN_BAR_HEIGHT;
  return Math.max(MIN_BAR_HEIGHT, Math.round((MAX_BAR_HEIGHT * value) / largest));
}

type ProofOfReserveProps = {
  entry?: ReserveEntry;
  isLoading: boolean;
};

export function ProofOfReserve({ entry, isLoading }: ProofOfReserveProps): React.ReactNode {
  const viewportWidth = useViewportWidth();
  const stageScale = viewportWidth < MOBILE_BREAKPOINT ? Math.min(1, (viewportWidth - 32) / 420) : 1;

  if (isLoading && !entry) {
    return (
      <div className="paper-stack p-[20px]">
        <div className="h-[11px] w-[120px] bg-[var(--canvas-soft)]" />
        <div className="mt-[10px] h-[44px] w-full bg-[var(--canvas-soft)]" />
        <div className="mt-[10px] h-[230px] w-full bg-[var(--canvas-soft)]" />
      </div>
    );
  }

  if (!entry) {
    return (
      <div className="paper-stack p-[20px]">
        <div className="text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--merah)]">Proof of reserve</div>
        <div className="mt-[10px] text-[14px] text-[var(--body)]">Reserve data unavailable</div>
      </div>
    );
  }

  const custodyAmount = rawTokenToNumber(entry.custodian_holdings) ?? 0;
  const supplyAmount = rawTokenToNumber(entry.on_chain_supply) ?? 0;
  const largest = Math.max(custodyAmount, supplyAmount);
  const isPegged = entry.peg_status === 'pegged';
  const attestedAge = relativeAge(entry.last_attested_at);

  return (
    <div className="paper-stack p-[20px]">
      <div className="text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--merah)]">Proof of reserve</div>
      <div className="mt-[6px] text-[22px] font-[400] leading-[1.2] [font-family:var(--font-fraunces,_Fraunces,_serif)]">
        {isPegged ? (
          <>Shares in custody match <em className="display-it">tokens on-chain</em>.</>
        ) : (
          <>Shares in custody and tokens on-chain are <em className="display-it">out of sync</em>.</>
        )}
      </div>
      <div className="mt-[10px] grid h-[230px] place-items-center [perspective:1000px]">
        <div
          style={{
            position: 'relative', width: 220, height: 80, transformStyle: 'preserve-3d',
            transform: `scale(${stageScale}) rotateX(58deg) rotateZ(-38deg)`,
          }}
        >
          <IsoBar x={20} y={10} width={BAR_FOOTPRINT} depth={BAR_FOOTPRINT} height={barHeight(custodyAmount, largest)} palette={CUSTODY_PALETTE} />
          <IsoBar x={120} y={10} width={BAR_FOOTPRINT} depth={BAR_FOOTPRINT} height={barHeight(supplyAmount, largest)} palette={SUPPLY_PALETTE} />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-[12px] border-t border-[var(--hairline)] pt-[12px]">
        <div>
          <div className="flex items-center gap-[6px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">
            <span className="h-[10px] w-[10px] bg-[var(--ink)]" />
            Shares in custody
          </div>
          <div className="mono mt-[4px] text-[14px] font-[600] leading-[normal]">{formatRawToken(entry.custodian_holdings)}</div>
        </div>
        <div>
          <div className="flex items-center gap-[6px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">
            <span className="h-[10px] w-[10px] bg-[var(--merah)]" />
            pStock supply
          </div>
          <div className="mono mt-[4px] text-[14px] font-[600] leading-[normal]">{formatRawToken(entry.on_chain_supply)}</div>
        </div>
      </div>
      <div className="mono mt-[12px] text-[11px] leading-[normal] text-[var(--body)]">
        Last attested {attestedAge.endsWith('ago') ? attestedAge : `${attestedAge} ago`}
      </div>
    </div>
  );
}

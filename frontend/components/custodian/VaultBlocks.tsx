'use client';

import type { ReserveEntry } from '@/http/custodian/custodianApi';
import { ISO_PALETTES, IsoBar } from '@/components/ui/IsoBar';
import { PStockMark } from '@/components/ui/PStockMark';
import { rawTokenToNumber } from '@/lib/stockDetail';
import { useViewportWidth } from '@/lib/useViewportWidth';

const MOBILE_BREAKPOINT = 720;
const COLUMNS = 4;
const COLUMN_STEP = 86;
const ROW_STEP = 90;
const FOOTPRINT = 60;
const MIN_HEIGHT = 20;
const HEIGHT_RANGE = 110;

export function VaultBlocks({ entries }: { entries: ReserveEntry[] }) {
  const viewportWidth = useViewportWidth();
  const stageScale = viewportWidth < MOBILE_BREAKPOINT ? Math.min(1, (viewportWidth - 32) / 420) : 1;
  const custody = entries.map(entry => rawTokenToNumber(entry.custodian_holdings) ?? 0);
  const largest = Math.max(0, ...custody);

  return (
    <div className="grid h-[320px] place-items-center overflow-hidden [perspective:1300px]">
      <div
        style={{
          position: 'relative', width: 340, height: 160, marginTop: 60, transformStyle: 'preserve-3d',
          transform: `scale(${stageScale}) rotateX(58deg) rotateZ(-36deg)`,
        }}
      >
        <div
          style={{
            position: 'absolute', left: -22, top: -22, right: -22, bottom: -22, background: '#16110e',
            backgroundImage: 'linear-gradient(#2a231e 1px,transparent 1px),linear-gradient(90deg,#2a231e 1px,transparent 1px)',
            backgroundSize: '20px 20px',
          }}
        />
        {entries.map((entry, index) => (
          <IsoBar
            key={entry.stock.ticker}
            x={(index % COLUMNS) * COLUMN_STEP}
            y={Math.floor(index / COLUMNS) * ROW_STEP}
            width={FOOTPRINT}
            depth={FOOTPRINT}
            height={MIN_HEIGHT + (largest > 0 ? (custody[index] / largest) * HEIGHT_RANGE : 0)}
            palette={index === 0 ? ISO_PALETTES[0] : ISO_PALETTES[4]}
            topContent={<PStockMark ticker={entry.stock.ticker} size={26} />}
          />
        ))}
      </div>
    </div>
  );
}

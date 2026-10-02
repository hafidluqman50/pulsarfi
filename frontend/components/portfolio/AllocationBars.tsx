'use client';

import { ISO_PALETTES, IsoBar } from '@/components/ui/IsoBar';
import { useViewportWidth } from '@/lib/useViewportWidth';

const MOBILE_BREAKPOINT = 720;
const SCENE_WIDTH = 360;
const MAX_FOOTPRINT = 46;
const MAX_STEP = 74;
const MAX_BAR_HEIGHT = 150;
const MIN_BAR_HEIGHT = 6;

export interface AllocationItem {
  label: string;
  value: number;
}

export function AllocationBars({ items, total }: { items: AllocationItem[]; total: number }) {
  const viewportWidth = useViewportWidth();
  const stageScale = viewportWidth < MOBILE_BREAKPOINT ? Math.min(1, (viewportWidth - 32) / 420) : 1;
  const largest = Math.max(0, ...items.map(item => item.value));
  const step = items.length > 1 ? Math.min(MAX_STEP, (SCENE_WIDTH - MAX_FOOTPRINT) / (items.length - 1)) : 0;
  const footprint = items.length > 1 ? Math.min(MAX_FOOTPRINT, step - 4) : MAX_FOOTPRINT;

  return (
    <div>
      <div className="text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--body)]">Allocation</div>
      <div className="grid h-[250px] place-items-center overflow-hidden [perspective:1200px]">
        <div
          style={{
            position: 'relative', width: SCENE_WIDTH, height: 60, marginTop: 80, transformStyle: 'preserve-3d',
            transform: `scale(${stageScale}) rotateX(58deg) rotateZ(-32deg)`,
          }}
        >
          <div
            style={{
              position: 'absolute', left: -24, top: -24, right: -24, bottom: -24, background: '#f3f0ea', border: '1px solid #e3ddd2',
              backgroundImage: 'linear-gradient(#e3ddd2 1px,transparent 1px),linear-gradient(90deg,#e3ddd2 1px,transparent 1px)',
              backgroundSize: '18px 18px',
            }}
          />
          {items.map((item, index) => (
            <IsoBar
              key={item.label}
              x={index * step}
              y={8}
              width={footprint}
              depth={footprint}
              height={largest > 0 ? Math.max(MIN_BAR_HEIGHT, Math.round((item.value / largest) * MAX_BAR_HEIGHT)) : MIN_BAR_HEIGHT}
              palette={ISO_PALETTES[index % ISO_PALETTES.length]}
            />
          ))}
        </div>
      </div>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-x-[20px] gap-y-[8px]">
        {items.map((item, index) => (
          <div key={item.label} className="flex items-center gap-[8px]">
            <span
              className="h-[10px] w-[10px] shrink-0 outline outline-1 outline-[rgba(22,17,14,0.15)]"
              style={{ background: ISO_PALETTES[index % ISO_PALETTES.length].top }}
            />
            <span className="text-[13px] font-[600]">{item.label}</span>
            <span className="mono ml-auto text-[12px] text-[var(--body)]">{total ? ((item.value / total) * 100).toFixed(1) : '0.0'}%</span>
          </div>
        ))}
      </div>
    </div>
  );
}

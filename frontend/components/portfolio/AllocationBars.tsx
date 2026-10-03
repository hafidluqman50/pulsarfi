'use client';

import { Fragment, useRef, useState, type CSSProperties, type PointerEvent } from 'react';
import { ISO_PALETTES } from '@/components/ui/IsoBar';
import { PStockMark } from '@/components/ui/PStockMark';
import { useViewportWidth } from '@/lib/useViewportWidth';

const MOBILE_BREAKPOINT = 720;
const SCENE_WIDTH = 376;
const MAX_COIN_SIZE = 46;
const MAX_STEP = 66;
const STACK_TOP = 8;
const MAX_COINS = 14;
const COIN_PITCH = 8;
const DEFAULT_ANGLE = -32;
const DRAG_FACTOR = 0.45;
const INK = '#16110e';

export interface AllocationItem {
  label: string;
  value: number;
}

function sceneTransform(scale: number, angle: number) {
  return `scale(${scale}) rotateX(58deg) rotateZ(${angle}deg)`;
}

const WALL: CSSProperties = { position: 'absolute', border: `1px solid ${INK}`, boxSizing: 'border-box' };

export function AllocationBars({ items, total }: { items: AllocationItem[]; total: number }) {
  const viewportWidth = useViewportWidth();
  const stageScale = viewportWidth < MOBILE_BREAKPOINT ? Math.min(1, (viewportWidth - 32) / 420) : 1;
  const [angle, setAngle] = useState(DEFAULT_ANGLE);
  const sceneRef = useRef<HTMLDivElement>(null);
  const drag = useRef<{ startX: number; startAngle: number; current: number } | null>(null);

  const largest = Math.max(0, ...items.map(item => item.value));
  const step = items.length > 1 ? Math.min(MAX_STEP, (SCENE_WIDTH - MAX_COIN_SIZE) / (items.length - 1)) : 0;
  const coinSize = items.length > 1 ? Math.min(MAX_COIN_SIZE, step - 4) : MAX_COIN_SIZE;
  const slotSize = coinSize + 12;
  const centerY = STACK_TOP + MAX_COIN_SIZE / 2;

  const handlePointerDown = (event: PointerEvent<HTMLDivElement>) => {
    drag.current = { startX: event.clientX, startAngle: angle, current: angle };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handlePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const active = drag.current;
    if (!active || !sceneRef.current) return;
    active.current = active.startAngle + (event.clientX - active.startX) * DRAG_FACTOR;
    sceneRef.current.style.transform = sceneTransform(stageScale, active.current);
  };

  const handlePointerUp = () => {
    const active = drag.current;
    drag.current = null;
    if (active && active.current !== active.startAngle) setAngle(active.current);
  };

  return (
    <div>
      <div className="text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--body)]">Allocation</div>
      <div
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerUp}
        className="grid h-[270px] place-items-center overflow-hidden [perspective:1200px]"
        style={{ cursor: 'grab', touchAction: 'pan-y', userSelect: 'none' }}
      >
        <div
          ref={sceneRef}
          style={{
            position: 'relative', width: SCENE_WIDTH, height: 60, marginTop: 70, transformStyle: 'preserve-3d',
            transform: sceneTransform(stageScale, angle),
          }}
        >
          <div style={{ position: 'absolute', left: -26, top: -30, width: 428, height: 120, background: '#c9bfae', transform: 'translateZ(-18px)' }} />
          <div style={{ ...WALL, left: -26, top: -48, width: 428, height: 18, background: '#e3ddd2', transformOrigin: '50% 100%', transform: 'translateZ(-18px) rotateX(-90deg)' }} />
          <div
            style={{
              ...WALL, left: -26, top: 72, width: 428, height: 18, background: '#e3ddd2',
              backgroundImage: 'linear-gradient(#c8102e,#c8102e)', backgroundSize: '100% 3px', backgroundPosition: '0 5px', backgroundRepeat: 'no-repeat',
              transformOrigin: '50% 100%', transform: 'translateZ(-18px) rotateX(-90deg)',
            }}
          />
          <div style={{ ...WALL, left: -44, top: -30, width: 18, height: 120, background: '#d6cfc1', transformOrigin: '100% 50%', transform: 'translateZ(-18px) rotateY(90deg)' }} />
          <div style={{ ...WALL, left: 384, top: -30, width: 18, height: 120, background: '#d6cfc1', transformOrigin: '100% 50%', transform: 'translateZ(-18px) rotateY(90deg)' }} />
          <div style={{ position: 'absolute', left: -26, top: -30, width: 428, height: 120, boxSizing: 'border-box', background: INK, border: `1px solid ${INK}`, padding: 7 }}>
            <div style={{ width: '100%', height: '100%', boxSizing: 'border-box', background: '#f3efe7', border: '1px solid #5a4a3a' }} />
          </div>

          {items.map((item, index) => {
            const centerX = index * step + MAX_COIN_SIZE / 2;
            return (
              <Fragment key={`slot-${item.label}`}>
                <div
                  style={{
                    position: 'absolute', left: centerX - slotSize / 2, top: centerY - slotSize / 2, width: slotSize, height: slotSize, boxSizing: 'border-box',
                    borderRadius: '50%', border: '1.5px dashed #a39988', background: '#ebe6dc', transform: 'translateZ(.3px)',
                  }}
                />
                <div
                  style={{
                    position: 'absolute', left: centerX - MAX_COIN_SIZE / 2, top: 62, width: MAX_COIN_SIZE, textAlign: 'center',
                    font: '700 8px var(--font-sans)', letterSpacing: '.06em', color: INK, transform: 'translateZ(.3px)',
                  }}
                >
                  {item.label}
                </div>
              </Fragment>
            );
          })}

          {items.map((item, index) => {
            const palette = ISO_PALETTES[index % ISO_PALETTES.length];
            const coinCount = largest > 0 ? Math.max(1, Math.round((item.value / largest) * MAX_COINS)) : 1;
            const centerX = index * step + MAX_COIN_SIZE / 2;
            return (
              <div
                key={`stack-${item.label}`}
                style={{ position: 'absolute', left: centerX - coinSize / 2, top: centerY - coinSize / 2, width: coinSize, height: coinSize, transformStyle: 'preserve-3d' }}
              >
                <div
                  style={{
                    position: 'absolute', left: -(70 - coinSize) / 2, top: -(70 - coinSize) / 2, width: 70, height: 70, borderRadius: '50%',
                    background: 'radial-gradient(closest-side,rgba(22,17,14,.22),transparent)', transform: 'translateZ(.5px)',
                  }}
                />
                {Array.from({ length: coinCount }, (_, coinIndex) => {
                  const base = coinIndex * COIN_PITCH;
                  return (
                    <Fragment key={coinIndex}>
                      <div style={{ position: 'absolute', inset: 0, borderRadius: '50%', background: '#a39988', transform: `translateZ(${base}px)` }} />
                      <div style={{ position: 'absolute', inset: 0, borderRadius: '50%', background: palette.top, transform: `translateZ(${base + 2.5}px)` }} />
                      <div style={{ position: 'absolute', inset: 0, borderRadius: '50%', background: '#d6cfc1', transform: `translateZ(${base + 5}px)` }} />
                      <div
                        style={{
                          position: 'absolute', inset: 0, borderRadius: '50%', background: '#fff', border: `1.5px solid ${INK}`,
                          boxShadow: `inset 0 0 0 3px #fff,inset 0 0 0 4px ${palette.top}`, display: 'grid', placeItems: 'center',
                          transform: `translateZ(${base + 7.5}px)`,
                        }}
                      >
                        {coinIndex === coinCount - 1 && (
                          <div style={{ pointerEvents: 'none' }}>
                            <PStockMark ticker={item.label} size={Math.round(coinSize * 0.52)} />
                          </div>
                        )}
                      </div>
                    </Fragment>
                  );
                })}
              </div>
            );
          })}
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

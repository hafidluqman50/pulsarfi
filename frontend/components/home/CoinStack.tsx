'use client';

import { Fragment, useEffect, useMemo, useRef, useSyncExternalStore, type PointerEvent } from 'react';
import { PStockMark } from '@/components/ui/PStockMark';
import { useReserves } from '@/http/custodian/hooks';
import { useMarketStocks } from '@/http/market/hooks';
import { fmtPct } from '@/lib/data';
import { rawTokenToNumber } from '@/lib/stockDetail';

const SLOTS = [[110, 90], [300, 80], [210, 190], [360, 220], [80, 230]] as const;
const EDGE_COLORS = ['#16110e', '#c8102e', '#1f4d8a', '#9a0c24', '#5a4a3a'] as const;
const COIN_HEIGHT = 11;
const EDGE_LAYERS = 8;
const MAX_COINS = 8;
const STAGE_HEIGHT = 300;
const STAGE_WIDTH = 480;

interface CoinStackData {
  ticker: string;
  coinCount: number;
  stackHeight: number;
  slotX: number;
  slotY: number;
  edgeColor: string;
  changeLabel: string;
  changeColor: string;
}

interface CoinStageProps {
  stacks: CoinStackData[];
  autoRotate: boolean;
  scale: number;
}

function CoinStage({ stacks, autoRotate, scale }: CoinStageProps) {
  const sceneRef = useRef<HTMLDivElement>(null);
  const labelRefs = useRef<(HTMLDivElement | null)[]>([]);
  const motion = useRef({ angle: -28, dragging: false, lastX: 0, velocity: 0 });
  const scaleRef = useRef(scale);

  useEffect(() => {
    scaleRef.current = scale;
  }, [scale]);

  useEffect(() => {
    let frameId = 0;
    const tick = () => {
      const state = motion.current;
      if (!state.dragging) {
        state.angle += (autoRotate ? 0.1 : 0) + state.velocity;
        state.velocity *= 0.94;
      }
      if (sceneRef.current) {
        sceneRef.current.style.transform = `scale(${scaleRef.current}) rotateX(60deg) rotateZ(${state.angle}deg)`;
      }
      labelRefs.current.forEach((labelElement, index) => {
        if (labelElement && stacks[index]) {
          labelElement.style.transform = `translate(-50%,-50%) translateZ(${stacks[index].stackHeight + 46}px) rotateZ(${-state.angle}deg) rotateX(-60deg)`;
        }
      });
      frameId = requestAnimationFrame(tick);
    };
    tick();
    return () => cancelAnimationFrame(frameId);
  }, [autoRotate, stacks]);

  const handlePointerDown = (event: PointerEvent<HTMLDivElement>) => {
    motion.current.dragging = true;
    motion.current.lastX = event.clientX;
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handlePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const state = motion.current;
    if (!state.dragging) return;
    const deltaX = event.clientX - state.lastX;
    state.lastX = event.clientX;
    state.angle += deltaX * 0.45;
    state.velocity = deltaX * 0.08;
  };

  const handlePointerUp = () => {
    motion.current.dragging = false;
  };

  return (
    <div
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerUp}
      style={{ width: '100%', height: '100%', display: 'grid', placeItems: 'center', perspective: 1400, cursor: 'grab', touchAction: 'none', userSelect: 'none' }}
    >
      <div ref={sceneRef} style={{ position: 'relative', width: 440, height: STAGE_HEIGHT, transformStyle: 'preserve-3d', marginTop: 40 }}>
        <div style={{ position: 'absolute', inset: 0, background: '#e3ddd2', transform: 'translateZ(-10px)' }} />
        <div style={{ position: 'absolute', inset: 0, background: '#efebe3', border: '1px solid #e3ddd2', transform: 'translateZ(-5px)' }} />
        <div style={{ position: 'absolute', inset: 0, background: '#f8f6f1', border: '1px solid #e3ddd2', backgroundImage: 'linear-gradient(#ebe6dc 1px,transparent 1px),linear-gradient(90deg,#ebe6dc 1px,transparent 1px)', backgroundSize: '24px 24px' }} />
        {stacks.map((stack, stackIndex) => (
          <Fragment key={stack.ticker}>
            <div style={{ position: 'absolute', left: stack.slotX - 70, top: stack.slotY - 70, width: 140, height: 140, borderRadius: '50%', background: 'radial-gradient(closest-side,rgba(22,17,14,.22),transparent)', transform: 'translateZ(0.5px)' }} />
            {Array.from({ length: stack.coinCount }, (_, coinIndex) => {
              const base = coinIndex * COIN_HEIGHT;
              return (
                <Fragment key={coinIndex}>
                  {Array.from({ length: EDGE_LAYERS }, (_, layerIndex) => (
                    <div
                      key={layerIndex}
                      style={{
                        position: 'absolute', left: stack.slotX - 48, top: stack.slotY - 48, width: 96, height: 96, borderRadius: '50%',
                        background: layerIndex < 2 ? '#a39988' : layerIndex % 3 === 0 ? stack.edgeColor : '#d6cfc1',
                        transform: `translateZ(${base + layerIndex * 1.3}px)`,
                      }}
                    />
                  ))}
                  <div
                    style={{
                      position: 'absolute', left: stack.slotX - 48, top: stack.slotY - 48, width: 96, height: 96, borderRadius: '50%',
                      background: '#fff', border: '2px solid #16110e',
                      boxShadow: `inset 0 0 0 5px #fff,inset 0 0 0 6px ${stack.edgeColor}`,
                      display: 'grid', placeItems: 'center', transform: `translateZ(${base + 10.5}px)`,
                    }}
                  >
                    {coinIndex === stack.coinCount - 1 && (
                      <div style={{ pointerEvents: 'none' }}>
                        <PStockMark ticker={stack.ticker} size={50} />
                      </div>
                    )}
                  </div>
                </Fragment>
              );
            })}
            <div
              ref={labelElement => { labelRefs.current[stackIndex] = labelElement; }}
              style={{ position: 'absolute', left: stack.slotX, top: stack.slotY, background: '#fff', border: '1px solid #16110e', padding: '5px 8px', whiteSpace: 'nowrap', boxShadow: '0 3px 0 -1px #fbfaf7,0 4px 0 -1px #16110e', pointerEvents: 'none' }}
            >
              <span style={{ fontSize: 11, fontWeight: 700, letterSpacing: '0.04em' }}>{stack.ticker}</span>{' '}
              <span className="mono" style={{ fontSize: 11, fontWeight: 500, color: stack.changeColor }}>{stack.changeLabel}</span>
            </div>
          </Fragment>
        ))}
      </div>
    </div>
  );
}

const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)';

function subscribeToResize(onChange: () => void) {
  window.addEventListener('resize', onChange);
  return () => window.removeEventListener('resize', onChange);
}

function useStageScale() {
  return useSyncExternalStore(
    subscribeToResize,
    () => Math.min(1, (Math.min(window.innerWidth, 720) - 48) / STAGE_WIDTH),
    () => 1,
  );
}

function subscribeToReducedMotion(onChange: () => void) {
  const query = window.matchMedia(REDUCED_MOTION_QUERY);
  query.addEventListener('change', onChange);
  return () => query.removeEventListener('change', onChange);
}

function usePrefersReducedMotion() {
  return useSyncExternalStore(
    subscribeToReducedMotion,
    () => window.matchMedia(REDUCED_MOTION_QUERY).matches,
    () => false,
  );
}

export function CoinStack() {
  const { data: reserves, isLoading } = useReserves();
  const { data: marketStocks = [] } = useMarketStocks();
  const scale = useStageScale();
  const reducedMotion = usePrefersReducedMotion();

  const stacks = useMemo<CoinStackData[]>(() => {
    const ranked = (reserves ?? [])
      .map(entry => ({ ticker: entry.stock.ticker, supply: rawTokenToNumber(entry.on_chain_supply) ?? 0 }))
      .sort((first, second) => second.supply - first.supply)
      .slice(0, SLOTS.length);
    const maxSupply = ranked[0]?.supply ?? 0;
    if (maxSupply <= 0) return [];
    const changeByTicker = new Map(marketStocks.map(stock => [stock.ticker, stock.change_24h]));
    return ranked.map((item, index) => {
      const coinCount = Math.max(2, Math.round((item.supply / maxSupply) * MAX_COINS));
      const change = changeByTicker.get(item.ticker) ?? 0;
      return {
        ticker: item.ticker,
        coinCount,
        stackHeight: coinCount * COIN_HEIGHT,
        slotX: SLOTS[index][0],
        slotY: SLOTS[index][1],
        edgeColor: EDGE_COLORS[index],
        changeLabel: fmtPct(change),
        changeColor: change >= 0 ? '#1f7a4b' : '#c8102e',
      };
    });
  }, [reserves, marketStocks]);

  const stageHeight = Math.round(STAGE_HEIGHT * scale);

  if (isLoading && !reserves) {
    return (
      <div className="mt-[32px]">
        <div className="skeleton" style={{ height: stageHeight, background: '#f3f0ea', animation: 'none' }} />
      </div>
    );
  }

  if (stacks.length === 0) return null;

  return (
    <div className="relative mt-[32px]">
      <div style={{ height: stageHeight, overflow: 'hidden' }}>
        <CoinStage stacks={stacks} autoRotate={!reducedMotion} scale={scale} />
      </div>
      <div className="flex flex-wrap justify-between gap-[8px] border-t border-[#e3ddd2] pt-[10px] text-[10px] leading-[normal] font-[600] uppercase tracking-[0.16em] text-[var(--body)]">
        <span>Drag to rotate · stack height = tokens in circulation</span>
        <span>Each coin backed 1:1 in custody</span>
      </div>
    </div>
  );
}

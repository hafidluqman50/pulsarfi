'use client';

import { Fragment, useEffect, useMemo, useRef, useState, type CSSProperties, type PointerEvent } from 'react';
import { getStockIcon, PStockMark } from '@/components/ui/PStockMark';
import { useReserves } from '@/http/custodian/hooks';
import { useMarketStocks } from '@/http/market/hooks';
import { fmtPct } from '@/lib/data';
import { rawTokenToNumber } from '@/lib/stockDetail';
import { usePrefersReducedMotion } from '@/lib/usePrefersReducedMotion';
import { TownScene3D } from './TownScene3D';
import { BUILDINGS, INK, PEOPLE, SKIN_TONES, type BuildingData, type PersonData, type TownStack } from './townData';

const SCENE_WIDTH = 560;
const SCENE_HEIGHT = 400;
const SCENE_BASE_WIDTH = 700;
const MAX_SCALE = 1.05;
const WRAPPER_HEIGHT = 540;
const SKELETON_HEIGHT = 420;
const COIN_HEIGHT = 11;
const MAX_COINS = 8;
const SLOTS = [[200, 150], [330, 140], [270, 240], [390, 250], [160, 262]] as const;
const EDGE_COLORS = [INK, '#c8102e', '#1f4d8a', '#9a0c24', '#5a4a3a'] as const;

interface PersonRefs {
  element: HTMLDivElement | null;
  shadow: HTMLDivElement | null;
  figure: SVGSVGElement | null;
  bubble: HTMLDivElement | null;
  text: HTMLSpanElement | null;
}

function windowFace(building: BuildingData, dense: boolean): CSSProperties {
  const rowHeight = dense ? 5 : 7;
  const rowPitch = dense ? 12 : 16;
  return {
    background: building.faceColor,
    backgroundImage: `repeating-linear-gradient(0deg,${building.faceColor} 0 ${rowHeight}px,transparent ${rowHeight}px ${rowPitch}px),repeating-linear-gradient(90deg,${building.faceColor} 0 5px,${building.windowColor} 5px 11px)`,
  };
}

function Building({ building, registerSign }: { building: BuildingData; registerSign: (element: HTMLDivElement | null) => void }) {
  const { x, y, width, depth, height } = building;
  const face: CSSProperties = { position: 'absolute', border: `1px solid ${INK}`, boxSizing: 'border-box' };
  return (
    <>
      <div style={{ ...face, ...windowFace(building, false), left: x, top: y - height, width, height, transformOrigin: '50% 100%', transform: 'rotateX(-90deg)' }} />
      <div style={{ ...face, ...windowFace(building, false), left: x, top: y + depth - height, width, height, transformOrigin: '50% 100%', transform: 'rotateX(-90deg)' }} />
      <div style={{ ...face, ...windowFace(building, true), left: x - height, top: y, width: height, height: depth, transformOrigin: '100% 50%', transform: 'rotateY(90deg)' }} />
      <div style={{ ...face, ...windowFace(building, true), left: x + width - height, top: y, width: height, height: depth, transformOrigin: '100% 50%', transform: 'rotateY(90deg)' }} />
      <div style={{ ...face, left: x, top: y, width, height: depth, background: building.roofColor, transform: `translateZ(${height}px)` }} />
      {building.sign && (
        <div
          data-z={height + 18}
          ref={registerSign}
          style={{
            position: 'absolute', left: x + width / 2, top: y + depth / 2, background: building.sign.background, color: '#fff',
            border: `1px solid ${INK}`, padding: '4px 7px', font: '700 10px var(--font-sans)', letterSpacing: '.14em', whiteSpace: 'nowrap', pointerEvents: 'none',
          }}
        >
          {building.sign.text}
        </div>
      )}
    </>
  );
}

function PersonFigure({ person, index }: { person: PersonData; index: number }) {
  const skin = SKIN_TONES[index % SKIN_TONES.length];
  const isStaff = person.kind === 'idx';
  return (
    <>
      {isStaff ? (
        <>
          <path d="M4 52 L5 27 Q14 20 23 27 L24 52 Z" fill={INK} />
          <path d="M10.5 22.5 L14 31 L17.5 22.5 Z" fill="#fff" />
          <path d="M13.3 24 L14.7 24 L15.2 32 L14 34 L12.8 32 Z" fill="#c8102e" />
          <rect x={17} y={32} width={4} height={5} fill="#fff" stroke="#c8102e" strokeWidth={0.8} />
          <circle cx={14} cy={13} r={7} fill={skin} stroke={INK} strokeWidth={1} />
          <path d="M7 12 Q8 5 14 5.5 Q20 5 21 11 Q17 8.5 12 9.5 Q9 10 7 12 Z" fill={INK} />
        </>
      ) : (
        <>
          <path d="M3 52 L4 28 Q14 19 24 28 L25 52 Z" fill={person.color} stroke={INK} strokeWidth={1} />
          <path d="M6 18 Q6 4 14 4 Q22 4 22 18 Z" fill={person.color} stroke={INK} strokeWidth={1} />
          <path d="M9 38 L19 38 L18 44 L10 44 Z" fill="rgba(0,0,0,.18)" />
          <circle cx={14} cy={14} r={6.2} fill={skin} stroke={INK} strokeWidth={1} />
          <rect x={19} y={30} width={6} height={9} rx={1} fill={INK} stroke="#fbfaf7" strokeWidth={0.6} />
        </>
      )}
      <circle cx={15.2} cy={13.5} r={0.9} fill={INK} />
      <circle cx={18.2} cy={13.5} r={0.9} fill={INK} />
    </>
  );
}

interface TownSceneProps {
  stacks: TownStack[];
  autoRotate: boolean;
  reducedMotion: boolean;
}

function TownScene({ stacks, autoRotate, reducedMotion }: TownSceneProps) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<HTMLDivElement>(null);
  const labelRefs = useRef<(HTMLDivElement | null)[]>([]);
  const floatRefs = useRef<(HTMLDivElement | null)[]>([]);
  const signRefs = useRef<HTMLDivElement[]>([]);
  const personRefs = useRef<PersonRefs[]>(PEOPLE.map(() => ({ element: null, shadow: null, figure: null, bubble: null, text: null })));
  const motion = useRef({ angle: -28, dragging: false, lastX: 0, velocity: 0 });
  const [wrapperWidth, setWrapperWidth] = useState(720);
  const scale = Math.min(MAX_SCALE, wrapperWidth / SCENE_BASE_WIDTH);
  const scaleRef = useRef(scale);

  useEffect(() => {
    scaleRef.current = scale;
  }, [scale]);

  useEffect(() => {
    const wrapper = wrapperRef.current;
    if (!wrapper) return;
    const observer = new ResizeObserver(([entry]) => setWrapperWidth(entry.contentRect.width));
    observer.observe(wrapper);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    let frameId = 0;
    let isVisible = true;
    const floatStartedAt = stacks.map(() => -9);
    const lastCycle = PEOPLE.map(() => -1);

    const tick = () => {
      if (!isVisible || document.hidden) {
        frameId = 0;
        if (document.hidden) {
          setTimeout(() => {
            if (!frameId) frameId = requestAnimationFrame(tick);
          }, 500);
        }
        return;
      }
      const state = motion.current;
      const time = reducedMotion ? 0 : performance.now() / 1000;
      if (!state.dragging) {
        state.angle += (autoRotate ? 0.08 : 0) + state.velocity;
        state.velocity *= 0.94;
      }
      const angleRadians = (state.angle * Math.PI) / 180;
      const billboard = `rotateZ(${-state.angle}deg) rotateX(-60deg)`;
      if (sceneRef.current) {
        sceneRef.current.style.transform = `scale(${scaleRef.current}) rotateX(60deg) rotateZ(${state.angle}deg)`;
      }
      labelRefs.current.forEach((element, index) => {
        if (element && stacks[index]) {
          element.style.transform = `translate(-50%,-50%) translateZ(${stacks[index].stackHeight + 46}px) ${billboard}`;
        }
      });
      signRefs.current.forEach((element) => {
        if (element) element.style.transform = `translate(-50%,-50%) translateZ(${element.dataset.z}px) ${billboard}`;
      });

      PEOPLE.forEach((person, index) => {
        const refs = personRefs.current[index];
        if (!refs.element) return;
        const stackIndex = (person.stackIndex ?? 0) % stacks.length;
        let x = person.x ?? 0;
        let y = person.y ?? 0;
        let direction = Math.sin(time * 1.6 + person.phase * 2) > 0.15 ? 1 : -1;
        let bob = 0;
        let text = '';
        let show = false;
        if (person.action === 'walk' && person.path) {
          const [[startX, startY], [endX, endY]] = person.path;
          const length = Math.hypot(endX - startX, endY - startY);
          const travelled = ((time + person.phase) * (person.speed ?? 20)) % (2 * length);
          const forward = travelled < length;
          const progress = forward ? travelled / length : 2 - travelled / length;
          x = startX + (endX - startX) * progress;
          y = startY + (endY - startY) * progress;
          const velocityX = (endX - startX) * (forward ? 1 : -1);
          const velocityY = (endY - startY) * (forward ? 1 : -1);
          direction = velocityX * Math.cos(angleRadians) - velocityY * Math.sin(angleRadians) >= 0 ? 1 : -1;
          bob = reducedMotion ? 0 : -Math.abs(Math.sin(time * 9 + person.phase)) * 2.5;
          show = (time + person.phase) % 9 < 2.5;
          text = person.say?.[0] ?? '';
        } else if (person.action === 'count') {
          const counter = (time + person.phase) % 7;
          show = counter < 4.5;
          text = `${stacks[stackIndex].ticker.slice(0, 4)} · ${Math.floor(counter * 2.4) + 1}…`;
          if (show) direction = Math.sin(time * 0.8 + person.phase) > -0.6 ? 1 : -1;
        } else {
          const say = person.say ?? [];
          const cycle = Math.floor((time + person.phase) / 5);
          const within = (time + person.phase) % 5;
          show = within < 2.2;
          text = say[cycle % say.length] ?? '';
          if (show && cycle !== lastCycle[index]) {
            lastCycle[index] = cycle;
            if (/BUY|SWAP/.test(text)) {
              floatStartedAt[stackIndex] = time;
              const floatElement = floatRefs.current[stackIndex];
              if (floatElement) floatElement.textContent = text.startsWith('SWAP') ? '⇄ swap' : '+1 buy';
            }
          }
          bob = show && within < 0.4 ? -Math.sin((within / 0.4) * Math.PI) * 4 : 0;
        }
        if (reducedMotion) show = false;
        refs.element.style.left = `${x - 14}px`;
        refs.element.style.top = `${y - 52}px`;
        refs.element.style.transform = `${billboard} translateY(${bob}px)`;
        if (refs.shadow) {
          refs.shadow.style.left = `${x - 12}px`;
          refs.shadow.style.top = `${y - 5}px`;
        }
        if (refs.figure) refs.figure.style.transform = `scaleX(${direction})`;
        if (refs.bubble) {
          refs.bubble.style.opacity = show ? '1' : '0';
          if (refs.text && refs.text.textContent !== text) refs.text.textContent = text;
        }
      });

      floatRefs.current.forEach((element, index) => {
        if (!element || !stacks[index]) return;
        const elapsed = time - floatStartedAt[index];
        const isOn = !reducedMotion && elapsed >= 0 && elapsed < 1.6;
        element.style.opacity = isOn ? String(1 - elapsed / 1.6) : '0';
        element.style.transform = `translate(-50%,-50%) translateZ(${stacks[index].stackHeight + 74 + (isOn ? elapsed * 34 : 0)}px) ${billboard}`;
      });
      frameId = requestAnimationFrame(tick);
    };

    const observer = new IntersectionObserver(([entry]) => {
      isVisible = entry.isIntersecting;
      if (isVisible && !frameId) frameId = requestAnimationFrame(tick);
    }, { rootMargin: '100px' });
    if (wrapperRef.current) observer.observe(wrapperRef.current);
    frameId = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(frameId);
      observer.disconnect();
    };
  }, [autoRotate, stacks, reducedMotion]);

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

  const registerSign = (element: HTMLDivElement | null) => {
    if (element && !signRefs.current.includes(element)) signRefs.current.push(element);
  };

  return (
    <div
      ref={wrapperRef}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerUp}
      style={{ width: '100%', height: Math.round(WRAPPER_HEIGHT * scale), overflow: 'hidden', display: 'flex', alignItems: 'center', justifyContent: 'center', perspective: 1600, cursor: 'grab', touchAction: 'pan-y', userSelect: 'none' }}
    >
      <div
        ref={sceneRef}
        style={{
          position: 'relative', flexShrink: 0, width: SCENE_WIDTH, height: SCENE_HEIGHT, transformStyle: 'preserve-3d', willChange: 'transform',
          marginTop: Math.round(10 * scale), transform: `scale(${scale}) rotateX(60deg) rotateZ(-28deg)`,
        }}
      >
        <div style={{ position: 'absolute', inset: 0, background: '#e3ddd2', transform: 'translateZ(-10px)' }} />
        <div style={{ position: 'absolute', inset: 0, background: '#efebe3', border: '1px solid #e3ddd2', transform: 'translateZ(-5px)' }} />
        <div style={{ position: 'absolute', inset: 0, background: '#f8f6f1', border: '1px solid #d9d1c4', backgroundImage: 'linear-gradient(#ebe6dc 1px,transparent 1px),linear-gradient(90deg,#ebe6dc 1px,transparent 1px)', backgroundSize: '24px 24px' }} />
        <div style={{ position: 'absolute', left: 0, right: 0, top: 318, height: 36, background: '#ebe6dc', borderTop: '1px dashed #c9bfae', borderBottom: '1px dashed #c9bfae', transform: 'translateZ(0.3px)' }} />
        <div style={{ position: 'absolute', left: 0, right: 0, top: 44, height: 28, background: '#ebe6dc', borderTop: '1px dashed #c9bfae', borderBottom: '1px dashed #c9bfae', transform: 'translateZ(0.3px)' }} />

        {BUILDINGS.map(building => (
          <Building key={building.id} building={building} registerSign={registerSign} />
        ))}

        {stacks.map((stack, stackIndex) => (
          <Fragment key={stack.ticker}>
            <div style={{ position: 'absolute', left: stack.slotX - 70, top: stack.slotY - 70, width: 140, height: 140, borderRadius: '50%', background: 'radial-gradient(closest-side,rgba(22,17,14,.22),transparent)', transform: 'translateZ(0.5px)' }} />
            {Array.from({ length: stack.coinCount }, (_, coinIndex) => {
              const base = coinIndex * COIN_HEIGHT;
              return (
                <Fragment key={coinIndex}>
                  {Array.from({ length: 4 }, (_, layerIndex) => (
                    <div
                      key={layerIndex}
                      style={{
                        position: 'absolute', left: stack.slotX - 44, top: stack.slotY - 44, width: 88, height: 88, borderRadius: '50%',
                        background: layerIndex === 0 ? '#a39988' : layerIndex === 2 ? stack.edgeColor : '#d6cfc1',
                        transform: `translateZ(${base + layerIndex * 2.8}px)`,
                      }}
                    />
                  ))}
                  <div
                    style={{
                      position: 'absolute', left: stack.slotX - 44, top: stack.slotY - 44, width: 88, height: 88, borderRadius: '50%',
                      background: '#fff', border: `2px solid ${INK}`, boxShadow: `inset 0 0 0 5px #fff,inset 0 0 0 6px ${stack.edgeColor}`,
                      display: 'grid', placeItems: 'center', transform: `translateZ(${base + 10.5}px)`,
                    }}
                  >
                    {coinIndex === stack.coinCount - 1 && (
                      <div style={{ pointerEvents: 'none' }}>
                        <PStockMark ticker={stack.ticker} size={46} />
                      </div>
                    )}
                  </div>
                </Fragment>
              );
            })}
            <div
              ref={element => { labelRefs.current[stackIndex] = element; }}
              style={{ position: 'absolute', left: stack.slotX, top: stack.slotY, background: '#fff', border: `1px solid ${INK}`, padding: '5px 8px', whiteSpace: 'nowrap', boxShadow: '0 3px 0 -1px #fbfaf7,0 4px 0 -1px #16110e', pointerEvents: 'none' }}
            >
              <span style={{ fontSize: 11, fontWeight: 700, letterSpacing: '0.04em' }}>{stack.ticker}</span>{' '}
              <span className="mono" style={{ fontSize: 11, fontWeight: 500, color: stack.changeColor }}>{stack.changeLabel}</span>
            </div>
            <div
              ref={element => { floatRefs.current[stackIndex] = element; }}
              className="mono"
              style={{ position: 'absolute', left: stack.slotX, top: stack.slotY, opacity: 0, background: '#1f7a4b', color: '#fff', padding: '3px 6px', fontSize: 10, fontWeight: 700, whiteSpace: 'nowrap', pointerEvents: 'none' }}
            >
              +1 buy
            </div>
          </Fragment>
        ))}

        {PEOPLE.map((person, index) => {
          const isStaff = person.kind === 'idx';
          return (
            <Fragment key={index}>
              <div
                ref={element => { personRefs.current[index].shadow = element; }}
                style={{ position: 'absolute', width: 24, height: 10, borderRadius: '50%', background: 'rgba(22,17,14,.2)', transform: 'translateZ(0.6px)' }}
              />
              <div
                ref={element => { personRefs.current[index].element = element; }}
                style={{ position: 'absolute', width: 28, height: 52, transformOrigin: '50% 100%', pointerEvents: 'none' }}
              >
                <svg
                  ref={element => { personRefs.current[index].figure = element; }}
                  width={28}
                  height={52}
                  viewBox="0 0 28 52"
                  style={{ display: 'block', overflow: 'visible' }}
                >
                  <PersonFigure person={person} index={index} />
                </svg>
                <div
                  ref={element => { personRefs.current[index].bubble = element; }}
                  className={isStaff ? undefined : 'mono'}
                  style={{
                    position: 'absolute', left: '50%', bottom: 58, transform: 'translateX(-50%)', opacity: 0, transition: 'opacity .2s',
                    background: '#fff', border: `1px solid ${INK}`, padding: '3px 7px', whiteSpace: 'nowrap', fontSize: 10, fontWeight: 600,
                    color: isStaff || person.color === '#e7b416' ? INK : person.color,
                  }}
                >
                  <span ref={element => { personRefs.current[index].text = element; }} />
                  <span style={{ position: 'absolute', left: '50%', bottom: -5, width: 8, height: 8, background: '#fff', borderRight: `1px solid ${INK}`, borderBottom: `1px solid ${INK}`, transform: 'translateX(-50%) rotate(45deg)' }} />
                </div>
              </div>
            </Fragment>
          );
        })}
      </div>
    </div>
  );
}

export function CoinStack() {
  const { data: reserves, isLoading } = useReserves();
  const { data: marketStocks = [] } = useMarketStocks();
  const reducedMotion = usePrefersReducedMotion();
  const [webglUnavailable, setWebglUnavailable] = useState(false);

  const stacks = useMemo<TownStack[]>(() => {
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
        iconUrl: getStockIcon(item.ticker),
      };
    });
  }, [reserves, marketStocks]);

  if (isLoading && !reserves) {
    return (
      <div className="mt-[32px]">
        <div className="skeleton" style={{ height: SKELETON_HEIGHT, background: '#f3f0ea', animation: 'none' }} />
      </div>
    );
  }

  if (stacks.length === 0) return null;

  return (
    <div className="relative mt-[32px]">
      <div>
        {webglUnavailable ? (
          <TownScene stacks={stacks} autoRotate={!reducedMotion} reducedMotion={reducedMotion} />
        ) : (
          <TownScene3D stacks={stacks} autoRotate={!reducedMotion} reducedMotion={reducedMotion} onUnsupported={() => setWebglUnavailable(true)} />
        )}
      </div>
      <div className="flex flex-wrap justify-between gap-[8px] border-t border-[#e3ddd2] pt-[10px] text-[10px] leading-[normal] font-[600] uppercase tracking-[0.16em] text-[var(--body)]">
        <span>Drag to rotate · stack height = tokens in circulation · traders from IDX &amp; on-chain</span>
        <span>Each coin backed 1:1 in custody</span>
      </div>
    </div>
  );
}

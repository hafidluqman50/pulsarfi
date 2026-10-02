'use client';

import { useEffect, useRef, useState } from 'react';
import { usePrefersReducedMotion } from '@/lib/usePrefersReducedMotion';

const GLYPHS = ' 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ.,+-%:';
const TICK_MS = 55;
const SETTLE_DELAY_FRAMES = 3;

interface SplitFlapProps {
  text: string;
  size?: number;
  color?: string;
}

export function SplitFlap({ text, size = 20, color }: SplitFlapProps) {
  const reducedMotion = usePrefersReducedMotion();
  const blank = text.replace(/./g, ' ');
  const [shown, setShown] = useState(blank);
  const previous = useRef(blank);

  useEffect(() => {
    const from = previous.current;
    previous.current = text;
    let frame = 0;
    const intervalId = setInterval(() => {
      frame += 1;
      let next = '';
      let settled = true;
      for (let index = 0; index < text.length; index += 1) {
        if (from[index] === text[index] || frame >= SETTLE_DELAY_FRAMES + index) {
          next += text[index];
        } else {
          settled = false;
          next += GLYPHS[Math.floor(Math.random() * GLYPHS.length)];
        }
      }
      setShown(next);
      if (settled) clearInterval(intervalId);
    }, TICK_MS);
    return () => clearInterval(intervalId);
  }, [text]);

  const display = reducedMotion ? text : shown;

  return (
    <span role="img" aria-label={text.trim()} style={{ display: 'inline-flex', gap: Math.max(2, Math.round(size * 0.1)) }}>
      {display.split('').map((character, index) => (
        <span
          key={index}
          aria-hidden="true"
          className="mono"
          style={{
            position: 'relative',
            width: Math.round(size * 0.74),
            height: Math.round(size * 1.3),
            display: 'inline-grid',
            placeItems: 'center',
            background: '#f3f0ea',
            color: color ?? 'var(--ink)',
            fontSize: size,
            fontWeight: 600,
            lineHeight: 1,
            borderRadius: 2,
            overflow: 'hidden',
            boxShadow: 'inset 0 -1px 0 #e3ddd2, 0 1px 0 #d9d1c4',
          }}
        >
          <span key={character} style={{ display: 'block', animation: reducedMotion ? 'none' : 'flip .12s ease-out' }}>
            {character === ' ' ? ' ' : character}
          </span>
          <span style={{ position: 'absolute', left: 0, right: 0, top: '50%', height: 1, background: 'rgba(22,17,14,.12)' }} />
        </span>
      ))}
    </span>
  );
}

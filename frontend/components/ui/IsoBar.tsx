'use client';

export interface IsoPalette {
  top: string;
  front: string;
  side: string;
}

export const ISO_PALETTES: IsoPalette[] = [
  { top: '#c8102e', front: '#9a0c24', side: '#7a0a1d' },
  { top: '#3a312b', front: '#16110e', side: '#000' },
  { top: '#1f4d8a', front: '#173a68', side: '#102a4c' },
  { top: '#5a4a3a', front: '#45382c', side: '#33291f' },
  { top: '#e3ddd2', front: '#bcb2a3', side: '#a39988' },
];

interface IsoBarProps {
  x: number;
  y: number;
  width: number;
  depth: number;
  height: number;
  palette: IsoPalette;
}

export function IsoBar({ x, y, width, depth, height, palette }: IsoBarProps) {
  return (
    <div style={{ position: 'absolute', left: x, top: y, width, height: depth, transformStyle: 'preserve-3d' }}>
      <div style={{ position: 'absolute', left: 0, top: 0, width, height, background: palette.front, transformOrigin: '0 0', transform: 'rotateX(90deg)' }} />
      <div style={{ position: 'absolute', left: 0, top: depth, width, height, background: palette.front, transformOrigin: '0 0', transform: 'rotateX(90deg)' }} />
      <div style={{ position: 'absolute', left: 0, top: 0, width: height, height: depth, background: palette.side, transformOrigin: '0 0', transform: 'rotateY(-90deg)' }} />
      <div style={{ position: 'absolute', left: width, top: 0, width: height, height: depth, background: palette.side, transformOrigin: '0 0', transform: 'rotateY(-90deg)' }} />
      <div style={{ position: 'absolute', inset: 0, background: palette.top, transform: `translateZ(${height}px)` }} />
    </div>
  );
}

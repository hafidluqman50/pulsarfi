'use client';

export function Sparkline({ data, width = 80, height = 24, positive, strokeWidth = 1.25 }: { data: number[]; width?: number; height?: number; positive: boolean; strokeWidth?: number }) {
  if (!data || data.length === 0) return null;
  const min = Math.min(...data), max = Math.max(...data);
  const range = max - min || 1;
  const stepX = width / (data.length - 1);
  const points = data.map((v, i) => `${(i * stepX).toFixed(1)},${(height - ((v - min) / range) * height).toFixed(1)}`).join(" ");
  const strokeColor = positive ? "#1f7a4b" : "#c8102e";
  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} style={{ display: "block" }}>
      <polyline fill="none" stroke={strokeColor} strokeWidth={strokeWidth} strokeLinejoin="round" strokeLinecap="round" points={points} />
    </svg>
  );
}

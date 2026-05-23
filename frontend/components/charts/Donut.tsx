'use client';

interface DonutSegment { label: string; value: number; }

interface DonutProps {
  data: DonutSegment[];
  size?: number;
  thickness?: number;
  palette?: string[];
}

export function Donut({ data, size = 180, thickness = 22, palette = [] }: DonutProps) {
  const total = data.reduce((runningTotal, segment) => runningTotal + segment.value, 0);
  const radius = (size - thickness) / 2;
  const centerX = size / 2;
  const centerY = size / 2;
  let startAngle = -Math.PI / 2;
  const segments = data.map((segment, segmentIndex) => {
    const endAngle = startAngle + (segment.value / total) * Math.PI * 2;
    const isLargeArc = endAngle - startAngle > Math.PI ? 1 : 0;
    const startX = centerX + Math.cos(startAngle) * radius;
    const startY = centerY + Math.sin(startAngle) * radius;
    const endX   = centerX + Math.cos(endAngle) * radius;
    const endY   = centerY + Math.sin(endAngle) * radius;
    const arcPath = `M ${startX} ${startY} A ${radius} ${radius} 0 ${isLargeArc} 1 ${endX} ${endY}`;
    const renderedSegment = {
      arcPath,
      color: palette[segmentIndex % palette.length] || "#c8102e",
      label: segment.label,
      value: segment.value,
      percentage: (segment.value / total) * 100,
    };
    startAngle = endAngle;
    return renderedSegment;
  });
  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} style={{ display: "block" }}>
      <circle cx={centerX} cy={centerY} r={radius} fill="none" stroke="var(--hairline)" strokeWidth={thickness} />
      {segments.map((renderedSegment, segmentIndex) => (
        <path key={segmentIndex} d={renderedSegment.arcPath} fill="none" stroke={renderedSegment.color} strokeWidth={thickness} strokeLinecap="butt" />
      ))}
    </svg>
  );
}

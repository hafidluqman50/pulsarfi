'use client';

import { useRef, useState, useEffect } from 'react';
import { TimePoint, fmtAxisDate } from '@/lib/data';

function smoothPath(points: [number, number][]): string {
  if (points.length < 2) return "";
  let pathString = `M ${points[0][0]} ${points[0][1]}`;
  for (let pointIndex = 0; pointIndex < points.length - 1; pointIndex++) {
    const previousPoint = points[pointIndex - 1] || points[pointIndex];
    const currentPoint  = points[pointIndex];
    const nextPoint     = points[pointIndex + 1];
    const farNextPoint  = points[pointIndex + 2] || nextPoint;
    const controlPoint1X = currentPoint[0] + (nextPoint[0] - previousPoint[0]) / 6;
    const controlPoint1Y = currentPoint[1] + (nextPoint[1] - previousPoint[1]) / 6;
    const controlPoint2X = nextPoint[0] - (farNextPoint[0] - currentPoint[0]) / 6;
    const controlPoint2Y = nextPoint[1] - (farNextPoint[1] - currentPoint[1]) / 6;
    pathString += ` C ${controlPoint1X.toFixed(2)} ${controlPoint1Y.toFixed(2)}, ${controlPoint2X.toFixed(2)} ${controlPoint2Y.toFixed(2)}, ${nextPoint[0]} ${nextPoint[1]}`;
  }
  return pathString;
}

interface AreaChartProps {
  data: TimePoint[];
  height?: number;
  color?: string;
  fill?: boolean;
  valueFormatter?: (value: number) => string;
  labelFormatter?: (timestamp: number, range: string) => string;
  range?: string;
}

export function AreaChart({
  data,
  height = 280,
  color = "var(--merah)",
  fill = true,
  valueFormatter = (value) => value.toFixed(2),
  labelFormatter,
  range = "1M",
}: AreaChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [containerSize, setContainerSize] = useState({ width: 800, height });
  const [hoverPoint, setHoverPoint] = useState<{ dataIndex: number; xPosition: number; yPosition: number } | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;
    const resizeObserver = new ResizeObserver((resizeEntries) => {
      for (const resizeEntry of resizeEntries) setContainerSize({ width: resizeEntry.contentRect.width, height });
    });
    resizeObserver.observe(containerRef.current);
    return () => resizeObserver.disconnect();
  }, [height]);

  if (!data || data.length === 0) return null;

  const paddingLeft = 56, paddingRight = 16, paddingTop = 16, paddingBottom = 28;
  const svgWidth  = containerSize.width;
  const svgHeight = containerSize.height;
  const innerWidth  = Math.max(0, svgWidth  - paddingLeft - paddingRight);
  const innerHeight = Math.max(0, svgHeight - paddingTop  - paddingBottom);

  const allValues = data.map(dataPoint => dataPoint.value);
  const minValue  = Math.min(...allValues);
  const maxValue  = Math.max(...allValues);
  const valuePadding = (maxValue - minValue) * 0.1 || maxValue * 0.05 || 1;
  const lowerBound   = minValue - valuePadding;
  const upperBound   = maxValue + valuePadding;
  const valueRange   = upperBound - lowerBound;

  const toXPosition = (dataIndex: number) => paddingLeft + (dataIndex / Math.max(1, data.length - 1)) * innerWidth;
  const toYPosition = (value: number)     => paddingTop  + (1 - (value - lowerBound) / valueRange) * innerHeight;

  const svgPoints: [number, number][] = data.map((dataPoint, dataIndex) => [toXPosition(dataIndex), toYPosition(dataPoint.value)]);
  const svgPath = smoothPath(svgPoints);

  const isPositive  = data[data.length - 1].value >= data[0].value;
  const strokeColor = color === "var(--merah)" ? (isPositive ? "var(--positive)" : "var(--negative)") : color;
  const gradientId  = `g-${range}-${isPositive ? "p" : "n"}`;

  const yTickCount   = 4;
  const yTickValues  = Array.from({ length: yTickCount + 1 }, (_, tickIndex) => lowerBound + (valueRange * tickIndex) / yTickCount);
  const horizontalTickCount = Math.min(5, Math.max(2, Math.floor(innerWidth / 110)));
  const xTickIndices = Array.from({ length: horizontalTickCount }, (_, tickIndex) => Math.round((tickIndex / (horizontalTickCount - 1)) * (data.length - 1)));

  const handleMouseMove = (mouseEvent: React.MouseEvent<SVGSVGElement>) => {
    const boundingRect  = mouseEvent.currentTarget.getBoundingClientRect();
    const mouseX        = mouseEvent.clientX - boundingRect.left;
    const horizontalRatio = (mouseX - paddingLeft) / innerWidth;
    const hoverIndex = Math.max(0, Math.min(data.length - 1, Math.round(horizontalRatio * (data.length - 1))));
    setHoverPoint({ dataIndex: hoverIndex, xPosition: toXPosition(hoverIndex), yPosition: toYPosition(data[hoverIndex].value) });
  };

  return (
    <div ref={containerRef} style={{ position: "relative", width: "100%" }}>
      <svg width="100%" height={svgHeight} viewBox={`0 0 ${svgWidth} ${svgHeight}`} preserveAspectRatio="none"
        onMouseMove={handleMouseMove} onMouseLeave={() => setHoverPoint(null)}
        style={{ display: "block", touchAction: "none" }}>
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={isPositive ? "#1f7a4b" : "#c8102e"} stopOpacity="0.16" />
            <stop offset="100%" stopColor={isPositive ? "#1f7a4b" : "#c8102e"} stopOpacity="0" />
          </linearGradient>
        </defs>
        {yTickValues.map((tickValue, tickIndex) => {
          const yPosition = toYPosition(tickValue);
          return (
            <g key={tickIndex}>
              <line x1={paddingLeft} x2={svgWidth - paddingRight} y1={yPosition} y2={yPosition} stroke="var(--hairline)" strokeWidth="1" strokeDasharray={tickIndex === 0 || tickIndex === yTickValues.length - 1 ? "0" : "2 4"} />
              <text x={paddingLeft - 8} y={yPosition + 3} textAnchor="end" fill="var(--body)" fontFamily="JetBrains Mono, monospace" fontSize="10">
                {valueFormatter(tickValue)}
              </text>
            </g>
          );
        })}
        {fill && (
          <path d={`${svgPath} L ${svgPoints[svgPoints.length - 1][0]} ${paddingTop + innerHeight} L ${svgPoints[0][0]} ${paddingTop + innerHeight} Z`}
            fill={`url(#${gradientId})`} />
        )}
        <path d={svgPath} fill="none" stroke={strokeColor} strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" />
        {xTickIndices.map((tickDataIndex, labelIndex) => {
          const xPosition = toXPosition(tickDataIndex);
          const labelText = labelFormatter ? labelFormatter(data[tickDataIndex].timestamp, range) : "";
          return <text key={labelIndex} x={xPosition} y={svgHeight - paddingBottom + 16} textAnchor="middle" fill="var(--body)" fontFamily="JetBrains Mono, monospace" fontSize="10">{labelText}</text>;
        })}
        {hoverPoint && (
          <g>
            <line x1={hoverPoint.xPosition} x2={hoverPoint.xPosition} y1={paddingTop} y2={paddingTop + innerHeight} stroke="var(--ink)" strokeWidth="1" strokeDasharray="2 3" />
            <circle cx={hoverPoint.xPosition} cy={hoverPoint.yPosition} r="5" fill="var(--canvas)" stroke={strokeColor} strokeWidth="1.5" />
          </g>
        )}
      </svg>
      {hoverPoint && (
        <div style={{
          position: "absolute",
          left: Math.min(Math.max(0, hoverPoint.xPosition - 60), Math.max(0, svgWidth - 120)),
          top: hoverPoint.yPosition - 60,
          background: "var(--ink)", color: "var(--putih)",
          padding: "8px 10px", pointerEvents: "none",
          fontSize: 11, lineHeight: 1.35, minWidth: 110,
          boxShadow: "0 4px 12px rgba(0,0,0,0.18)",
        }}>
          <div className="mono" style={{ opacity: 0.7, fontSize: 10, letterSpacing: 0.5 }}>
            {labelFormatter ? labelFormatter(data[hoverPoint.dataIndex].timestamp, range === "1D" ? "1D-full" : "tooltip") : ""}
          </div>
          <div className="mono" style={{ fontSize: 14, fontWeight: 600 }}>{valueFormatter(data[hoverPoint.dataIndex].value)}</div>
        </div>
      )}
    </div>
  );
}

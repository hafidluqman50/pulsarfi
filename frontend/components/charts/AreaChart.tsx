'use client';

import { useRef, useEffect } from 'react';
import {
  createChart,
  AreaSeries,
  ColorType,
  LineStyle,
  CrosshairMode,
  type IChartApi,
  type ISeriesApi,
  type UTCTimestamp,
} from 'lightweight-charts';
import type { TimePoint } from '@/lib/data';

// Design system colors — hardcoded because lightweight-charts needs literal values, not CSS vars
const COLORS = {
  canvas:   '#fbfaf7',
  hairline: '#e3ddd2',
  body:     '#6b635c',
  ink:      '#16110e',
  positive: '#1f7a4b',
  negative: '#c8102e',
};

interface AreaChartProps {
  data: TimePoint[];
  height?: number;
  valueFormatter?: (value: number) => string;
}

function toSeriesPoint(point: TimePoint): { time: UTCTimestamp; value: number } {
  return { time: Math.floor(point.timestamp / 1000) as UTCTimestamp, value: point.value };
}

function dedupeAscending(
  points: { time: UTCTimestamp; value: number }[],
): { time: UTCTimestamp; value: number }[] {
  const sorted = [...points].sort((a, b) => a.time - b.time);
  return sorted.filter((p, i) => i === 0 || p.time !== sorted[i - 1].time);
}

// Responsibility: mount/update/destroy a lightweight-charts area chart for any TimePoint[] dataset.
// Does not own range logic, data fetching, or any business concern.
export function AreaChart({ data, height = 280, valueFormatter }: AreaChartProps) {
  const containerRef      = useRef<HTMLDivElement>(null);
  const chartRef          = useRef<IChartApi | null>(null);
  const seriesRef         = useRef<ISeriesApi<'Area'> | null>(null);
  const formatterRef      = useRef(valueFormatter);
  formatterRef.current    = valueFormatter; // stable ref avoids chart recreation on prop change

  // Mount chart once — recreate only if height changes
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const chart = createChart(container, {
      width:  container.clientWidth,
      height,
      layout: {
        background:  { type: ColorType.Solid, color: COLORS.canvas },
        textColor:   COLORS.body,
        fontFamily:  '"JetBrains Mono", ui-monospace, monospace',
        fontSize:    10,
        attributionLogo: false,
      },
      grid: {
        vertLines: { visible: false },
        horzLines: { color: COLORS.hairline, style: LineStyle.Dashed },
      },
      crosshair: {
        mode:     CrosshairMode.Normal,
        vertLine: { color: COLORS.ink, style: LineStyle.Dashed, width: 1, labelBackgroundColor: COLORS.ink },
        horzLine: { color: COLORS.ink, style: LineStyle.Dashed, width: 1, labelBackgroundColor: COLORS.ink },
      },
      rightPriceScale: {
        borderVisible: false,
        textColor:     COLORS.body,
        scaleMargins:  { top: 0.12, bottom: 0.08 },
      },
      timeScale: {
        borderVisible:  false,
        timeVisible:    true,
        secondsVisible: false,
        fixLeftEdge:    true,
        fixRightEdge:   true,
      },
      localization: {
        priceFormatter: (value: number) =>
          formatterRef.current ? formatterRef.current(value) : value.toLocaleString('id-ID', { maximumFractionDigits: 0 }),
      },
    });

    const series = chart.addSeries(AreaSeries, {
      topColor:         'rgba(31,122,75,0.12)',
      bottomColor:      'rgba(31,122,75,0)',
      lineColor:        COLORS.positive,
      lineWidth:        2,
      lastValueVisible: true,
      priceLineVisible: false,
    });

    chartRef.current  = chart;
    seriesRef.current = series;

    const observer = new ResizeObserver(() => {
      if (containerRef.current && chartRef.current) {
        chartRef.current.applyOptions({ width: containerRef.current.clientWidth });
      }
    });
    observer.observe(container);

    return () => {
      observer.disconnect();
      chart.remove();
      chartRef.current  = null;
      seriesRef.current = null;
    };
  }, [height]); // eslint-disable-line react-hooks/exhaustive-deps

  // Update series data and direction-based color on every data change
  useEffect(() => {
    const series = seriesRef.current;
    if (!series || data.length === 0) return;

    const isPositive = data[data.length - 1].value >= data[0].value;
    const lineColor  = isPositive ? COLORS.positive : COLORS.negative;
    const topColor   = isPositive ? 'rgba(31,122,75,0.12)' : 'rgba(200,16,46,0.10)';

    series.applyOptions({ lineColor, topColor, bottomColor: 'rgba(0,0,0,0)' });
    series.setData(dedupeAscending(data.map(toSeriesPoint)));
    chartRef.current?.timeScale().fitContent();
  }, [data]);

  return <div ref={containerRef} style={{ width: '100%' }} />;
}

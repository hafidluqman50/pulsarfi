'use client';

import { useEffect, useRef, useState } from 'react';
import {
  createChart,
  LineSeries,
  ColorType,
  LineStyle,
  CrosshairMode,
  type IChartApi,
  type UTCTimestamp,
} from 'lightweight-charts';
import { Donut } from '@/components/charts/Donut';
import { getPortfolioChart } from '@/http/agent/chatApi';
import { getStockHistory } from '@/http/market/priceApi';

const COLORS = {
  canvas: '#fbfaf7',
  hairline: '#e3ddd2',
  body: '#6b635c',
  ink: '#16110e',
  positive: '#1f7a4b',
  negative: '#c8102e',
  merah: '#c8102e',
};

const DONUT_PALETTE = ['#c8102e', '#16110e', '#1f7a4b', '#6b635c', '#a8730a'];

const TIMEFRAMES = ['1D', '1M', '3M', '1Y', 'YTD'] as const;
type Timeframe = (typeof TIMEFRAMES)[number];

type ChartPayload = {
  chartQ?: string;
  lens: string;
  ticker?: string;
  lensNote?: string;
  data: unknown;
};

function parseDate(date: string): UTCTimestamp {
  return (Date.parse(date) / 1000) as UTCTimestamp;
}

type SeriesPoint = { time: UTCTimestamp; value: number };

function dedupeAscendingKeepLast(points: SeriesPoint[]): SeriesPoint[] {
  const sorted = [...points].sort((a, b) => a.time - b.time);
  const out: SeriesPoint[] = [];
  for (const p of sorted) {
    if (out.length > 0 && out[out.length - 1].time === p.time) {
      out[out.length - 1] = p;
    } else {
      out.push(p);
    }
  }
  return out;
}

function LineChart({ series }: { series: { name: string; color: string; points: SeriesPoint[] }[] }) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const chart: IChartApi = createChart(container, {
      width: container.clientWidth,
      height: 220,
      layout: {
        background: { type: ColorType.Solid, color: COLORS.canvas },
        textColor: COLORS.body,
        fontFamily: '"JetBrains Mono", ui-monospace, monospace',
        fontSize: 10,
        attributionLogo: false,
      },
      grid: {
        vertLines: { visible: false },
        horzLines: { color: COLORS.hairline, style: LineStyle.Dashed },
      },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderVisible: false, textColor: COLORS.body, scaleMargins: { top: 0.12, bottom: 0.08 } },
      timeScale: { borderVisible: false, timeVisible: true, secondsVisible: false },
    });

    for (const s of series) {
      const lineSeries = chart.addSeries(LineSeries, {
        color: s.color,
        lineWidth: 2,
        lastValueVisible: true,
        priceLineVisible: false,
        title: s.name,
      });
      lineSeries.setData(dedupeAscendingKeepLast(s.points));
    }
    chart.timeScale().fitContent();

    const observer = new ResizeObserver(() => chart.applyOptions({ width: container.clientWidth }));
    observer.observe(container);

    return () => {
      observer.disconnect();
      chart.remove();
    };
  }, [series]);

  return <div ref={containerRef} style={{ width: '100%' }} />;
}

function BarChart({ entries }: { entries: { label: string; value: number }[] }) {
  const max = Math.max(...entries.map((e) => Math.abs(e.value)), 1);
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {entries.map((e) => (
        <div key={e.label} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ width: 64, fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--body)', flex: 'none' }}>{e.label}</span>
          <div style={{ flex: 1, background: 'var(--hairline)', height: 14, position: 'relative' }}>
            <div style={{ width: `${(Math.abs(e.value) / max) * 100}%`, background: COLORS.merah, height: '100%' }} />
          </div>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--body)', flex: 'none', textAlign: 'right', width: 96 }}>
            {e.value.toLocaleString('id-ID', { maximumFractionDigits: 0 })}
          </span>
        </div>
      ))}
    </div>
  );
}

function EmptyChart() {
  return <div style={{ fontSize: 12.5, color: 'var(--body)', padding: '8px 0' }}>No data to chart.</div>;
}

function TimeframeTabs({ value, onChange }: { value: Timeframe; onChange: (t: Timeframe) => void }) {
  return (
    <div style={{ display: 'flex', gap: 4, marginBottom: 10 }}>
      {TIMEFRAMES.map((t) => (
        <button
          key={t}
          onClick={() => onChange(t)}
          style={{
            appearance: 'none',
            cursor: 'pointer',
            border: `1px solid ${t === value ? 'var(--ink)' : 'var(--hairline)'}`,
            background: t === value ? 'var(--ink)' : 'transparent',
            color: t === value ? 'var(--canvas)' : 'var(--body)',
            font: '600 10.5px/1 var(--font-mono)',
            padding: '5px 9px',
          }}
        >
          {t}
        </button>
      ))}
    </div>
  );
}

const TIME_SERIES_LENSES = new Set(['net_worth_vs_index', 'price_line', 'cumulative_return']);

export function PortfolioChart({ payload }: { payload: ChartPayload }) {
  const { lens, ticker } = payload;
  const [data, setData] = useState(payload.data);
  const [timeframe, setTimeframe] = useState<Timeframe>('1M');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setData(payload.data);
  }, [payload.data]);

  async function handleTimeframeChange(next: Timeframe) {
    setTimeframe(next);
    setLoading(true);
    try {
      if (lens === 'price_line' && ticker) {
        const history = await getStockHistory(ticker, next);
        // Full ISO datetime, not date-only (.slice(0, 10) used to truncate
        // this) — 1D/1W come back with minute-level points from the
        // backend (yahooRangeParams), and collapsing them all to the same
        // calendar date made every intraday point dedupe into one, so the
        // chart looked flat/broken for anything but a daily-or-coarser range.
        setData(history.map((p) => ({ date: new Date(p.timestamp).toISOString(), price: p.value })));
      } else {
        setData(await getPortfolioChart(lens, next));
      }
    } finally {
      setLoading(false);
    }
  }

  const tabs = TIME_SERIES_LENSES.has(lens) ? <TimeframeTabs value={timeframe} onChange={handleTimeframeChange} /> : null;
  const opacity = loading ? 0.5 : 1;

  if (lens === 'allocation') {
    const slices = (Array.isArray(data) ? data : []) as { ticker: string; valueIdr: string; percentage: number }[];
    if (slices.length === 0) return <EmptyChart />;
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
        <Donut data={slices.map((s) => ({ label: s.ticker, value: s.percentage }))} size={140} thickness={18} palette={DONUT_PALETTE} />
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {slices.map((s, i) => (
            <div key={s.ticker} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12 }}>
              <span style={{ width: 10, height: 10, background: DONUT_PALETTE[i % DONUT_PALETTE.length], display: 'inline-block', flex: 'none' }} />
              <span style={{ fontFamily: 'var(--font-mono)' }}>{s.ticker}</span>
              <span style={{ color: 'var(--body)' }}>{s.percentage.toFixed(1)}%</span>
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (lens === 'net_worth_vs_index') {
    const points = (Array.isArray(data) ? data : []) as { date: string; netWorthIdr: string; indexValue: number }[];
    return (
      <div style={{ opacity }}>
        {tabs}
        {points.length === 0 ? (
          <EmptyChart />
        ) : (
          <LineChart
            series={[
              { name: 'Net worth', color: COLORS.positive, points: points.map((p) => ({ time: parseDate(p.date), value: Number(p.netWorthIdr) })) },
              { name: 'IDX30', color: COLORS.body, points: points.map((p) => ({ time: parseDate(p.date), value: p.indexValue })) },
            ]}
          />
        )}
      </div>
    );
  }

  if (lens === 'price_line') {
    const points = (Array.isArray(data) ? data : []) as { date: string; price: number }[];
    return (
      <div style={{ opacity }}>
        {tabs}
        {points.length === 0 ? <EmptyChart /> : <LineChart series={[{ name: 'Price', color: COLORS.merah, points: points.map((p) => ({ time: parseDate(p.date), value: p.price })) }]} />}
      </div>
    );
  }

  if (lens === 'cumulative_return') {
    const points = (Array.isArray(data) ? data : []) as { date: string; returnPercent: number }[];
    return (
      <div style={{ opacity }}>
        {tabs}
        {points.length === 0 ? <EmptyChart /> : <LineChart series={[{ name: 'Return %', color: COLORS.positive, points: points.map((p) => ({ time: parseDate(p.date), value: p.returnPercent })) }]} />}
      </div>
    );
  }

  if (lens === 'comparison_bar') {
    const entries = (Array.isArray(data) ? data : []) as { label: string; value: number }[];
    if (entries.length === 0) return <EmptyChart />;
    return <BarChart entries={entries} />;
  }

  return <EmptyChart />;
}

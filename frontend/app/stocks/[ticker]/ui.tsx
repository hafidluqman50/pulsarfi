'use client';

import { useState, useMemo } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { Layout } from '@/components/layout/Layout';
import { SwapModal } from '@/components/ui/SwapModal';
import { PStockMark } from '@/components/ui/PStockMark';
import { AreaChart } from '@/components/charts/AreaChart';
import {
  PSTOCKS, PStock, Token, sliceRange,
  fmtIDRX, fmtPct, fmtNum, fmtAxisDate,
  TimePoint, DEFAULT_PORTFOLIO, Balances,
} from '@/lib/data';

function buildStockTimeSeries(stock: PStock, days = 365): TimePoint[] {
  let seed = 0;
  for (const character of stock.ticker) seed = ((seed * 31 + character.charCodeAt(0)) >>> 0);
  const rng = () => (seed = (seed * 1103515245 + 12345) >>> 0) / 0xffffffff;

  const endPrice    = stock.price;
  const startFactor = stock.change24h >= 0 ? 0.62 + rng() * 0.28 : 1.08 + rng() * 0.28;
  const startPrice  = endPrice * startFactor;
  const trend       = (endPrice - startPrice) / days;

  const outputPoints: TimePoint[] = [];
  let currentPrice = startPrice;
  const today = new Date(2026, 4, 23);

  for (let dayIndex = 0; dayIndex < days; dayIndex++) {
    const noise = (rng() - 0.5) * currentPrice * 0.024;
    const shock = rng() < 0.018 ? (rng() - 0.38) * currentPrice * 0.07 : 0;
    currentPrice = Math.max(endPrice * 0.22, currentPrice + trend + noise + shock);
    const date = new Date(today);
    date.setDate(date.getDate() - (days - 1 - dayIndex));
    outputPoints.push({ timestamp: date.getTime(), value: currentPrice });
  }
  outputPoints[outputPoints.length - 1].value = endPrice;
  return outputPoints;
}

const STOCK_NEWS: Record<string, { headline: string; source: string; time: string; tag: string }[]> = {
  BUMIP: [
    { headline: "Bumi Resources Coal Exports Hit 3.8Mt in April, Beating Consensus by 14%", source: "IDX Daily", time: "2h ago", tag: "Earnings" },
    { headline: "South Sumatra Mining Corridor Eyes Capacity Expansion Amid Asian Demand Surge", source: "Kontan", time: "5h ago", tag: "Sector" },
    { headline: "BUMI Targets Kalimantan Pit Stake Acquisition as Thermal Prices Rally", source: "Bloomberg ID", time: "1d ago", tag: "M&A" },
  ],
  ENRGP: [
    { headline: "Energi Mega Persada Signs Long-Term LNG Supply Agreement with South Korean Utility", source: "IDX Daily", time: "1h ago", tag: "Deal" },
    { headline: "Indonesia Gas Output Projected to Grow 12% Through 2027 on New Block Approvals", source: "Antara Ekonomi", time: "4h ago", tag: "Sector" },
    { headline: "ENRG Q1 Revenue Climbs 22% Year-on-Year on Elevated Realized Prices", source: "Kontan", time: "1d ago", tag: "Earnings" },
  ],
  KIJAP: [
    { headline: "Kawasan Industri Jababeka Secures Tier-1 EV Battery Manufacturer as Phase IV Anchor Tenant", source: "Bisnis ID", time: "3h ago", tag: "Leasing" },
    { headline: "Foreign Direct Investment Into Java Industrial Estates Rises 31% in Q1", source: "BPS Report", time: "6h ago", tag: "Sector" },
    { headline: "KIJA Announces 240-Hectare Land Bank Acquisition in Bekasi Expansion Zone", source: "IDX Daily", time: "2d ago", tag: "Expansion" },
  ],
  TLKMP: [
    { headline: "Telkom Indonesia Launches 5G Enterprise Corridor Across 12 Major Cities", source: "TechAsia", time: "2h ago", tag: "Product" },
    { headline: "TLKM Dividend Yield Reaches 6.1%, Drawing Institutional Rotation Into Telecoms", source: "IDX Daily", time: "5h ago", tag: "Dividend" },
    { headline: "Telkomsel Crosses 200M Subscribers on Broadband Push in Eastern Indonesia", source: "Kontan", time: "1d ago", tag: "Growth" },
  ],
  BBRIP: [
    { headline: "Bank Rakyat Indonesia NPL Ratio Drops to 2.4%, Five-Year Low on Credit Quality", source: "OJK Report", time: "1h ago", tag: "Credit" },
    { headline: "BRI Disburses Rp 145 Trillion in MSME Loans via Digital Channels in Q1", source: "Bisnis ID", time: "4h ago", tag: "Lending" },
    { headline: "BBRI Rights Issue 2.3x Oversubscribed on Positive Macro and Rural Banking Outlook", source: "IDX Daily", time: "2d ago", tag: "Capital" },
  ],
  GOTOP: [
    { headline: "GoTo Reports First EBITDA-Positive Quarter Since 2022 IPO on Cost Discipline", source: "DealStreetAsia", time: "30m ago", tag: "Earnings" },
    { headline: "Gojek Expands Into 14 New Cities With Electric Fleet Partner Swap Launched", source: "TechAsia", time: "3h ago", tag: "Expansion" },
    { headline: "Tokopedia GMV Grows 34% Year-on-Year as Social Commerce Flywheel Gains Momentum", source: "Kontan", time: "1d ago", tag: "Growth" },
  ],
  ASIIP: [
    { headline: "Astra International Posts Record EV Unit Sales in Q1 2026 on Tax Incentive Tailwind", source: "IDX Daily", time: "2h ago", tag: "Sales" },
    { headline: "ASII Agri Division Eyes Palm Oil Export Ramp-Up Following New Biofuel Mandate", source: "Bloomberg ID", time: "5h ago", tag: "Sector" },
    { headline: "Astra Financial Services AUM Crosses $12 Billion Mark as Retail Lending Expands", source: "Bisnis ID", time: "1d ago", tag: "Finance" },
  ],
  UNVRP: [
    { headline: "Unilever Indonesia Premiumizes Portfolio as Volume Growth Moderates in Low-Income Segment", source: "Kontan", time: "1h ago", tag: "Strategy" },
    { headline: "UNVR Rolls Out Plastic-Neutral Packaging Across Java Distribution Network", source: "Antara", time: "3h ago", tag: "ESG" },
    { headline: "Consumer Staples Index Outperforms Composite on Defensive Rotation by Foreign Funds", source: "IDX Daily", time: "1d ago", tag: "Sector" },
  ],
};

const TIMEFRAME_OPTIONS = ['1D', '1W', '1M', '3M', '1Y'] as const;

export function StockDetailPage() {
  const params = useParams();
  const router = useRouter();
  const ticker = typeof params.ticker === 'string' ? params.ticker : '';

  const stock = PSTOCKS.find(stockItem => stockItem.ticker === ticker);

  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('1M');
  const [tradeToken, setTradeToken]               = useState<Token | null>(null);
  const [balances, setBalances]                   = useState<Balances>(DEFAULT_PORTFOLIO);

  const fullSeries  = useMemo(() => stock ? buildStockTimeSeries(stock, 365) : [], [ticker]);
  const chartData   = useMemo(() => sliceRange(fullSeries, selectedTimeframe), [fullSeries, selectedTimeframe]);

  if (!stock) {
    return (
      <Layout>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', flex: 1, padding: '80px 24px' }}>
          <div style={{ textAlign: 'center' }}>
            <div className="display" style={{ fontSize: 32, marginBottom: 12 }}>Not found</div>
            <div style={{ color: 'var(--body)', marginBottom: 24 }}>No pStock with ticker "{ticker}"</div>
            <button className="btn btn-primary" onClick={() => router.push('/stocks')}>Back to Markets</button>
          </div>
        </div>
      </Layout>
    );
  }

  const isPositive  = stock.change24h >= 0;
  const newsItems   = STOCK_NEWS[stock.ticker] ?? [];

  return (
    <Layout>
      <div className="container pad-x" style={{ paddingTop: 28, paddingBottom: 64 }}>

        {/* Breadcrumb */}
        <nav style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 24 }}>
          <button
            onClick={() => router.push('/stocks')}
            style={{ appearance: 'none', border: 0, background: 'transparent', padding: 0, cursor: 'pointer', fontSize: 13, color: 'var(--body)', fontFamily: 'inherit' }}
          >
            Markets
          </button>
          <span style={{ color: 'var(--hairline-strong)', fontSize: 13 }}>/</span>
          <span className="mono" style={{ fontSize: 13, fontWeight: 700, color: 'var(--ink)' }}>{stock.ticker}</span>
        </nav>

        {/* Stock header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 28, gap: 16, flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
            <PStockMark ticker={stock.ticker} size={52} />
            <div>
              <div className="eyebrow" style={{ color: 'var(--body)', marginBottom: 4 }}>
                {stock.sector} · IDX: {stock.ipo} · Tokenized on Arbitrum
              </div>
              <div className="display" style={{ fontSize: 28, lineHeight: 1.15 }}>{stock.name}</div>
              <div style={{ marginTop: 10, display: 'flex', alignItems: 'baseline', gap: 16, flexWrap: 'wrap' }}>
                <span className="mono" style={{ fontSize: 28, fontWeight: 700, letterSpacing: '-0.02em' }}>
                  {fmtIDRX(stock.price)}
                </span>
                <span className="mono" style={{ fontSize: 15, color: isPositive ? 'var(--positive)' : 'var(--negative)' }}>
                  {fmtPct(stock.change24h)} 24h
                </span>
              </div>
            </div>
          </div>
          <button
            className="btn btn-merah"
            onClick={() => setTradeToken(stock)}
            style={{ padding: '13px 28px', fontSize: 14, flexShrink: 0 }}
          >
            Trade {stock.ticker}
          </button>
        </div>

        {/* Chart */}
        <div style={{ marginBottom: 20 }}>
          <div className="stock-chart-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
            <div className="range-pills">
              {TIMEFRAME_OPTIONS.map(timeframe => (
                <button
                  key={timeframe}
                  className={selectedTimeframe === timeframe ? 'active' : ''}
                  onClick={() => setSelectedTimeframe(timeframe)}
                >
                  {timeframe}
                </button>
              ))}
            </div>
            <span className="mono stock-supply-info" style={{ fontSize: 11, color: 'var(--body)' }}>
              {fmtNum(stock.supply, 0)} tokens in circulation
            </span>
          </div>
          <div style={{ border: '1px solid var(--hairline)', background: 'var(--putih)', padding: '8px 0 0' }}>
            <AreaChart
              data={chartData}
              height={260}
              valueFormatter={value => fmtIDRX(value)}
              labelFormatter={(timestamp, range) => fmtAxisDate(timestamp, range)}
              range={selectedTimeframe}
            />
          </div>
        </div>

        {/* Stats row */}
        <div className="stock-stats-grid">
          {[
            { label: 'IDX Ticker', value: stock.ipo },
            { label: 'Sector',     value: stock.sector },
            { label: 'Supply',     value: fmtNum(stock.supply, 0) },
            { label: 'Mkt Cap',    value: fmtIDRX(stock.price * stock.supply) },
          ].map(statItem => (
            <div key={statItem.label} className="stock-stats-cell">
              <div className="eyebrow" style={{ fontSize: 9, marginBottom: 5, color: 'var(--body)' }}>{statItem.label}</div>
              <div className="mono" style={{ fontSize: 14, fontWeight: 700 }}>{statItem.value}</div>
            </div>
          ))}
        </div>

        {/* News */}
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 14 }}>
            <span className="eyebrow">Market Intelligence</span>
            <span className="mono" style={{ fontSize: 10, color: 'var(--body)' }}>IDX · Realtime Feed</span>
          </div>
          <div style={{ border: '1px solid var(--hairline)', background: 'var(--putih)' }}>
            {newsItems.map((newsItem, newsIndex) => (
              <div
                key={newsIndex}
                style={{ padding: '18px 20px', borderBottom: newsIndex < newsItems.length - 1 ? '1px solid var(--hairline)' : 'none' }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16 }}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontWeight: 600, fontSize: 14, lineHeight: 1.45, marginBottom: 8 }}>{newsItem.headline}</div>
                    <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                      <span className="eyebrow" style={{ fontSize: 9, color: 'var(--body)' }}>{newsItem.source}</span>
                      <span style={{ fontSize: 11, color: 'var(--body)' }}>{newsItem.time}</span>
                    </div>
                  </div>
                  <span
                    className="eyebrow"
                    style={{ fontSize: 9, padding: '3px 8px', border: '1px solid var(--hairline)', color: 'var(--body)', flexShrink: 0, marginTop: 2 }}
                  >
                    {newsItem.tag}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>

      </div>

      {tradeToken && (
        <SwapModal
          defaultOut={tradeToken}
          balances={balances}
          setBalances={setBalances}
          onClose={() => setTradeToken(null)}
        />
      )}
    </Layout>
  );
}

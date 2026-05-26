import { PStock, TimePoint } from '@/lib/data';

export const STOCK_TIMEFRAME_OPTIONS = ['1D', '1W', '1M', '3M', '1Y'] as const;
export const STOCK_LOT_SIZE = 100;

export function buildStockTimeSeries(stock: PStock, days = 365): TimePoint[] {
  let seed = 0;
  for (const character of stock.ticker) seed = ((seed * 31 + character.charCodeAt(0)) >>> 0);
  const rng = () => (seed = (seed * 1103515245 + 12345) >>> 0) / 0xffffffff;

  const endPrice = stock.price;
  const startFactor = stock.change24h >= 0 ? 0.62 + rng() * 0.28 : 1.08 + rng() * 0.28;
  const startPrice = endPrice * startFactor;
  const trend = (endPrice - startPrice) / days;
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

export function buildMinuteTimeSeries(values: number[]): TimePoint[] {
  const now = Date.now();
  const start = now - Math.max(values.length - 1, 0) * 60_000;
  return values.map((value, index) => ({
    timestamp: start + index * 60_000,
    value,
  }));
}

export function rawTokenToNumber(raw?: string, decimals = 18): number | null {
  if (!raw) return null;
  const clean = raw.replace(/[^0-9]/g, '') || '0';
  const padded = clean.padStart(decimals + 1, '0');
  const whole = padded.slice(0, -decimals) || '0';
  const fraction = padded.slice(-decimals).replace(/0+$/, '').slice(0, 6);
  const value = Number(fraction ? `${whole}.${fraction}` : whole);
  return Number.isFinite(value) ? value : null;
}

export const STOCK_NEWS: Record<string, { headline: string; source: string; time: string; tag: string }[]> = {
  BUMIP: [
    { headline: 'Bumi Resources Coal Exports Hit 3.8Mt in April, Beating Consensus by 14%', source: 'IDX Daily', time: '2h ago', tag: 'Earnings' },
    { headline: 'South Sumatra Mining Corridor Eyes Capacity Expansion Amid Asian Demand Surge', source: 'Kontan', time: '5h ago', tag: 'Sector' },
    { headline: 'BUMI Targets Kalimantan Pit Stake Acquisition as Thermal Prices Rally', source: 'Bloomberg ID', time: '1d ago', tag: 'M&A' },
  ],
  ENRGP: [
    { headline: 'Energi Mega Persada Signs Long-Term LNG Supply Agreement with South Korean Utility', source: 'IDX Daily', time: '1h ago', tag: 'Deal' },
    { headline: 'Indonesia Gas Output Projected to Grow 12% Through 2027 on New Block Approvals', source: 'Antara Ekonomi', time: '4h ago', tag: 'Sector' },
    { headline: 'ENRG Q1 Revenue Climbs 22% Year-on-Year on Elevated Realized Prices', source: 'Kontan', time: '1d ago', tag: 'Earnings' },
  ],
  KIJAP: [
    { headline: 'Kawasan Industri Jababeka Secures Tier-1 EV Battery Manufacturer as Phase IV Anchor Tenant', source: 'Bisnis ID', time: '3h ago', tag: 'Leasing' },
    { headline: 'Foreign Direct Investment Into Java Industrial Estates Rises 31% in Q1', source: 'BPS Report', time: '6h ago', tag: 'Sector' },
    { headline: 'KIJA Announces 240-Hectare Land Bank Acquisition in Bekasi Expansion Zone', source: 'IDX Daily', time: '2d ago', tag: 'Expansion' },
  ],
  TLKMP: [
    { headline: 'Telkom Indonesia Launches 5G Enterprise Corridor Across 12 Major Cities', source: 'TechAsia', time: '2h ago', tag: 'Product' },
    { headline: 'TLKM Dividend Yield Reaches 6.1%, Drawing Institutional Rotation Into Telecoms', source: 'IDX Daily', time: '5h ago', tag: 'Dividend' },
    { headline: 'Telkomsel Crosses 200M Subscribers on Broadband Push in Eastern Indonesia', source: 'Kontan', time: '1d ago', tag: 'Growth' },
  ],
  BBRIP: [
    { headline: 'Bank Rakyat Indonesia NPL Ratio Drops to 2.4%, Five-Year Low on Credit Quality', source: 'OJK Report', time: '1h ago', tag: 'Credit' },
    { headline: 'BRI Disburses Rp 145 Trillion in MSME Loans via Digital Channels in Q1', source: 'Bisnis ID', time: '4h ago', tag: 'Lending' },
    { headline: 'BBRI Rights Issue 2.3x Oversubscribed on Positive Macro and Rural Banking Outlook', source: 'IDX Daily', time: '2d ago', tag: 'Capital' },
  ],
  GOTOP: [
    { headline: 'GoTo Reports First EBITDA-Positive Quarter Since 2022 IPO on Cost Discipline', source: 'DealStreetAsia', time: '30m ago', tag: 'Earnings' },
    { headline: 'Gojek Expands Into 14 New Cities With Electric Fleet Partner Swap Launched', source: 'TechAsia', time: '3h ago', tag: 'Expansion' },
    { headline: 'Tokopedia GMV Grows 34% Year-on-Year as Social Commerce Flywheel Gains Momentum', source: 'Kontan', time: '1d ago', tag: 'Growth' },
  ],
  ASIIP: [
    { headline: 'Astra International Posts Record EV Unit Sales in Q1 2026 on Tax Incentive Tailwind', source: 'IDX Daily', time: '2h ago', tag: 'Sales' },
    { headline: 'ASII Agri Division Eyes Palm Oil Export Ramp-Up Following New Biofuel Mandate', source: 'Bloomberg ID', time: '5h ago', tag: 'Sector' },
    { headline: 'Astra Financial Services AUM Crosses $12 Billion Mark as Retail Lending Expands', source: 'Bisnis ID', time: '1d ago', tag: 'Finance' },
  ],
  UNVRP: [
    { headline: 'Unilever Indonesia Premiumizes Portfolio as Volume Growth Moderates in Low-Income Segment', source: 'Kontan', time: '1h ago', tag: 'Strategy' },
    { headline: 'UNVR Rolls Out Plastic-Neutral Packaging Across Java Distribution Network', source: 'Antara', time: '3h ago', tag: 'ESG' },
    { headline: 'Consumer Staples Index Outperforms Composite on Defensive Rotation by Foreign Funds', source: 'IDX Daily', time: '1d ago', tag: 'Sector' },
  ],
};

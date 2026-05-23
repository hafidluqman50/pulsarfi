export interface PStock {
  ticker: string;
  name: string;
  sector: string;
  price: number;
  change24h: number;
  supply: number;
  ipo: string;
  isStable?: false;
}

export interface Stable {
  ticker: string;
  name: string;
  price: number;
  isStable: true;
  sector?: string;
  supply?: number;
  ipo?: string;
  change24h?: number;
}

export type Token = PStock | Stable;

// Prices denominated in IDRX (1 IDRX = 1 IDR, rate ~16,142 IDR/USD)
export const PSTOCKS: PStock[] = [
  { ticker: "BUMIP", name: "Pulsar Bumi Resources",       sector: "Energy",         price: 245,   change24h: +4.21, supply: 18_240_000, ipo: "BUMI" },
  { ticker: "ENRGP", name: "Pulsar Energi Mega",           sector: "Energy",         price: 378,   change24h: +1.84, supply: 12_400_000, ipo: "ENRG" },
  { ticker: "KIJAP", name: "Pulsar Kawasan Industri",      sector: "Infrastructure", price: 144,   change24h: -0.61, supply: 9_120_000,  ipo: "KIJA" },
  { ticker: "TLKMP", name: "Pulsar Telkom Indonesia",      sector: "Telecom",        price: 2_973, change24h: +0.42, supply: 6_800_000,  ipo: "TLKM" },
  { ticker: "BBRIP", name: "Pulsar Bank Rakyat",           sector: "Financial",      price: 4_779, change24h: -1.12, supply: 5_240_000,  ipo: "BBRI" },
  { ticker: "GOTOP", name: "Pulsar GoTo Gojek Tokopedia",  sector: "Technology",     price: 98,    change24h: +7.93, supply: 28_900_000, ipo: "GOTO" },
  { ticker: "ASIIP", name: "Pulsar Astra International",   sector: "Industrials",    price: 5_188, change24h: +0.18, supply: 4_120_000,  ipo: "ASII" },
  { ticker: "UNVRP", name: "Pulsar Unilever Indonesia",    sector: "Consumer",       price: 2_400, change24h: -0.34, supply: 3_840_000,  ipo: "UNVR" },
];

export const STABLES: Stable[] = [
  { ticker: "IDRX", name: "Indonesian Rupiah X", price: 1.0, isStable: true },
];

export const ALL_TOKENS: Token[] = [...STABLES, ...PSTOCKS];

export type Balances = Record<string, number>;

export const DEFAULT_PORTFOLIO: Balances = {
  "IDRX":  5_000_000,
  "BUMIP": 84_320,
  "ENRGP": 21_500,
  "TLKMP": 4_120,
  "GOTOP": 156_400,
};

export const DEFAULT_COST_BASIS: Record<string, number> = {
  "IDRX":  1.00,
  "BUMIP": 201,
  "ENRGP": 405,
  "TLKMP": 3_054,
  "BBRIP": 4_910,
  "GOTOP": 78,
  "KIJAP": 152,
  "ASIIP": 5_246,
  "UNVRP": 2_437,
};

export function tokenByTicker(ticker: string): Token {
  return ALL_TOKENS.find(token => token.ticker === ticker) ?? STABLES[0];
}

export function seriesFor(ticker: string, points = 24): number[] {
  let seed = 0;
  for (const character of ticker) seed = ((seed * 31 + character.charCodeAt(0)) >>> 0);
  const rng = () => (seed = (seed * 1103515245 + 12345) >>> 0) / 0xffffffff;
  const outputValues: number[] = [];
  let currentValue = 100;
  for (let pointIndex = 0; pointIndex < points; pointIndex++) {
    currentValue += (rng() - 0.5) * 6 + (rng() - 0.45) * 2;
    outputValues.push(currentValue);
  }
  return outputValues;
}

export interface TimePoint { timestamp: number; value: number; }

export function portfolioTimeSeries(seedTicker = "PORTFOLIO", days = 365, start = 18000, end = 26840): TimePoint[] {
  let seed = 0;
  for (const character of seedTicker) seed = ((seed * 31 + character.charCodeAt(0)) >>> 0);
  const rng = () => (seed = (seed * 1103515245 + 12345) >>> 0) / 0xffffffff;
  const outputPoints: TimePoint[] = [];
  const trend = (end - start) / days;
  let currentValue = start;
  const today = new Date(2026, 4, 23);
  for (let dayIndex = 0; dayIndex < days; dayIndex++) {
    const noise = (rng() - 0.5) * currentValue * 0.018;
    const event = rng() < 0.015 ? (rng() - 0.4) * currentValue * 0.06 : 0;
    currentValue = Math.max(start * 0.6, currentValue + trend + noise + event);
    const date = new Date(today);
    date.setDate(date.getDate() - (days - 1 - dayIndex));
    outputPoints.push({ timestamp: date.getTime(), value: currentValue });
  }
  return outputPoints;
}

export function sliceRange(series: TimePoint[], range: string): TimePoint[] {
  const daysByRange: Record<string, number> = { "1D": 1, "1W": 7, "1M": 30, "3M": 90, "1Y": 365, "ALL": series.length };
  const pointCount = Math.min(series.length, daysByRange[range] || 30);
  if (range === "1D") {
    const lastValue = series[series.length - 1].value;
    const previousValue = series[series.length - 2]?.value ?? lastValue;
    const today = new Date(2026, 4, 23);
    const hourlyPoints: TimePoint[] = [];
    for (let hourIndex = 0; hourIndex < 24; hourIndex++) {
      const hourFraction = hourIndex / 23;
      const drift = previousValue + (lastValue - previousValue) * hourFraction;
      const noise = Math.sin(hourIndex * 0.6) * lastValue * 0.004 + (Math.random() - 0.5) * lastValue * 0.003;
      const date = new Date(today);
      date.setHours(hourIndex, 0, 0, 0);
      hourlyPoints.push({ timestamp: date.getTime(), value: drift + noise });
    }
    return hourlyPoints;
  }
  return series.slice(series.length - pointCount);
}

export function fmtIDRX(value: number, decimals = 0): string {
  if (value == null || isNaN(value)) return "—";
  return value.toLocaleString("id-ID", { minimumFractionDigits: decimals, maximumFractionDigits: decimals }) + " IDRX";
}

// Keep fmtUSD only for network fees (still charged in USD on Arbitrum)
export function fmtUSD(value: number, opts: { min?: number; max?: number } = {}): string {
  if (value == null || isNaN(value)) return "—";
  const min = opts.min ?? 2;
  const max = opts.max ?? 2;
  return value.toLocaleString("en-US", { style: "currency", currency: "USD", minimumFractionDigits: min, maximumFractionDigits: max });
}

export function fmtNum(value: number, max = 4): string {
  if (value == null || isNaN(value)) return "—";
  return value.toLocaleString("en-US", { maximumFractionDigits: max });
}

export function fmtPct(value: number): string {
  if (value == null || isNaN(value)) return "—";
  return `${value >= 0 ? "+" : ""}${value.toFixed(2)}%`;
}

export function shortAddr(address: string): string {
  if (!address) return "";
  return `${address.slice(0, 6)}…${address.slice(-4)}`;
}

export function fmtIDR(value: number): string {
  if (value == null || isNaN(value)) return "—";
  return "Rp " + Math.round(value).toLocaleString("id-ID");
}

export function fmtAxisDate(timestamp: number, range: string): string {
  const date = new Date(timestamp);
  const formatOptions = (() => {
    if (range === "1D")      return { hour: "2-digit" as const, minute: "2-digit" as const };
    if (range === "1D-full") return { hour: "2-digit" as const, minute: "2-digit" as const };
    if (range === "1W")      return { weekday: "short" as const, day: "2-digit" as const };
    if (range === "1M")      return { day: "2-digit" as const, month: "short" as const };
    if (range === "3M")      return { day: "2-digit" as const, month: "short" as const };
    if (range === "1Y")      return { month: "short" as const, year: "2-digit" as const };
    if (range === "ALL")     return { month: "short" as const, year: "2-digit" as const };
    if (range === "tooltip") return { day: "2-digit" as const, month: "short" as const, year: "numeric" as const };
    return { day: "2-digit" as const, month: "short" as const };
  })();
  return date.toLocaleDateString("en-GB", formatOptions);
}

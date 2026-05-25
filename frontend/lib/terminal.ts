export interface LogLine {
  timestamp: string;
  level: string;
  text: string;
}

export const DEFAULT_LOG: LogLine[] = [
  { timestamp: "14:02:11.482", level: "INFO", text: "[boot] horizon-bridge v0.4.1 starting · operator=msig-3/5" },
  { timestamp: "14:02:11.501", level: "INFO", text: "[rpc]  connected → arbitrum-sepolia · chain=421614 · height=98,341,008" },
  { timestamp: "14:02:11.518", level: "INFO", text: "[idx]  connected → broker.mirae.co.id · session=valid" },
  { timestamp: "14:02:12.044", level: "OK",   text: "[oracle] price feed warm · 8 markets · staleness=2.1s" },
  { timestamp: "14:02:12.231", level: "OK",   text: "[attest] last proof-of-reserves verified · 8 min ago" },
  { timestamp: "14:02:12.402", level: "INFO", text: "[ready] awaiting operator command…" },
];

export function currentTimestamp(): string {
  const now = new Date();
  const pad = (n: number) => (n < 10 ? "0" + n : "" + n);
  return `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}.${("00" + now.getMilliseconds()).slice(-3)}`;
}

export function randomHex(length: number): string {
  return Array.from({ length }, () => Math.floor(Math.random() * 16).toString(16)).join("");
}

export function randomDigits(length: number): string {
  return Array.from({ length }, () => Math.floor(Math.random() * 10)).join("");
}

export const delay = (ms: number) => new Promise<void>(resolve => setTimeout(resolve, ms));

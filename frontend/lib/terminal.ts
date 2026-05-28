export interface LogLine {
  timestamp: string;
  level: string;
  text: string;
}


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

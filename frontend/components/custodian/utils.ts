import type { CustodianRequest, ReserveEntry } from '@/http/custodian/custodianApi';

export function requestKey(request: CustodianRequest): string {
  return `${request.kind}-${request.on_chain_id}`;
}

export function queueSummary(requests?: CustodianRequest[]): string {
  if (!requests) return "loading requests";
  const retail = requests.filter(r => r.source === "retail").length;
  const institutional = requests.filter(r => r.source === "institutional").length;
  return `${requests.length} pending · ${retail} retail · ${institutional} institutional`;
}

export function lastAttestationLabel(entries: ReserveEntry[]): string {
  if (!entries.length) return "last attestation · none";
  const latest = entries.reduce((current, entry) =>
    new Date(entry.last_attested_at).getTime() > new Date(current.last_attested_at).getTime() ? entry : current
  );
  return `last attestation · ${relativeAge(latest.last_attested_at)}`;
}

export function formatRawToken(raw: string, decimals = 18): string {
  if (!raw) return "—";
  const clean = raw.replace(/[^0-9]/g, "") || "0";
  const padded = clean.padStart(decimals + 1, "0");
  const whole = padded.slice(0, -decimals) || "0";
  const fraction = padded.slice(-decimals).replace(/0+$/, "").slice(0, 4);
  const wholeFormatted = Number(whole).toLocaleString("en-US");
  return fraction ? `${wholeFormatted}.${fraction}` : wholeFormatted;
}

export function formatRawIDR(raw: string): string {
  const clean = raw.replace(/[^0-9]/g, "") || "0";
  const padded = clean.padStart(3, "0");
  const whole = padded.slice(0, -2) || "0";
  return `Rp ${Number(whole).toLocaleString("id-ID")}`;
}

export function relativeAge(value: string): string {
  const then = new Date(value).getTime();
  if (!then) return "—";
  const diffSeconds = Math.max(0, Math.floor((Date.now() - then) / 1000));
  if (diffSeconds < 60) return `${diffSeconds}s ago`;
  const diffMinutes = Math.floor(diffSeconds / 60);
  if (diffMinutes < 60) return `${diffMinutes} min`;
  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return `${diffHours} h`;
  return `${Math.floor(diffHours / 24)} d`;
}

export function shortHash(hash: string): string {
  return `${hash.slice(0, 10)}…${hash.slice(-6)}`;
}

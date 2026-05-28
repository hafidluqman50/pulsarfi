'use client';

import { useAccount } from 'wagmi';
import { fmtIDRCompact, shortAddr } from '@/lib/data';
import { useCustodianRequests, useCustodianStats, useReserves } from '@/http/custodian/hooks';
import { MintOrderForm } from './MintOrderForm';
import { RequestQueue } from './RequestQueue';
import { ReservesTable } from './ReservesTable';
import { queueSummary, lastAttestationLabel } from './utils';

function Metric({ label, value, sub, tone = "default", isLoading = false }: { label: string; value: string; sub: string; tone?: "ink" | "merah" | "default"; isLoading?: boolean }): React.ReactNode {
  const isInk = tone === "ink", isMerah = tone === "merah";
  return (
    <div className="stat-card" style={{ padding: 22, background: isInk ? "var(--ink)" : isMerah ? "var(--merah)" : "var(--canvas)", color: (isInk || isMerah) ? "var(--putih)" : "var(--ink)", border: "1px solid " + (isInk ? "var(--ink)" : isMerah ? "var(--merah)" : "var(--hairline)") }}>
      <div className="eyebrow" style={{ color: (isInk || isMerah) ? "rgba(255,255,255,0.7)" : "var(--body)" }}>{label}</div>
      {isLoading
        ? <div className="skeleton" style={{ height: 38, width: 140, marginTop: 8 }} />
        : <div className="display tnum" style={{ fontSize: 32, marginTop: 8, lineHeight: 1, letterSpacing: "-0.02em" }}>{value}</div>
      }
      <div style={{ marginTop: 8, fontSize: 12, color: (isInk || isMerah) ? "rgba(255,255,255,0.6)" : "var(--body)" }}>{sub}</div>
    </div>
  );
}


export function CustodianView(): React.ReactNode {
  const { address, isConnected } = useAccount();
  const { data: stats, isLoading: statsLoading } = useCustodianStats();
  const { data: requestsData, isLoading: requestsLoading } = useCustodianRequests();
  const { data: reserves, isLoading: reservesLoading } = useReserves();

  return (
    <div className="pad-x" style={{ padding: "32px 24px 16px" }}>
      <div className="hairline-strong" style={{ paddingBottom: 18 }}>
        <div className="eyebrow" style={{ color: "var(--merah)", marginBottom: 12 }}>Horizon Labs · Custodian Console · {isConnected ? shortAddr(address!) : "OPS-MASTER"}</div>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-end", gap: 24, flexWrap: "wrap" }}>
          <div>
            <h1 className="display hero-display" style={{ fontSize: 56, margin: 0, lineHeight: 1, letterSpacing: "-0.028em" }}>
              The <span className="display-it">custodian bridge</span>.
            </h1>
            <p style={{ marginTop: 12, color: "var(--body)", maxWidth: 580, fontFamily: '"Fraunces", serif', fontSize: 17, fontWeight: 300 }}>
              Trigger institutional buy & sell orders on IDX. Each fill is attested by the custodian and triggers 1:1 mint or burn of pStock supply on Arbitrum.
            </p>
          </div>
        </div>
      </div>

      <div className="grid-4col" style={{ marginTop: 24 }}>
        <Metric label="Assets Under Custody" value={stats ? fmtIDRCompact(stats.assets_under_custody_idr) : "—"} sub="total custodian holdings" tone="ink" isLoading={statsLoading} />
        <Metric label="24h Mint Volume" value={stats ? fmtIDRCompact(String(parseFloat(stats.mint_volume_24h_idrx) / 100)) : "—"} sub={stats ? `${stats.mint_count_24h} mints · ${stats.burn_count_24h} burns` : "—"} tone="merah" isLoading={statsLoading} />
        <Metric label="Pending requests" value={stats ? String(stats.pending_requests.total) : "—"} sub={stats ? `${stats.pending_requests.mints} mint · ${stats.pending_requests.redeems} redeem` : "—"} isLoading={statsLoading} />
        <Metric label="IDR settlement vault" value="Rp 12.8 M" sub="@ Bank Mandiri 1900" />
      </div>

      <MintOrderForm />

      <div style={{ marginTop: 56 }}>
        <div className="hairline-strong" style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", paddingBottom: 12, flexWrap: "wrap", gap: 8 }}>
          <h2 className="display section-title" style={{ fontSize: 32, margin: 0, letterSpacing: "-0.02em" }}>Request queue</h2>
          <div className="eyebrow" style={{ color: "var(--body)" }}>{queueSummary(requestsData?.items)}</div>
        </div>
        <RequestQueue requests={requestsData?.items ?? []} isLoading={requestsLoading} currentAddress={address} />
      </div>

      <div style={{ marginTop: 56 }}>
        <div className="hairline-strong" style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", paddingBottom: 12, flexWrap: "wrap", gap: 8 }}>
          <h2 className="display section-title" style={{ fontSize: 32, margin: 0, letterSpacing: "-0.02em" }}>Physical vault & proof of reserves</h2>
          <div className="eyebrow" style={{ color: "var(--body)" }}>{lastAttestationLabel(reserves ?? [])}</div>
        </div>
        <ReservesTable entries={reserves ?? []} isLoading={reservesLoading} />
      </div>
    </div>
  );
}

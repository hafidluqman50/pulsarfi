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
    <div className={`stat-card border p-[22px] ${
      isInk
        ? "border-[var(--ink)] bg-[var(--ink)] text-[var(--putih)]"
        : isMerah
          ? "border-[var(--merah)] bg-[var(--merah)] text-[var(--putih)]"
          : "border-[var(--hairline)] bg-[var(--canvas)] text-[var(--ink)]"
    }`}>
      <div className={`eyebrow ${isInk || isMerah ? "!text-[rgba(255,255,255,0.7)]" : "!text-[var(--body)]"}`}>{label}</div>
      {isLoading
        ? <div className="skeleton mt-[8px] h-[38px] w-[140px]" />
        : <div className="display tnum mt-[8px] !text-[32px] !leading-none !tracking-[-0.02em]">{value}</div>
      }
      <div className={`mt-[8px] text-[12px] ${isInk || isMerah ? "text-[rgba(255,255,255,0.6)]" : "text-[var(--body)]"}`}>{sub}</div>
    </div>
  );
}


export function CustodianView(): React.ReactNode {
  const { address, isConnected } = useAccount();
  const { data: stats, isLoading: statsLoading } = useCustodianStats();
  const { data: requestsData, isLoading: requestsLoading } = useCustodianRequests();
  const { data: reserves, isLoading: reservesLoading } = useReserves();

  return (
    <div className="pad-x !px-[24px] !pb-[16px] !pt-[32px]">
      <div className="hairline-strong pb-[18px]">
        <div className="eyebrow mb-[12px] !text-[var(--merah)]">Horizon Labs · Custodian Console · {isConnected ? shortAddr(address!) : "OPS-MASTER"}</div>
        <div className="flex flex-wrap items-end justify-between gap-[24px]">
          <div>
            <h1 className="display hero-display !m-[0] !text-[56px] !leading-none !tracking-[-0.028em]">
              The <span className="display-it">custodian bridge</span>.
            </h1>
            <p className="mt-[12px] max-w-[580px] text-[17px] font-light text-[var(--body)] [font-family:var(--font-fraunces,_Fraunces,_serif)]">
              Trigger institutional buy & sell orders on IDX. Each fill is attested by the custodian and triggers 1:1 mint or burn of pStock supply on Arbitrum.
            </p>
          </div>
        </div>
      </div>

      <div className="grid-4col mt-[24px]">
        <Metric label="Assets Under Custody" value={stats ? fmtIDRCompact(stats.assets_under_custody_idr) : "—"} sub="total custodian holdings" tone="ink" isLoading={statsLoading} />
        <Metric label="24h Mint Volume" value={stats ? fmtIDRCompact(String(parseFloat(stats.mint_volume_24h_idrx) / 100)) : "—"} sub={stats ? `${stats.mint_count_24h} mints · ${stats.burn_count_24h} burns` : "—"} tone="merah" isLoading={statsLoading} />
        <Metric label="Pending requests" value={stats ? String(stats.pending_requests.total) : "—"} sub={stats ? `${stats.pending_requests.mints} mint · ${stats.pending_requests.redeems} redeem` : "—"} isLoading={statsLoading} />
        <Metric label="IDR settlement vault" value="Rp 12.8 M" sub="@ Bank Mandiri 1900" />
      </div>

      <MintOrderForm />

      <div className="mt-[56px]">
        <div className="hairline-strong flex flex-wrap items-baseline justify-between gap-[8px] pb-[12px]">
          <h2 className="display section-title !m-[0] !text-[32px] !tracking-[-0.02em]">Request queue</h2>
          <div className="eyebrow !text-[var(--body)]">{queueSummary(requestsData?.items)}</div>
        </div>
        <RequestQueue requests={requestsData?.items ?? []} isLoading={requestsLoading} currentAddress={address} />
      </div>

      <div className="mt-[56px]">
        <div className="hairline-strong flex flex-wrap items-baseline justify-between gap-[8px] pb-[12px]">
          <h2 className="display section-title !m-[0] !text-[32px] !tracking-[-0.02em]">Physical vault & proof of reserves</h2>
          <div className="eyebrow !text-[var(--body)]">{lastAttestationLabel(reserves ?? [])}</div>
        </div>
        <ReservesTable entries={reserves ?? []} isLoading={reservesLoading} />
      </div>
    </div>
  );
}

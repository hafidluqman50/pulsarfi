'use client';

import { useAccount } from 'wagmi';
import { fmtIDRCompact, shortAddr } from '@/lib/data';
import { useCustodianRequests, useCustodianStats, useReserves, useWalletVerifications } from '@/http/custodian/hooks';
import { KYCRegistry } from './KYCRegistry';
import { MintOrderForm } from './MintOrderForm';
import { RequestQueue } from './RequestQueue';
import { VaultBlocks } from './VaultBlocks';
import { ReservesTable } from './ReservesTable';
import { queueSummary, lastAttestationLabel } from './utils';

const METRIC_TONES = {
  ink: {
    box: 'border-[var(--ink)] bg-[var(--ink)] text-[var(--putih)] shadow-[0_5px_0_-2px_#fbfaf7,0_6px_0_-2px_#16110e,0_11px_0_-4px_#fbfaf7,0_12px_0_-4px_#4a423d,0_24px_30px_-16px_rgba(22,17,14,0.35)]',
    sub: 'text-[rgba(255,255,255,0.7)]',
  },
  merah: {
    box: 'border-[var(--merah)] bg-[var(--merah)] text-[var(--putih)] shadow-[0_5px_0_-2px_#fbfaf7,0_6px_0_-2px_#c8102e,0_11px_0_-4px_#fbfaf7,0_12px_0_-4px_#f0d3cf,0_24px_30px_-16px_rgba(154,12,36,0.35)]',
    sub: 'text-[rgba(255,255,255,0.75)]',
  },
  default: {
    box: 'border-[var(--hairline)] bg-[var(--canvas)] text-[var(--ink)] shadow-[0_5px_0_-2px_#fbfaf7,0_6px_0_-2px_#e3ddd2,0_11px_0_-4px_#fbfaf7,0_12px_0_-4px_#e3ddd2,0_24px_30px_-16px_rgba(22,17,14,0.2)]',
    sub: 'text-[var(--body)]',
  },
} as const;

const SECTION_HEADER = 'flex flex-wrap items-baseline justify-between gap-[8px] border-b border-[var(--ink)] pb-[12px]';
const SECTION_TITLE = 'display m-[0] !text-[32px] !leading-[normal] !tracking-[-0.02em]';
const SECTION_LABEL = 'text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--body)]';

function Metric({ label, value, sub, tone = "default", isLoading = false }: { label: string; value: string; sub: string; tone?: "ink" | "merah" | "default"; isLoading?: boolean }): React.ReactNode {
  const look = METRIC_TONES[tone];
  return (
    <div className={`border p-[22px] transition-transform duration-300 ease-[cubic-bezier(.2,.7,.3,1)] hover:-translate-y-[4px] ${look.box}`}>
      <div className={`text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] ${look.sub}`}>{label}</div>
      {isLoading
        ? <div className="mt-[8px] h-[32px] w-[140px] bg-[var(--canvas-soft)]" />
        : <div className="display tnum mt-[8px] !text-[32px] !leading-none !tracking-[-0.02em]">{value}</div>
      }
      <div className={`mt-[8px] text-[12px] ${look.sub}`}>{sub}</div>
    </div>
  );
}

export function CustodianView(): React.ReactNode {
  const { address, isConnected } = useAccount();
  const { data: stats, isLoading: statsLoading } = useCustodianStats();
  const { data: requestsData, isLoading: requestsLoading } = useCustodianRequests();
  const { data: reserves, isLoading: reservesLoading } = useReserves();
  const { data: kycRecords, isLoading: kycLoading } = useWalletVerifications();

  return (
    <div className="mx-auto w-full max-w-[1440px] px-[32px] pb-[64px] pt-[32px] max-[719px]:px-[16px] max-[719px]:pb-[48px] max-[719px]:pt-[24px]">
      <div className="border-b border-[var(--ink)] pb-[18px]">
        <div className="mb-[12px] text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--merah)]">Horizon Labs · Custodian Console · {isConnected ? shortAddr(address!) : "OPS-MASTER"}</div>
        <h1 className="display m-[0] !text-[length:clamp(40px,4.6vw,56px)] !leading-none !tracking-[-0.028em]">
          The <span className="display-it">custodian bridge</span>.
        </h1>
        <p className="mt-[12px] max-w-[580px] text-[17px] font-[300] leading-[1.55] text-[var(--body)] [font-family:var(--font-fraunces,_Fraunces,_serif)]">
          Trigger institutional buy & sell orders on IDX. Each fill is attested by the custodian and triggers 1:1 mint or burn of pStock supply on Arbitrum.
        </p>
      </div>

      <div className="mt-[28px] grid grid-cols-[repeat(auto-fit,minmax(min(100%,220px),1fr))] gap-[24px]">
        <Metric label="Assets Under Custody" value={stats ? fmtIDRCompact(stats.assets_under_custody_idr) : "—"} sub="total custodian holdings" tone="ink" isLoading={statsLoading} />
        <Metric label="24h Mint Volume" value={stats ? fmtIDRCompact(String(parseFloat(stats.mint_volume_24h_idrx) / 100)) : "—"} sub={stats ? `${stats.mint_count_24h} mints · ${stats.burn_count_24h} burns` : "—"} tone="merah" isLoading={statsLoading} />
        <Metric label="Pending requests" value={stats ? String(stats.pending_requests.total) : "—"} sub={stats ? `${stats.pending_requests.mints} mint · ${stats.pending_requests.redeems} redeem` : "—"} isLoading={statsLoading} />
        <Metric label="Reserve records" value={reserves ? String(reserves.length) : "—"} sub={reserves ? lastAttestationLabel(reserves) : "—"} isLoading={reservesLoading} />
      </div>

      <MintOrderForm />

      <div className="mt-[56px]">
        <div className={SECTION_HEADER}>
          <h2 className={SECTION_TITLE}>Request queue</h2>
          <div className={SECTION_LABEL}>{queueSummary(requestsData?.items)}</div>
        </div>
        <RequestQueue requests={requestsData?.items ?? []} isLoading={requestsLoading} currentAddress={address} />
      </div>

      <div className="mt-[56px]">
        <div className={SECTION_HEADER}>
          <h2 className={SECTION_TITLE}>Physical vault & proof of reserves</h2>
          <div className={SECTION_LABEL}>{lastAttestationLabel(reserves ?? [])}</div>
        </div>
        <div className="mt-[20px] grid grid-cols-[repeat(auto-fit,minmax(min(100%,420px),1fr))] items-center gap-[32px]">
          <VaultBlocks entries={reserves ?? []} />
          <ReservesTable entries={reserves ?? []} isLoading={reservesLoading} />
        </div>
      </div>

      <div className="mt-[56px]">
        <div className={SECTION_HEADER}>
          <h2 className={SECTION_TITLE}>KYC & redemption access</h2>
          <div className={SECTION_LABEL}>{kycRecords?.length ?? 0} verified</div>
        </div>
        <KYCRegistry records={kycRecords ?? []} isLoading={kycLoading} />
      </div>
    </div>
  );
}

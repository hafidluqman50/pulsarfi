'use client';

export function StockDetailSkeleton(): React.ReactNode {
  return (
    <div className="container pad-x !pb-[64px] !pt-[28px]">
      <div className="mb-[24px] flex items-center gap-[8px]">
        <div className="skeleton h-[14px] w-[72px]" />
        <div className="skeleton h-[14px] w-[46px]" />
      </div>
      <div className="mb-[28px] flex flex-wrap items-start justify-between gap-[16px]">
        <div className="flex items-start gap-[16px]">
          <div className="skeleton h-[52px] w-[52px]" />
          <div>
            <div className="skeleton h-[13px] w-[260px]" />
            <div className="skeleton mt-[10px] h-[34px] w-[360px] max-w-full" />
            <div className="skeleton mt-[14px] h-[30px] w-[180px]" />
          </div>
        </div>
        <div className="skeleton h-[44px] w-[136px]" />
      </div>
      <div className="border border-[var(--hairline)] bg-[var(--putih)] p-[12px]">
        <div className="skeleton h-[340px] w-full" />
      </div>
    </div>
  );
}

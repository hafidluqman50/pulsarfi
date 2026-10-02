'use client';

export function StockDetailSkeleton(): React.ReactNode {
  return (
    <div className="mx-auto w-full max-w-[1440px] px-[32px] pb-[64px] pt-[32px] max-[719px]:px-[16px] max-[719px]:pb-[48px] max-[719px]:pt-[24px]">
      <div className="h-[13px] w-[90px] bg-[var(--canvas-soft)]" />
      <div className="mt-[18px] flex flex-wrap items-end justify-between gap-[24px] border-b border-[var(--ink)] pb-[20px]">
        <div className="flex items-center gap-[18px]">
          <div className="h-[72px] w-[72px] bg-[var(--canvas-soft)]" />
          <div>
            <div className="h-[40px] w-[200px] bg-[var(--canvas-soft)]" />
            <div className="mt-[8px] h-[17px] w-[260px] max-w-full bg-[var(--canvas-soft)]" />
          </div>
        </div>
        <div>
          <div className="ml-auto h-[52px] w-[170px] bg-[var(--canvas-soft)]" />
          <div className="ml-auto mt-[8px] h-[15px] w-[140px] bg-[var(--canvas-soft)]" />
        </div>
      </div>
      <div className="mt-[28px] grid grid-cols-[repeat(auto-fit,minmax(min(100%,520px),1fr))] items-start gap-[40px]">
        <div>
          <div className="mb-[12px] h-[20px] w-[160px] bg-[var(--canvas-soft)]" />
          <div className="paper-stack px-[16px] pb-[0] pt-[12px]">
            <div className="skeleton h-[260px] w-full" />
          </div>
        </div>
        <div className="paper-stack p-[20px]">
          <div className="h-[11px] w-[120px] bg-[var(--canvas-soft)]" />
          <div className="mt-[10px] h-[44px] w-full bg-[var(--canvas-soft)]" />
          <div className="mt-[10px] h-[230px] w-full bg-[var(--canvas-soft)]" />
        </div>
      </div>
    </div>
  );
}

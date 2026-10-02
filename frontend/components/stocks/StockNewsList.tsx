'use client';

import type { StockNewsItem } from '@/lib/stockDetail';

const TAG_ROTATIONS = [2, -2, 1];

export function StockNewsList({ newsItems }: { newsItems: StockNewsItem[] }): React.ReactNode {
  return (
    <div>
      <div className="mt-[44px] flex items-baseline justify-between border-b border-[var(--ink)] pb-[10px]">
        <h2 className="display m-[0] !text-[28px] !leading-[normal] !tracking-[-0.02em]">Market Intelligence</h2>
        <span className="text-[11px] font-[600] uppercase leading-[normal] tracking-[0.16em] text-[var(--body)]">IDX · Realtime Feed</span>
      </div>
      <div className="paper-stack mt-[16px]">
        {newsItems.map((newsItem, newsIndex) => (
          <div
            key={newsIndex}
            className="flex items-start justify-between gap-[16px] border-b border-[var(--hairline)] px-[20px] py-[18px]"
          >
            <div>
              <div className="mb-[8px] text-[18px] font-[400] leading-[1.35] [font-family:var(--font-fraunces,_Fraunces,_serif)]">
                {newsItem.headline}
              </div>
              <div className="flex items-center gap-[12px]">
                <span className="text-[9px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">{newsItem.source}</span>
                <span className="text-[11px] text-[var(--body)]">{newsItem.time}</span>
              </div>
            </div>
            <span
              className="shrink-0 border border-[var(--hairline)] bg-[var(--canvas)] px-[8px] py-[3px] text-[9px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]"
              style={{ transform: `rotate(${TAG_ROTATIONS[newsIndex % TAG_ROTATIONS.length]}deg)` }}
            >
              {newsItem.tag}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

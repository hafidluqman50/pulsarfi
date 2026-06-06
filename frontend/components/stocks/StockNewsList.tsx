'use client';

import type { StockNewsItem } from '@/lib/stockDetail';

export function StockNewsList({ newsItems }: { newsItems: StockNewsItem[] }): React.ReactNode {
  return (
    <div>
      <div className="mb-[14px] flex items-baseline justify-between">
        <span className="eyebrow">Market Intelligence</span>
        <span className="mono text-[10px] text-[var(--body)]">IDX · Realtime Feed</span>
      </div>
      <div className="border border-[var(--hairline)] bg-[var(--putih)]">
        {newsItems.map((newsItem, newsIndex) => (
          <div
            key={newsIndex}
            className={`px-[20px] py-[18px] ${newsIndex < newsItems.length - 1 ? 'border-b border-[var(--hairline)]' : ''}`}
          >
            <div className="flex items-start justify-between gap-[16px]">
              <div className="min-w-0 flex-1">
                <div className="mb-[8px] text-[14px] font-semibold leading-[1.45]">{newsItem.headline}</div>
                <div className="flex items-center gap-[12px]">
                  <span className="eyebrow !text-[9px] !text-[var(--body)]">{newsItem.source}</span>
                  <span className="text-[11px] text-[var(--body)]">{newsItem.time}</span>
                </div>
              </div>
              <span
                className="eyebrow mt-[2px] shrink-0 border border-[var(--hairline)] px-[8px] py-[3px] !text-[9px] !text-[var(--body)]"
              >
                {newsItem.tag}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

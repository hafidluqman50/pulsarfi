'use client';

import { AreaChart } from '@/components/charts/AreaChart';
import type { PriceHistoryPoint } from '@/http/market/priceApi';
import { fmtIDRX, fmtNum } from '@/lib/data';
import { STOCK_TIMEFRAME_OPTIONS } from '@/lib/stockDetail';

type StockDetailChartProps = {
  chartData: PriceHistoryPoint[];
  isHistoryLoading: boolean;
  isReservesLoading: boolean;
  selectedTimeframe: string;
  stockTicker: string;
  tokenSupply: number | null;
  onTimeframeChange: (timeframe: string) => void;
};

export function StockDetailChart({
  chartData,
  isHistoryLoading,
  isReservesLoading,
  selectedTimeframe,
  stockTicker,
  tokenSupply,
  onTimeframeChange,
}: StockDetailChartProps): React.ReactNode {
  return (
    <div className="mb-[20px]">
      <div className="stock-chart-header mb-[10px] flex items-center justify-between">
        <div className="range-pills">
          {STOCK_TIMEFRAME_OPTIONS.map(timeframe => (
            <button
              key={timeframe}
              className={selectedTimeframe === timeframe ? 'active' : ''}
              onClick={() => onTimeframeChange(timeframe)}
            >
              {timeframe}
            </button>
          ))}
        </div>
        <span className="mono stock-supply-info text-[11px] text-[var(--body)]">
          {isReservesLoading || tokenSupply == null ? (
            <span className="skeleton inline-block h-[13px] w-[150px] align-[-2px]" />
          ) : (
            <>Total supply · {fmtNum(tokenSupply, 0)} {stockTicker}</>
          )}
        </span>
      </div>
      <div className="border border-[var(--hairline)] bg-[var(--putih)] pb-[4px] pt-[12px]">
        {isHistoryLoading || chartData.length === 0 ? (
          <div className="skeleton h-[340px] w-full" />
        ) : (
          <AreaChart
            data={chartData}
            height={340}
            valueFormatter={value => fmtIDRX(value)}
          />
        )}
      </div>
    </div>
  );
}

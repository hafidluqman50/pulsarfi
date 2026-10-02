'use client';

import { AreaChart } from '@/components/charts/AreaChart';
import type { PriceHistoryPoint } from '@/http/market/priceApi';
import { fmtIDRX } from '@/lib/data';
import { STOCK_TIMEFRAME_OPTIONS } from '@/lib/stockDetail';
import { ChartSkeleton } from './ChartSkeleton';

type StockDetailChartProps = {
  chartData: PriceHistoryPoint[];
  isHistoryLoading: boolean;
  selectedTimeframe: string;
  onTimeframeChange: (timeframe: string) => void;
};

const CHART_HEIGHT = 260;

export function StockDetailChart({
  chartData,
  isHistoryLoading,
  selectedTimeframe,
  onTimeframeChange,
}: StockDetailChartProps): React.ReactNode {
  return (
    <div>
      <div className="mb-[12px] flex flex-wrap items-center justify-between gap-[12px]">
        <div className="display !text-[20px] !leading-[normal]">Price · IDX</div>
        <div className="range-pills range-pills-full">
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
      </div>
      <div className="paper-stack px-[16px] pb-[0] pt-[12px]">
        {isHistoryLoading || chartData.length === 0 ? (
          <ChartSkeleton height={CHART_HEIGHT} />
        ) : (
          <AreaChart
            data={chartData}
            height={CHART_HEIGHT}
            paper
            valueFormatter={value => fmtIDRX(value)}
          />
        )}
      </div>
    </div>
  );
}

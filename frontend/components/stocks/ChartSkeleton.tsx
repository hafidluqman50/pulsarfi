'use client';

export function ChartSkeleton({ height = 220 }: { height?: number }): React.ReactNode {
  return <div className="skeleton w-full" style={{ height }} />;
}

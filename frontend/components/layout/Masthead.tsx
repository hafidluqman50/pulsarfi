'use client';

export function Masthead({ network = "Arbitrum Sepolia", pegStatus = "1:1 PEGGED" }) {
  const today = new Date(2026, 4, 23);
  const formattedDate = today.toLocaleDateString("en-GB", { weekday: "long", day: "2-digit", month: "long", year: "numeric" }).toUpperCase();

  return (
    <div className="hairline masthead-band flex items-center justify-between px-[32px] py-[8px] text-[11px] tracking-[0.14em] text-[var(--ink-soft)] font-[500] uppercase bg-[var(--canvas)]">
      <div className="flex gap-[24px] items-center">
        <span>{formattedDate}</span>
        <span className="text-[var(--body)]">· No. 0142 · Jakarta / Global</span>
      </div>
      <div className="flex gap-[24px] items-center">
        <span className="inline-flex items-center gap-[8px]">
          <span className="pulsar" />
          {pegStatus}
        </span>
        <span className="text-[var(--body)]">·</span>
        <span>{network}</span>
        <span className="text-[var(--body)]">·</span>
        <span>Block #98,341,022</span>
      </div>
    </div>
  );
}

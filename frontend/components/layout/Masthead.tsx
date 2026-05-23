'use client';

export function Masthead({ network = "Arbitrum Sepolia", pegStatus = "1:1 PEGGED" }) {
  const today = new Date(2026, 4, 23);
  const fmt = today.toLocaleDateString("en-GB", { weekday: "long", day: "2-digit", month: "long", year: "numeric" }).toUpperCase();
  return (
    <div className="hairline masthead-band" style={{
      alignItems: "center", justifyContent: "space-between",
      padding: "8px 32px", fontSize: 11, letterSpacing: "0.14em",
      color: "var(--ink-soft)", fontWeight: 500, textTransform: "uppercase",
      background: "var(--canvas)", display: "flex",
    }}>
      <div style={{ display: "flex", gap: 24, alignItems: "center" }}>
        <span>{fmt}</span>
        <span style={{ color: "var(--body)" }}>· No. 0142 · Jakarta / Global</span>
      </div>
      <div style={{ display: "flex", gap: 24, alignItems: "center" }}>
        <span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
          <span className="pulsar" />
          {pegStatus}
        </span>
        <span style={{ color: "var(--body)" }}>·</span>
        <span>{network}</span>
        <span style={{ color: "var(--body)" }}>·</span>
        <span>Block #98,341,022</span>
      </div>
    </div>
  );
}

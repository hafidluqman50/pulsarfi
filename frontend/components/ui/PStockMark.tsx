'use client';

const MARKS: Record<string, { fill: string; glyph: string, icon:string }> = {
  BUMIP: { fill: "#16110e", glyph: "B", icon: '/logos/BUMI.png' },
  ENRGP: { fill: "#c8102e", glyph: "E", icon: '/logos/ENRG.png' },
  BRPTP: { fill: "#1a5276", glyph: "B", icon: '/logos/BRPT.png' },
  PTROP: { fill: "#7d6608", glyph: "P", icon: '/logos/PTRO.png' },
  BBRIP: { fill: "#9a0c24", glyph: "R", icon: '/logos/BBRI.png' },
  BMRIP: { fill: "#003d7a", glyph: "M", icon: '/logos/BMRI.png' },
  BBCAP: { fill: "#003087", glyph: "C", icon: '/logos/BBCA.png' },
  BDMNP: { fill: "#e65c00", glyph: "D", icon: '/logos/BDMN.png' },
  IDRX:  { fill: "#1a7a4a", glyph: "₹", icon: '/logos/IDRX.png' },
};

export function PStockMark({ ticker, size = 28 }: { ticker: string; size?: number }) {
  const tokenMark = MARKS[ticker] ?? { fill: "#16110e", glyph: ticker?.[0] || "•" };
  return (
    <div style={{
      width: size, height: size, color: "#fff",
      display: "flex", alignItems: "center", justifyContent: "center",
      fontFamily: '"Fraunces", serif', fontWeight: 500, fontSize: size * 0.5,
      letterSpacing: "-0.02em", borderRadius: 0, flex: `0 0 ${size}px`,
    }}>
      <img src={tokenMark?.icon} alt="Logo Stocks" />
    </div>
  );
}

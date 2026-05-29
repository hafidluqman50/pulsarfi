'use client';

const MARKS: Record<string, { fill: string; glyph: string, icon:string }> = {
  BUMIP: { fill: "#16110e", glyph: "B", icon: '/logos/BUMI.png' },
  ENRGP: { fill: "#c8102e", glyph: "E", icon: '/logos/ENRG.png' },
  KIJAP: { fill: "#2a231e", glyph: "K", icon: '/logos/KIJA.png' },
  TLKMP: { fill: "#1f4d8a", glyph: "T", icon: '/logos/TLKM.png' },
  BBRIP: { fill: "#9a0c24", glyph: "R", icon: '/logos/BBRI.png' },
  GOTOP: { fill: "#16110e", glyph: "G", icon: '/logos/GOTO.png' },
  ASIIP: { fill: "#5a4a3a", glyph: "A", icon: '/logos/ASII.png' },
  UNVRP: { fill: "#1a3a6e", glyph: "U", icon: '/logos/UNVR.png' },
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

'use client';

const MARKS: Record<string, { fill: string; glyph: string }> = {
  BUMIP: { fill: "#16110e", glyph: "B" },
  ENRGP: { fill: "#c8102e", glyph: "E" },
  KIJAP: { fill: "#2a231e", glyph: "K" },
  TLKMP: { fill: "#1f4d8a", glyph: "T" },
  BBRIP: { fill: "#9a0c24", glyph: "R" },
  GOTOP: { fill: "#16110e", glyph: "G" },
  ASIIP: { fill: "#5a4a3a", glyph: "A" },
  UNVRP: { fill: "#1a3a6e", glyph: "U" },
  IDRX:  { fill: "#1a7a4a", glyph: "₹" },
};

export function PStockMark({ ticker, size = 28 }: { ticker: string; size?: number }) {
  const tokenMark = MARKS[ticker] ?? { fill: "#16110e", glyph: ticker?.[0] || "•" };
  return (
    <div style={{
      width: size, height: size, background: tokenMark.fill, color: "#fff",
      display: "flex", alignItems: "center", justifyContent: "center",
      fontFamily: '"Fraunces", serif', fontWeight: 500, fontSize: size * 0.5,
      letterSpacing: "-0.02em", borderRadius: 0, flex: `0 0 ${size}px`,
    }}>
      {tokenMark.glyph}
    </div>
  );
}

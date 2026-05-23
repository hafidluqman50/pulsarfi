'use client';

export function Wordmark({ size = 26 }: { size?: number }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, lineHeight: 1 }}>
      <div className="pulsar" style={{ width: 8, height: 8 }} />
      <div className="display" style={{ fontSize: size, fontWeight: 500, letterSpacing: "-0.025em", color: "inherit" }}>
        Pulsar<span className="display-it" style={{ fontStyle: "italic", fontWeight: 400 }}>Fi</span>
      </div>
    </div>
  );
}

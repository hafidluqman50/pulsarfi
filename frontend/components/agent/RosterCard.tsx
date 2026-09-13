// Pixel-matched to Agent Chat.dc.html's "Three agents" card.
export function RosterCard() {
  const agents = [
    { t: 'SUPERVISOR', d: 'Reads the instruction, splits it into tasks, and holds anything that needs your signature.' },
    { t: 'ANALYZER', d: 'Scores the named sources, prices the position, and checks the order against your caps.' },
    { t: 'EXECUTOR', d: 'Builds the swap, submits the signed transaction, and reports the fill back here.' },
  ];

  return (
    <div style={{ border: '1px solid var(--hairline-strong)', background: 'var(--canvas-soft)' }}>
      <div style={{ padding: '9px 11px', display: 'flex', alignItems: 'baseline', gap: '8px 10px', flexWrap: 'wrap' }}>
        <span style={{ font: '600 9.5px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase', color: 'var(--body)' }}>Three agents</span>
        <span style={{ marginLeft: 'auto', fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--merah)', whiteSpace: 'nowrap' }}>analyzer &rarr; executor</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(150px,1fr))' }}>
        {agents.map((ag) => (
          <div key={ag.t} style={{ borderTop: '1px solid var(--hairline)', padding: '10px 11px' }}>
            <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--putih)', background: 'var(--ink)', padding: '3px 5px', display: 'inline-block' }}>{ag.t}</div>
            <div style={{ fontSize: 11.5, color: 'var(--body)', lineHeight: 1.45, marginTop: 6 }}>{ag.d}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

export type QuasarDestination = 'chat' | 'tasks' | 'history' | 'activity' | 'risk';

type MenuPanelProps = {
  active: QuasarDestination;
  onSelect: (destination: QuasarDestination) => void;
};

// Pixel-matched to Agent Chat.dc.html's menu list (isMenu section). Risk
// profile is still explicitly out of scope for this pass
// (agent-task-manager-rebuild.md §5/§9) — listed to match the reference's
// menu shape, disabled rather than faked. Activity log shipped.
export function MenuPanel({ active, onSelect }: MenuPanelProps) {
  const items: { n: string; key: QuasarDestination; t: string; d: string; disabled?: boolean }[] = [
    { n: '01', key: 'tasks', t: 'Tasks', d: 'Every Task Quasar has opened, armed, or is still running.' },
    { n: '02', key: 'history', t: 'Chat history', d: 'Every conversation you have had with Quasar.' },
    { n: '03', key: 'activity', t: 'Activity log', d: 'Every step across every one of your own Tasks, filterable by who ran it.' },
    { n: '04', key: 'risk', t: 'Risk profile', d: 'Read-only limits derived from your KYC and trading history.', disabled: true },
  ];

  return (
    <div className="rise" style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column' }}>
      {items.map((item) => (
        <button
          key={item.key}
          disabled={item.disabled}
          onClick={() => !item.disabled && onSelect(item.key)}
          style={{
            appearance: 'none',
            border: 0,
            borderBottom: '1px solid var(--hairline)',
            cursor: item.disabled ? 'not-allowed' : 'pointer',
            background: active === item.key ? 'var(--canvas-soft)' : 'var(--canvas)',
            padding: '16px 14px',
            textAlign: 'left',
            display: 'flex',
            gap: 12,
            alignItems: 'baseline',
            opacity: item.disabled ? 0.5 : 1,
          }}
        >
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--merah)', flex: 'none' }}>{item.n}</span>
          <span>
            <span style={{ display: 'block', fontSize: 15, fontWeight: 600, lineHeight: 1.3 }}>{item.t}</span>
            <span style={{ display: 'block', fontSize: 12.5, color: 'var(--body)', lineHeight: 1.45, marginTop: 3 }}>{item.d}</span>
          </span>
          {item.disabled && (
            <span style={{ marginLeft: 'auto', fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--ticker)', flex: 'none' }}>soon</span>
          )}
        </button>
      ))}
    </div>
  );
}

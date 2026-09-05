export type QuasarDestination = 'chat' | 'tasks' | 'history' | 'activity' | 'risk';

type MenuPanelProps = {
  active: QuasarDestination;
  onSelect: (destination: QuasarDestination) => void;
};

// Activity log and Risk profile are explicitly out of scope for this pass
// (agent-task-manager-rebuild.md §5/§9) — listed as destinations so the
// menu shape matches the design reference, but disabled rather than faked.
export function MenuPanel({ active, onSelect }: MenuPanelProps) {
  const items: { key: QuasarDestination; label: string; disabled?: boolean }[] = [
    { key: 'tasks', label: 'Tasks' },
    { key: 'history', label: 'Chat history' },
    { key: 'activity', label: 'Activity log', disabled: true },
    { key: 'risk', label: 'Risk profile', disabled: true },
  ];

  return (
    <div className="hairline flex flex-col">
      {items.map((item) => (
        <button
          key={item.key}
          disabled={item.disabled}
          className={`hairline-top flex items-center justify-between px-[16px] py-[12px] text-left text-[13px] ${
            active === item.key ? 'font-semibold' : ''
          } ${item.disabled ? 'cursor-not-allowed text-[var(--body)] opacity-50' : ''}`}
          onClick={() => !item.disabled && onSelect(item.key)}
        >
          <span>{item.label}</span>
          {item.disabled && <span className="text-[11px] text-[var(--body)]">soon</span>}
        </button>
      ))}
    </div>
  );
}

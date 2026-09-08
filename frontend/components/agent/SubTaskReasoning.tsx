'use client';

const AGENT_DISPLAY_NAMES: Record<string, string> = {
  supervisor: 'Quasar',
  analyzer: 'Nova',
  executor: 'Comet',
};

// agentDisplayName shows each role's persona name (Quasar/Nova/Comet), never
// the raw "supervisor"/"analyzer"/"executor" role string — those are what
// SubTaskRecorder.Record's own agentName argument stores (it feeds the hash
// chain, so it stays as-is there), this is display-only.
export function agentDisplayName(agent: string): string {
  return AGENT_DISPLAY_NAMES[agent] ?? agent;
}

export function statusColor(status: string): string {
  if (status === 'needs_input') return 'var(--merah)';
  if (status === 'failed' || status === 'error') return 'var(--negative)';
  if (status === 'done' || status === 'completed') return 'var(--positive)';
  return 'var(--ticker)';
}

export function humanizeKey(key: string): string {
  return key
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

// stepDisplayName prefers the model-authored label (informative, in the
// conversation's own language) — falls back to a humanized step_name only
// for rows recorded before this existed (label is null), or if a caller
// ever leaves it empty.
export function stepDisplayName(label: string | null | undefined, stepName: string): string {
  return label && label.trim() !== '' ? label : humanizeKey(stepName);
}

function stringifyValue(value: unknown): string {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (value == null) return '—';
  return JSON.stringify(value);
}

type ChecklistRow = { label: string; value: string };
type Structured = { prose: string | null; checklist: ChecklistRow[] };

function toStructured(raw: string): Structured | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof parsed !== 'object' || parsed === null) return null;

  const obj = parsed as Record<string, unknown>;
  const prose = typeof obj.reasoning === 'string' ? obj.reasoning : null;
  const rows: ChecklistRow[] = [];
  for (const [key, value] of Object.entries(obj)) {
    if (key === 'reasoning') continue;
    if (Array.isArray(value)) {
      value.forEach((item, i) => {
        if (item && typeof item === 'object') {
          const record = item as Record<string, unknown>;
          const label = typeof record.source === 'string' ? record.source : `${humanizeKey(key)} ${i + 1}`;
          const summary = typeof record.summary === 'string' ? record.summary : stringifyValue(item);
          rows.push({ label, value: summary });
        } else {
          rows.push({ label: `${humanizeKey(key)} ${i + 1}`, value: stringifyValue(item) });
        }
      });
      continue;
    }
    rows.push({ label: humanizeKey(key), value: stringifyValue(value) });
  }
  return { prose, checklist: rows };
}

export function StructuredOrProse({ raw }: { raw: string }) {
  const structured = toStructured(raw);
  if (!structured) {
    return <div style={{ fontSize: 12.5, lineHeight: 1.5, color: 'var(--ink-soft)' }}>{raw}</div>;
  }
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      {structured.prose && <div style={{ fontSize: 12.5, lineHeight: 1.5, color: 'var(--ink-soft)' }}>{structured.prose}</div>}
      {structured.checklist.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 7 }}>
          {structured.checklist.map((row, i) => (
            <div key={i} style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 9 }}>
                <span style={{ width: 13, height: 13, flex: 'none', border: '1px solid var(--positive)', background: 'var(--positive)', color: 'var(--putih)', font: '700 8px/12px var(--font-sans)', textAlign: 'center' }}>
                  ✓
                </span>
                <span style={{ fontSize: 12.5, lineHeight: 1.4, overflowWrap: 'break-word', minWidth: 0 }}>{row.label}</span>
              </div>
              {/* Both label and value can be a long LLM-authored string
                  (e.g. an evidence citation), not just a short fixed field
                  name like "Confidence" — stacking value under label, both
                  wrapping normally, is what keeps this safe regardless of
                  either side's length; forcing them onto one line with a
                  fixed-width label previously squeezed the value into a
                  sliver, wrapping it one character per line. */}
              <span style={{ marginLeft: 22, fontFamily: 'var(--font-mono)', fontSize: 10, lineHeight: 1.4, color: 'var(--ticker)', overflowWrap: 'break-word' }}>
                {row.value}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

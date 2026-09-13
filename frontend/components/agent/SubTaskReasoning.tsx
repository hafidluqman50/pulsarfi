'use client';

import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

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

const TX_HASH_REGEX = /(^|[^a-zA-Z0-9_\/\[])(0x[a-fA-F0-9]{64})(?![a-zA-Z0-9_\]\)])/g;

export function linkifyTxHashes(text: string): string {
  if (!text) return text;
  return text.replace(TX_HASH_REGEX, (_match, prefix, hash) => {
    return `${prefix}[${hash}](https://sepolia.arbiscan.io/tx/${hash})`;
  });
}

export function MessageMarkdown({ content, fontSize = 14.5 }: { content: string; fontSize?: number }) {
  let displayContent = content;
  if (content && typeof content === 'string') {
    const trimmed = content.trim();
    if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('```json') && trimmed.endsWith('```'))) {
      try {
        const clean = trimmed.replace(/^```json\s*/, '').replace(/^```\s*/, '').replace(/\s*```$/, '');
        const parsed = JSON.parse(clean);
        if (parsed.reasoning) {
          displayContent = parsed.reasoning;
        } else if (parsed.reply) {
          displayContent = parsed.reply;
        } else if (parsed.message) {
          displayContent = parsed.message;
        } else if (parsed.explanation) {
          displayContent = parsed.explanation;
        }
      } catch {
        // keep original content
      }
    }
    displayContent = linkifyTxHashes(displayContent);
  }

  return (
    <div style={{ fontSize, lineHeight: 1.55, overflowWrap: 'anywhere', wordBreak: 'break-word', minWidth: 0 }}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: ({ children }) => <p style={{ margin: '0 0 10px' }}>{children}</p>,
          strong: ({ children }) => <strong style={{ fontWeight: 600, color: 'var(--ink)' }}>{children}</strong>,
          ul: ({ children }) => <ul style={{ margin: '0 0 10px', paddingLeft: 20 }}>{children}</ul>,
          ol: ({ children }) => <ol style={{ margin: '0 0 10px', paddingLeft: 20 }}>{children}</ol>,
          li: ({ children }) => <li style={{ marginBottom: 4 }}>{children}</li>,
          a: ({ children, href }) => {
            const isTx = href?.includes('/tx/0x');
            return (
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                style={{
                  color: 'var(--merah)',
                  fontFamily: isTx ? 'var(--font-mono)' : 'inherit',
                  textDecoration: 'underline',
                  textUnderlineOffset: '3px',
                  overflowWrap: 'anywhere',
                  wordBreak: 'break-all',
                }}
              >
                {children}
              </a>
            );
          },
          table: ({ children }) => (
            <div style={{ overflowX: 'auto', margin: '0 0 10px' }}>
              <table style={{ borderCollapse: 'collapse', width: '100%', fontSize: 13 }}>{children}</table>
            </div>
          ),
          thead: ({ children }) => <thead style={{ borderBottom: '1px solid var(--hairline-strong)' }}>{children}</thead>,
          th: ({ children }) => <th style={{ textAlign: 'left', padding: '6px 10px', fontWeight: 600, color: 'var(--ink)' }}>{children}</th>,
          td: ({ children }) => <td style={{ padding: '6px 10px', borderTop: '1px solid var(--hairline)' }}>{children}</td>,
          code: ({ children }) => (
            <code style={{ fontFamily: 'var(--font-mono)', fontSize: 12.5, background: 'var(--canvas-soft)', padding: '1px 4px' }}>{children}</code>
          ),
        }}
      >
        {displayContent}
      </ReactMarkdown>
    </div>
  );
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

export function statusDisplayName(status: string): string {
  return status.toUpperCase();
}

// stepDisplayName prefers the model-authored label (informative, in the
// conversation's own language) — falls back to humanizeKey only if empty.
export function stepDisplayName(label: string | null | undefined, stepName: string): string {
  if (label && label.trim() !== '') {
    return label;
  }
  return humanizeKey(stepName);
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
    return (
      <div style={{ color: 'var(--ink-soft)' }}>
        <MessageMarkdown content={raw} fontSize={12.5} />
      </div>
    );
  }
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      {structured.prose && (
        <div style={{ color: 'var(--ink-soft)' }}>
          <MessageMarkdown content={structured.prose} fontSize={12.5} />
        </div>
      )}
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

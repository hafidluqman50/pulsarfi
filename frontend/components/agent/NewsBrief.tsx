'use client';

type NewsEvidenceItem = {
  source: string;
  url?: string;
  published_at?: string;
  excerpt?: string;
  image_url?: string;
};

function formatPublishedAt(value?: string): string | null {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleString('id-ID', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
}

// One evidence item, straight from analyzer_agent's own sourced/dated
// citations (task_service.go's NewsEvidenceItem) — never re-summarized or
// embellished here, only laid out. A missing published_at/image_url means
// the source page genuinely didn't provide one; both are simply omitted,
// never guessed.
function NewsBriefItem({ item }: { item: NewsEvidenceItem }) {
  const publishedAt = formatPublishedAt(item.published_at);
  return (
    <div style={{ display: 'flex', gap: 12, padding: '12px 0', borderTop: '1px solid var(--hairline)' }}>
      {item.image_url && (
        // eslint-disable-next-line @next/next/no-img-element -- external, arbitrary trusted-domain source images; next/image's domain allowlist isn't worth maintaining for this
        <img
          src={item.image_url}
          alt=""
          style={{ width: 64, height: 64, objectFit: 'cover', flex: 'none', border: '1px solid var(--hairline)', background: 'var(--canvas-soft)' }}
        />
      )}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4, minWidth: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <span style={{ font: '700 9px/1.3 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase', color: 'var(--merah)', border: '1px solid var(--merah)', padding: '2px 4px' }}>
            {item.source}
          </span>
          {publishedAt && <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)' }}>{publishedAt}</span>}
        </div>
        {item.excerpt && <div style={{ fontSize: 12.5, lineHeight: 1.45, color: 'var(--ink-soft)' }}>{item.excerpt}</div>}
        {item.url && (
          <a href={item.url} target="_blank" rel="noopener noreferrer" style={{ fontSize: 11, color: 'var(--body)', textDecoration: 'underline' }}>
            {item.url}
          </a>
        )}
      </div>
    </div>
  );
}

export function NewsBrief({ uiProps }: { uiProps: unknown }) {
  const items = Array.isArray(uiProps) ? (uiProps as NewsEvidenceItem[]) : [];
  if (items.length === 0) return null;
  return (
    <div className="rise" style={{ border: '1px solid var(--hairline)', borderLeft: '2px solid var(--merah)', background: 'var(--putih)', padding: '13px 15px' }}>
      <div style={{ font: '700 9px/1.3 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 4 }}>
        Sumber · {items.length} berita
      </div>
      {items.map((item, i) => (
        <NewsBriefItem key={item.url ?? i} item={item} />
      ))}
    </div>
  );
}

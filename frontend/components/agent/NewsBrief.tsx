'use client';

type NewsEvidenceItem = {
  title?: string;
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

function cleanExcerpt(text?: string): string {
  if (!text) return '';
  let clean = text
    // Remove markdown images: ![alt](url)
    .replace(/!\[.*?\]\(.*?\)/g, '')
    // Remove common navigation/header boilerplate links
    .replace(/\[(?:\+?\s*(?:login|masuk|daftar|icon|home|baca e-paper|konten interaktif|kompas\.com|detik\.com|bisnis indonesia|foto|video)).*?\]\(.*?\)/gi, '')
    // Convert remaining markdown links [text](url) -> text
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    // Remove markdown syntax characters
    .replace(/[*#_`~>]/g, ' ')
    // Collapse whitespace
    .replace(/\s+/g, ' ')
    .trim();

  // Strip leading punctuation/symbols like "-", "*", "+", "|", ":"
  clean = clean.replace(/^[-+*|:\s]+/, '');

  if (clean.length > 220) {
    clean = clean.slice(0, 220).trim() + '…';
  }
  return clean;
}

function deriveTitle(item: NewsEvidenceItem): string {
  if (item.title && item.title.trim()) {
    return item.title.replace(/^\[Sumber Eksternal\]\s*/i, '').trim();
  }
  if (!item.url) return item.source;
  try {
    const parsedUrl = new URL(item.url);
    const pathParts = parsedUrl.pathname.split('/').filter(Boolean);
    const lastPart = pathParts[pathParts.length - 1] || '';
    const slug = lastPart.replace(/\.[a-zA-Z0-9]+$/, '');
    if (slug.includes('-') || slug.includes('_')) {
      const words = slug
        .split(/[-_]+/)
        .filter((w) => !/^\d+$/.test(w) && w.length > 0)
        .map((w) => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase());
      if (words.length > 0) {
        return words.join(' ');
      }
    }
  } catch {
    // ignore
  }
  return `${item.source} - Berita Terkait`;
}

function NewsBriefItem({ item }: { item: NewsEvidenceItem }) {
  const publishedAt = formatPublishedAt(item.published_at);
  const title = deriveTitle(item);
  const excerpt = cleanExcerpt(item.excerpt);

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
      <div style={{ display: 'flex', flexDirection: 'column', gap: 5, minWidth: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <span style={{ font: '700 9px/1.3 var(--font-sans)', letterSpacing: '.1em', textTransform: 'uppercase', color: 'var(--merah)', border: '1px solid var(--merah)', padding: '2px 4px' }}>
            {item.source}
          </span>
          {publishedAt && <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--ticker)' }}>{publishedAt}</span>}
        </div>
        {title && (
          <a
            href={item.url}
            target="_blank"
            rel="noopener noreferrer"
            style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)', textDecoration: 'none', lineHeight: 1.4 }}
            onMouseEnter={(e) => (e.currentTarget.style.textDecoration = 'underline')}
            onMouseLeave={(e) => (e.currentTarget.style.textDecoration = 'none')}
          >
            {title}
          </a>
        )}
        {excerpt && <div style={{ fontSize: 12, lineHeight: 1.5, color: 'var(--ink-soft)' }}>{excerpt}</div>}
        {item.url && (
          <a
            href={item.url}
            target="_blank"
            rel="noopener noreferrer"
            style={{ fontSize: 11, color: 'var(--merah)', textDecoration: 'underline', marginTop: 2, display: 'inline-flex', alignItems: 'center', gap: 3 }}
          >
            Buka artikel di {item.source} ↗
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

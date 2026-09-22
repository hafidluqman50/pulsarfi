'use client';

import { useState } from 'react';

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

const ACRONYMS = new Set(['IHSG', 'BI', 'BEI', 'IDX', 'OJK', 'FED', 'AS', 'RI', 'APBN', 'BUMN', 'IPO', 'BBRI', 'BBCA', 'BMRI', 'BBNI', 'BUMI', 'BRPT', 'PTRO', 'ENRG', 'ANTM', 'TLKM', 'GOTO', 'EMTK']);
const NAV_TOKENS = new Set(['home', 'beranda', 'news', 'bisnis', 'showbiz', 'tekno', 'otomotif', 'bola', 'lifestyle', 'foto', 'video', 'pemilu', 'cek', 'fakta']);

function extractImageUrl(item: NewsEvidenceItem): string | null {
  if (item.image_url && item.image_url.trim()) {
    return item.image_url.trim();
  }
  if (!item.excerpt) return null;
  const match = item.excerpt.match(/!\[[\s\S]*?\]\((https?:\/\/[^\s)]+)\)/i);
  if (match && match[1]) {
    return match[1].replace(/[)\s]+$/, '');
  }
  return null;
}

function cleanExcerpt(text?: string): string {
  if (!text) return '';
  let clean = text
    // 1. Remove closed markdown links wrapping images: [![alt](img)](url)
    .replace(/\[\s*!\[[\s\S]*?\]\([\s\S]*?\)\s*\]\([\s\S]*?\)/gi, '')
    // 2. Remove closed standalone markdown images: ![alt](url)
    .replace(/!\[[\s\S]*?\]\([\s\S]*?\)/gi, '')
    // 3. Remove unclosed trailing markdown image or link tags (e.g. ![alt]( or ![alt] or [alt]( or unclosed [)
    .replace(/!\[[^\]]*\]\([^)]*$/gi, '')
    .replace(/!\[[^\]]*$/gi, '')
    .replace(/\[[^\]]*\]\([^)]*$/gi, '')
    .replace(/\[[^\]]*$/gi, '')
    // 4. Remove empty markdown links: [](url) or [ ](url)
    .replace(/\[\s*\]\([\s\S]*?\)/gi, '')
    // 5. Remove common navigation/header boilerplate links
    .replace(/\[(?:\+?\s*(?:login|masuk|daftar|icon|home|beranda|baca e-paper|konten interaktif|kompas\.com|detik\.com|bisnis indonesia|liputan6|foto|video|news|bisnis|showbiz|tekno|otomotif))[\s\S]*?\]\([\s\S]*?\)/gi, '')
    // 6. Convert remaining markdown links [text](url) -> text
    .replace(/\[([^\]]+)\]\([^)]+\)/gi, '$1')
    // 7. Remove raw URLs in http(s)://...
    .replace(/https?:\/\/[^\s)]+/gi, '')
    // 8. Remove markdown formatting characters
    .replace(/[*#_`~>]/g, ' ')
    // 9. Collapse whitespace
    .replace(/\s+/g, ' ')
    .trim();

  // Strip leading/trailing punctuation & symbols like "-", "*", "+", "|", ":", "/", "!"
  clean = clean.replace(/^[-+*|:\/!\s]+/, '').replace(/[-+*|:\/!(\s]+$/, '').trim();

  // If the result is just a series of nav menu items or too short to be a real sentence (< 25 chars), discard it
  if (clean.length < 25) {
    return '';
  }

  // Discard if mostly navigation tokens (>40% of words)
  const words = clean.toLowerCase().split(/\s+/);
  const navWordCount = words.filter((w) => NAV_TOKENS.has(w)).length;
  if (words.length > 0 && navWordCount / words.length > 0.4) {
    return '';
  }

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
        .map((w) => {
          const upper = w.toUpperCase();
          if (ACRONYMS.has(upper)) return upper;
          return w.charAt(0).toUpperCase() + w.slice(1).toLowerCase();
        });
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
  const initialImage = extractImageUrl(item);
  const [imgSrc, setImgSrc] = useState<string | null>(initialImage);
  const sourceAbbr = (item.source || 'NEWS').split('.')[0].replace(/[^a-zA-Z0-9]/g, '').slice(0, 4).toUpperCase();

  return (
    <div style={{ display: 'flex', gap: 12, padding: '12px 0', borderTop: '1px solid var(--hairline)' }}>
      {imgSrc ? (
        // eslint-disable-next-line @next/next/no-img-element -- external, arbitrary trusted-domain source images; next/image's domain allowlist isn't worth maintaining for this
        <img
          src={imgSrc}
          alt=""
          onError={() => setImgSrc(null)}
          style={{ width: 64, height: 64, objectFit: 'cover', flex: 'none', border: '1px solid var(--hairline)', background: 'var(--canvas-soft)', borderRadius: 2 }}
        />
      ) : (
        <div
          style={{
            width: 64,
            height: 64,
            flex: 'none',
            border: '1px solid var(--hairline)',
            background: 'var(--canvas-soft)',
            borderRadius: 2,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 4,
            color: 'var(--ticker)',
          }}
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M4 22h16a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v16a2 2 0 0 1-2 2Zm0 0a2 2 0 0 1-2-2v-9c0-1.1.9-2 2-2h2" />
            <path d="M18 14h-8" />
            <path d="M15 18h-5" />
            <path d="M10 6h8v4h-8V6Z" />
          </svg>
          <span style={{ fontSize: 9, fontWeight: 700, letterSpacing: '.05em', color: 'var(--ticker)' }}>{sourceAbbr}</span>
        </div>
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

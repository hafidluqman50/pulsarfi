'use client';

import { Icon } from './Icon';

interface AccordionProps {
  open: boolean;
  onToggle: () => void;
  summary: React.ReactNode;
  children: React.ReactNode;
}

export function Accordion({ open, onToggle, summary, children }: AccordionProps) {
  return (
    <div className="hairline-top">
      <button onClick={onToggle} style={{
        appearance: "none", border: 0, background: "transparent", width: "100%",
        display: "flex", alignItems: "center", justifyContent: "space-between",
        padding: "12px 0", cursor: "pointer", color: "inherit", font: "inherit", textAlign: "left",
      }}>
        <span style={{ fontSize: 13, color: "var(--body)" }}>{summary}</span>
        <span style={{ transition: "transform .15s", transform: open ? "rotate(180deg)" : "none", color: "var(--body)" }}>
          <Icon name="chevron-down" size={16} />
        </span>
      </button>
      {open && (
        <div style={{ paddingBottom: 12, display: "flex", flexDirection: "column", gap: 8, fontSize: 13 }}>
          {children}
        </div>
      )}
    </div>
  );
}

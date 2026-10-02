'use client';

interface AccordionProps {
  open: boolean;
  onToggle: () => void;
  summary: React.ReactNode;
  children: React.ReactNode;
}

export function Accordion({ open, onToggle, summary, children }: AccordionProps) {
  return (
    <div>
      <button onClick={onToggle} className="mono" style={{
        appearance: "none", border: 0, borderBottom: "1px dashed var(--hairline)", background: "transparent", width: "100%",
        display: "flex", alignItems: "center", justifyContent: "space-between",
        padding: "12px 0", cursor: "pointer", color: "inherit", fontSize: 13, textAlign: "left",
      }}>
        <span>{summary}</span>
        <span style={{ fontSize: 11, color: "var(--body)" }}>{open ? "▴" : "▾"}</span>
      </button>
      {open && (
        <div style={{ padding: "12px 0", display: "flex", flexDirection: "column", gap: 8, fontSize: 13 }}>
          {children}
        </div>
      )}
    </div>
  );
}

export function RosterCard() {
  return (
    <div className="hairline p-[16px]">
      <div className="mb-[10px] flex items-center justify-between">
        <span className="eyebrow !text-[var(--body)]">Three agents</span>
        <span className="mono text-[11px] text-[var(--body)]">analyzer -&gt; executor</span>
      </div>
      <div className="flex flex-col gap-[10px] text-[13px]">
        <div>
          <span className="font-semibold">SUPERVISOR</span>
          <span className="ml-[8px] text-[var(--body)]">
            Reads the instruction, splits it into tasks, and holds anything that needs your signature.
          </span>
        </div>
        <div>
          <span className="font-semibold">ANALYZER</span>
          <span className="ml-[8px] text-[var(--body)]">
            Scores the named sources, prices the position, and checks the order against your caps.
          </span>
        </div>
        <div>
          <span className="font-semibold">EXECUTOR</span>
          <span className="ml-[8px] text-[var(--body)]">
            Builds the swap, submits the signed transaction, and reports the fill back here.
          </span>
        </div>
      </div>
    </div>
  );
}

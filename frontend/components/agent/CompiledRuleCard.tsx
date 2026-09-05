type CompiledRuleCardProps = {
  lockedSummary: string;
  bands: { label: string; value: string }[];
};

// Always renders "fully autonomous, no signature per fill" — this is not
// configurable per Task, so it's a literal, not a prop, to make it
// impossible for a future edit to accidentally introduce per-fill wording.
export function CompiledRuleCard({ lockedSummary, bands }: CompiledRuleCardProps) {
  return (
    <div className="compiled-rule-card hairline p-[16px]">
      <p className="text-[13px]">{lockedSummary}</p>
      <div className="mt-[12px] flex flex-col gap-[4px]">
        {bands.map((band) => (
          <div key={band.label} className="flex justify-between text-[12px]">
            <span className="text-[var(--body)]">{band.label}</span>
            <span>{band.value}</span>
          </div>
        ))}
        <div className="hairline-top mt-[6px] flex justify-between pt-[8px] text-[12px]">
          <span className="text-[var(--body)]">Approval</span>
          <span>fully autonomous, no signature per fill</span>
        </div>
        <div className="flex justify-between text-[12px]">
          <span className="text-[var(--body)]">Your gate</span>
          <span>creating this Task — asked, never assumed</span>
        </div>
      </div>
    </div>
  );
}

'use client';

import { useState } from 'react';
import { useAccount, useWriteContract } from 'wagmi';
import { erc20Abi, type Address } from 'viem';
import { useArmTask } from '@/http/agent/hooks';

type ArmPanelProps = {
  taskId: number;
  isActionable: boolean;
  tokenAddress?: Address;
  tokenSymbol?: string;
  totalBudget?: string;
  durationSec: number;
};

// Pixel-matched to Agent Chat.dc.html's "Confirm and arm" card. Two
// independent signatures: arm() first (Task hash commitment,
// backend-signed via AGENT_ROLE), then the owner's own approve() (this
// component's own wallet call — never proxied through the backend, since
// only the owner can grant that allowance). The approve() step needs a
// real deployed AgentTaskManager address (NEXT_PUBLIC_AGENT_TASK_MANAGER_ADDRESS)
// — until it's set, arm() itself still works (backend stub for step 2).
export function ArmPanel({ taskId, isActionable, tokenAddress, tokenSymbol, totalBudget, durationSec }: ArmPanelProps) {
  const [acknowledged, setAcknowledged] = useState(false);
  const [step, setStep] = useState<'idle' | 'arming' | 'approving' | 'armed' | 'error'>('idle');
  const { address } = useAccount();
  const { writeContractAsync } = useWriteContract();
  const armTask = useArmTask(taskId);

  const managerAddress = process.env.NEXT_PUBLIC_AGENT_TASK_MANAGER_ADDRESS as Address | undefined;
  const needsApproval = isActionable && !!tokenAddress && !!totalBudget && !!managerAddress;

  async function handleArm() {
    setStep('arming');
    try {
      await armTask.mutateAsync({ totalBudget: totalBudget ?? '0', durationSec });
      if (needsApproval) {
        setStep('approving');
        await writeContractAsync({
          address: tokenAddress!,
          abi: erc20Abi,
          functionName: 'approve',
          args: [managerAddress!, BigInt(totalBudget!)],
        });
      }
      setStep('armed');
    } catch {
      setStep('error');
    }
  }

  const disabled = !acknowledged || step === 'arming' || step === 'approving' || step === 'armed' || !address;
  const armLabel = step === 'idle' || step === 'error' ? 'Arm' : step === 'arming' ? 'Arming…' : step === 'approving' ? 'Approving…' : 'Armed';

  const chainSteps = [
    { n: '01', call: 'createTask + grantTradePermission', detail: "Commits the Task hash on-chain, signed by Quasar's operator wallet." },
    ...(needsApproval ? [{ n: '02', call: `approve(${tokenSymbol ?? 'token'})`, detail: 'Your own wallet grants a capped allowance the contract pulls from at execution.' }] : []),
  ];

  return (
    <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
      <div style={{ background: 'var(--ink)', color: 'var(--canvas)', padding: '11px 13px', display: 'flex', alignItems: 'baseline', gap: 9, flexWrap: 'wrap' }}>
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>Confirm and arm</span>
      </div>

      <div style={{ padding: 13, borderBottom: '1px solid var(--hairline)', fontSize: 13, lineHeight: 1.55, color: 'var(--ink-soft)' }}>
        {tokenAddress
          ? `Your ${tokenSymbol ?? 'position'} never leaves your wallet. You are not funding an agent wallet — you grant an allowance the contract pulls from at execution, capped, and revocable by you without asking anyone.`
          : 'This Task carries no trade — arming it only commits the request and its reasoning chain on-chain.'}
      </div>

      {chainSteps.map((cs) => (
        <div key={cs.n} style={{ borderBottom: '1px solid var(--hairline)', padding: '12px 13px', display: 'flex', gap: 11, alignItems: 'baseline', flexWrap: 'wrap' }}>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--merah)', flex: 'none' }}>{cs.n}</span>
          <span style={{ flex: '1 1 150px', minWidth: 0 }}>
            <span style={{ display: 'block', fontSize: 13, fontWeight: 600, lineHeight: 1.35 }}>{cs.call}</span>
            <span style={{ display: 'block', fontSize: 11.5, color: 'var(--body)', lineHeight: 1.45, marginTop: 3 }}>{cs.detail}</span>
          </span>
        </div>
      ))}

      <div style={{ padding: 13 }}>
        <label style={{ display: 'flex', gap: 10, alignItems: 'flex-start', cursor: 'pointer' }}>
          <input
            type="checkbox"
            checked={acknowledged}
            onChange={(e) => setAcknowledged(e.target.checked)}
            style={{ width: 15, height: 15, accentColor: 'var(--merah)', margin: '2px 0 0', flex: 'none' }}
          />
          <span style={{ fontSize: 12.5, lineHeight: 1.5, color: 'var(--ink-soft)' }}>
            I understand this is the only time I am asked. Once armed, Quasar acts inside these caps on its own until I disarm Task {taskId} — and I stay
            responsible for it.
          </span>
        </label>
        <button
          onClick={handleArm}
          disabled={disabled}
          style={{ appearance: 'none', border: 0, cursor: disabled ? 'not-allowed' : 'pointer', width: '100%', background: disabled ? 'var(--hairline-strong)' : 'var(--merah)', color: 'var(--putih)', font: '600 13.5px/1 var(--font-sans)', padding: 14, textAlign: 'left', marginTop: 12 }}
        >
          {armLabel}
        </button>
        {step === 'error' && <p style={{ marginTop: 8, fontSize: 12, color: 'var(--negative)' }}>Arming failed — try again.</p>}
        <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--body)', lineHeight: 1.6, marginTop: 10 }}>
          {needsApproval ? 'Two signatures in one flow: the Task hash, then the allowance.' : 'One signature: the Task hash.'} This is the human gate —
          after it, every Sub Task is mirrored on-chain instead of confirmed by you.
        </div>
      </div>
    </div>
  );
}

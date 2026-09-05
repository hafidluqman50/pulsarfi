'use client';

import { useState } from 'react';
import { useAccount, useWriteContract } from 'wagmi';
import { erc20Abi, type Address } from 'viem';
import { useArmTask } from '@/http/agent/hooks';

type ArmPanelProps = {
  taskId: number;
  isActionable: boolean;
  tokenAddress?: Address;
  totalBudget?: string;
  durationSec: number;
};

// Two independent signatures: arm() first (Task hash commitment,
// backend-signed via AGENT_ROLE), then the owner's own approve() (this
// component's own wallet call — never proxied through the backend, since
// only the owner can grant that allowance). The approve() step needs a
// real deployed AgentTaskManager address (NEXT_PUBLIC_AGENT_TASK_MANAGER_ADDRESS)
// — until the contract is actually deployed, arm() itself still works
// (backend stub), but this second step has nothing real to approve to yet.
export function ArmPanel({ taskId, isActionable, tokenAddress, totalBudget, durationSec }: ArmPanelProps) {
  const [acknowledged, setAcknowledged] = useState(false);
  const [step, setStep] = useState<'idle' | 'arming' | 'approving' | 'armed' | 'error'>('idle');
  const { address } = useAccount();
  const { writeContractAsync } = useWriteContract();
  const armTask = useArmTask(taskId);

  const managerAddress = process.env.NEXT_PUBLIC_AGENT_TASK_MANAGER_ADDRESS as Address | undefined;

  async function handleArm() {
    setStep('arming');
    try {
      await armTask.mutateAsync({ totalBudget: totalBudget ?? '0', durationSec });

      if (isActionable && tokenAddress && totalBudget && managerAddress) {
        setStep('approving');
        await writeContractAsync({
          address: tokenAddress,
          abi: erc20Abi,
          functionName: 'approve',
          args: [managerAddress, BigInt(totalBudget)],
        });
      }
      setStep('armed');
    } catch {
      setStep('error');
    }
  }

  return (
    <div className="arm-panel hairline p-[16px]">
      <p className="text-[13px]">
        Your {tokenAddress ? 'position' : 'request'} never leaves your wallet — you grant an allowance the contract
        pulls from at execution, capped, and revocable by you without asking anyone.
      </p>
      <label className="mt-[12px] flex items-center gap-[8px] text-[13px]">
        <input type="checkbox" checked={acknowledged} onChange={(e) => setAcknowledged(e.target.checked)} />
        I understand this arms Task {taskId} with no per-fill confirmation after this step.
      </label>
      <button
        className="btn btn-ghost !mt-[12px] !border !border-[var(--ink)] !px-[16px] !py-[8px] !text-[13px]"
        disabled={!acknowledged || step === 'arming' || step === 'approving' || step === 'armed' || !address}
        onClick={handleArm}
      >
        {step === 'idle' || step === 'error' ? 'Arm' : step === 'arming' ? 'Arming…' : step === 'approving' ? 'Approving…' : 'Armed'}
      </button>
      {step === 'error' && <p className="mt-[8px] text-[12px] text-[var(--negative)]">Arming failed — try again.</p>}
      <p className="mt-[8px] text-[12px] text-[var(--body)]">
        Two signatures in one flow: the Task hash, then the allowance. This is the human gate — after it, every Sub
        Task is mirrored on-chain instead of confirmed by you.
      </p>
    </div>
  );
}

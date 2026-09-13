'use client';

import { useState, useEffect, useRef } from 'react';
import { useAccount, useWriteContract, useReadContract, usePublicClient } from 'wagmi';
import { erc20Abi, BaseError, type Address } from 'viem';
import { toast } from 'sonner';
import { useArmTask, useExecuteTask, useAgentTasks } from '@/http/agent/hooks';
import { appChainId } from '@/lib/wagmi';
import { type CardContract, parseCardContract } from './cardContract';

type ArmPanelProps = {
  taskId: number;
  isActionable: boolean;
  tokenAddress?: Address;
  tokenSymbol?: string;
  totalBudget?: string;
  durationSec: number;
  initialArmed?: boolean;
  contract?: CardContract;
};

function parseBudgetProp(val?: string): string {
  if (!val) return '100000';
  const num = Number(val);
  if (isNaN(num) || num <= 0) return '100000';
  return num.toString();
}

function formatNumberWithDots(val: string | number): string {
  const digits = String(val).replace(/\D/g, '');
  if (!digits) return '';
  return Number(digits).toLocaleString();
}

function formatBudgetPreview(val: string | number, symbol: string): string {
  const digits = String(val).replace(/\D/g, '');
  const num = Number(digits);
  if (!num || isNaN(num) || num <= 0) return '';
  return `${num.toLocaleString()} ${symbol}`;
}

function formatContractError(error: unknown): string {
  const rawMsg =
    error instanceof BaseError
      ? error.shortMessage || error.message
      : error instanceof Error
      ? error.message
      : String(error);

  if (rawMsg.includes('max fee per gas less than block base fee') || rawMsg.includes('underpriced')) {
    return 'Gas fee too low: maxFeePerGas is below block base fee. In MetaMask, select "Market" or "Aggressive" gas, or do not set custom fee below network base fee.';
  }
  if (rawMsg.includes('User rejected')) {
    return 'Transaction rejected by user in wallet.';
  }
  if (rawMsg.includes('insufficient funds')) {
    return 'Insufficient ETH balance in wallet to pay for transaction gas fees.';
  }
  if (rawMsg.includes('execution reverted')) {
    return 'Transaction reverted on smart contract.';
  }
  return error instanceof BaseError ? error.shortMessage || error.message : error instanceof Error ? error.message : 'Transaction execution failed';
}

export function ArmPanel({ taskId, isActionable, tokenAddress, tokenSymbol, totalBudget, durationSec, initialArmed, contract: passedContract }: ArmPanelProps) {
  const [acknowledged, setAcknowledged] = useState(false);
  const [step, setStep] = useState<'idle' | 'arming' | 'approving' | 'executing' | 'executed' | 'error'>('idle');
  const [subProgress, setSubProgress] = useState<string>('');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const isExecutingRef = useRef(false);
  const { address } = useAccount();
  const publicClient = usePublicClient();
  const { writeContractAsync } = useWriteContract();
  const armTask = useArmTask(taskId);
  const executeTask = useExecuteTask(taskId);
  const { data: tasks = [] } = useAgentTasks();
  const currentTask = tasks.find((t) => t.id === taskId);
  const isTaskExecuted = currentTask?.status === 'executed';

  const contract = passedContract ?? parseCardContract(currentTask);

  const effectiveTokenAddress = tokenAddress ?? (process.env.NEXT_PUBLIC_IDRX_ADDRESS as Address | undefined);
  const effectiveTokenSymbol = tokenSymbol ?? 'IDRX';

  const [budgetDisplay, setBudgetDisplay] = useState(() => parseBudgetProp(totalBudget));
  const [userEdited, setUserEdited] = useState(false);

  useEffect(() => {
    if (!userEdited && totalBudget && Number(totalBudget) > 0) {
      setBudgetDisplay(parseBudgetProp(totalBudget));
    }
  }, [totalBudget, userEdited]);

  const effectiveRawBudget = BigInt(Math.max(1, Math.floor(Number(budgetDisplay) || 100000)) * 100).toString();

  const managerAddress = process.env.NEXT_PUBLIC_AGENT_TASK_MANAGER_ADDRESS as Address | undefined;
  const needsApproval = isActionable && !!effectiveTokenAddress && !!effectiveRawBudget && !!managerAddress;

  const { data: onChainAllowance, refetch: refetchAllowance } = useReadContract({
    address: effectiveTokenAddress,
    abi: erc20Abi,
    functionName: 'allowance',
    args: address && managerAddress ? [address, managerAddress] : undefined,
    query: {
      enabled: Boolean(address && managerAddress && effectiveTokenAddress),
    },
  });

  const hasSufficientAllowance = Boolean(
    onChainAllowance !== undefined && onChainAllowance >= BigInt(effectiveRawBudget)
  );

  const isArmed = Boolean(initialArmed || currentTask?.armed_at);
  const missingBudget = isActionable && (!budgetDisplay || Number(budgetDisplay) <= 0);

  async function handleArm() {
    if (missingBudget || !address || isExecutingRef.current) return;
    isExecutingRef.current = true;
    setErrorMessage(null);
    try {
      const alreadyArmed = Boolean(initialArmed || currentTask?.armed_at);
      if (!alreadyArmed) {
        setStep('arming');
        setSubProgress(contract.button_labels.arming);
        await armTask.mutateAsync({ totalBudget: effectiveRawBudget, durationSec });
      }

      if (needsApproval && !hasSufficientAllowance) {
        setStep('approving');
        setSubProgress(contract.button_labels.approving);

        let feeOverrides: { maxFeePerGas?: bigint; maxPriorityFeePerGas?: bigint } = {};
        if (publicClient) {
          try {
            const fees = await publicClient.estimateFeesPerGas();
            if (fees.maxFeePerGas) {
              feeOverrides.maxFeePerGas = (fees.maxFeePerGas * BigInt(125)) / BigInt(100);
              feeOverrides.maxPriorityFeePerGas = fees.maxPriorityFeePerGas ?? BigInt(100000000);
            }
          } catch (e) {
            console.warn('Failed to estimate fees per gas, using wallet defaults:', e);
          }
        }

        let writeRequest: any = {
          address: effectiveTokenAddress!,
          abi: erc20Abi,
          functionName: 'approve',
          args: [managerAddress!, BigInt(effectiveRawBudget)],
          chainId: appChainId,
          ...feeOverrides,
        };

        if (publicClient && address) {
          try {
            const { request: simRequest } = await publicClient.simulateContract({
              address: effectiveTokenAddress!,
              abi: erc20Abi,
              functionName: 'approve',
              args: [managerAddress!, BigInt(effectiveRawBudget)],
              account: address,
            });
            writeRequest = {
              ...simRequest,
              chainId: appChainId,
              ...feeOverrides,
            };
          } catch (simErr) {
            console.error('Simulate contract approve failed:', simErr);
            throw simErr;
          }
        }

        const txHash = await writeContractAsync(writeRequest);
        setSubProgress(contract.button_labels.submitting_approve || contract.button_labels.approving);

        if (publicClient) {
          const receipt = await publicClient.waitForTransactionReceipt({ hash: txHash });
          if (receipt.status !== 'success') {
            throw new Error('ERC20 approve transaction failed or reverted on-chain.');
          }
        }
        await refetchAllowance();
      }

      setStep('executing');
      setSubProgress(contract.button_labels.executing);
      const res = await executeTask.mutateAsync();

      if (res?.status === 'executed' && res.trades && res.trades.length > 0) {
        setStep('executed');
        toast.success('Transaction executed on-chain!', {
          description: 'Trade completed on Uniswap V4.',
        });
      } else {
        setStep('error');
        setErrorMessage(contract.footnotes.execution_failed);
        toast.error('Transaction was not executed on-chain', {
          description: 'See latest message from Comet in chat thread.',
        });
      }
    } catch (err: unknown) {
      setStep('error');
      const msg = formatContractError(err);
      setErrorMessage(msg);
      toast.error('Failed to arm or execute transaction', { description: msg });
    } finally {
      isExecutingRef.current = false;
    }
  }

  const disabled =
    isTaskExecuted ||
    missingBudget ||
    (!isArmed && !acknowledged) ||
    step === 'arming' ||
    step === 'approving' ||
    step === 'executing' ||
    step === 'executed' ||
    !address;

  const armLabel = (() => {
    if (isTaskExecuted || step === 'executed') {
      return contract.button_labels.executed;
    }
    if (step === 'arming' || step === 'approving' || step === 'executing') {
      return subProgress || contract.button_labels.executing;
    }
    if (!address) {
      return contract.button_labels.connect_wallet;
    }
    if (missingBudget) {
      return contract.button_labels.enter_budget;
    }
    if (!isArmed && !acknowledged) {
      return contract.button_labels.acknowledge_required;
    }
    return contract.button_labels.ready;
  })();

  return (
    <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
      <div style={{ background: 'var(--ink)', color: 'var(--canvas)', padding: '11px 13px', display: 'flex', alignItems: 'baseline', gap: 9, flexWrap: 'wrap' }}>
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>
          {isArmed ? contract.arm_title_armed : contract.arm_title_ready}
        </span>
      </div>

      <div style={{ padding: 13, borderBottom: '1px solid var(--hairline)', fontSize: 13, lineHeight: 1.55, color: 'var(--ink-soft)' }}>
        {effectiveTokenAddress ? contract.arm_description : contract.no_trade_description}
      </div>

      {!isArmed && (
        <div style={{ padding: '12px 13px', borderBottom: '1px solid var(--hairline)' }}>
          <label style={{ display: 'block', font: '700 9.5px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 6 }}>
            {contract.budget_label}
          </label>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 7 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <input
                type="text"
                value={formatNumberWithDots(budgetDisplay)}
                onChange={(e) => {
                  setUserEdited(true);
                  const raw = e.target.value.replace(/\D/g, '');
                  setBudgetDisplay(raw);
                }}
                disabled={step !== 'idle' && step !== 'error'}
                placeholder={contract.budget_placeholder}
                style={{
                  appearance: 'none',
                  border: '1px solid var(--hairline-strong)',
                  background: 'var(--canvas)',
                  color: 'var(--ink)',
                  font: '600 13px/1.3 var(--font-mono)',
                  padding: '7px 10px',
                  width: '180px',
                }}
              />
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--body)' }}>{effectiveTokenSymbol}</span>
            </div>

            {budgetDisplay && Number(budgetDisplay) > 0 && (
              <div style={{ fontSize: 11.5, fontFamily: 'var(--font-mono)', color: 'var(--positive)', background: 'var(--canvas-soft)', border: '1px solid var(--hairline)', padding: '4px 8px', alignSelf: 'flex-start' }}>
                ✓ {formatBudgetPreview(budgetDisplay, effectiveTokenSymbol)}
              </div>
            )}

            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginTop: 2 }}>
              <span style={{ fontSize: 9.5, color: 'var(--ticker)', alignSelf: 'center', marginRight: 2, fontFamily: 'var(--font-sans)', textTransform: 'uppercase', letterSpacing: '.08em' }}>
                {contract.preset_label}
              </span>
              {contract.presets.map((p) => (
                <button
                  key={p.value}
                  type="button"
                  onClick={() => {
                    setUserEdited(true);
                    setBudgetDisplay(p.value);
                  }}
                  disabled={step !== 'idle' && step !== 'error'}
                  style={{
                    appearance: 'none',
                    cursor: 'pointer',
                    border: `1px solid ${budgetDisplay === p.value ? 'var(--ink)' : 'var(--hairline-strong)'}`,
                    background: budgetDisplay === p.value ? 'var(--ink)' : 'transparent',
                    color: budgetDisplay === p.value ? 'var(--canvas)' : 'var(--body)',
                    font: '500 10.5px/1 var(--font-mono)',
                    padding: '4px 7px',
                  }}
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      {contract.steps.map((cs) => (
        <div key={cs.n} style={{ borderBottom: '1px solid var(--hairline)', padding: '12px 13px', display: 'flex', gap: 11, alignItems: 'baseline', flexWrap: 'wrap' }}>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--merah)', flex: 'none' }}>{cs.n}</span>
          <span style={{ flex: '1 1 150px', minWidth: 0 }}>
            <span style={{ display: 'block', fontSize: 13, fontWeight: 600, lineHeight: 1.35 }}>{cs.call}</span>
            <span style={{ display: 'block', fontSize: 11.5, color: 'var(--body)', lineHeight: 1.45, marginTop: 3 }}>{cs.detail}</span>
          </span>
        </div>
      ))}

      <div style={{ padding: 13 }}>
        {!isArmed && (
          <label style={{ display: 'flex', gap: 10, alignItems: 'flex-start', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={acknowledged}
              onChange={(e) => setAcknowledged(e.target.checked)}
              style={{ width: 15, height: 15, accentColor: 'var(--merah)', margin: '2px 0 0', flex: 'none' }}
            />
            <span style={{ fontSize: 12.5, lineHeight: 1.5, color: 'var(--ink-soft)' }}>
              {contract.disclaimer.replace('{id}', String(taskId))}
            </span>
          </label>
        )}

        <button
          onClick={handleArm}
          disabled={disabled}
          style={{ appearance: 'none', border: 0, cursor: disabled ? 'not-allowed' : 'pointer', width: '100%', background: disabled ? 'var(--hairline-strong)' : 'var(--merah)', color: 'var(--putih)', font: '600 13.5px/1 var(--font-sans)', padding: 14, textAlign: 'left', marginTop: 12 }}
        >
          {armLabel}
        </button>
        {step === 'error' && (
          <p style={{ marginTop: 8, fontSize: 12, color: 'var(--negative)', lineHeight: 1.45 }}>
            {errorMessage || contract.footnotes.execution_failed}
          </p>
        )}
        {step === 'executed' && (
          <p style={{ marginTop: 8, fontSize: 12, color: 'var(--positive)' }}>
            {contract.footnotes.executed_success}
          </p>
        )}
        <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--body)', lineHeight: 1.6, marginTop: 10 }}>
          {contract.footnotes.signatures_needed}
        </div>
      </div>
    </div>
  );
}

'use client';

import { useState, useEffect, useRef } from 'react';
import { useAccount, useWriteContract, useReadContract, usePublicClient } from 'wagmi';
import { erc20Abi, BaseError, type Address } from 'viem';
import { toast } from 'sonner';
import { useArmTask, useExecuteTask, useAgentTasks } from '@/http/agent/hooks';
import { appChainId } from '@/lib/wagmi';
import { type CardContract, type CardPreset, parseCardContract } from './cardContract';

type ArmPanelProps = {
  taskId: number;
  isActionable: boolean;
  tokenAddress?: Address;
  tokenSymbol?: string;
  side?: 'buy' | 'sell';
  shape?: 'scalp' | 'swing' | 'investment' | string;
  totalBudget?: string;
  durationSec?: number;
  initialArmed?: boolean;
  contract?: CardContract;
};

function parseBudgetProp(val?: string, isSell?: boolean): string {
  if (!val) return isSell ? '1' : '100000';
  const num = Number(val);
  if (isNaN(num) || num <= 0) return isSell ? '1' : '100000';
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

export function ArmPanel({ taskId, isActionable, tokenAddress, tokenSymbol, side = 'buy', shape, totalBudget, durationSec, initialArmed, contract: passedContract }: ArmPanelProps) {
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

  const parsedTrigger = (() => {
    if (!currentTask?.trigger_description) return null;
    try {
      return JSON.parse(currentTask.trigger_description);
    } catch {
      return null;
    }
  })();

  const effectiveSide: 'buy' | 'sell' = (() => {
    const s = (side || parsedTrigger?.side || '').toLowerCase();
    if (s === 'sell' || s === 'jual') return 'sell';
    return 'buy';
  })();

  const isSell = effectiveSide === 'sell';
  const decimals = isSell ? 18 : 2;

  const effectiveTokenAddress = isSell
    ? (tokenAddress || (parsedTrigger?.token_address || parsedTrigger?.stock_contract_address as Address | undefined))
    : (process.env.NEXT_PUBLIC_IDRX_ADDRESS as Address | undefined);

  const effectiveTokenSymbol = isSell
    ? (tokenSymbol || parsedTrigger?.token_symbol || parsedTrigger?.stock_ticker || parsedTrigger?.resolved_ticker || 'TOKEN')
    : 'IDRX';

  const effectiveShape: 'scalp' | 'swing' | 'investment' = (() => {
    const raw = (shape || parsedTrigger?.shape || '').toLowerCase();
    if (raw.includes('swing')) return 'swing';
    if (raw.includes('invest') || raw.includes('dca')) return 'investment';
    return 'scalp';
  })();

  const [budgetDisplay, setBudgetDisplay] = useState(() => parseBudgetProp(totalBudget, isSell));
  const [maxPerTradeDisplay, setMaxPerTradeDisplay] = useState('');
  const [cooldownSec, setCooldownSec] = useState<number>(0);
  const [swingHorizonDays, setSwingHorizonDays] = useState<number>(14);
  const [isRecurring, setIsRecurring] = useState(effectiveShape === 'investment');
  const [showAdvanced, setShowAdvanced] = useState(effectiveShape === 'investment');
  const [userEdited, setUserEdited] = useState(false);

  useEffect(() => {
    if (!userEdited && totalBudget && Number(totalBudget) > 0) {
      setBudgetDisplay(parseBudgetProp(totalBudget, isSell));
    }
  }, [totalBudget, userEdited, isSell]);

  const parsedNum = Math.max(1, Math.floor(Number(budgetDisplay) || (isSell ? 1 : 100000)));
  const effectiveRawBudget = (BigInt(parsedNum) * (BigInt(10) ** BigInt(decimals))).toString();

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

        let maxPerTradeRaw: string | undefined = undefined;
        if (maxPerTradeDisplay && Number(maxPerTradeDisplay) > 0) {
          const mNum = Math.max(1, Math.floor(Number(maxPerTradeDisplay)));
          maxPerTradeRaw = (BigInt(mNum) * (BigInt(10) ** BigInt(decimals))).toString();
        }

        const effectiveDuration = effectiveShape === 'swing'
          ? swingHorizonDays * 24 * 60 * 60
          : effectiveShape === 'investment'
          ? 30 * 24 * 60 * 60
          : (durationSec || 24 * 60 * 60);

        await armTask.mutateAsync({
          totalBudget: effectiveRawBudget,
          durationSec: effectiveDuration,
          tokenAddress: effectiveTokenAddress,
          maxAmountPerTrade: maxPerTradeRaw,
          cooldownInterval: cooldownSec > 0 ? cooldownSec : undefined,
          isRecurring: effectiveShape === 'investment' ? isRecurring : false,
        });
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

  const sellPresets: CardPreset[] = (contract.sell_presets && contract.sell_presets.length > 0)
    ? contract.sell_presets
    : [
        { label: '1 Token', value: '1' },
        { label: '5 Tokens', value: '5' },
        { label: '10 Tokens', value: '10' },
        { label: '50 Tokens', value: '50' },
        { label: '100 Tokens', value: '100' },
      ];
  const activePresets = isSell ? sellPresets : contract.presets;
  const budgetLabel = isSell
    ? (contract.sell_budget_label || 'Stock Sale Quantity Limit ({token})').replace(/{token}/g, effectiveTokenSymbol)
    : contract.budget_label;
  const budgetPlaceholder = isSell
    ? (contract.sell_budget_placeholder || '10')
    : contract.budget_placeholder;

  const shapeBadgeText = contract.shape_badge || (
    effectiveShape === 'scalp' ? 'SCALP SPOT SWAP' :
    effectiveShape === 'swing' ? 'SWING TRADE · HORIZON MONITORING' :
    'INVESTMENT / DCA PLAN'
  );

  return (
    <div className="rise" style={{ border: '1px solid var(--ink)', background: 'var(--putih)' }}>
      <div style={{ background: 'var(--ink)', color: 'var(--canvas)', padding: '11px 13px', display: 'flex', alignItems: 'baseline', gap: 9, flexWrap: 'wrap' }}>
        <span style={{ font: '700 10px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase' }}>
          {isArmed ? contract.arm_title_armed : contract.arm_title_ready}
        </span>
        <span style={{ fontSize: 9.5, fontFamily: 'var(--font-mono)', background: 'var(--canvas)', color: 'var(--ink)', padding: '2px 6px', letterSpacing: '.05em', textTransform: 'uppercase', fontWeight: 600 }}>
          {shapeBadgeText}
        </span>
      </div>

      <div style={{ padding: 13, borderBottom: '1px solid var(--hairline)', fontSize: 13, lineHeight: 1.55, color: 'var(--ink-soft)' }}>
        {effectiveTokenAddress
          ? isSell
            ? (contract.sell_arm_description || 'Your {token} tokens never leave your wallet before execution occurs. You are granting an allowance limit that the smart contract pulls upon execution, strictly capped, and revokable at any time.').replace(/{token}/g, effectiveTokenSymbol)
            : contract.arm_description
          : contract.no_trade_description}
      </div>

      {!isArmed && (
        <div style={{ padding: '12px 13px', borderBottom: '1px solid var(--hairline)' }}>
          <label style={{ display: 'block', font: '700 9.5px/1 var(--font-sans)', letterSpacing: '.14em', textTransform: 'uppercase', color: 'var(--ticker)', marginBottom: 6 }}>
            {budgetLabel}
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
                placeholder={budgetPlaceholder}
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
              {activePresets.map((p) => (
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

            {/* Swing Horizon Selector */}
            {effectiveShape === 'swing' && (
              <div style={{ marginTop: 10, padding: 10, background: 'var(--canvas-soft)', border: '1px solid var(--hairline)', display: 'flex', flexDirection: 'column', gap: 8 }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 6 }}>
                  <label style={{ fontSize: 10, textTransform: 'uppercase', letterSpacing: '.08em', color: 'var(--ticker)', fontWeight: 700 }}>
                    {contract.horizon_label || 'Holding Horizon'}
                  </label>
                  <div style={{ display: 'flex', gap: 4 }}>
                    {[7, 14, 30].map((d) => (
                      <button
                        key={d}
                        type="button"
                        onClick={() => setSwingHorizonDays(d)}
                        disabled={step !== 'idle' && step !== 'error'}
                        style={{
                          appearance: 'none',
                          cursor: 'pointer',
                          border: `1px solid ${swingHorizonDays === d ? 'var(--ink)' : 'var(--hairline-strong)'}`,
                          background: swingHorizonDays === d ? 'var(--ink)' : 'transparent',
                          color: swingHorizonDays === d ? 'var(--canvas)' : 'var(--body)',
                          font: '600 10.5px/1 var(--font-mono)',
                          padding: '4px 8px',
                        }}
                      >
                        {d}d
                      </button>
                    ))}
                  </div>
                </div>
                <div style={{ fontSize: 11, color: 'var(--body)', lineHeight: 1.4 }}>
                  {contract.horizon_notice || 'Proactive Alert: You will be notified 24 hours prior to horizon expiry to decide whether to exit or hold.'}
                </div>
              </div>
            )}

            {/* Investment DCA & Guardrails */}
            {effectiveShape === 'investment' && (
              <div style={{ marginTop: 10 }}>
                <button
                  type="button"
                  onClick={() => setShowAdvanced(!showAdvanced)}
                  style={{
                    appearance: 'none',
                    border: 'none',
                    background: 'transparent',
                    padding: 0,
                    cursor: 'pointer',
                    color: 'var(--merah)',
                    font: '600 11px/1.3 var(--font-sans)',
                    textDecoration: 'underline',
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 4,
                  }}
                >
                  {showAdvanced
                    ? `▾ ${contract.guardrail_hide_label || 'Hide Guardrail & DCA Options'}`
                    : `▸ ${contract.guardrail_label || 'DCA & Transaction Limits (Optional)'}`}
                </button>

                {showAdvanced && (
                  <div style={{ marginTop: 8, padding: 10, background: 'var(--canvas-soft)', border: '1px solid var(--hairline)', display: 'flex', flexDirection: 'column', gap: 10 }}>
                    <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', fontSize: 12 }}>
                      <input
                        type="checkbox"
                        checked={isRecurring}
                        onChange={(e) => setIsRecurring(e.target.checked)}
                        style={{ accentColor: 'var(--merah)', width: 14, height: 14 }}
                      />
                      <span style={{ fontWeight: 600, color: 'var(--ink)' }}>{contract.recurring_label || 'Recurring Execution (DCA)'}</span>
                    </label>

                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                      <label style={{ fontSize: 10, textTransform: 'uppercase', letterSpacing: '.08em', color: 'var(--ticker)', fontWeight: 700 }}>
                        {(contract.max_per_trade_label || 'Max per Trade ({token})').replace(/{token}/g, effectiveTokenSymbol)}
                      </label>
                      <input
                        type="text"
                        value={formatNumberWithDots(maxPerTradeDisplay)}
                        onChange={(e) => setMaxPerTradeDisplay(e.target.value.replace(/\D/g, ''))}
                        placeholder={contract.max_per_trade_placeholder || 'Unlimited (entire budget)'}
                        disabled={step !== 'idle' && step !== 'error'}
                        style={{
                          appearance: 'none',
                          border: '1px solid var(--hairline-strong)',
                          background: 'var(--canvas)',
                          color: 'var(--ink)',
                          font: '600 12px/1.3 var(--font-mono)',
                          padding: '6px 8px',
                          width: '180px',
                        }}
                      />
                    </div>

                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                      <label style={{ fontSize: 10, textTransform: 'uppercase', letterSpacing: '.08em', color: 'var(--ticker)', fontWeight: 700 }}>
                        {contract.cooldown_label || 'Cooldown Between Trades'}
                      </label>
                      <select
                        value={cooldownSec}
                        onChange={(e) => setCooldownSec(Number(e.target.value))}
                        disabled={step !== 'idle' && step !== 'error'}
                        style={{
                          border: '1px solid var(--hairline-strong)',
                          background: 'var(--canvas)',
                          color: 'var(--ink)',
                          font: '500 12px/1.3 var(--font-mono)',
                          padding: '6px 8px',
                          width: '180px',
                        }}
                      >
                        <option value={0}>{contract.cooldown_none_option || 'No Cooldown (All at once)'}</option>
                        <option value={3600}>{contract.cooldown_1h_option || '1 Hour'}</option>
                        <option value={14400}>{contract.cooldown_4h_option || '4 Hours'}</option>
                        <option value={86400}>{contract.cooldown_1d_option || '1 Day (24 Hours)'}</option>
                        <option value={604800}>{contract.cooldown_1w_option || '7 Days (1 Week)'}</option>
                      </select>
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {contract.steps.map((cs) => {
        let callText = cs.call;
        let detailText = cs.detail;
        if (isSell) {
          callText = callText.replace(/IDRX/g, effectiveTokenSymbol);
          detailText = detailText.replace(/IDRX/g, effectiveTokenSymbol);
        }
        return (
          <div key={cs.n} style={{ borderBottom: '1px solid var(--hairline)', padding: '12px 13px', display: 'flex', gap: 11, alignItems: 'baseline', flexWrap: 'wrap' }}>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--merah)', flex: 'none' }}>{cs.n}</span>
            <span style={{ flex: '1 1 150px', minWidth: 0 }}>
              <span style={{ display: 'block', fontSize: 13, fontWeight: 600, lineHeight: 1.35 }}>{callText}</span>
              <span style={{ display: 'block', fontSize: 11.5, color: 'var(--body)', lineHeight: 1.45, marginTop: 3 }}>{detailText}</span>
            </span>
          </div>
        );
      })}

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

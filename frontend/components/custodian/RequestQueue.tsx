'use client';

import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  recordMintApproval,
  recordMintRejection,
  recordRedeemApproval,
  recordRedeemRejection,
  type CustodianRequest,
} from '@/http/custodian/custodianApi';
import {
  useApproveMint,
  useApproveRedeem,
  useExecuteMint,
  useExecuteRejectMint,
  useExecuteRedeem,
  useExecuteRejectRedeem,
  useRejectMint,
  useRejectRedeem,
} from '@/http/custodian/contractHooks';
import { PStockMark } from '@/components/ui/PStockMark';
import { AttestorsModal } from './AttestorsModal';
import { requestKey, formatRawToken, formatRawIDR, relativeAge, shortHash } from './utils';

const THRESHOLD = 3;

const QUEUE_GRID = 'grid grid-cols-[32px_1fr_1fr_1fr_1fr_1fr_1fr_minmax(max-content,2fr)] gap-[12px]';
const ACTION_BASE = 'cursor-pointer appearance-none whitespace-nowrap text-[12px] font-[600] disabled:cursor-not-allowed';
const OUTLINE_BUTTON = `${ACTION_BASE} border border-[var(--ink)] bg-[var(--putih)] px-[12px] py-[6px] text-[var(--ink)]`;
const APPROVE_BUTTON = `${ACTION_BASE} border-0 bg-[var(--ink)] px-[14px] py-[7px] text-[var(--putih)] shadow-[0_3px_0_-1px_#fff,0_4px_0_-1px_#16110e]`;
const EXECUTE_MERAH = `${ACTION_BASE} border-0 bg-[var(--merah)] px-[14px] py-[7px] text-[var(--putih)] shadow-[0_3px_0_-1px_#fff,0_4px_0_-1px_#9a0c24]`;

interface RequestQueueProps {
  requests: CustodianRequest[];
  isLoading: boolean;
  currentAddress?: string;
}

function EmptyRow({ text }: { text: string }): React.ReactNode {
  return (
    <div className="border-b border-[var(--hairline)] py-[18px] text-[13px] text-[var(--body)]">
      {text}
    </div>
  );
}

export function RequestQueue({ requests, isLoading, currentAddress }: RequestQueueProps): React.ReactNode {
  const [completedRequests, setCompletedRequests] = useState<Record<string, string>>({});
  const [activeRequest, setActiveRequest] = useState<CustodianRequest | null>(null);
  const queryClient = useQueryClient();
  const approveMint        = useApproveMint();
  const rejectMint         = useRejectMint();
  const executeMint        = useExecuteMint();
  const executeRejectMint  = useExecuteRejectMint();
  const approveRedeem      = useApproveRedeem();
  const rejectRedeem       = useRejectRedeem();
  const executeRedeem      = useExecuteRedeem();
  const executeRejectRedeem = useExecuteRejectRedeem();

  async function handleExecuteMint(request: CustodianRequest): Promise<void> {
    const toastId = toast.loading(`Executing mint REQ-${request.on_chain_id}…`, { description: "Sign tx in wallet" });
    try {
      const txHash = await executeMint.mutateAsync(BigInt(request.on_chain_id));
      setCompletedRequests(prev => ({ ...prev, [requestKey(request)]: "executed" }));
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
      toast.success(`Mint executed`, { id: toastId, description: `${shortHash(txHash)} · tokens minted`, duration: 4000 });
    } catch (error: unknown) {
      toast.error(`Execute failed`, { id: toastId, description: (error instanceof Error ? error.message : "Execution failed").slice(0, 120) });
    }
  }

  async function handleExecuteRejectMint(request: CustodianRequest): Promise<void> {
    const toastId = toast.loading(`Cancelling REQ-${request.on_chain_id}…`, { description: "Sign tx in wallet" });
    try {
      const txHash = await executeRejectMint.mutateAsync(BigInt(request.on_chain_id));
      setCompletedRequests(prev => ({ ...prev, [requestKey(request)]: "rejected" }));
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
      toast.info(`Mint cancelled`, { id: toastId, description: `${shortHash(txHash)} · proposal rejected`, duration: 4000 });
    } catch (error: unknown) {
      toast.error(`Cancel failed`, { id: toastId, description: (error instanceof Error ? error.message : "Cancellation failed").slice(0, 120) });
    }
  }

  async function handleExecuteRedeem(request: CustodianRequest): Promise<void> {
    const toastId = toast.loading(`Executing redeem REQ-${request.on_chain_id}…`, { description: 'Sign tx in wallet' });
    try {
      const txHash = await executeRedeem.mutateAsync(BigInt(request.on_chain_id));
      setCompletedRequests(prev => ({ ...prev, [requestKey(request)]: 'executed' }));
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
      toast.success(`Redeem executed`, { id: toastId, description: `${shortHash(txHash)} · tokens burned`, duration: 4000 });
    } catch (error: unknown) {
      toast.error(`Execute redeem failed`, { id: toastId, description: (error instanceof Error ? error.message : 'Execution failed').slice(0, 120) });
    }
  }

  async function handleExecuteRejectRedeem(request: CustodianRequest): Promise<void> {
    const toastId = toast.loading(`Rejecting redeem REQ-${request.on_chain_id}…`, { description: 'Sign tx in wallet' });
    try {
      const txHash = await executeRejectRedeem.mutateAsync(BigInt(request.on_chain_id));
      setCompletedRequests(prev => ({ ...prev, [requestKey(request)]: 'rejected' }));
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
      toast.info(`Redeem rejected`, { id: toastId, description: `${shortHash(txHash)} · tokens returned`, duration: 4000 });
    } catch (error: unknown) {
      toast.error(`Reject execution failed`, { id: toastId, description: (error instanceof Error ? error.message : 'Rejection failed').slice(0, 120) });
    }
  }

  async function handleApproveMint(request: CustodianRequest): Promise<void> {
    const toastId = toast.loading(`Approving mint REQ-${request.on_chain_id}`, { description: `${formatRawToken(request.token_amount)} ${request.ticker}` });
    try {
      const txHash = await approveMint.mutateAsync(BigInt(request.on_chain_id));
      await recordMintApproval(request.on_chain_id, txHash);
      setCompletedRequests(prev => ({ ...prev, [requestKey(request)]: 'approved' }));
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
      toast.success(`Mint REQ-${request.on_chain_id} approved`, { id: toastId, description: `${shortHash(txHash)} recorded`, duration: 3500 });
    } catch (error: unknown) {
      toast.error('Mint approval failed', { id: toastId, description: (error instanceof Error ? error.message : 'Approval failed').slice(0, 120) });
    }
  }

  async function handleRejectMint(request: CustodianRequest): Promise<void> {
    const toastId = toast.loading(`Rejecting mint REQ-${request.on_chain_id}`, { description: `${formatRawToken(request.token_amount)} ${request.ticker}` });
    try {
      const txHash = await rejectMint.mutateAsync(BigInt(request.on_chain_id));
      await recordMintRejection(request.on_chain_id, txHash);
      setCompletedRequests(prev => ({ ...prev, [requestKey(request)]: 'rejected' }));
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
      toast.info(`Mint REQ-${request.on_chain_id} rejected`, { id: toastId, description: `${shortHash(txHash)} recorded`, duration: 3500 });
    } catch (error: unknown) {
      toast.error('Mint rejection failed', { id: toastId, description: (error instanceof Error ? error.message : 'Rejection failed').slice(0, 120) });
    }
  }

  async function handleApproveRedeem(request: CustodianRequest): Promise<void> {
    const toastId = toast.loading(`Approving redeem REQ-${request.on_chain_id}`, { description: `${formatRawToken(request.token_amount)} ${request.ticker}` });
    try {
      const txHash = await approveRedeem.mutateAsync(BigInt(request.on_chain_id));
      await recordRedeemApproval(request.on_chain_id, txHash);
      setCompletedRequests(prev => ({ ...prev, [requestKey(request)]: 'approved' }));
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
      toast.success(`Redeem REQ-${request.on_chain_id} approved`, { id: toastId, description: `${shortHash(txHash)} recorded`, duration: 3500 });
    } catch (error: unknown) {
      toast.error('Redeem approval failed', { id: toastId, description: (error instanceof Error ? error.message : 'Approval failed').slice(0, 120) });
    }
  }

  async function handleRejectRedeem(request: CustodianRequest): Promise<void> {
    const toastId = toast.loading(`Rejecting redeem REQ-${request.on_chain_id}`, { description: `${formatRawToken(request.token_amount)} ${request.ticker}` });
    try {
      const txHash = await rejectRedeem.mutateAsync(BigInt(request.on_chain_id));
      await recordRedeemRejection(request.on_chain_id, txHash);
      setCompletedRequests(prev => ({ ...prev, [requestKey(request)]: 'rejected' }));
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
      toast.info(`Redeem REQ-${request.on_chain_id} rejected`, { id: toastId, description: `${shortHash(txHash)} recorded`, duration: 3500 });
    } catch (error: unknown) {
      toast.error('Redeem rejection failed', { id: toastId, description: (error instanceof Error ? error.message : 'Rejection failed').slice(0, 120) });
    }
  }

  return (
    <>
      {activeRequest && <AttestorsModal request={activeRequest} onClose={() => setActiveRequest(null)} />}
      <div className="paper-stack mt-[16px] overflow-x-auto px-[clamp(12px,2vw,20px)]">
        <div className="min-w-[980px]">
          <div className={`${QUEUE_GRID} border-b border-[var(--ink)] py-[12px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]`}>
            {["", "ID", "Type", "Asset", "Quantity", "IDR notional", "Waited", ""].map((heading, columnIndex) => (
              <span key={columnIndex} className={columnIndex >= 4 && columnIndex <= 6 ? "text-right" : "text-left"}>{heading}</span>
            ))}
          </div>
          {isLoading && <EmptyRow text="Loading custodian requests…" />}
          {!isLoading && requests.length === 0 && <EmptyRow text="No pending mint or redeem requests" />}
          {requests.map(request => {
            const status      = completedRequests[requestKey(request)];
            const isMint      = request.kind === "mint";
            const quantity    = formatRawToken(request.token_amount);
            const idrNotional = request.idrx_amount ? formatRawIDR(request.idrx_amount) : "—";

            // Mint-specific
            const isRequester = isMint &&
              !!currentAddress &&
              !!request.requester_address &&
              request.requester_address.toLowerCase() === currentAddress.toLowerCase();
            const canExecute  = request.approval_count >= THRESHOLD;
            const canCancel   = request.reject_count   >= THRESHOLD;

            // Redeem-specific
            const hasAttested = !isMint && !!currentAddress &&
              (request.attestors ?? []).some(a => a.wallet_address.toLowerCase() === currentAddress.toLowerCase());
            const isApproveInitiator = !isMint && !!currentAddress && !!request.approve_initiator_address &&
              request.approve_initiator_address.toLowerCase() === currentAddress.toLowerCase();
            const isRejectInitiator = !isMint && !!currentAddress && !!request.reject_initiator_address &&
              request.reject_initiator_address.toLowerCase() === currentAddress.toLowerCase();
            const canExecuteRedeem = request.approval_count >= THRESHOLD;
            const canExecuteRejectRedeem = request.reject_count >= THRESHOLD;

            return (
              <div key={requestKey(request)} className={`${QUEUE_GRID} items-center border-b border-[var(--hairline)] py-[14px] transition-opacity duration-300 ${status ? "opacity-[0.55]" : "opacity-100"}`}>
                <span className={`grid h-[32px] w-[32px] place-items-center text-[var(--putih)] ${isMint ? "bg-[var(--merah)] shadow-[2px_2px_0_#e8b4bd]" : "bg-[var(--ink)] shadow-[2px_2px_0_#bcb2a3]"}`}>
                  {isMint ? "↑" : "↓"}
                </span>
                <span className="mono text-[13px] leading-[normal]">REQ-{request.on_chain_id}</span>
                <span className={`text-[11px] font-[700] uppercase leading-[normal] tracking-[0.06em] ${isMint ? "text-[var(--merah)]" : "text-[var(--ink)]"}`}>{request.kind} · {request.source}</span>
                <span className="flex items-center gap-[8px]"><PStockMark ticker={request.ticker} size={22} /><span className="text-[13px] font-[600]">{request.ticker}</span></span>
                <span className="mono text-right text-[13px] leading-[normal]">{quantity}</span>
                <span className="mono text-right text-[13px] leading-[normal]">{idrNotional}</span>
                <span className="mono text-right text-[12px] leading-[normal] text-[var(--body)]">{relativeAge(request.created_at)}</span>
                <span className="flex items-center justify-end gap-[8px]">
                  {(request.attestors?.length ?? 0) > 0 && (
                    <button
                      onClick={() => setActiveRequest(request)}
                      className="inline-flex cursor-pointer items-center gap-[5px] whitespace-nowrap border border-[var(--hairline-strong)] bg-[var(--putih)] px-[10px] py-[5px] text-[11px] font-[600] text-[var(--body)] [font-family:var(--font-inter,_Inter,_sans-serif)] hover:border-[var(--ink)] hover:text-[var(--ink)]"
                    >
                      <span className="h-[6px] w-[6px] rounded-full bg-[var(--merah)]" />
                      {request.attestors!.length}/5
                    </button>
                  )}

                  {/* ── Mint actions ── */}
                  {!status && isMint && isRequester && canExecute && (
                    <button className={EXECUTE_MERAH} onClick={() => handleExecuteMint(request)}>Execute Mint</button>
                  )}
                  {!status && isMint && isRequester && !canExecute && canCancel && (
                    <button className={OUTLINE_BUTTON} onClick={() => handleExecuteRejectMint(request)}>Cancel Mint</button>
                  )}
                  {!status && isMint && isRequester && !canExecute && !canCancel && (
                    <div className="flex items-center gap-[8px]">
                      <span className="mono whitespace-nowrap text-[11px] text-[var(--body)]">{request.approval_count}/{THRESHOLD} approve · {request.reject_count}/{THRESHOLD} reject</span>
                      <button className={`${OUTLINE_BUTTON} !border-[var(--hairline)] opacity-40`} disabled>Cancel Mint</button>
                      <button className={`${EXECUTE_MERAH} opacity-40`} disabled>Execute Mint</button>
                    </div>
                  )}
                  {!status && isMint && !isRequester && (
                    <>
                      <button className={OUTLINE_BUTTON} onClick={() => handleRejectMint(request)}>Reject</button>
                      <button className={APPROVE_BUTTON} onClick={() => handleApproveMint(request)}>Approve</button>
                    </>
                  )}

                  {/* ── Redeem actions ── */}
                  {!status && !isMint && !hasAttested && (
                    <>
                      <button className={OUTLINE_BUTTON} onClick={() => handleRejectRedeem(request)}>Reject</button>
                      <button className={APPROVE_BUTTON} onClick={() => handleApproveRedeem(request)}>Approve</button>
                    </>
                  )}
                  {!status && !isMint && hasAttested && isApproveInitiator && canExecuteRedeem && (
                    <button className={APPROVE_BUTTON} onClick={() => handleExecuteRedeem(request)}>Execute Redeem</button>
                  )}
                  {!status && !isMint && hasAttested && isRejectInitiator && canExecuteRejectRedeem && (
                    <button className={OUTLINE_BUTTON} onClick={() => handleExecuteRejectRedeem(request)}>Cancel Redeem</button>
                  )}
                  {!status && !isMint && hasAttested && !(isApproveInitiator && canExecuteRedeem) && !(isRejectInitiator && canExecuteRejectRedeem) && (
                    <div className="flex items-center gap-[8px]">
                      <span className="mono whitespace-nowrap text-[11px] text-[var(--body)]">{request.approval_count}/{THRESHOLD} approve · {request.reject_count}/{THRESHOLD} reject</span>
                      <button className={`${OUTLINE_BUTTON} !border-[var(--hairline)] opacity-40`} disabled>Voted</button>
                    </div>
                  )}

                  {status && (
                    <span className={`-rotate-3 border px-[8px] py-[4px] text-[10px] font-[700] uppercase leading-[normal] tracking-[0.14em] ${status === "rejected" ? "border-[var(--negative)] text-[var(--negative)]" : "border-[var(--positive)] text-[var(--positive)]"}`}>
                      {status === "executed"
                        ? isMint ? "Executed · tokens minted" : "Executed · tokens burned"
                        : status === "approved" ? "Approved"
                        : "Rejected"}
                    </span>
                  )}
                </span>
              </div>
            );
          })}
        </div>
      </div>
    </>
  );
}

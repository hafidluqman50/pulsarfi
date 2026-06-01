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
import { Icon } from '@/components/ui/Icon';
import { PStockMark } from '@/components/ui/PStockMark';
import { AttestorsModal } from './AttestorsModal';
import { requestKey, formatRawToken, formatRawIDR, relativeAge, shortHash } from './utils';

const THRESHOLD = 3;

interface RequestQueueProps {
  requests: CustodianRequest[];
  isLoading: boolean;
  currentAddress?: string;
}

function EmptyRow({ text }: { text: string }): React.ReactNode {
  return (
    <div className="hairline py-[18px] text-[13px] text-[var(--body)]">
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
      <div>
        <div className="hairline table-head-desktop grid grid-cols-[32px_1fr_1fr_1fr_1fr_1fr_1fr_1.6fr] gap-[12px] py-[12px]">
          {["", "ID", "Type", "Asset", "Quantity", "IDR notional", "Waited", ""].map((heading, columnIndex) => (
            <div key={columnIndex} className={`eyebrow !text-[var(--body)] ${columnIndex >= 4 && columnIndex <= 6 ? "text-right" : "text-left"}`}>{heading}</div>
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
            <div key={requestKey(request)} className={`hairline table-row-stack grid grid-cols-[32px_1fr_1fr_1fr_1fr_1fr_1fr_1.6fr] items-center gap-[12px] py-[16px] ${status ? "opacity-[0.55]" : "opacity-100"}`}>
              <div className={`flex h-[32px] w-[32px] shrink-0 items-center justify-center border border-[var(--ink)] text-[var(--putih)] ${isMint ? "bg-[var(--merah)]" : "bg-[var(--ink)]"}`}>
                <Icon name={isMint ? "arrow-up" : "arrow-down"} size={14} />
              </div>
              <div className="row-cell"><span className="row-cell-label">ID</span><div className="row-cell-value"><span className="mono text-[13px]">REQ-{request.on_chain_id}</span></div></div>
              <div className="row-cell"><span className="row-cell-label">Type</span><div className="row-cell-value"><span className={`text-[11px] font-bold uppercase tracking-[0.06em] ${isMint ? "text-[var(--merah)]" : "text-[var(--ink)]"}`}>{request.kind} · {request.source}</span></div></div>
              <div className="row-cell"><span className="row-cell-label">Asset</span><div className="row-cell-value"><div className="flex items-center gap-[8px]"><PStockMark ticker={request.ticker} size={22} /><span className="text-[13px] font-semibold">{request.ticker}</span></div></div></div>
              <div className="row-cell text-right"><span className="row-cell-label">Quantity</span><div className="row-cell-value"><span className="mono text-[13px]">{quantity}</span></div></div>
              <div className="row-cell text-right"><span className="row-cell-label">IDR notional</span><div className="row-cell-value"><span className="mono text-[13px]">{idrNotional}</span></div></div>
              <div className="row-cell text-right"><span className="row-cell-label">Waited</span><div className="row-cell-value"><span className="mono text-[12px] text-[var(--body)]">{relativeAge(request.created_at)}</span></div></div>
              <div className="col-actions pos-actions flex items-center justify-end gap-[8px]">
                {(request.attestors?.length ?? 0) > 0 && (
                  <button
                    onClick={() => setActiveRequest(request)}
                    className="inline-flex cursor-pointer items-center gap-[5px] border border-[var(--hairline-strong)] bg-transparent px-[10px] py-[4px] text-[11px] font-semibold tracking-[0.06em] text-[var(--body)] [font-family:var(--font-inter,_Inter,_sans-serif)]"
                  >
                    <span className="h-[6px] w-[6px] rounded-[999px] bg-[var(--merah)]" />
                    {request.attestors!.length}/5
                  </button>
                )}

                {/* ── Mint actions ── */}
                {!status && isMint && isRequester && canExecute && (
                  <button className="btn btn-merah !px-[14px] !py-[6px]" onClick={() => handleExecuteMint(request)}>Execute Mint</button>
                )}
                {!status && isMint && isRequester && !canExecute && canCancel && (
                  <button className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px]" onClick={() => handleExecuteRejectMint(request)}>Cancel Mint</button>
                )}
                {!status && isMint && isRequester && !canExecute && !canCancel && (
                  <div className="flex items-center gap-[8px]">
                    <span className="mono text-[11px] text-[var(--body)]">{request.approval_count}/{THRESHOLD} approve · {request.reject_count}/{THRESHOLD} reject</span>
                    <button className="btn btn-ghost !cursor-not-allowed !border !border-[var(--hairline)] !px-[12px] !py-[6px] !opacity-40" disabled>Cancel Mint</button>
                    <button className="btn btn-merah !cursor-not-allowed !px-[14px] !py-[6px] !opacity-40" disabled>Execute Mint</button>
                  </div>
                )}
                {!status && isMint && !isRequester && (
                  <>
                    <button className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px]" onClick={() => handleRejectMint(request)}>Reject</button>
                    <button className="btn btn-primary !px-[14px] !py-[6px]" onClick={() => handleApproveMint(request)}>Approve</button>
                  </>
                )}

                {/* ── Redeem actions ── */}
                {!status && !isMint && !hasAttested && (
                  <>
                    <button className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px]" onClick={() => handleRejectRedeem(request)}>Reject</button>
                    <button className="btn btn-primary !px-[14px] !py-[6px]" onClick={() => handleApproveRedeem(request)}>Approve</button>
                  </>
                )}
                {!status && !isMint && hasAttested && isApproveInitiator && canExecuteRedeem && (
                  <button className="btn btn-primary !px-[14px] !py-[6px]" onClick={() => handleExecuteRedeem(request)}>Execute Redeem</button>
                )}
                {!status && !isMint && hasAttested && isRejectInitiator && canExecuteRejectRedeem && (
                  <button className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px]" onClick={() => handleExecuteRejectRedeem(request)}>Cancel Redeem</button>
                )}
                {!status && !isMint && hasAttested && !(isApproveInitiator && canExecuteRedeem) && !(isRejectInitiator && canExecuteRejectRedeem) && (
                  <div className="flex items-center gap-[8px]">
                    <span className="mono text-[11px] text-[var(--body)]">{request.approval_count}/{THRESHOLD} approve · {request.reject_count}/{THRESHOLD} reject</span>
                    <button className="btn btn-ghost !cursor-not-allowed !border !border-[var(--hairline)] !px-[12px] !py-[6px] !opacity-40" disabled>Voted</button>
                  </div>
                )}

                {status && (
                  <span className={`text-[11px] font-bold uppercase tracking-[0.06em] ${status === "executed" || status === "approved" ? "text-[var(--positive)]" : "text-[var(--body)]"}`}>
                    {status === "executed"
                      ? isMint ? "Executed · tokens minted" : "Executed · tokens burned"
                      : status === "approved" ? "Approved"
                      : "Rejected"}
                  </span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </>
  );
}

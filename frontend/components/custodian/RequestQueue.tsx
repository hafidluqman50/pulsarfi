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
    <div className="hairline" style={{ padding: "18px 0", color: "var(--body)", fontSize: 13 }}>
      {text}
    </div>
  );
}

export function RequestQueue({ requests, isLoading, currentAddress }: RequestQueueProps): React.ReactNode {
  const [completedRequests, setCompletedRequests] = useState<Record<string, string>>({});
  const [activeRequest, setActiveRequest] = useState<CustodianRequest | null>(null);
  const queryClient = useQueryClient();
  const approveMint       = useApproveMint();
  const rejectMint        = useRejectMint();
  const executeMint       = useExecuteMint();
  const executeRejectMint = useExecuteRejectMint();
  const approveRedeem     = useApproveRedeem();
  const rejectRedeem      = useRejectRedeem();

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

  async function handleApprove(request: CustodianRequest): Promise<void> {
    const toastId = toast.loading(`Approving REQ-${request.on_chain_id}`, { description: `${request.kind} ${formatRawToken(request.token_amount)} ${request.ticker}` });
    try {
      const txHash = request.kind === 'mint'
        ? await approveMint.mutateAsync(BigInt(request.on_chain_id))
        : await approveRedeem.mutateAsync(BigInt(request.on_chain_id));
      if (request.kind === 'mint') await recordMintApproval(request.on_chain_id, txHash);
      else await recordRedeemApproval(request.on_chain_id, txHash);
      setCompletedRequests(prev => ({ ...prev, [requestKey(request)]: "approved" }));
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
      toast.success(`REQ-${request.on_chain_id} approved`, { id: toastId, description: `${shortHash(txHash)} recorded`, duration: 3500 });
    } catch (error: unknown) {
      toast.error(`Approval failed`, { id: toastId, description: (error instanceof Error ? error.message : "Approval failed").slice(0, 120) });
    }
  }

  async function handleReject(request: CustodianRequest): Promise<void> {
    const toastId = toast.loading(`Rejecting REQ-${request.on_chain_id}`, { description: `${request.kind} ${formatRawToken(request.token_amount)} ${request.ticker}` });
    try {
      const txHash = request.kind === 'mint'
        ? await rejectMint.mutateAsync(BigInt(request.on_chain_id))
        : await rejectRedeem.mutateAsync(BigInt(request.on_chain_id));
      if (request.kind === 'mint') await recordMintRejection(request.on_chain_id, txHash);
      else await recordRedeemRejection(request.on_chain_id, txHash);
      setCompletedRequests(prev => ({ ...prev, [requestKey(request)]: "rejected" }));
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
      toast.info(`REQ-${request.on_chain_id} rejected`, { id: toastId, description: `${shortHash(txHash)} recorded`, duration: 3500 });
    } catch (error: unknown) {
      toast.error(`Rejection failed`, { id: toastId, description: (error instanceof Error ? error.message : "Rejection failed").slice(0, 120) });
    }
  }

  return (
    <>
      {activeRequest && <AttestorsModal request={activeRequest} onClose={() => setActiveRequest(null)} />}
      <div>
        <div className="hairline table-head-desktop" style={{ display: "grid", gridTemplateColumns: "auto 1fr 1fr 1fr 1fr 1fr 1fr 1.6fr", gap: 12, padding: "12px 0" }}>
          {["", "ID", "Type", "Asset", "Quantity", "IDR notional", "Waited", ""].map((heading, columnIndex) => (
            <div key={columnIndex} className="eyebrow" style={{ color: "var(--body)", textAlign: columnIndex >= 4 && columnIndex <= 6 ? "right" : "left" }}>{heading}</div>
          ))}
        </div>
        {isLoading && <EmptyRow text="Loading custodian requests…" />}
        {!isLoading && requests.length === 0 && <EmptyRow text="No pending mint or redeem requests" />}
        {requests.map(request => {
          const status      = completedRequests[requestKey(request)];
          const isMint      = request.kind === "mint";
          const quantity    = formatRawToken(request.token_amount);
          const idrNotional = request.idrx_amount ? formatRawIDR(request.idrx_amount) : "—";
          const isRequester = isMint &&
            !!currentAddress &&
            !!request.requester_address &&
            request.requester_address.toLowerCase() === currentAddress.toLowerCase();
          const canExecute  = request.approval_count >= THRESHOLD;
          const canCancel   = request.reject_count   >= THRESHOLD;

          return (
            <div key={requestKey(request)} className="hairline table-row-stack" style={{ display: "grid", gridTemplateColumns: "auto 1fr 1fr 1fr 1fr 1fr 1fr 1.6fr", gap: 12, padding: "16px 0", alignItems: "center", opacity: status ? 0.55 : 1 }}>
              <div style={{ width: 32, height: 32, border: "1px solid var(--ink)", display: "flex", alignItems: "center", justifyContent: "center", background: isMint ? "var(--merah)" : "var(--ink)", color: "var(--putih)", flexShrink: 0 }}>
                <Icon name={isMint ? "arrow-up" : "arrow-down"} size={14} />
              </div>
              <div className="row-cell"><span className="row-cell-label">ID</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13 }}>REQ-{request.on_chain_id}</span></div></div>
              <div className="row-cell"><span className="row-cell-label">Type</span><div className="row-cell-value"><span style={{ fontSize: 11, fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: isMint ? "var(--merah)" : "var(--ink)" }}>{request.kind} · {request.source}</span></div></div>
              <div className="row-cell"><span className="row-cell-label">Asset</span><div className="row-cell-value"><div style={{ display: "flex", alignItems: "center", gap: 8 }}><PStockMark ticker={request.ticker} size={22} /><span style={{ fontWeight: 600, fontSize: 13 }}>{request.ticker}</span></div></div></div>
              <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Quantity</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13 }}>{quantity}</span></div></div>
              <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">IDR notional</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 13 }}>{idrNotional}</span></div></div>
              <div className="row-cell" style={{ textAlign: "right" }}><span className="row-cell-label">Waited</span><div className="row-cell-value"><span className="mono" style={{ fontSize: 12, color: "var(--body)" }}>{relativeAge(request.created_at)}</span></div></div>
              <div className="col-actions pos-actions" style={{ display: "flex", gap: 8, justifyContent: "flex-end", alignItems: "center" }}>
                {(request.attestors?.length ?? 0) > 0 && (
                  <button
                    onClick={() => setActiveRequest(request)}
                    style={{ display: "inline-flex", alignItems: "center", gap: 5, padding: "4px 10px", border: "1px solid var(--hairline-strong)", background: "transparent", cursor: "pointer", fontSize: 11, fontFamily: "Inter", fontWeight: 600, letterSpacing: "0.06em", color: "var(--body)" }}
                  >
                    <span style={{ width: 6, height: 6, borderRadius: 999, background: "var(--merah)" }} />
                    {request.attestors!.length}/5
                  </button>
                )}
                {!status && isRequester && canExecute && (
                  <button className="btn btn-merah" onClick={() => handleExecuteMint(request)} style={{ padding: "6px 14px" }}>Execute Mint</button>
                )}
                {!status && isRequester && !canExecute && canCancel && (
                  <button className="btn btn-ghost" onClick={() => handleExecuteRejectMint(request)} style={{ padding: "6px 12px", border: "1px solid var(--ink)" }}>Cancel Mint</button>
                )}
                {!status && isRequester && !canExecute && !canCancel && (
                  <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                    <span className="mono" style={{ fontSize: 11, color: "var(--body)" }}>{request.approval_count}/{THRESHOLD} approve · {request.reject_count}/{THRESHOLD} reject</span>
                    <button className="btn btn-ghost" disabled style={{ padding: "6px 12px", border: "1px solid var(--hairline)", opacity: 0.4, cursor: "not-allowed" }}>Cancel Mint</button>
                    <button className="btn btn-merah" disabled style={{ padding: "6px 14px", opacity: 0.4, cursor: "not-allowed" }}>Execute Mint</button>
                  </div>
                )}
                {!status && !isRequester && (
                  <>
                    <button className="btn btn-ghost" onClick={() => handleReject(request)} style={{ padding: "6px 12px", border: "1px solid var(--ink)" }}>Reject</button>
                    <button className="btn btn-primary" onClick={() => handleApprove(request)} style={{ padding: "6px 14px" }}>Approve</button>
                  </>
                )}
                {status && (
                  <span style={{ fontSize: 11, fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", color: status === "executed" || status === "approved" ? "var(--positive)" : "var(--body)" }}>
                    {status === "executed" ? "Executed · tokens minted" : status === "approved" ? "Approved" : "Rejected"}
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

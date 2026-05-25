import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { parseEventLogs } from 'viem';
import { usePublicClient } from 'wagmi';
import { type LogLine, DEFAULT_LOG, currentTimestamp, randomDigits, delay } from '@/lib/terminal';
import { useRequestMint, buildAttestationHash, type MintDestination } from './contractHooks';
import { recordMintRequest } from './custodianApi';
import { PULSAR_PROTOCOL_ABI } from '@/lib/abi/pulsar_protocol_abi';

export function useTerminalLog() {
  const [log, setLog] = useState<LogLine[]>(DEFAULT_LOG);

  function appendLog(line: Omit<LogLine, 'timestamp'>) {
    setLog(prev => [...prev, { timestamp: currentTimestamp(), ...line }]);
  }

  return { log, appendLog };
}

export interface MintPipelineParams {
  ticker: string;
  stockName: string;
  idxTicker: string;
  quantity: string;
  idrPrice: number;
  idrTotal: number;
  destination: 'liquidity' | 'wallet';
}

export function useMintPipeline(appendLog: (line: Omit<LogLine, 'timestamp'>) => void) {
  const [running, setRunning] = useState(false);
  const { mutateAsync: requestMint, isPending } = useRequestMint();
  const publicClient = usePublicClient();
  const queryClient = useQueryClient();

  async function run(params: MintPipelineParams) {
    const { ticker, stockName, idxTicker, quantity, idrPrice, idrTotal, destination } = params;
    setRunning(true);

    appendLog({ level: "INFO", text: `[idx-broker] order received → BUY ${quantity} ${idxTicker} @ market` });
    const toastId = toast.loading(`Sourcing ${quantity} ${idxTicker} on IDX…`, {
      description: "Awaiting fill from broker desk",
    });

    await delay(800);
    appendLog({ level: "INFO", text: `[idx-broker] partial fill ${Math.floor(parseInt(quantity) * 0.6)}/${quantity} @ Rp ${idrPrice}` });
    await delay(700);
    appendLog({ level: "INFO", text: `[idx-broker] order filled · avg ${idrPrice} IDR · ref IDX-${randomDigits(8)}` });
    appendLog({ level: "INFO", text: `[custody] settlement T+2 scheduled · custodian: PT HORIZON KUSTODIAN` });
    await delay(800);

    try {
      const tokenAmount     = BigInt(quantity) * BigInt(10 ** 18);
      const idrxAmount      = BigInt(Math.round(idrTotal)) * BigInt(100);
      const dest: MintDestination = destination === 'liquidity' ? 1 : 0;
      const attestationHash = buildAttestationHash(ticker, quantity, idrxAmount);

      toast.loading(`Calling requestMint() on-chain…`, { id: toastId, description: "Sign tx in your wallet" });
      appendLog({ level: "INFO", text: `[bridge] attestation ${attestationHash.slice(0, 10)}…${attestationHash.slice(-6)} · dest=${destination}` });

      const txHash = await requestMint({ ticker, stockName, idxTicker, tokenAmount, idrxAmount, attestationHash, destination: dest });

      appendLog({ level: "OK",   text: `[evm] requestMint() → tx ${txHash.slice(0, 10)}…${txHash.slice(-6)} submitted` });
      toast.loading("Waiting for confirmation…", { id: toastId, description: "Recording proposal after receipt" });

      if (!publicClient) throw new Error("Public client not ready");
      const receipt = await publicClient.waitForTransactionReceipt({ hash: txHash });
      const logs = parseEventLogs({
        abi: PULSAR_PROTOCOL_ABI,
        eventName: "MintRequested",
        logs: receipt.logs,
      });
      const proposalId = Number(logs[0]?.args.proposalId);
      if (!Number.isFinite(proposalId)) throw new Error("MintRequested event not found");

      await recordMintRequest({
        on_chain_id: proposalId,
        ticker,
        token_amount: tokenAmount.toString(),
        idrx_amount: idrxAmount.toString(),
        attestation_hash: attestationHash,
        destination: dest === 1 ? "liquidity_pool" : "operator_wallet",
        tx_hash: txHash,
      });
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });

      appendLog({ level: "INFO", text: `[backend] mint proposal recorded · proposal=${proposalId}` });
      appendLog({ level: "INFO", text: `[multisig] proposal created · awaiting 2 more custodian approvals (threshold 3/5)` });
      toast.success("Mint proposal submitted", { id: toastId, description: `Proposal #${proposalId} recorded · 2 more approvals needed`, duration: 4500 });
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "Transaction rejected";
      appendLog({ level: "ERR", text: `[evm] requestMint failed · ${message.slice(0, 80)}` });
      toast.error("Mint failed", { id: toastId, description: message.slice(0, 100) });
    }

    setRunning(false);
  }

  return { run, running, isPending };
}

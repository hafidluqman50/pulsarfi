import { useMutation, useQueryClient } from '@tanstack/react-query';
import { BaseError, type Address } from 'viem';
import { usePublicClient, useWriteContract } from 'wagmi';
import { IDRX_ABI } from '@/lib/abi/idrx_abi';
import { PULSAR_STOCK_ABI } from '@/lib/abi/pulsar_stock_abi';
import { useEnsureAppChain } from '@/lib/useEnsureAppChain';
import { appChainId } from '@/lib/wagmi';
import { recordStockTransfer } from './transactionApi';

const ERC20_TRANSFER_TOPIC = '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef';

export interface TransferTokenInput {
  token_address: Address;
  ticker: string;
  from: Address;
  to: Address;
  amount: bigint;
  estimated_idrx_amount?: bigint;
  is_stable: boolean;
}

function findTransferLogIndex(
  logs: Array<{ address: Address; topics: readonly string[]; logIndex?: number | bigint }>,
  tokenAddress: Address,
): number {
  const transferLog = logs.find((log) => (
    log.address.toLowerCase() === tokenAddress.toLowerCase()
    && log.topics[0]?.toLowerCase() === ERC20_TRANSFER_TOPIC
  ));
  const logIndex = transferLog?.logIndex;
  if (typeof logIndex === 'bigint') return Number(logIndex);
  if (typeof logIndex === 'number') return logIndex;
  return 0;
}

function formatTransferError(error: unknown): string {
  if (error instanceof BaseError) {
    if (error.shortMessage.includes('User rejected')) return 'User rejected the transaction.';
    if (error.shortMessage.includes('insufficient funds')) return 'Wallet balance is not enough.';
    if (error.shortMessage.includes('execution reverted')) return 'Transfer simulation reverted.';
    return error.shortMessage;
  }
  return error instanceof Error ? error.message : 'Transfer failed';
}

export function useTransferToken() {
  const publicClient = usePublicClient();
  const queryClient = useQueryClient();
  const { writeContractAsync } = useWriteContract();
  const ensureAppChain = useEnsureAppChain();

  return useMutation({
    mutationFn: async (input: TransferTokenInput) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (input.amount <= BigInt(0)) throw new Error('Transfer amount is invalid');
      await ensureAppChain();

      let txHash: Address;
      let blockNumber: bigint;
      let logIndex = 0;
      try {
        const { request } = await publicClient.simulateContract({
          address: input.token_address,
          abi: input.is_stable ? IDRX_ABI : PULSAR_STOCK_ABI,
          functionName: 'transfer',
          args: [input.to, input.amount],
          account: input.from,
        });
        txHash = await writeContractAsync({
          ...request,
          chainId: appChainId,
        });
        const receipt = await publicClient.waitForTransactionReceipt({ hash: txHash });
        blockNumber = receipt.blockNumber;
        logIndex = input.is_stable ? 0 : findTransferLogIndex(receipt.logs, input.token_address);
      } catch (error) {
        throw new Error(formatTransferError(error));
      }

      if (!input.is_stable) {
        try {
          await recordStockTransfer({
            ticker: input.ticker,
            tx_hash: txHash,
            from_address: input.from,
            to_address: input.to,
            idrx_amount: (input.estimated_idrx_amount ?? BigInt(0)).toString(),
            stock_amount: input.amount.toString(),
            block_number: Number(blockNumber),
            log_index: logIndex,
          });
        } catch (error) {
          const message = error instanceof Error ? error.message : 'Portfolio record failed';
          throw new Error(`Transfer confirmed, but portfolio record failed: ${message}`);
        }
      }

      return txHash;
    },
    onSuccess: async (_txHash, input) => {
      await queryClient.invalidateQueries({ queryKey: ['market-stocks'] });
      await queryClient.invalidateQueries({ queryKey: ['stock-transactions', input.from] });
      await queryClient.invalidateQueries({ queryKey: ['stock-transactions', input.to] });
    },
  });
}

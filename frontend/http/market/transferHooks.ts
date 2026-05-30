import { useMutation, useQueryClient } from '@tanstack/react-query';
import { BaseError, type Address } from 'viem';
import { useChainId, usePublicClient, useSwitchChain, useWriteContract } from 'wagmi';
import { arbitrumSepolia } from 'wagmi/chains';
import { IDRX_ABI } from '@/lib/abi/idrx_abi';
import { PULSAR_STOCK_ABI } from '@/lib/abi/pulsar_stock_abi';

export interface TransferTokenInput {
  token_address: Address;
  from: Address;
  to: Address;
  amount: bigint;
  is_stable: boolean;
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
  const chainId = useChainId();
  const { switchChainAsync } = useSwitchChain();

  return useMutation({
    mutationFn: async (input: TransferTokenInput) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (input.amount <= BigInt(0)) throw new Error('Transfer amount is invalid');
      if (chainId !== arbitrumSepolia.id) {
        await switchChainAsync({ chainId: arbitrumSepolia.id });
      }

      try {
        const { request } = await publicClient.simulateContract({
          address: input.token_address,
          abi: input.is_stable ? IDRX_ABI : PULSAR_STOCK_ABI,
          functionName: 'transfer',
          args: [input.to, input.amount],
          account: input.from,
        });
        const txHash = await writeContractAsync({
          ...request,
          chainId: arbitrumSepolia.id,
        });
        await publicClient.waitForTransactionReceipt({ hash: txHash });
        return txHash;
      } catch (error) {
        throw new Error(formatTransferError(error));
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['market-stocks'] });
    },
  });
}

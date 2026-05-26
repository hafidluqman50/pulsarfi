import { useMutation, useQueryClient } from '@tanstack/react-query';
import { BaseError, ContractFunctionRevertedError, parseEventLogs, type Address } from 'viem';
import { usePublicClient, useWriteContract } from 'wagmi';
import { IDRX_ABI } from '@/lib/abi/idrx_abi';
import { PULSAR_PROTOCOL_ABI } from '@/lib/abi/pulsar_protocol_abi';
import { PULSAR_STOCK_ABI } from '@/lib/abi/pulsar_stock_abi';
import { recordStockTransaction } from './transactionApi';

export interface ExecuteSwapInput {
  ticker: string;
  wallet_address: Address;
  token_address: Address;
  amount_in: bigint;
  amount_out_min: bigint;
  buy_stock: boolean;
  input_is_stable: boolean;
}

function shortAddress(address: string): string {
  return `${address.slice(0, 6)}...${address.slice(-4)}`;
}

function formatSwapError(error: unknown, ticker: string): string {
  if (!(error instanceof BaseError)) {
    return error instanceof Error ? error.message : 'Swap failed';
  }

  const revertError = error.walk((cause) => cause instanceof ContractFunctionRevertedError);
  if (revertError instanceof ContractFunctionRevertedError) {
    const name = revertError.data?.errorName;
    const args = revertError.data?.args ?? [];

    if (name === 'StockNotFound') return `${String(args[0] ?? ticker)} pool is not deployed on-chain.`;
    if (name === 'KYCRequired') return `Wallet ${shortAddress(String(args[0] ?? ''))} is not KYC approved.`;
    if (name === 'InvalidAmount') return 'Swap amount is invalid.';
  }

  if (error.shortMessage.includes('User rejected')) return 'User rejected the transaction.';
  if (error.shortMessage.includes('insufficient allowance')) return 'Token allowance is not enough.';
  if (error.shortMessage.includes('insufficient funds')) return 'Wallet balance is not enough for gas or token amount.';
  if (error.shortMessage.includes('execution reverted')) return 'Swap simulation reverted. Check KYC, liquidity, and slippage.';

  return error.shortMessage;
}

export function useExecuteSwap() {
  const publicClient = usePublicClient();
  const queryClient = useQueryClient();
  const { writeContractAsync } = useWriteContract();
  const protocolAddress = process.env.NEXT_PUBLIC_PULSAR_PROTOCOL_ADDRESS as Address | undefined;

  return useMutation({
    mutationFn: async (input: ExecuteSwapInput) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!protocolAddress) throw new Error('Protocol unavailable');
      if (input.amount_in <= BigInt(0)) throw new Error('Swap amount is invalid');

      const tokenAbi = input.input_is_stable ? IDRX_ABI : PULSAR_STOCK_ABI;

      try {
        const allowance = await publicClient.readContract({
          address: input.token_address,
          abi: tokenAbi,
          functionName: 'allowance',
          args: [input.wallet_address, protocolAddress],
        }) as bigint;

        if (allowance < input.amount_in) {
          const { request: approveRequest } = await publicClient.simulateContract({
            address: input.token_address,
            abi: tokenAbi,
            functionName: 'approve',
            args: [protocolAddress, input.amount_in],
            account: input.wallet_address,
          });
          const approveHash = await writeContractAsync(approveRequest);
          await publicClient.waitForTransactionReceipt({ hash: approveHash });
        }
      } catch (error) {
        throw new Error(formatSwapError(error, input.ticker));
      }

      let txHash: Address;
      try {
        const { request } = await publicClient.simulateContract({
          address: protocolAddress,
          abi: PULSAR_PROTOCOL_ABI,
          functionName: 'swap',
          args: [
            input.ticker,
            input.amount_in,
            input.amount_out_min,
            input.buy_stock,
          ],
          account: input.wallet_address,
        });
        txHash = await writeContractAsync(request);
      } catch (error) {
        throw new Error(formatSwapError(error, input.ticker));
      }

      const receipt = await publicClient.waitForTransactionReceipt({ hash: txHash });
      const logs = parseEventLogs({
        abi: PULSAR_PROTOCOL_ABI,
        eventName: 'TokensSwapped',
        logs: receipt.logs,
      });
      const event = logs[0]?.args as {
        ticker?: string;
        buyStock?: boolean;
        amountIn?: bigint;
        amountOut?: bigint;
      } | undefined;
      if (!event || event.amountIn === undefined || event.amountOut === undefined || event.buyStock === undefined) {
        throw new Error('TokensSwapped event not found');
      }

      await recordStockTransaction({
        ticker: input.ticker,
        tx_hash: txHash,
        wallet_address: input.wallet_address,
        side: event.buyStock ? 'buy' : 'sell',
        idrx_amount: (event.buyStock ? event.amountIn : event.amountOut).toString(),
        stock_amount: (event.buyStock ? event.amountOut : event.amountIn).toString(),
        block_number: Number(receipt.blockNumber),
      });

      return txHash;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['market-stocks'] });
    },
  });
}

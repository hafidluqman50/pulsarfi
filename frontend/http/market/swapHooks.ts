import { useMutation, useQueryClient } from '@tanstack/react-query';
import { BaseError, ContractFunctionRevertedError, parseEventLogs, type Address, type Log } from 'viem';
import { useReadContract, usePublicClient, useWriteContract } from 'wagmi';
import { IDRX_ABI } from '@/lib/abi/idrx_abi';
import { PULSAR_PROTOCOL_ABI } from '@/lib/abi/pulsar_protocol_abi';
import { PULSAR_STOCK_ABI } from '@/lib/abi/pulsar_stock_abi';
import { useEnsureAppChain } from '@/lib/useEnsureAppChain';
import { appChainId } from '@/lib/wagmi';
import { recordStockTransaction } from './transactionApi';

// PulsarSwapHook fee events. The hook takes the protocol fee (always in IDRX) as
// an ERC-6909 claim and emits it: BuySideFeeTaken on a buy (IDRX is the input),
// HookFee on a sell (IDRX is the output). Parsed from the receipt by event
// signature, so the hook address is not needed here.
const HOOK_FEE_EVENTS_ABI = [
  {
    type: 'event',
    name: 'BuySideFeeTaken',
    inputs: [
      { name: 'poolId', type: 'bytes32', indexed: true },
      { name: 'sender', type: 'address', indexed: true },
      { name: 'feeIdrx', type: 'uint256', indexed: false },
    ],
    anonymous: false,
  },
  {
    type: 'event',
    name: 'HookFee',
    inputs: [
      { name: 'poolId', type: 'bytes32', indexed: true },
      { name: 'sender', type: 'address', indexed: true },
      { name: 'feeAmount0', type: 'uint128', indexed: false },
      { name: 'feeAmount1', type: 'uint128', indexed: false },
    ],
    anonymous: false,
  },
] as const;

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

/**
 * Reads the exact protocol fee (in raw IDRX) the hook took for this swap, from the
 * receipt's hook events: BuySideFeeTaken.feeIdrx on a buy, HookFee's non-zero
 * amount on a sell. Returns 0n when swapFeeBps is 0 (no fee event emitted).
 */
function extractHookFeeIdrx(logs: Log[], buyStock: boolean): bigint {
  if (buyStock) {
    const buyLogs = parseEventLogs({ abi: HOOK_FEE_EVENTS_ABI, eventName: 'BuySideFeeTaken', logs });
    return (buyLogs[0]?.args as { feeIdrx?: bigint } | undefined)?.feeIdrx ?? BigInt(0);
  }
  const sellLogs = parseEventLogs({ abi: HOOK_FEE_EVENTS_ABI, eventName: 'HookFee', logs });
  const args = sellLogs[0]?.args as { feeAmount0?: bigint; feeAmount1?: bigint } | undefined;
  return (args?.feeAmount0 ?? BigInt(0)) + (args?.feeAmount1 ?? BigInt(0));
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

export function useSwapFeeBps() {
  const protocolAddress = process.env.NEXT_PUBLIC_PULSAR_PROTOCOL_ADDRESS as Address | undefined;

  return useReadContract({
    address: protocolAddress,
    abi: PULSAR_PROTOCOL_ABI,
    functionName: 'swapFeeBps',
    query: { enabled: Boolean(protocolAddress) },
  });
}

export function useExecuteSwap() {
  const publicClient = usePublicClient();
  const queryClient = useQueryClient();
  const { writeContractAsync } = useWriteContract();
  const ensureAppChain = useEnsureAppChain();
  const protocolAddress = process.env.NEXT_PUBLIC_PULSAR_PROTOCOL_ADDRESS as Address | undefined;

  return useMutation({
    mutationFn: async (input: ExecuteSwapInput) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!protocolAddress) throw new Error('Protocol unavailable');
      if (input.amount_in <= BigInt(0)) throw new Error('Swap amount is invalid');
      await ensureAppChain();

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
          const approveHash = await writeContractAsync({
            ...approveRequest,
            chainId: appChainId,
          });
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
          functionName: 'swapV4',
          args: [
            input.ticker,
            input.amount_in,
            input.amount_out_min,
            input.buy_stock,
          ],
          account: input.wallet_address,
        });
        txHash = await writeContractAsync({
          ...request,
          chainId: appChainId,
        });
      } catch (error) {
        throw new Error(formatSwapError(error, input.ticker));
      }

      const receipt = await publicClient.waitForTransactionReceipt({ hash: txHash });
      const logs = parseEventLogs({
        abi: PULSAR_PROTOCOL_ABI,
        eventName: 'V4Swapped',
        logs: receipt.logs,
      });
      // amountOut is the net amount the user received (the hook's IDRX fee is
      // already skimmed on the sell side before the protocol forwards it).
      const event = logs[0]?.args as {
        buyStock?: boolean;
        amountIn?: bigint;
        amountOut?: bigint;
      } | undefined;
      if (!event || event.amountIn === undefined || event.amountOut === undefined || event.buyStock === undefined) {
        throw new Error('V4Swapped event not found');
      }

      const protocolFeeIdrx = extractHookFeeIdrx(receipt.logs, event.buyStock);

      await recordStockTransaction({
        ticker: input.ticker,
        tx_hash: txHash,
        wallet_address: input.wallet_address,
        side: event.buyStock ? 'buy' : 'sell',
        idrx_amount: (event.buyStock ? event.amountIn : event.amountOut).toString(),
        stock_amount: (event.buyStock ? event.amountOut : event.amountIn).toString(),
        protocol_fee_idrx: protocolFeeIdrx.toString(),
        block_number: Number(receipt.blockNumber),
      });

      return txHash;
    },
    onSuccess: async (_txHash, input) => {
      await queryClient.invalidateQueries({ queryKey: ['market-stocks'] });
      await queryClient.invalidateQueries({ queryKey: ['stock-transactions', input.wallet_address] });
    },
  });
}

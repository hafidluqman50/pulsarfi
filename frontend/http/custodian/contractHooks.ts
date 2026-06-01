import { useMutation, useQueryClient } from '@tanstack/react-query';
import { BaseError, ContractFunctionRevertedError, encodePacked, keccak256 } from 'viem';
import { useAccount, usePublicClient, useWriteContract } from 'wagmi';
import { IDRX_ABI } from '@/lib/abi/idrx_abi';
import { PULSAR_PROTOCOL_ABI } from '@/lib/abi/pulsar_protocol_abi';
import { useEnsureAppChain } from '@/lib/useEnsureAppChain';
import { appChainId } from '@/lib/wagmi';
import { createWalletVerification, recordMintExecution, recordMintRejectExecution, recordRedeemExecution, recordRedeemRejectionExecution } from './custodianApi';

const PROTOCOL_ADDRESS = process.env.NEXT_PUBLIC_PULSAR_PROTOCOL_ADDRESS as `0x${string}`;
const MINT_THRESHOLD = 3;
const ZERO_ADDRESS = '0x0000000000000000000000000000000000000000';

export interface RequestMintParams {
  ticker: string;
  stockName: string;
  idxTicker: string;
  tokenAmount: bigint;
  idrxAmount: bigint;
  attestationHash: `0x${string}`;
}

export function buildAttestationHash(ticker: string, quantity: string, idrxAmount: bigint): `0x${string}` {
  return keccak256(encodePacked(
    ['string', 'string', 'uint256'],
    [ticker, quantity, idrxAmount]
  ));
}

function shortAddress(address: string): string {
  return `${address.slice(0, 6)}…${address.slice(-4)}`;
}

async function ensureIdrxAllowance(params: {
  publicClient: NonNullable<ReturnType<typeof usePublicClient>>;
  writeContractAsync: ReturnType<typeof useWriteContract>['writeContractAsync'];
  idrxAddress: `0x${string}`;
  owner: `0x${string}`;
  amount: bigint;
}) {
  if (params.amount <= BigInt(0)) return;

  const allowance = await params.publicClient.readContract({
    address: params.idrxAddress,
    abi: IDRX_ABI,
    functionName: 'allowance',
    args: [params.owner, PROTOCOL_ADDRESS],
  }) as bigint;

  if (allowance >= params.amount) return;

  const balance = await params.publicClient.readContract({
    address: params.idrxAddress,
    abi: IDRX_ABI,
    functionName: 'balanceOf',
    args: [params.owner],
  }) as bigint;

  if (balance < params.amount) {
    throw new Error(`IDRX balance is not enough for LP funding. Need ${params.amount.toString()}, balance ${balance.toString()}.`);
  }

  const hash = await params.writeContractAsync({
    address: params.idrxAddress,
    abi: IDRX_ABI,
    functionName: 'approve',
    args: [PROTOCOL_ADDRESS, params.amount],
    chainId: appChainId,
  });
  await params.publicClient.waitForTransactionReceipt({ hash });
}

interface ExecuteMintContext {
  proposalId: bigint;
  ticker: string;
  requester: `0x${string}`;
  caller: `0x${string}`;
  approvalCount: number;
  executed: boolean;
  stockAddress: `0x${string}`;
}

function formatExecuteMintError(err: unknown, context: ExecuteMintContext): string {
  if (err instanceof BaseError) {
    const revertError = err.walk((e) => e instanceof ContractFunctionRevertedError);

    if (revertError instanceof ContractFunctionRevertedError) {
      const name = revertError.data?.errorName;
      const args = revertError.data?.args ?? [];

      if (name === 'ProposalNotFound') return `Proposal #${context.proposalId.toString()} not found on-chain.`;
      if (name === 'ProposalAlreadyExecuted') return `Proposal #${context.proposalId.toString()} is already executed.`;
      if (name === 'NotRequester') return `Only requester can execute mint. Switch wallet to ${shortAddress(String(args[1] ?? context.requester))}.`;
      if (name === 'ThresholdNotMet') return `Need ${String(args[2] ?? MINT_THRESHOLD)} approvals before execute. Current ${String(args[1] ?? context.approvalCount)}/${String(args[2] ?? MINT_THRESHOLD)}.`;
    }

    if (context.requester === ZERO_ADDRESS) return `Proposal #${context.proposalId.toString()} not found on-chain.`;
    if (context.executed) return `Proposal #${context.proposalId.toString()} is already executed.`;
    if (context.requester.toLowerCase() !== context.caller.toLowerCase()) {
      return `Only requester can execute mint. Switch wallet to ${shortAddress(context.requester)}.`;
    }
    if (context.approvalCount < MINT_THRESHOLD) {
      return `Need ${MINT_THRESHOLD} approvals before execute. Current ${context.approvalCount}/${MINT_THRESHOLD}.`;
    }

    const poolStep = context.stockAddress === ZERO_ADDRESS ? `deploying ${context.ticker} pool` : `${context.ticker} liquidity`;
    return `Liquidity provisioning failed while ${poolStep}. Requester and approvals are valid; router/addLiquidity reverted without a reason.`;
  }

  return err instanceof Error ? err.message : 'executeMint simulation failed';
}

export function useRequestMint() {
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const { address } = useAccount();
  const ensureAppChain = useEnsureAppChain();

  return useMutation({
    mutationFn: async (params: RequestMintParams) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!address) throw new Error('Wallet not connected');
      await ensureAppChain();

      try {
        const { request } = await publicClient.simulateContract({
          address: PROTOCOL_ADDRESS,
          abi: PULSAR_PROTOCOL_ABI,
          functionName: 'requestMint',
          args: [
            params.ticker,
            params.stockName,
            params.idxTicker,
            params.tokenAmount,
            params.idrxAmount,
            params.attestationHash,
          ],
          account: address,
        });
        return writeContractAsync({ ...request, chainId: appChainId });
      } catch (err) {
        throw new Error(err instanceof Error ? err.message : 'requestMint simulation failed');
      }
    },
  });
}

export function useApproveMint() {
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const { address } = useAccount();
  const ensureAppChain = useEnsureAppChain();

  return useMutation({
    mutationFn: async (proposalId: bigint) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!address) throw new Error('Wallet not connected');
      await ensureAppChain();

      try {
        const { request } = await publicClient.simulateContract({
          address: PROTOCOL_ADDRESS,
          abi: PULSAR_PROTOCOL_ABI,
          functionName: 'approveMint',
          args: [proposalId],
          account: address,
        });
        return writeContractAsync({ ...request, chainId: appChainId });
      } catch (err) {
        throw new Error(err instanceof Error ? err.message : 'approveMint simulation failed');
      }
    },
  });
}

export function useRejectMint() {
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const { address } = useAccount();
  const ensureAppChain = useEnsureAppChain();

  return useMutation({
    mutationFn: async (proposalId: bigint) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!address) throw new Error('Wallet not connected');
      await ensureAppChain();

      try {
        const { request } = await publicClient.simulateContract({
          address: PROTOCOL_ADDRESS,
          abi: PULSAR_PROTOCOL_ABI,
          functionName: 'rejectMint',
          args: [proposalId],
          account: address,
        });
        return writeContractAsync({ ...request, chainId: appChainId });
      } catch (err) {
        throw new Error(err instanceof Error ? err.message : 'rejectMint simulation failed');
      }
    },
  });
}

export function useApproveRedeem() {
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const { address } = useAccount();
  const ensureAppChain = useEnsureAppChain();

  return useMutation({
    mutationFn: async (requestId: bigint) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!address) throw new Error('Wallet not connected');
      await ensureAppChain();

      try {
        const { request } = await publicClient.simulateContract({
          address: PROTOCOL_ADDRESS,
          abi: PULSAR_PROTOCOL_ABI,
          functionName: 'approveRedeem',
          args: [requestId],
          account: address,
        });
        return writeContractAsync({ ...request, chainId: appChainId });
      } catch (err) {
        throw new Error(err instanceof Error ? err.message : 'approveRedeem simulation failed');
      }
    },
  });
}

export function useRejectRedeem() {
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const { address } = useAccount();
  const ensureAppChain = useEnsureAppChain();

  return useMutation({
    mutationFn: async (requestId: bigint) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!address) throw new Error('Wallet not connected');
      await ensureAppChain();

      try {
        const { request } = await publicClient.simulateContract({
          address: PROTOCOL_ADDRESS,
          abi: PULSAR_PROTOCOL_ABI,
          functionName: 'rejectRedeem',
          args: [requestId],
          account: address,
        });
        return writeContractAsync({ ...request, chainId: appChainId });
      } catch (err) {
        throw new Error(err instanceof Error ? err.message : 'rejectRedeem simulation failed');
      }
    },
  });
}

export function useExecuteMint() {
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const { address } = useAccount();
  const ensureAppChain = useEnsureAppChain();

  return useMutation({
    mutationFn: async (proposalId: bigint) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!address) throw new Error('Wallet not connected');
      await ensureAppChain();

      const proposal = await publicClient.readContract({
        address: PROTOCOL_ADDRESS,
        abi: PULSAR_PROTOCOL_ABI,
        functionName: 'proposals',
        args: [proposalId],
      });
      const ticker = proposal[0] as string;
      const idrxAmount = proposal[4] as bigint;
      const requester = proposal[7] as `0x${string}`;
      const approvalCount = Number(proposal[8]);
      const executed = Boolean(proposal[9]);
      const stockAddress = await publicClient.readContract({
        address: PROTOCOL_ADDRESS,
        abi: PULSAR_PROTOCOL_ABI,
        functionName: 'stocks',
        args: [ticker],
      }) as `0x${string}`;

      try {
        if (idrxAmount > BigInt(0)) {
          const idrxAddress = await publicClient.readContract({
            address: PROTOCOL_ADDRESS,
            abi: PULSAR_PROTOCOL_ABI,
            functionName: 'idrx',
          }) as `0x${string}`;

          await ensureIdrxAllowance({
            publicClient,
            writeContractAsync,
            idrxAddress,
            owner: address,
            amount: idrxAmount,
          });
        }

        const { request } = await publicClient.simulateContract({
          address: PROTOCOL_ADDRESS,
          abi: PULSAR_PROTOCOL_ABI,
          functionName: 'executeMint',
          args: [proposalId],
          account: address,
        });

        const hash = await writeContractAsync({
          ...request,
          chainId: appChainId,
        });
        await publicClient.waitForTransactionReceipt({ hash });

        const deployedStockAddress = await publicClient.readContract({
          address: PROTOCOL_ADDRESS,
          abi: PULSAR_PROTOCOL_ABI,
          functionName: 'stocks',
          args: [ticker],
        }) as `0x${string}`;

        await recordMintExecution(Number(proposalId), hash, deployedStockAddress);
        return hash;
      } catch (err) {
        console.error('executeMint raw error:', err);
        throw new Error(formatExecuteMintError(err, {
          proposalId,
          ticker,
          requester,
          caller: address,
          approvalCount,
          executed,
          stockAddress,
        }));
      }
    },
  });
}

export function useExecuteRejectMint() {
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const ensureAppChain = useEnsureAppChain();

  return useMutation({
    mutationFn: async (proposalId: bigint) => {
      if (!publicClient) throw new Error('Public client not ready');
      await ensureAppChain();

      const hash = await writeContractAsync({
        address: PROTOCOL_ADDRESS,
        abi: PULSAR_PROTOCOL_ABI,
        functionName: 'executeRejectMint',
        args: [proposalId],
        chainId: appChainId,
      });
      await publicClient.waitForTransactionReceipt({ hash });
      await recordMintRejectExecution(Number(proposalId), hash);
      return hash;
    },
  });
}

export function useExecuteRedeem() {
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const { address } = useAccount();
  const ensureAppChain = useEnsureAppChain();

  return useMutation({
    mutationFn: async (requestId: bigint) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!address) throw new Error('Wallet not connected');
      await ensureAppChain();

      const { request } = await publicClient.simulateContract({
        address: PROTOCOL_ADDRESS,
        abi: PULSAR_PROTOCOL_ABI,
        functionName: 'executeRedeem',
        args: [requestId],
        account: address,
      });
      const hash = await writeContractAsync({ ...request, chainId: appChainId });
      const receipt = await publicClient.waitForTransactionReceipt({ hash });
      await recordRedeemExecution(Number(requestId), hash, Number(receipt.blockNumber));
      return hash;
    },
  });
}

export function useExecuteRejectRedeem() {
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const { address } = useAccount();
  const ensureAppChain = useEnsureAppChain();

  return useMutation({
    mutationFn: async (requestId: bigint) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!address) throw new Error('Wallet not connected');
      await ensureAppChain();

      const { request } = await publicClient.simulateContract({
        address: PROTOCOL_ADDRESS,
        abi: PULSAR_PROTOCOL_ABI,
        functionName: 'executeReject',
        args: [requestId],
        account: address,
      });
      const hash = await writeContractAsync({ ...request, chainId: appChainId });
      const receipt = await publicClient.waitForTransactionReceipt({ hash });
      await recordRedeemRejectionExecution(Number(requestId), hash, Number(receipt.blockNumber));
      return hash;
    },
  });
}

export interface RecordKYCParams {
  walletAddress: `0x${string}`;
  type: 'retail' | 'institution';
  fullName: string;
  email: string;
  document: File;
}

export function useRecordKYC() {
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const { address } = useAccount();
  const ensureAppChain = useEnsureAppChain();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (params: RecordKYCParams) => {
      if (!publicClient) throw new Error('Public client not ready');
      if (!address) throw new Error('Wallet not connected');
      await ensureAppChain();

      const { request } = await publicClient.simulateContract({
        address: PROTOCOL_ADDRESS,
        abi: PULSAR_PROTOCOL_ABI,
        functionName: 'approveKYC',
        args: [params.walletAddress],
        account: address,
      });

      const hash = await writeContractAsync({ ...request, chainId: appChainId });
      await publicClient.waitForTransactionReceipt({ hash });

      const body = new FormData();
      body.append('wallet_address', params.walletAddress);
      body.append('type', params.type);
      body.append('full_name', params.fullName);
      body.append('email', params.email);
      body.append('approval_tx_hash', hash);
      body.append('document', params.document);
      await createWalletVerification(body);

      return hash;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['custodian'] });
    },
  });
}

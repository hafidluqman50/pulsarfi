import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
	type Address,
	BaseError,
	ContractFunctionRevertedError,
	parseEventLogs,
} from "viem";
import { usePublicClient, useWriteContract } from "wagmi";
import { IDRX_ABI } from "@/lib/abi/idrx_abi";
import { PULSAR_PROTOCOL_ABI } from "@/lib/abi/pulsar_protocol_abi";
import { PULSAR_STOCK_ABI } from "@/lib/abi/pulsar_stock_abi";
import { useEnsureAppChain } from "@/lib/useEnsureAppChain";
import { appChainId } from "@/lib/wagmi";
import { recordRedeemRequest } from "./redeemApi";

const UNISWAP_V2_ROUTER_ABI = [
	{
		type: "function",
		name: "getAmountsOut",
		stateMutability: "view",
		inputs: [
			{ name: "amountIn", type: "uint256" },
			{ name: "path", type: "address[]" },
		],
		outputs: [{ name: "amounts", type: "uint256[]" }],
	},
] as const;

export interface RequestRedeemInput {
	ticker: string;
	tokenAmount: bigint;
	walletAddress: Address;
	stockContractAddress: Address;
}

function shortAddress(address: string): string {
	return `${address.slice(0, 6)}...${address.slice(-4)}`;
}

function formatRedeemError(error: unknown): string {
	if (!(error instanceof BaseError)) {
		return error instanceof Error ? error.message : "Redeem failed";
	}

	const revertError = error.walk(
		(cause) => cause instanceof ContractFunctionRevertedError,
	);
	if (revertError instanceof ContractFunctionRevertedError) {
		const name = revertError.data?.errorName;
		const args = revertError.data?.args ?? [];

		if (name === "KYCRequired") {
			return `Wallet ${shortAddress(String(args[0] ?? ""))} is not KYC verified. Contact the custodian to get verified.`;
		}
		if (name === "StockNotFound")
			return `Token ${String(args[0] ?? "")} not found in protocol.`;
	}

	if (error.shortMessage.includes("User rejected"))
		return "Transaction rejected by user.";
	if (error.shortMessage.includes("insufficient allowance"))
		return "Insufficient token allowance.";
	if (error.shortMessage.includes("insufficient funds"))
		return "Insufficient balance for gas.";

	return error.shortMessage;
}

export function useRequestRedeem() {
	const publicClient = usePublicClient();
	const queryClient = useQueryClient();
	const { writeContractAsync } = useWriteContract();
	const ensureAppChain = useEnsureAppChain();
	const protocolAddress = process.env.NEXT_PUBLIC_PULSAR_PROTOCOL_ADDRESS as
		| Address
		| undefined;
	const idrxAddress = process.env.NEXT_PUBLIC_IDRX_ADDRESS as
		| Address
		| undefined;

	return useMutation({
		mutationFn: async (input: RequestRedeemInput) => {
			if (!publicClient) throw new Error("Public client not ready");
			if (!protocolAddress) throw new Error("Protocol unavailable");

			await ensureAppChain();

			// 1. Approve stock token allowance
			try {
				const stockAllowance = (await publicClient.readContract({
					address: input.stockContractAddress,
					abi: PULSAR_STOCK_ABI,
					functionName: "allowance",
					args: [input.walletAddress, protocolAddress],
				})) as bigint;

				if (stockAllowance < input.tokenAmount) {
					const { request: approveReq } = await publicClient.simulateContract({
						address: input.stockContractAddress,
						abi: PULSAR_STOCK_ABI,
						functionName: "approve",
						args: [protocolAddress, input.tokenAmount],
						account: input.walletAddress,
					});
					const approveHash = await writeContractAsync({
						...approveReq,
						chainId: appChainId,
					});
					await publicClient.waitForTransactionReceipt({ hash: approveHash });
				}
			} catch (error) {
				throw new Error(formatRedeemError(error));
			}

			// 2. Approve IDRX for fee — only if redeemFeeBps > 0
			if (idrxAddress) {
				try {
					const feeBps = (await publicClient.readContract({
						address: protocolAddress,
						abi: PULSAR_PROTOCOL_ABI,
						functionName: "redeemFeeBps",
					})) as bigint;

					if (feeBps > BigInt(0)) {
						const routerAddress = (await publicClient.readContract({
							address: protocolAddress,
							abi: PULSAR_PROTOCOL_ABI,
							functionName: "router",
						})) as Address;
						const amounts = (await publicClient.readContract({
							address: routerAddress,
							abi: UNISWAP_V2_ROUTER_ABI,
							functionName: "getAmountsOut",
							args: [
								input.tokenAmount,
								[input.stockContractAddress, idrxAddress],
							],
						})) as readonly bigint[];
						const feeIdrx = ((amounts[1] ?? BigInt(0)) * feeBps) / BigInt(10_000);

						const idrxAllowance = (await publicClient.readContract({
							address: idrxAddress,
							abi: IDRX_ABI,
							functionName: "allowance",
							args: [input.walletAddress, protocolAddress],
						})) as bigint;

						if (feeIdrx > BigInt(0) && idrxAllowance < feeIdrx) {
							const { request: idrxApproveReq } =
								await publicClient.simulateContract({
									address: idrxAddress,
									abi: IDRX_ABI,
									functionName: "approve",
									args: [protocolAddress, feeIdrx],
									account: input.walletAddress,
								});
							const idrxApproveHash = await writeContractAsync({
								...idrxApproveReq,
								chainId: appChainId,
							});
							await publicClient.waitForTransactionReceipt({
								hash: idrxApproveHash,
							});
						}
					}
				} catch (error) {
					throw new Error(formatRedeemError(error));
				}
			}

			// 3. Call requestRedeem on SC
			let txHash: Address;
			try {
				const { request } = await publicClient.simulateContract({
					address: protocolAddress,
					abi: PULSAR_PROTOCOL_ABI,
					functionName: "requestRedeem",
					args: [input.ticker, input.tokenAmount],
					account: input.walletAddress,
				});
				txHash = await writeContractAsync({ ...request, chainId: appChainId });
			} catch (error) {
				throw new Error(formatRedeemError(error));
			}

			// 4. Wait for receipt + parse event
			const receipt = await publicClient.waitForTransactionReceipt({
				hash: txHash,
			});
			const logs = parseEventLogs({
				abi: PULSAR_PROTOCOL_ABI,
				eventName: "RedeemRequested",
				logs: receipt.logs,
			});

			const event = logs[0]?.args as
				| {
						requestId?: bigint;
						user?: Address;
						ticker?: string;
						tokenAmount?: bigint;
						feeIdrx?: bigint;
				  }
				| undefined;

			if (event?.requestId === undefined)
				throw new Error("RedeemRequested event not found");

			// 5. Record to backend
			await recordRedeemRequest({
				on_chain_id: Number(event?.requestId ?? 0),
				ticker: input.ticker,
				token_amount: (event?.tokenAmount ?? input.tokenAmount).toString(),
				fee_idrx: (event?.feeIdrx ?? BigInt(0)).toString(),
				user_address: input.walletAddress,
				tx_hash: txHash,
			});

			return txHash;
		},
		onSuccess: async () => {
			await queryClient.invalidateQueries({ queryKey: ["market-stocks"] });
		},
	});
}

"use client";

import { ConnectButton } from "@rainbow-me/rainbowkit";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { type Address } from "viem";
import { useAccount } from "wagmi";
import { Accordion } from "@/components/ui/Accordion";
import { Icon } from "@/components/ui/Icon";
import { PStockMark } from "@/components/ui/PStockMark";
import { Sparkline } from "@/components/ui/Sparkline";
import { TokenSelectModal } from "@/components/ui/TokenSelectModal";
import { useExecuteSwap } from "@/http/market/swapHooks";
import { useMarketTokens, useWalletTokenBalances } from "@/http/market/tokenHooks";
import { useMarketStocks, useProtocolStats } from "@/http/market/hooks";
import {
	STABLES,
	fmtIDRX,
	fmtIDRXCompact,
	fmtNum,
	fmtPct,
	seriesFor,
	type Token,
} from "@/lib/data";
import {
	buildSwapQuote,
	stockTickerForSwap,
	swapToastDescription,
	tokenAddress,
	type MarketToken,
} from "@/lib/swap";

interface HeadlineConfig {
	eyebrow: string;
	line1: string;
	line2: string;
	line3: string;
}

interface SwapViewProps {
	headline: HeadlineConfig;
}

export function SwapView({ headline }: SwapViewProps) {
	const { address, isConnected } = useAccount();
	const executeSwap = useExecuteSwap();
	const marketTokens = useMarketTokens();
	const walletBalances = useWalletTokenBalances(marketTokens);
	const { data: protocolStats } = useProtocolStats();
	const { data: marketStocksRaw = [] } = useMarketStocks();

	const pegDeviation = useMemo(() => {
		const withPool = marketStocksRaw.filter(s => s.pool_price > 0 && s.price > 0);
		if (withPool.length === 0) return null;
		const total = withPool.reduce((sum, s) => {
			const idxLotPrice = s.price * 100;
			return sum + Math.abs(s.pool_price - idxLotPrice) / idxLotPrice * 100;
		}, 0);
		return total / withPool.length;
	}, [marketStocksRaw]);

	const [inputTicker, setInputTicker] = useState("IDRX");
	const [outputTicker, setOutputTicker] = useState("BUMIP");
	const [amount, setAmount] = useState("");
	const [pickerFor, setPickerFor] = useState<"in" | "out" | null>(null);
	const [detailsOpen, setDetailsOpen] = useState(true);
	const [slippage, setSlippage] = useState(0.5);
	const [showSettings, setShowSettings] = useState(false);

	const inputToken = useMemo(
		() =>
			STABLES.find((token) => token.ticker === inputTicker) ??
			marketTokens.find((token) => token.ticker === inputTicker) ??
			STABLES[0],
		[inputTicker, marketTokens],
	);
	const outputToken = useMemo(
		() =>
			marketTokens.find((token) => token.ticker === outputTicker) ??
			STABLES.find((token) => token.ticker === outputTicker) ??
			marketTokens[0],
		[outputTicker, marketTokens],
	);

	const inputTokensForPicker = outputToken?.isStable ? marketTokens : STABLES;
	const outputTokensForPicker = inputToken.isStable ? marketTokens : STABLES;

	const idrxAddress = process.env.NEXT_PUBLIC_IDRX_ADDRESS as Address | undefined;
	const protocolAddress = process.env.NEXT_PUBLIC_PULSAR_PROTOCOL_ADDRESS as Address | undefined;
	const inputAddress = tokenAddress(inputToken, idrxAddress);

	const quote = useMemo(
		() => buildSwapQuote(inputToken, outputToken ?? STABLES[0], amount, slippage),
		[amount, inputToken, outputToken, slippage],
	);

	const inputBalance = walletBalances[inputToken.ticker] ?? 0;
	const outputBalance = walletBalances[outputToken?.ticker ?? ""] ?? 0;
	const busy = executeSwap.isPending;
	const insufficient = isConnected && quote.inputAmount > inputBalance;

	function setTradeAmount(value: string) {
		const [whole, ...fractions] = value.split(".");
		setAmount(fractions.length > 0 ? `${whole}.${fractions.join("")}` : whole);
	}

	function flip() {
		if (!outputToken) return;
		setInputTicker(outputToken.ticker);
		setOutputTicker(inputToken.ticker);
		setAmount("");
	}

	function selectToken(token: Token) {
		if (pickerFor === "in") setInputTicker(token.ticker);
		if (pickerFor === "out") setOutputTicker(token.ticker);
		setPickerFor(null);
	}

	async function swap() {
		if (!address || !inputAddress || !outputToken) return;

		const toastId = toast.loading("Executing swap pipeline…", {
			description: swapToastDescription(inputToken, outputToken, quote),
		});
		try {
			const txHash = await executeSwap.mutateAsync({
				ticker: stockTickerForSwap(inputToken, outputToken),
				wallet_address: address,
				token_address: inputAddress,
				amount_in: quote.amountIn,
				amount_out_min: quote.amountOutMin,
				buy_stock: Boolean(inputToken.isStable),
				input_is_stable: Boolean(inputToken.isStable),
			});
			toast.success("Swap executed", {
				id: toastId,
				description: `Tx ${txHash.slice(0, 10)}...${txHash.slice(-6)}`,
				duration: 6000,
			});
			setAmount("");
		} catch (error) {
			const message = error instanceof Error ? error.message : "Swap failed";
			toast.error("Swap failed", {
				id: toastId,
				description: message.slice(0, 120),
				duration: 7000,
			});
		}
	}

	let ctaText: string;
	let ctaAction: (() => void) | undefined;
	let ctaDisabled = false;
	if (!isConnected) {
		ctaText = "Connect Wallet";
	} else if (!quote.inputAmount) {
		ctaText = "Enter an amount";
		ctaDisabled = true;
	} else if (!quote.rate) {
		ctaText = "Pool unavailable";
		ctaDisabled = true;
	} else if (insufficient) {
		ctaText = `Insufficient ${inputToken.ticker}`;
		ctaDisabled = true;
	} else if (!protocolAddress || !inputAddress) {
		ctaText = "Protocol unavailable";
		ctaDisabled = true;
	} else {
		ctaText = busy ? "Executing…" : "Execute Swap";
		ctaAction = swap;
	}
	if (busy) ctaDisabled = true;

	const percentButton = (pct: number) => (
		<button
			type="button"
			key={pct}
			onClick={() =>
				setTradeAmount(
					(inputBalance * pct || 0).toFixed(inputToken.isStable ? 2 : 4),
				)
			}
			className="cursor-pointer appearance-none border border-[var(--hairline-strong)] bg-transparent px-[10px] py-[4px] text-[11px] font-semibold uppercase tracking-[0.08em] [font-family:var(--font-inter,_Inter,_sans-serif)]"
		>
			{pct === 1 ? "Max" : `${pct * 100}%`}
		</button>
	);

	return (
		<div className="grid-2col !px-[24px] !py-[40px]">
			{/* LEFT — Editorial */}
			<div>
				<h1 className="display hero-display !m-[0] !text-[70px] !font-normal !leading-[0.96] !tracking-[-0.03em]">
					{headline.line1}
					<br />
					<span className="display-it">{headline.line2}</span>
					<br />
					{headline.line3}
				</h1>
				<p className="mt-[28px] max-w-[540px] text-[18px] font-light leading-[1.55] text-[var(--ink-soft)] [font-family:var(--font-fraunces,_Fraunces,_serif)]">
					Eight blue-chip equities from the Indonesia Stock Exchange, tokenized
					1:1 on Arbitrum. Trade{" "}
					<em className="display-it">BUMIP, TLKMP, GOTOP</em> and others at any
					hour, settle in seconds, and exit gap risk on the weekend. A custodian
					holds the underlying; arbitrageurs maintain the peg.
				</p>

				<div className="hairline-top grid-3col mt-[36px] pt-[24px]">
					<Stat
						label="24h Volume"
						value={protocolStats ? `${fmtIDRXCompact(protocolStats.volume_24h)} IDRX` : "—"}
						sub={protocolStats ? `across ${protocolStats.pair_count} pairs` : "loading…"}
					/>
					<Stat
						label="Total Value Locked"
						value={protocolStats ? `${fmtIDRXCompact(protocolStats.tvl_idrx)} IDRX` : "—"}
						sub="net IDRX in pools"
					/>
					<Stat
						label="Peg Deviation"
						value={pegDeviation !== null ? `${pegDeviation.toFixed(2)}%` : "—"}
						sub={pegDeviation !== null ? "pool vs IDX price" : "no pool data"}
					/>
				</div>

				<div className="mt-[36px]">
					<div className="eyebrow mb-[14px] !text-[var(--body)]">
						Top movers · 24h
					</div>
					<MoversList tokens={marketTokens} />
				</div>
			</div>

			{/* RIGHT — Swap card */}
			<div className="swap-sticky sticky top-[24px]">
				<div className="card swap-card-shadow p-[0] shadow-[12px_12px_0_0_rgba(22,17,14,0.08)]">
					<div className="hairline flex items-center justify-between px-[20px] py-[16px]">
						<div className="display !text-[22px] !font-medium">
							Swap
						</div>
						<div className="flex items-center gap-[14px]">
							<button
								type="button"
								className="btn btn-ghost !p-[4px]"
								onClick={() => setShowSettings((s) => !s)}
								aria-label="Settings"
							>
								<Icon name="settings" size={16} />
							</button>
						</div>
					</div>

					{showSettings && (
						<div className="hairline bg-[var(--canvas)] px-[20px] py-[14px]">
							<div className="eyebrow mb-[8px] !text-[var(--body)]">
								Slippage Tolerance
							</div>
							<div className="flex gap-[8px]">
								{[0.1, 0.5, 1.0].map((s) => (
									<button
										type="button"
										key={s}
										onClick={() => setSlippage(s)}
										className={`cursor-pointer appearance-none border border-[var(--ink)] px-[14px] py-[8px] text-[13px] font-semibold [font-family:var(--font-inter,_Inter,_sans-serif)] ${
											slippage === s
												? "bg-[var(--ink)] text-[var(--putih)]"
												: "bg-transparent text-[var(--ink)]"
										}`}
									>
										{s}%
									</button>
								))}
								<div className="flex-1" />
								<input
									type="number"
									step="0.1"
									value={slippage}
									onChange={(e) =>
										setSlippage(parseFloat(e.target.value) || 0)
									}
									className="input mono !w-[90px] !text-right"
								/>
							</div>
						</div>
					)}

					<SwapField
						label="You pay"
						token={inputToken}
						balance={inputBalance}
						amount={amount}
						onAmount={setTradeAmount}
						onSelect={() => setPickerFor("in")}
						actions={
							isConnected ? (
								<div className="flex gap-[6px]">
									{[0.25, 0.5, 1].map(percentButton)}
								</div>
							) : undefined
						}
					/>

					<div className="relative h-[0]">
						<button
							type="button"
							onClick={flip}
							aria-label="Flip"
							className="absolute left-1/2 top-[-18px] flex h-[36px] w-[36px] -translate-x-1/2 cursor-pointer items-center justify-center border border-[var(--ink)] bg-[var(--canvas)] text-[var(--ink)]"
						>
							<Icon name="swap" size={14} />
						</button>
					</div>

					<SwapField
						label="You receive"
						token={outputToken ?? STABLES[0]}
						balance={outputBalance}
						amount={
							quote.outputAmount
								? quote.outputAmount.toFixed(outputToken?.isStable ? 2 : 4)
								: ""
						}
						readOnly
						onSelect={() => setPickerFor("out")}
						actions={
							<div className="mono text-[11px] text-[var(--body)]">
								≈ {fmtIDRX(quote.inputAmount * inputToken.price)}
							</div>
						}
					/>

					<div className="px-[20px] pb-[16px] pt-[4px]">
						<Accordion
							open={detailsOpen}
							onToggle={() => setDetailsOpen((open) => !open)}
							summary={<span>{quote.rateSummary}</span>}
						>
							<DetailRow
								k="Expected output"
								v={`${fmtNum(quote.outputAmount, 4)} ${outputToken?.ticker ?? ""}`}
							/>
							<DetailRow
								k="Minimum received"
								v={`${fmtNum(quote.minReceived, 4)} ${outputToken?.ticker ?? ""}`}
								hint={`slippage ${slippage}%`}
							/>
							<DetailRow
								k="LP fee"
								v={`${fmtNum(quote.inputAmount * 0.003, 4)} ${inputToken.ticker}`}
								hint="0.30%"
							/>
							<DetailRow
								k="Route"
								v={
									<span className="inline-flex items-center gap-[6px]">
										<span className="mono">{inputToken.ticker}</span>
										<Icon name="chevron-right" size={11} />
										<span className="mono text-[var(--merah)]">
											{outputToken?.ticker ?? ""}
										</span>
										<span className="ml-[6px] text-[var(--body)]">
											· Uniswap V2
										</span>
									</span>
								}
							/>
						</Accordion>
					</div>

					<div className="px-[20px] pb-[20px]">
						{!isConnected ? (
							<ConnectButton.Custom>
								{({ openConnectModal }) => (
									<button
										type="button"
										onClick={openConnectModal}
										className="btn btn-merah !w-full !px-[20px] !py-[16px] !text-[15px] !tracking-[0.03em]"
									>
										Connect Wallet
									</button>
								)}
							</ConnectButton.Custom>
						) : (
							<button
								type="button"
								onClick={ctaAction}
								disabled={ctaDisabled}
								className="btn btn-merah !w-full !px-[20px] !py-[16px] !text-[15px] !tracking-[0.03em]"
							>
								{busy && (
									<span className="mr-[8px] inline-block align-[-2px]">
										<Icon name="loader" size={14} />
									</span>
								)}
								{ctaText}
							</button>
						)}
						{isConnected && !insufficient && quote.inputAmount > 0 && (
							<div className="mt-[12px] text-center text-[11px] tracking-[0.04em] text-[var(--body)]">
								Settles instantly on-chain · Custody peg 1:1 verified 8 min ago
							</div>
						)}
					</div>
				</div>

				<div className="mt-[18px] text-[12px] leading-[1.6] text-[var(--body)] [font-family:var(--font-fraunces,_Fraunces,_serif)]">
					By trading, you affirm you are not a resident of restricted
					jurisdictions and that these tokens are cryptographic receipts fully
					backed 1:1 by physical IDX-listed equities held in custody by{" "}
					<em>PT Horizon Kustodian Indonesia</em>.
				</div>
			</div>

			<TokenSelectModal
				open={!!pickerFor}
				tokens={pickerFor === "in" ? inputTokensForPicker : outputTokensForPicker}
				balances={walletBalances}
				title={pickerFor === "in" ? "Pay with" : "Receive"}
				excludeTicker={
					pickerFor === "in" ? outputToken?.ticker : inputToken.ticker
				}
				onSelect={selectToken}
				onClose={() => setPickerFor(null)}
			/>
		</div>
	);
}

function SwapField({
	label,
	token,
	balance,
	amount,
	onAmount,
	onSelect,
	readOnly,
	actions,
}: {
	label: string;
	token: Token;
	balance?: number;
	amount: string;
	onAmount?: (value: string) => void;
	onSelect: () => void;
	readOnly?: boolean;
	actions?: React.ReactNode;
}) {
	return (
		<div className="hairline px-[20px] pb-[16px] pt-[20px]">
			<div className="mb-[12px] flex items-baseline justify-between">
				<span className="eyebrow !text-[var(--body)]">
					{label}
				</span>
				<span className="mono text-[12px] text-[var(--body)]">
					Bal {balance != null ? fmtNum(balance, 4) : "—"}
				</span>
			</div>
			<div className="flex items-center gap-[16px]">
				<input
					inputMode="decimal"
					placeholder="0.00"
					value={amount}
					readOnly={readOnly}
					onChange={
						onAmount
							? (e) => {
									const v = e.target.value.replace(/[^0-9.]/g, "");
									onAmount(v);
								}
							: undefined
					}
					className="swap-input-amt w-full min-w-0 flex-1 appearance-none border-0 bg-transparent p-[0] text-[36px] font-normal tracking-[-0.02em] text-[var(--ink)] outline-none [font-family:var(--font-fraunces,_Fraunces,_serif)]"
				/>
				<button
					type="button"
					onClick={onSelect}
					className="flex cursor-pointer appearance-none items-center gap-[10px] border border-[var(--ink)] bg-[var(--canvas)] py-[8px] pl-[8px] pr-[12px] text-[var(--ink)] [font:inherit]"
				>
					<PStockMark ticker={token.ticker} size={26} />
					<span className="text-[14px] font-semibold">{token.ticker}</span>
					<Icon name="chevron-down" size={14} />
				</button>
			</div>
			<div className="mt-[10px] flex min-h-[22px] items-center justify-between">
				<span className="mono text-[12px] text-[var(--body)]">
					{token.name}
				</span>
				<div>{actions}</div>
			</div>
		</div>
	);
}

export function DetailRow({
	k,
	v,
	hint,
}: {
	k: string;
	v: React.ReactNode;
	hint?: string;
}) {
	return (
		<div className="flex items-baseline justify-between gap-[12px]">
			<span className="text-[13px] text-[var(--body)]">{k}</span>
			<span className="mono text-right text-[13px]">
				{v}
				{hint && (
					<span className="ml-[6px] text-[11px] text-[var(--body)] [font-family:var(--font-inter,_Inter,_sans-serif)]">
						· {hint}
					</span>
				)}
			</span>
		</div>
	);
}

function Stat({
	label,
	value,
	sub,
}: {
	label: string;
	value: string;
	sub: string;
}) {
	return (
		<div>
			<div className="eyebrow !text-[var(--body)]">
				{label}
			</div>
			<div className="display mt-[6px] !text-[32px] !leading-none !tracking-[-0.02em]">
				{value}
			</div>
			<div className="mt-[6px] text-[12px] text-[var(--body)]">
				{sub}
			</div>
		</div>
	);
}

function MoversList({ tokens }: { tokens: MarketToken[] }) {
	const { data: marketStocks = [], isLoading } = useMarketStocks();

	const movers = useMemo(() => {
		const liveTokens = tokens.length > 0 ? tokens : [];
		return [...liveTokens]
			.sort((a, b) => Math.abs(b.change24h) - Math.abs(a.change24h))
			.slice(0, 5);
	}, [tokens]);

	const sparklineData = useMemo(() => {
		const stockMap = new Map(marketStocks.map((stock) => [stock.ticker, stock]));
		return movers.reduce<Record<string, number[]>>((accumulator, token) => {
			const stock = stockMap.get(token.ticker);
			accumulator[token.ticker] =
				stock?.sparkline_7d?.length ? stock.sparkline_7d : seriesFor(token.ticker, 28);
			return accumulator;
		}, {});
	}, [marketStocks, movers]);

	if (isLoading && tokens.length === 0) {
		return (
			<div className="hairline-top">
				{Array.from({ length: 5 }, (_, skeletonIndex) => (
					<div
						key={skeletonIndex}
						className="hairline mover-row grid grid-cols-[auto_1fr_auto_auto_auto] items-center gap-[16px] py-[14px]"
					>
						<div className="skeleton h-[32px] w-[32px] shrink-0" />
						<div>
							<div className="skeleton mb-[4px] h-[14px] w-[60px]" />
							<div className="skeleton h-[11px] w-[110px]" />
						</div>
						<div className="mover-spark" />
						<div className="skeleton ml-auto h-[14px] w-[80px]" />
						<div className="skeleton ml-auto h-[13px] w-[56px]" />
					</div>
				))}
			</div>
		);
	}

	return (
		<div className="hairline-top">
			{movers.map((token) => {
				const isPositive = token.change24h >= 0;
				return (
					<div
						key={token.ticker}
						className="hairline row-hover mover-row grid grid-cols-[auto_1fr_auto_auto_auto] items-center gap-[16px] py-[14px]"
					>
						<PStockMark ticker={token.ticker} size={32} />
						<div className="min-w-0">
							<div className="text-[14px] font-semibold">{token.ticker}</div>
							<div className="truncate text-[12px] text-[var(--body)]">
								{token.name} · {token.sector}
							</div>
						</div>
						<div className="mover-spark">
							<Sparkline
								data={sparklineData[token.ticker] ?? []}
								positive={isPositive}
							/>
						</div>
						<div className="mono min-w-[80px] text-right text-[14px]">
							{fmtIDRX(token.price)}
						</div>
						<div
							className={`mono min-w-[76px] text-right text-[13px] ${
								isPositive ? "text-[var(--positive)]" : "text-[var(--negative)]"
							}`}
						>
							{fmtPct(token.change24h)}
						</div>
					</div>
				);
			})}
		</div>
	);
}

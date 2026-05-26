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
import { useMarketStocks } from "@/http/market/hooks";
import {
	STABLES,
	fmtIDRX,
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
			style={{
				appearance: "none",
				border: "1px solid var(--hairline-strong)",
				background: "transparent",
				padding: "4px 10px",
				fontSize: 11,
				fontWeight: 600,
				fontFamily: "Inter",
				cursor: "pointer",
				letterSpacing: "0.08em",
				textTransform: "uppercase",
			}}
		>
			{pct === 1 ? "Max" : `${pct * 100}%`}
		</button>
	);

	return (
		<div className="grid-2col pad-x" style={{ padding: "40px 24px" }}>
			{/* LEFT — Editorial */}
			<div>
				<h1
					className="display hero-display"
					style={{
						fontSize: 70,
						lineHeight: 0.96,
						letterSpacing: "-0.03em",
						margin: 0,
						fontWeight: 400,
					}}
				>
					{headline.line1}
					<br />
					<span className="display-it">{headline.line2}</span>
					<br />
					{headline.line3}
				</h1>
				<p
					style={{
						marginTop: 28,
						fontSize: 18,
						lineHeight: 1.55,
						color: "var(--ink-soft)",
						maxWidth: 540,
						fontFamily: '"Fraunces", serif',
						fontWeight: 300,
					}}
				>
					Eight blue-chip equities from the Indonesia Stock Exchange, tokenized
					1:1 on Arbitrum. Trade{" "}
					<em className="display-it">BUMIP, TLKMP, GOTOP</em> and others at any
					hour, settle in seconds, and exit gap risk on the weekend. A custodian
					holds the underlying; arbitrageurs maintain the peg.
				</p>

				<div
					className="hairline-top grid-3col"
					style={{ marginTop: 36, paddingTop: 24 }}
				>
					<Stat label="24h Volume" value="77.8M IDRX" sub="across 8 pairs" />
					<Stat
						label="Total Value Locked"
						value="458M IDRX"
						sub="↑ 12.3% week"
					/>
					<Stat label="Peg Deviation" value="0.03%" sub="within tolerance" />
				</div>

				<div style={{ marginTop: 36 }}>
					<div
						className="eyebrow"
						style={{ marginBottom: 14, color: "var(--body)" }}
					>
						Top movers · 24h
					</div>
					<MoversList tokens={marketTokens} />
				</div>
			</div>

			{/* RIGHT — Swap card */}
			<div className="swap-sticky" style={{ position: "sticky", top: 24 }}>
				<div
					className="card swap-card-shadow"
					style={{ padding: 0, boxShadow: "12px 12px 0 0 rgba(22,17,14,0.08)" }}
				>
					<div
						className="hairline"
						style={{
							padding: "16px 20px",
							display: "flex",
							alignItems: "center",
							justifyContent: "space-between",
						}}
					>
						<div className="display" style={{ fontSize: 22, fontWeight: 500 }}>
							Swap
						</div>
						<div style={{ display: "flex", gap: 14, alignItems: "center" }}>
							<div className="eyebrow" style={{ color: "var(--body)" }}>
								v2 ROUTER
							</div>
							<button
								type="button"
								className="btn btn-ghost"
								style={{ padding: 4 }}
								onClick={() => setShowSettings((s) => !s)}
								aria-label="Settings"
							>
								<Icon name="settings" size={16} />
							</button>
						</div>
					</div>

					{showSettings && (
						<div
							className="hairline"
							style={{ padding: "14px 20px", background: "var(--canvas)" }}
						>
							<div
								className="eyebrow"
								style={{ color: "var(--body)", marginBottom: 8 }}
							>
								Slippage Tolerance
							</div>
							<div style={{ display: "flex", gap: 8 }}>
								{[0.1, 0.5, 1.0].map((s) => (
									<button
										type="button"
										key={s}
										onClick={() => setSlippage(s)}
										style={{
											appearance: "none",
											padding: "8px 14px",
											border: "1px solid var(--ink)",
											background: slippage === s ? "var(--ink)" : "transparent",
											color: slippage === s ? "var(--putih)" : "var(--ink)",
											fontSize: 13,
											fontWeight: 600,
											cursor: "pointer",
											fontFamily: "Inter",
										}}
									>
										{s}%
									</button>
								))}
								<div style={{ flex: 1 }} />
								<input
									type="number"
									step="0.1"
									value={slippage}
									onChange={(e) =>
										setSlippage(parseFloat(e.target.value) || 0)
									}
									className="input mono"
									style={{ width: 90, textAlign: "right" }}
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
								<div style={{ display: "flex", gap: 6 }}>
									{[0.25, 0.5, 1].map(percentButton)}
								</div>
							) : undefined
						}
					/>

					<div style={{ position: "relative", height: 0 }}>
						<button
							type="button"
							onClick={flip}
							aria-label="Flip"
							style={{
								position: "absolute",
								left: "50%",
								top: -18,
								transform: "translateX(-50%)",
								width: 36,
								height: 36,
								background: "var(--canvas)",
								border: "1px solid var(--ink)",
								cursor: "pointer",
								color: "var(--ink)",
								display: "flex",
								alignItems: "center",
								justifyContent: "center",
								borderRadius: 0,
							}}
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
							<div className="mono" style={{ fontSize: 11, color: "var(--body)" }}>
								≈ {fmtIDRX(quote.inputAmount * inputToken.price)}
							</div>
						}
					/>

					<div style={{ padding: "4px 20px 16px" }}>
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
									<span
										style={{
											display: "inline-flex",
											alignItems: "center",
											gap: 6,
										}}
									>
										<span className="mono">{inputToken.ticker}</span>
										<Icon name="chevron-right" size={11} />
										<span className="mono" style={{ color: "var(--merah)" }}>
											{outputToken?.ticker ?? ""}
										</span>
										<span style={{ color: "var(--body)", marginLeft: 6 }}>
											· Uniswap V2
										</span>
									</span>
								}
							/>
						</Accordion>
					</div>

					<div style={{ padding: "0 20px 20px" }}>
						{!isConnected ? (
							<ConnectButton.Custom>
								{({ openConnectModal }) => (
									<button
										type="button"
										onClick={openConnectModal}
										className="btn btn-merah"
										style={{
											width: "100%",
											padding: "16px 20px",
											fontSize: 15,
											letterSpacing: "0.03em",
										}}
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
								className="btn btn-merah"
								style={{
									width: "100%",
									padding: "16px 20px",
									fontSize: 15,
									letterSpacing: "0.03em",
								}}
							>
								{busy && (
									<span
										style={{
											display: "inline-block",
											verticalAlign: "-2px",
											marginRight: 8,
										}}
									>
										<Icon name="loader" size={14} />
									</span>
								)}
								{ctaText}
							</button>
						)}
						{isConnected && !insufficient && quote.inputAmount > 0 && (
							<div
								style={{
									marginTop: 12,
									fontSize: 11,
									color: "var(--body)",
									textAlign: "center",
									letterSpacing: "0.04em",
								}}
							>
								Settles instantly on-chain · Custody peg 1:1 verified 8 min ago
							</div>
						)}
					</div>
				</div>

				<div
					style={{
						marginTop: 18,
						fontSize: 12,
						color: "var(--body)",
						lineHeight: 1.6,
						fontFamily: '"Fraunces", serif',
					}}
				>
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
		<div className="hairline" style={{ padding: "20px 20px 16px" }}>
			<div
				style={{
					display: "flex",
					justifyContent: "space-between",
					alignItems: "baseline",
					marginBottom: 12,
				}}
			>
				<span className="eyebrow" style={{ color: "var(--body)" }}>
					{label}
				</span>
				<span className="mono" style={{ fontSize: 12, color: "var(--body)" }}>
					Bal {balance != null ? fmtNum(balance, 4) : "—"}
				</span>
			</div>
			<div style={{ display: "flex", alignItems: "center", gap: 16 }}>
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
					className="mono swap-input-amt"
					style={{
						appearance: "none",
						border: 0,
						outline: 0,
						flex: 1,
						minWidth: 0,
						background: "transparent",
						fontSize: 36,
						fontWeight: 400,
						color: "var(--ink)",
						padding: 0,
						fontFamily: '"Fraunces", serif',
						letterSpacing: "-0.02em",
						width: "100%",
					}}
				/>
				<button
					type="button"
					onClick={onSelect}
					style={{
						appearance: "none",
						display: "flex",
						alignItems: "center",
						gap: 10,
						padding: "8px 12px 8px 8px",
						border: "1px solid var(--ink)",
						background: "var(--canvas)",
						cursor: "pointer",
						color: "var(--ink)",
						font: "inherit",
					}}
				>
					<PStockMark ticker={token.ticker} size={26} />
					<span style={{ fontWeight: 600, fontSize: 14 }}>{token.ticker}</span>
					<Icon name="chevron-down" size={14} />
				</button>
			</div>
			<div
				style={{
					display: "flex",
					justifyContent: "space-between",
					alignItems: "center",
					marginTop: 10,
					minHeight: 22,
				}}
			>
				<span className="mono" style={{ fontSize: 12, color: "var(--body)" }}>
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
		<div
			style={{
				display: "flex",
				justifyContent: "space-between",
				alignItems: "baseline",
				gap: 12,
			}}
		>
			<span style={{ color: "var(--body)", fontSize: 13 }}>{k}</span>
			<span
				style={{
					textAlign: "right",
					fontSize: 13,
					fontFamily: '"JetBrains Mono", monospace',
				}}
			>
				{v}
				{hint && (
					<span
						style={{
							marginLeft: 6,
							color: "var(--body)",
							fontFamily: "Inter",
							fontSize: 11,
						}}
					>
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
			<div className="eyebrow" style={{ color: "var(--body)" }}>
				{label}
			</div>
			<div
				className="display"
				style={{
					fontSize: 32,
					marginTop: 6,
					lineHeight: 1,
					letterSpacing: "-0.02em",
				}}
			>
				{value}
			</div>
			<div style={{ marginTop: 6, fontSize: 12, color: "var(--body)" }}>
				{sub}
			</div>
		</div>
	);
}

function MoversList({ tokens }: { tokens: MarketToken[] }) {
	const { data: marketStocks = [] } = useMarketStocks();

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

	return (
		<div className="hairline-top">
			{movers.map((token) => {
				const isPositive = token.change24h >= 0;
				return (
					<div
						key={token.ticker}
						className="hairline row-hover mover-row"
						style={{
							display: "grid",
							gridTemplateColumns: "auto 1fr auto auto auto",
							alignItems: "center",
							gap: 16,
							padding: "14px 0",
						}}
					>
						<PStockMark ticker={token.ticker} size={32} />
						<div style={{ minWidth: 0 }}>
							<div style={{ fontWeight: 600, fontSize: 14 }}>{token.ticker}</div>
							<div
								style={{
									fontSize: 12,
									color: "var(--body)",
									whiteSpace: "nowrap",
									overflow: "hidden",
									textOverflow: "ellipsis",
								}}
							>
								{token.name} · {token.sector}
							</div>
						</div>
						<div className="mover-spark">
							<Sparkline
								data={sparklineData[token.ticker] ?? []}
								positive={isPositive}
							/>
						</div>
						<div
							className="mono"
							style={{ minWidth: 80, textAlign: "right", fontSize: 14 }}
						>
							{fmtIDRX(token.price)}
						</div>
						<div
							className="mono"
							style={{
								minWidth: 76,
								textAlign: "right",
								fontSize: 13,
								color: isPositive ? "var(--positive)" : "var(--negative)",
							}}
						>
							{fmtPct(token.change24h)}
						</div>
					</div>
				);
			})}
		</div>
	);
}

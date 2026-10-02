'use client';

import { useMemo, useState } from 'react';
import { type Address } from 'viem';
import { useAccount } from 'wagmi';
import { ConnectButton } from '@rainbow-me/rainbowkit';
import { toast } from 'sonner';
import { STABLES, tokenByTicker, fmtNum, Token } from '@/lib/data';
import {
  buildSwapQuote,
  stockTickerForSwap,
  swapToastDescription,
  tokenAddress,
} from '@/lib/swap';
import { useExecuteSwap, useSwapFeeBps } from '@/http/market/swapHooks';
import { useMarketTokens, useWalletTokenBalances } from '@/http/market/tokenHooks';
import { Icon } from './Icon';
import { PStockMark } from './PStockMark';
import { Accordion } from './Accordion';
import { TokenSelectModal } from './TokenSelectModal';

interface SwapModalProps {
  defaultOut: Token;
  onClose: () => void;
}

export function SwapModal({ defaultOut, onClose }: SwapModalProps) {
  const { address, isConnected } = useAccount();
  const executeSwap = useExecuteSwap();
  const { data: swapFeeBpsRaw } = useSwapFeeBps();
  const swapFeeBps = Number(swapFeeBpsRaw ?? BigInt(0));
  const marketTokens = useMarketTokens();
  const walletBalances = useWalletTokenBalances(marketTokens);

  const [inputTicker, setInputTicker] = useState('IDRX');
  const [outputTicker, setOutputTicker] = useState(defaultOut.ticker);
  const [amount, setAmount] = useState('');
  const [pickerFor, setPickerFor] = useState<'in' | 'out' | null>(null);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [slippage, setSlippage] = useState(0.5);
  const [showSettings, setShowSettings] = useState(false);
  const [flipDegrees, setFlipDegrees] = useState(0);

  const inputToken = useMemo(
    () => STABLES.find(token => token.ticker === inputTicker)
      ?? marketTokens.find(token => token.ticker === inputTicker)
      ?? tokenByTicker('IDRX'),
    [inputTicker, marketTokens],
  );
  const outputToken = useMemo(
    () => marketTokens.find(token => token.ticker === outputTicker)
      ?? STABLES.find(token => token.ticker === outputTicker)
      ?? defaultOut,
    [defaultOut, outputTicker, marketTokens],
  );
  const inputTokens = outputToken.isStable ? marketTokens : STABLES;
  const outputTokens = inputToken.isStable ? marketTokens : STABLES;

  const idrxAddress = process.env.NEXT_PUBLIC_IDRX_ADDRESS as Address | undefined;
  const protocolAddress = process.env.NEXT_PUBLIC_PULSAR_PROTOCOL_ADDRESS as Address | undefined;
  const inputAddress = tokenAddress(inputToken, idrxAddress);
  const quote = useMemo(
    () => buildSwapQuote(inputToken, outputToken, amount, slippage, swapFeeBps),
    [amount, inputToken, outputToken, slippage, swapFeeBps],
  );
  const inputBalance = walletBalances[inputToken.ticker] ?? 0;
  const outputBalance = walletBalances[outputToken.ticker] ?? 0;
  const busy = executeSwap.isPending;
  const insufficient = isConnected && quote.inputAmount > inputBalance;

  const cta = buildCta({
    isConnected,
    hasAmount: quote.inputAmount > 0,
    hasRate: quote.rate > 0,
    insufficient,
    protocolReady: Boolean(protocolAddress && inputAddress),
    busy,
    inputTicker: inputToken.ticker,
  });

  function setTradeAmount(value: string) {
    const [whole, ...fractions] = value.split('.');
    setAmount(fractions.length > 0 ? `${whole}.${fractions.join('')}` : whole);
  }

  function selectToken(token: Token) {
    if (pickerFor === 'in') {
      setInputTicker(token.ticker);
    }
    if (pickerFor === 'out') {
      setOutputTicker(token.ticker);
    }
    setPickerFor(null);
  }

  function flip() {
    setInputTicker(outputToken.ticker);
    setOutputTicker(inputToken.ticker);
    setAmount('');
    setFlipDegrees(degrees => degrees + 180);
  }

  async function swap() {
    if (!address || !inputAddress) return;

    const toastId = toast.loading('Executing swap pipeline...', {
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
      toast.success('Swap executed', { id: toastId, description: `Tx ${txHash.slice(0, 10)}...${txHash.slice(-6)}`, duration: 6000 });
      setAmount('');
      onClose();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Swap failed';
      toast.error('Swap failed', { id: toastId, description: message.slice(0, 120), duration: 7000 });
    }
  }

  return (
    <>
      <div className="overlay" onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(22,17,14,0.32)', zIndex: 300, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}>
        <div className="rise paper-sheaf" onClick={event => event.stopPropagation()} style={{ width: 440, maxWidth: '100%' }}>
          <div className="sheet">
            <div style={{ padding: '16px 20px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--hairline)' }}>
              <div className="display" style={{ fontSize: 22, fontWeight: 500, lineHeight: 'normal' }}>Trade {inputToken.isStable ? outputToken.ticker : inputToken.ticker}</div>
              <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                <button type="button" onClick={() => setShowSettings(s => !s)} aria-label="Settings" style={{ appearance: 'none', width: 30, height: 30, cursor: 'pointer', fontSize: 14, border: `1px solid ${showSettings ? 'var(--ink)' : 'var(--hairline)'}`, background: showSettings ? 'var(--ink)' : '#fff', color: showSettings ? '#fff' : 'var(--ink)' }}>⚙</button>
                <button type="button" onClick={onClose} aria-label="Close" style={{ appearance: 'none', width: 30, height: 30, cursor: 'pointer', fontSize: 13, border: '1px solid var(--hairline)', background: '#fff', color: 'var(--ink)' }}>✕</button>
              </div>
            </div>

            {showSettings && (
              <div style={{ padding: '14px 20px', background: 'var(--canvas-soft)', borderBottom: '1px solid var(--hairline)' }}>
                <div style={{ fontSize: 10, fontWeight: 600, letterSpacing: '0.14em', textTransform: 'uppercase', color: 'var(--body)', marginBottom: 8 }}>Slippage Tolerance</div>
                <div style={{ display: 'flex', gap: 6 }}>
                  {[0.1, 0.5, 1.0].map(s => (
                    <button type="button" key={s} onClick={() => setSlippage(s)} style={{ appearance: 'none', padding: '8px 14px', border: `1px solid ${slippage === s ? 'var(--ink)' : 'var(--hairline-strong)'}`, background: slippage === s ? 'var(--ink)' : '#fff', color: slippage === s ? '#fff' : 'var(--ink)', fontSize: 13, fontWeight: 600, cursor: 'pointer', fontFamily: 'Inter', boxShadow: slippage === s ? '0 3px 0 -1px #f3f0ea, 0 4px 0 -1px #16110e' : 'none' }}>
                      {s}%
                    </button>
                  ))}
                  <div style={{ flex: 1 }} />
                  <input
                    type="number"
                    step="0.1"
                    min="0.1"
                    max="1.0"
                    value={slippage}
                    onChange={e => {
                      const val = parseFloat(e.target.value) || 0;
                      setSlippage(Math.min(1.0, Math.max(0.1, val)));
                    }}
                    className="input mono"
                    style={{ width: 90, textAlign: 'right' }}
                  />
                </div>
              </div>
            )}

            <ModalField label="You pay" token={inputToken} balance={inputBalance} amount={amount} onAmount={setTradeAmount} onSelect={() => setPickerFor('in')} />

            <div style={{ position: 'relative', height: 0 }}>
              <button onClick={flip} aria-label="Flip" style={{ position: 'absolute', left: '50%', marginLeft: -18, top: -18, zIndex: 2, width: 36, height: 36, background: 'var(--canvas)', border: '1px solid var(--ink)', cursor: 'pointer', color: 'var(--ink)', fontSize: 14, display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 0, boxShadow: '0 3px 0 -1px #fff, 0 4px 0 -1px #16110e', transform: `rotate(${flipDegrees}deg)`, transition: 'transform .4s cubic-bezier(.2,.7,.3,1)' }}>
                ⇅
              </button>
            </div>

            <ModalField
              label="You receive"
              token={outputToken}
              balance={outputBalance}
              amount={quote.outputAmount ? quote.outputAmount.toFixed(outputToken.isStable ? 2 : 4) : ''}
              readOnly
              onSelect={() => setPickerFor('out')}
              hint={quote.inputAmount && quote.grossOutputAmount ? `${fmtNum(quote.grossOutputAmount, 4)} before LP fee` : undefined}
            />

            <div style={{ padding: '0 20px' }}>
              <Accordion open={detailsOpen} onToggle={() => setDetailsOpen(open => !open)} summary={<span>{quote.rateSummary}</span>}>
                <DetailRow k="Min received" v={`${fmtNum(quote.minReceived, 4)} ${outputToken.ticker}`} hint={`${slippage}% slippage`} />
                <DetailRow k="LP fee" v={`${fmtNum(quote.inputAmount * 0.003, 4)} ${inputToken.ticker}`} hint="0.30%" />
                {quote.protocolFeeBps > 0 && (
                  <DetailRow
                    k="Protocol fee"
                    v={`${fmtNum(quote.inputAmount * (quote.protocolFeeBps / 10_000), 4)} ${inputToken.ticker}`}
                    hint={`${(quote.protocolFeeBps / 100).toFixed(2)}%`}
                  />
                )}
              </Accordion>
            </div>

            <div style={{ padding: '16px 20px 20px' }}>
              {!isConnected ? (
                <ConnectButton.Custom>
                  {({ openConnectModal }) => (
                    <button className="btn btn-merah" onClick={openConnectModal} style={{ width: '100%', padding: '15px 20px', fontSize: 15, letterSpacing: '0.03em' }}>Connect Wallet</button>
                  )}
                </ConnectButton.Custom>
              ) : (
                <button onClick={swap} disabled={cta.disabled} className={`btn btn-merah${busy ? ' is-busy' : ''}`} style={{ width: '100%', padding: '15px 20px', fontSize: 15, letterSpacing: '0.03em' }}>
                  {busy && <span style={{ display: 'inline-block', verticalAlign: '-2px', marginRight: 8 }}><Icon name="loader" size={14} /></span>}
                  {cta.text}
                </button>
              )}
            </div>
          </div>
        </div>
      </div>

      <TokenSelectModal
        open={!!pickerFor}
        tokens={pickerFor === 'in' ? inputTokens : outputTokens}
        balances={walletBalances}
        title={pickerFor === 'in' ? 'Pay with' : 'Receive'}
        excludeTicker={pickerFor === 'in' ? outputToken.ticker : inputToken.ticker}
        onSelect={selectToken}
        onClose={() => setPickerFor(null)}
      />
    </>
  );
}

function buildCta(params: {
  isConnected: boolean;
  hasAmount: boolean;
  hasRate: boolean;
  insufficient: boolean;
  protocolReady: boolean;
  busy: boolean;
  inputTicker: string;
}) {
  if (!params.isConnected) return { text: 'Connect Wallet', disabled: false };
  if (!params.hasAmount) return { text: 'Enter an amount', disabled: true };
  if (!params.hasRate) return { text: 'Pool unavailable', disabled: true };
  if (params.insufficient) return { text: `Insufficient ${params.inputTicker}`, disabled: true };
  if (!params.protocolReady) return { text: 'Protocol unavailable', disabled: true };
  return { text: params.busy ? 'Executing...' : 'Execute Swap', disabled: params.busy };
}

function ModalField({ label, token, balance, amount, onAmount, onSelect, readOnly, hint }: {
  label: string; token: Token; balance: number; amount: string;
  onAmount?: (value: string) => void; onSelect: () => void; readOnly?: boolean; hint?: string;
}) {
  return (
    <div style={{ padding: '16px 20px 14px', borderBottom: '1px solid var(--hairline)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 10 }}>
        <span style={{ fontSize: 10, fontWeight: 600, letterSpacing: '0.14em', textTransform: 'uppercase', color: 'var(--body)' }}>{label}</span>
        <span className="mono" style={{ fontSize: 12, color: 'var(--body)' }}>{token.isStable ? 'Amount' : 'Holding'} {fmtNum(balance, 4)}</span>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <input
          inputMode="decimal"
          placeholder="0.00"
          value={amount}
          readOnly={readOnly}
          onChange={onAmount ? event => onAmount(event.target.value.replace(/[^0-9.]/g, '')) : undefined}
          className={readOnly ? 'swap-readonly-input' : undefined}
          style={{ appearance: 'none', border: 0, outline: 0, flex: 1, background: 'transparent', fontSize: 32, color: 'var(--ink)', padding: 0, fontFamily: '"Fraunces", serif', fontWeight: 400, letterSpacing: '-0.02em' }}
        />
        <button onClick={onSelect} style={{ appearance: 'none', display: 'flex', alignItems: 'center', gap: 8, padding: '6px 10px 6px 8px', border: '1px solid var(--ink)', background: 'var(--canvas)', cursor: 'pointer', color: 'var(--ink)', font: 'inherit', boxShadow: '0 3px 0 -1px #fbfaf7, 0 4px 0 -1px #bcb2a3' }}>
          <PStockMark ticker={token.ticker} size={22} />
          <span style={{ fontWeight: 600, fontSize: 13 }}>{token.ticker}</span>
          <span style={{ fontSize: 10 }}>▾</span>
        </button>
      </div>
      {hint && <div className="mono" style={{ fontSize: 11, color: 'var(--body)', marginTop: 6 }}>{hint}</div>}
    </div>
  );
}

function DetailRow({ k, v, hint }: { k: string; v: string; hint?: string }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 12 }}>
      <span style={{ color: 'var(--body)', fontSize: 13 }}>{k}</span>
      <span className="mono" style={{ fontSize: 13 }}>
        {v}{hint && <span style={{ marginLeft: 6, color: 'var(--body)', fontFamily: 'Inter', fontSize: 11 }}>· {hint}</span>}
      </span>
    </div>
  );
}

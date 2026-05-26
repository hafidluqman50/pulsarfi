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
import { useExecuteSwap } from '@/http/market/swapHooks';
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
  const marketTokens = useMarketTokens();
  const walletBalances = useWalletTokenBalances(marketTokens);

  const [inputTicker, setInputTicker] = useState('IDRX');
  const [outputTicker, setOutputTicker] = useState(defaultOut.ticker);
  const [amount, setAmount] = useState('');
  const [pickerFor, setPickerFor] = useState<'in' | 'out' | null>(null);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [slippage, setSlippage] = useState(0.5);
  const [showSettings, setShowSettings] = useState(false);

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
  const quote = useMemo(() => buildSwapQuote(inputToken, outputToken, amount, slippage), [amount, inputToken, outputToken, slippage]);
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
      <div className="overlay" onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(22,17,14,0.45)', zIndex: 300, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}>
        <div className="modal" onClick={event => event.stopPropagation()} style={{ background: 'var(--putih)', width: 440, maxWidth: '100%', border: '1px solid var(--ink)', boxShadow: '12px 12px 0 0 rgba(22,17,14,0.10)' }}>
          <div className="hairline" style={{ padding: '16px 20px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div className="display" style={{ fontSize: 22, fontWeight: 500 }}>Trade {inputToken.isStable ? outputToken.ticker : inputToken.ticker}</div>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <button type="button" className="btn btn-ghost" onClick={() => setShowSettings(s => !s)} style={{ padding: 4 }} aria-label="Settings"><Icon name="settings" size={16} /></button>
              <button type="button" className="btn btn-ghost" onClick={onClose} style={{ padding: 4 }} aria-label="Close"><Icon name="x" size={16} /></button>
            </div>
          </div>

          {showSettings && (
            <div className="hairline" style={{ padding: '14px 20px', background: 'var(--canvas)' }}>
              <div className="eyebrow" style={{ color: 'var(--body)', marginBottom: 8 }}>Slippage Tolerance</div>
              <div style={{ display: 'flex', gap: 8 }}>
                {[0.1, 0.5, 1.0].map(s => (
                  <button type="button" key={s} onClick={() => setSlippage(s)} style={{ appearance: 'none', padding: '8px 14px', border: '1px solid var(--ink)', background: slippage === s ? 'var(--ink)' : 'transparent', color: slippage === s ? 'var(--putih)' : 'var(--ink)', fontSize: 13, fontWeight: 600, cursor: 'pointer', fontFamily: 'Inter' }}>
                    {s}%
                  </button>
                ))}
                <div style={{ flex: 1 }} />
                <input type="number" step="0.1" value={slippage} onChange={e => setSlippage(parseFloat(e.target.value) || 0)} className="input mono" style={{ width: 90, textAlign: 'right' }} />
              </div>
            </div>
          )}

          <ModalField label="You pay" token={inputToken} balance={inputBalance} amount={amount} onAmount={setTradeAmount} onSelect={() => setPickerFor('in')} />

          <div style={{ position: 'relative', height: 0 }}>
            <button onClick={flip} aria-label="Flip" style={{ position: 'absolute', left: '50%', top: -18, transform: 'translateX(-50%)', width: 36, height: 36, background: 'var(--canvas)', border: '1px solid var(--ink)', cursor: 'pointer', color: 'var(--ink)', display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 0 }}>
              <Icon name="swap" size={14} />
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

          <div style={{ padding: '4px 20px 4px' }}>
            <Accordion open={detailsOpen} onToggle={() => setDetailsOpen(open => !open)} summary={<span>{quote.rateSummary}</span>}>
              <DetailRow k="Min received" v={`${fmtNum(quote.minReceived, 4)} ${outputToken.ticker}`} hint={`${slippage}% slippage`} />
              <DetailRow k="LP fee" v={`${fmtNum(quote.inputAmount * 0.003, 4)} ${inputToken.ticker}`} hint="0.30%" />

            </Accordion>
          </div>

          <div style={{ padding: '8px 20px 20px' }}>
            {!isConnected ? (
              <ConnectButton.Custom>
                {({ openConnectModal }) => (
                  <button className="btn btn-merah" onClick={openConnectModal} style={{ width: '100%', padding: '14px 20px', fontSize: 15 }}>Connect Wallet</button>
                )}
              </ConnectButton.Custom>
            ) : (
              <button onClick={swap} disabled={cta.disabled} className="btn btn-merah" style={{ width: '100%', padding: '14px 20px', fontSize: 15 }}>
                {busy && <span style={{ display: 'inline-block', verticalAlign: '-2px', marginRight: 8 }}><Icon name="loader" size={14} /></span>}
                {cta.text}
              </button>
            )}
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
    <div className="hairline" style={{ padding: '16px 20px 14px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 10 }}>
        <span className="eyebrow" style={{ color: 'var(--body)' }}>{label}</span>
        <span className="mono" style={{ fontSize: 12, color: 'var(--body)' }}>{token.isStable ? 'Amount' : 'Holding'} {fmtNum(balance, 4)}</span>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <input
          inputMode="decimal"
          placeholder="0.00"
          value={amount}
          readOnly={readOnly}
          onChange={onAmount ? event => onAmount(event.target.value.replace(/[^0-9.]/g, '')) : undefined}
          className="mono"
          style={{ appearance: 'none', border: 0, outline: 0, flex: 1, background: 'transparent', fontSize: 32, color: 'var(--ink)', padding: 0, fontFamily: '"Fraunces", serif', letterSpacing: '-0.02em' }}
        />
        <button onClick={onSelect} style={{ appearance: 'none', display: 'flex', alignItems: 'center', gap: 8, padding: '6px 10px 6px 8px', border: '1px solid var(--ink)', background: 'var(--canvas)', cursor: 'pointer', color: 'var(--ink)', font: 'inherit' }}>
          <PStockMark ticker={token.ticker} size={22} />
          <span style={{ fontWeight: 600, fontSize: 13 }}>{token.ticker}</span>
          <Icon name="chevron-down" size={13} />
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

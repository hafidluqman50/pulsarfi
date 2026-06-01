import React, {useState} from 'react';

const ARBITRUM_SEPOLIA_CHAIN_ID = '0x66eee';
const FAUCET_ADDRESS = '0x286954bE9b8a2B52f2A61432Fa448C5287e4dDEA';
const DRIP_CALL_DATA = '0x9f678cca';

type WalletProvider = {
  request: (args: {method: string; params?: unknown[]}) => Promise<unknown>;
};

declare global {
  interface Window {
    ethereum?: WalletProvider;
  }
}

async function ensureArbitrumSepolia(ethereum: WalletProvider) {
  try {
    await ethereum.request({
      method: 'wallet_switchEthereumChain',
      params: [{chainId: ARBITRUM_SEPOLIA_CHAIN_ID}],
    });
  } catch (error) {
    const code = typeof error === 'object' && error !== null && 'code' in error ? (error as {code?: number}).code : undefined;
    if (code !== 4902) throw error;

    await ethereum.request({
      method: 'wallet_addEthereumChain',
      params: [
        {
          chainId: ARBITRUM_SEPOLIA_CHAIN_ID,
          chainName: 'Arbitrum Sepolia',
          nativeCurrency: {name: 'Ethereum', symbol: 'ETH', decimals: 18},
          rpcUrls: ['https://sepolia-rollup.arbitrum.io/rpc'],
          blockExplorerUrls: ['https://sepolia.arbiscan.io'],
        },
      ],
    });
  }
}

export default function FaucetButton(): JSX.Element {
  const [status, setStatus] = useState('Ready to mint 100,000 testnet IDRX once every 24 hours.');
  const [busy, setBusy] = useState(false);

  async function drip() {
    const ethereum = window.ethereum;
    if (!ethereum) {
      setStatus('No browser wallet found. Open this page in a browser with MetaMask or another compatible wallet.');
      return;
    }

    setBusy(true);
    try {
      const accounts = (await ethereum.request({method: 'eth_requestAccounts'})) as string[];
      const from = accounts[0];
      if (!from) throw new Error('Wallet is not connected.');

      setStatus('Checking Arbitrum Sepolia network...');
      await ensureArbitrumSepolia(ethereum);

      setStatus('Waiting for faucet transaction confirmation in your wallet...');
      const txHash = (await ethereum.request({
        method: 'eth_sendTransaction',
        params: [
          {
            from,
            to: FAUCET_ADDRESS,
            data: DRIP_CALL_DATA,
          },
        ],
      })) as string;

      setStatus(`Transaction sent: ${txHash.slice(0, 10)}...${txHash.slice(-6)}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Faucet transaction failed.';
      setStatus(message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="faucet-panel">
      <strong>Mint testnet IDRX from the faucet</strong>
      <p>
        This button calls <code>drip()</code> on the IDRXFaucet contract.
        Each wallet can claim 100,000 IDRX once every 24 hours.
      </p>
      <div className="faucet-actions">
        <button className="button button--primary" type="button" onClick={drip} disabled={busy}>
          {busy ? 'Processing...' : 'Mint 100,000 IDRX'}
        </button>
        <a
          className="button button--secondary"
          href={`https://sepolia.arbiscan.io/address/${FAUCET_ADDRESS}#writeContract`}
          target="_blank"
          rel="noreferrer"
        >
          Open in Arbiscan
        </a>
      </div>
      <div className="faucet-status">{status}</div>
    </div>
  );
}

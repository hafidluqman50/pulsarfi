'use client';

import { useState } from 'react';
import { toast } from 'sonner';
import { getWalletVerificationDocumentUrl, type WalletVerification } from '@/http/custodian/custodianApi';
import { useRecordKYC } from '@/http/custodian/contractHooks';
import { shortHash } from './utils';

const KYC_GRID = 'grid grid-cols-[1.2fr_1.4fr_1.4fr_0.8fr_1fr_1fr_1fr] gap-[14px]';
const FIELD_LABEL = 'mb-[8px] block text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]';
const FIELD_INPUT = 'input !bg-[var(--canvas)] !p-[12px] !text-[14px]';

function shortWallet(address: string): string {
  return `${address.slice(0, 6)}…${address.slice(-4)}`;
}

function dateLabel(value?: string | null): string {
  if (!value) return '—';
  return new Intl.DateTimeFormat('en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  }).format(new Date(value));
}

function EmptyRow({ text }: { text: string }): React.ReactNode {
  return (
    <div className="border-b border-[var(--hairline)] py-[18px] text-[13px] text-[var(--body)]">
      {text}
    </div>
  );
}

function AddKYCModal({ onClose }: { onClose: () => void }): React.ReactNode {
  const [wallet, setWallet] = useState('');
  const [fullName, setFullName] = useState('');
  const [email, setEmail] = useState('');
  const [type, setType] = useState<'retail' | 'institution'>('retail');
  const [document, setDocument] = useState<File | null>(null);
  const recordKYC = useRecordKYC();
  const busy = recordKYC.isPending;

  async function submit(event: React.FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const normalizedWallet = wallet.trim().toLowerCase();
    if (!/^0x[a-fA-F0-9]{40}$/.test(normalizedWallet)) {
      toast.error('Invalid wallet address');
      return;
    }
    if (!document) {
      toast.error('Signed statement PDF is required');
      return;
    }

    const toastId = toast.loading('Writing KYC on-chain…', { description: 'Sign custodian transaction in wallet' });
    try {
      await recordKYC.mutateAsync({
        walletAddress: normalizedWallet as `0x${string}`,
        type,
        fullName: fullName.trim(),
        email: email.trim(),
        document,
      });

      toast.success('KYC wallet verified', { id: toastId, description: `${shortWallet(normalizedWallet)} recorded`, duration: 4000 });
      onClose();
    } catch (error) {
      toast.error('KYC verification failed', {
        id: toastId,
        description: (error instanceof Error ? error.message : 'Unable to verify wallet').slice(0, 140),
        duration: 7000,
      });
    }
  }

  return (
    <div className="overlay fixed inset-[0] z-[9999] flex items-center justify-center bg-[rgba(22,17,14,0.32)] px-[16px]">
      <form onSubmit={submit} className="rise paper-sheaf w-full max-w-[520px]">
        <div className="sheet">
          <div className="flex items-start justify-between border-b border-[var(--hairline)] px-[24px] py-[20px]">
            <div>
              <div className="mb-[4px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">KYC registry</div>
              <div className="display !text-[20px] !font-[500] !leading-[normal]">Add verified wallet</div>
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label="Close"
              className="h-[30px] w-[30px] cursor-pointer appearance-none border border-[var(--hairline)] bg-[var(--putih)] text-[13px] text-[var(--ink)]"
            >
              ✕
            </button>
          </div>

          <div className="grid gap-[16px] px-[24px] py-[22px]">
            <label>
              <span className={FIELD_LABEL}>Wallet address</span>
              <input className={`${FIELD_INPUT} mono !font-[500]`} value={wallet} onChange={event => setWallet(event.target.value)} placeholder="0x..." />
            </label>
            <div className="grid grid-cols-[repeat(auto-fit,minmax(160px,1fr))] gap-[16px]">
              <label>
                <span className={FIELD_LABEL}>Full name</span>
                <input className={FIELD_INPUT} value={fullName} onChange={event => setFullName(event.target.value)} />
              </label>
              <label>
                <span className={FIELD_LABEL}>Email</span>
                <input className={FIELD_INPUT} type="email" value={email} onChange={event => setEmail(event.target.value)} />
              </label>
            </div>
            <div className="grid grid-cols-[repeat(auto-fit,minmax(160px,1fr))] gap-[16px]">
              <label>
                <span className={FIELD_LABEL}>Type</span>
                <select className={`${FIELD_INPUT} mono !font-[500]`} value={type} onChange={event => setType(event.target.value as 'retail' | 'institution')}>
                  <option value="retail">retail</option>
                  <option value="institution">institution</option>
                </select>
              </label>
              <label>
                <span className={FIELD_LABEL}>Signed statement</span>
                <span className="relative flex cursor-pointer items-center gap-[8px] border border-dashed border-[var(--hairline-strong)] bg-[var(--canvas-soft)] px-[12px] py-[11px] text-[12px] leading-[normal] text-[var(--body)]">
                  <span className="text-[14px]">⎙</span>
                  <span className="min-w-0 truncate">{document ? document.name : 'Drop PDF / PNG / JPG'}</span>
                  <input
                    className="absolute inset-[0] h-full w-full cursor-pointer opacity-0"
                    type="file"
                    accept="application/pdf,image/png,image/jpeg"
                    onChange={event => setDocument(event.target.files?.[0] ?? null)}
                  />
                </span>
              </label>
            </div>
          </div>

          <div className="flex justify-end gap-[10px] border-t border-[var(--hairline)] px-[24px] py-[16px]">
            <button type="button" className="cursor-pointer appearance-none border border-[var(--ink)] bg-[var(--putih)] px-[14px] py-[8px] text-[13px] font-[600] disabled:cursor-not-allowed disabled:opacity-50" onClick={onClose} disabled={busy}>Cancel</button>
            <button type="submit" className="btn btn-primary !px-[16px] !py-[9px] !text-[13px] !shadow-none" disabled={busy}>
              {busy ? 'Submitting…' : 'Record verified wallet'}
            </button>
          </div>
        </div>
      </form>
    </div>
  );
}

export function KYCRegistry({ records, isLoading }: { records: WalletVerification[]; isLoading?: boolean }): React.ReactNode {
  const [modalOpen, setModalOpen] = useState(false);

  async function openDocument(record: WalletVerification): Promise<void> {
    const toastId = toast.loading('Generating document link…');
    try {
      const url = await getWalletVerificationDocumentUrl(record.id);
      window.open(url, '_blank', 'noopener,noreferrer');
      toast.success('Document link ready', { id: toastId, description: 'Signed URL expires in 5 minutes', duration: 3500 });
    } catch (error) {
      toast.error('Document unavailable', {
        id: toastId,
        description: (error instanceof Error ? error.message : 'Unable to open document').slice(0, 120),
      });
    }
  }

  return (
    <>
      {modalOpen && <AddKYCModal onClose={() => setModalOpen(false)} />}
      <div className="mt-[14px] flex flex-wrap items-center justify-between gap-[12px]">
        <div className="text-[13px] text-[var(--body)]">
          Verified redemption access. Signed statements live in private storage.
        </div>
        <button
          className="cursor-pointer appearance-none border-0 bg-[var(--ink)] px-[14px] py-[9px] text-[13px] font-[600] text-[var(--putih)] shadow-[0_3px_0_-1px_#fbfaf7,0_4px_0_-1px_#16110e]"
          onClick={() => setModalOpen(true)}
        >
          Add verified wallet
        </button>
      </div>
      <div className="paper-stack mt-[14px] overflow-x-auto px-[clamp(12px,2vw,20px)]">
        <div className="min-w-[900px]">
          <div className={`${KYC_GRID} border-b border-[var(--ink)] py-[12px] text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]`}>
            {['Wallet', 'Name', 'Email', 'Type', 'Verified', 'Tx', ''].map((heading, columnIndex) => (
              <span key={columnIndex} className={columnIndex >= 4 ? 'text-right' : 'text-left'}>{heading}</span>
            ))}
          </div>
          {isLoading && <EmptyRow text="Loading KYC registry…" />}
          {!isLoading && records.length === 0 && <EmptyRow text="No verified wallets recorded yet" />}
          {records.map(record => (
            <div key={record.id} className={`${KYC_GRID} items-center border-b border-[var(--hairline)] py-[14px] last:border-b-0`}>
              <span className="mono text-[12px] leading-[normal]">{shortWallet(record.wallet_address)}</span>
              <span className="text-[13px] font-[600] leading-[normal]">{record.full_name ?? '—'}</span>
              <span className="text-[13px] leading-[normal] text-[var(--body)]">{record.email ?? '—'}</span>
              <span className="text-[10px] font-[600] uppercase leading-[normal] tracking-[0.14em] text-[var(--body)]">{record.type}</span>
              <span className="mono text-right text-[12px] leading-[normal] text-[var(--body)]">{dateLabel(record.verified_at)}</span>
              <span className="text-right">
                {record.approval_tx_hash
                  ? <a className="mono text-[12px] leading-[normal] text-[var(--body)] underline-offset-[3px] hover:underline" href={`https://sepolia.arbiscan.io/tx/${record.approval_tx_hash}`} target="_blank" rel="noopener noreferrer">{shortHash(record.approval_tx_hash)}</a>
                  : <span className="mono text-[12px] leading-[normal] text-[var(--body)]">—</span>}
              </span>
              <span className="flex justify-end">
                <button
                  className="cursor-pointer appearance-none border border-[var(--ink)] bg-[var(--putih)] px-[12px] py-[6px] text-[12px] font-[600]"
                  onClick={() => openDocument(record)}
                >
                  View statement
                </button>
              </span>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}

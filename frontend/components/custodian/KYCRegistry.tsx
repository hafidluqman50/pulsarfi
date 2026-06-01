'use client';

import { useState } from 'react';
import { toast } from 'sonner';
import { getWalletVerificationDocumentUrl, type WalletVerification } from '@/http/custodian/custodianApi';
import { useRecordKYC } from '@/http/custodian/contractHooks';
import { shortHash } from './utils';

function shortWallet(address: string): string {
  return `${address.slice(0, 6)}…${address.slice(-4)}`;
}

function dateLabel(value?: string | null): string {
  if (!value) return '—';
  return new Intl.DateTimeFormat('en', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  }).format(new Date(value));
}

function EmptyRow({ text }: { text: string }): React.ReactNode {
  return (
    <div className="hairline py-[18px] text-[13px] text-[var(--body)]">
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
    <div className="fixed inset-[0] z-[9999] flex items-center justify-center bg-[rgba(0,0,0,0.55)] px-[16px] backdrop-blur-[2px]">
      <form onSubmit={submit} className="w-full max-w-[520px] bg-[var(--putih)] shadow-[0_32px_80px_rgba(0,0,0,0.28)]">
        <div className="flex items-start justify-between border-b border-[var(--hairline)] px-[24px] py-[20px]">
          <div>
            <div className="eyebrow mb-[4px] !text-[var(--body)]">KYC registry</div>
            <div className="text-[20px] font-semibold text-[var(--ink)] [font-family:var(--font-fraunces,_Fraunces,_serif)]">Add verified wallet</div>
          </div>
          <button type="button" onClick={onClose} className="mt-[2px] cursor-pointer border-0 bg-transparent pb-[0] pl-[16px] pr-[0] pt-[0] text-[22px] leading-none text-[var(--body)]">&times;</button>
        </div>

        <div className="grid gap-[16px] px-[24px] py-[22px]">
          <label>
            <span className="eyebrow mb-[8px] block !text-[var(--body)]">Wallet address</span>
            <input className="input mono" value={wallet} onChange={event => setWallet(event.target.value)} placeholder="0x..." />
          </label>
          <div className="grid grid-cols-1 gap-[16px] md:grid-cols-2">
            <label>
              <span className="eyebrow mb-[8px] block !text-[var(--body)]">Full name</span>
              <input className="input" value={fullName} onChange={event => setFullName(event.target.value)} />
            </label>
            <label>
              <span className="eyebrow mb-[8px] block !text-[var(--body)]">Email</span>
              <input className="input" type="email" value={email} onChange={event => setEmail(event.target.value)} />
            </label>
          </div>
          <div className="grid grid-cols-1 gap-[16px] md:grid-cols-2">
            <label>
              <span className="eyebrow mb-[8px] block !text-[var(--body)]">Type</span>
              <select className="input mono" value={type} onChange={event => setType(event.target.value as 'retail' | 'institution')}>
                <option value="retail">retail</option>
                <option value="institution">institution</option>
              </select>
            </label>
            <label>
              <span className="eyebrow mb-[8px] block !text-[var(--body)]">Signed statement</span>
              <input
                className="block w-full cursor-pointer border border-[var(--hairline-strong)] bg-[var(--canvas)] px-[12px] py-[11px] text-[12px] text-[var(--body)] [font-family:var(--font-inter,_Inter,_sans-serif)]"
                type="file"
                accept="application/pdf,image/png,image/jpeg"
                onChange={event => setDocument(event.target.files?.[0] ?? null)}
              />
            </label>
          </div>
        </div>

        <div className="flex justify-end gap-[10px] border-t border-[var(--hairline)] px-[24px] py-[16px]">
          <button type="button" className="btn btn-ghost !border !border-[var(--ink)] !px-[14px] !py-[8px]" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="submit" className="btn btn-primary !px-[16px] !py-[8px]" disabled={busy}>
            {busy ? 'Submitting…' : 'Record verified wallet'}
          </button>
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
      <div className="mb-[14px] flex flex-wrap items-center justify-between gap-[12px]">
        <div className="text-[13px] text-[var(--body)]">
          Verified redemption access. Signed statements live in private storage.
        </div>
        <button className="btn btn-primary !px-[14px] !py-[8px]" onClick={() => setModalOpen(true)}>
          Add verified wallet
        </button>
      </div>
      <div className="kyc-table">
        <div className="hairline table-head-desktop grid grid-cols-[1.2fr_1.4fr_1.4fr_0.8fr_1fr_1fr_1fr] gap-[14px] py-[12px]">
          {['Wallet', 'Name', 'Email', 'Type', 'Verified', 'Tx', ''].map((heading, columnIndex) => (
            <div key={heading} className={`eyebrow !text-[var(--body)] ${columnIndex >= 4 ? 'text-right' : 'text-left'}`}>{heading}</div>
          ))}
        </div>
        {isLoading && <EmptyRow text="Loading KYC registry…" />}
        {!isLoading && records.length === 0 && <EmptyRow text="No verified wallets recorded yet" />}
        {records.map(record => (
          <div key={record.id} className="hairline table-row-stack grid grid-cols-[1.2fr_1.4fr_1.4fr_0.8fr_1fr_1fr_1fr] items-center gap-[14px] py-[16px]">
            <div className="row-cell"><span className="row-cell-label">Wallet</span><div className="row-cell-value"><span className="mono text-[12px]">{shortWallet(record.wallet_address)}</span></div></div>
            <div className="row-cell"><span className="row-cell-label">Name</span><div className="row-cell-value"><span className="text-[13px] font-semibold">{record.full_name ?? '—'}</span></div></div>
            <div className="row-cell"><span className="row-cell-label">Email</span><div className="row-cell-value"><span className="text-[13px] text-[var(--body)]">{record.email ?? '—'}</span></div></div>
            <div className="row-cell"><span className="row-cell-label">Type</span><div className="row-cell-value"><span className="eyebrow !text-[var(--body)]">{record.type}</span></div></div>
            <div className="row-cell text-right"><span className="row-cell-label">Verified</span><div className="row-cell-value"><span className="mono text-[12px] text-[var(--body)]">{dateLabel(record.verified_at)}</span></div></div>
            <div className="row-cell text-right"><span className="row-cell-label">Tx</span><div className="row-cell-value">{record.approval_tx_hash ? <a className="mono text-[12px] text-[var(--body)] underline-offset-[3px] hover:underline" href={`https://sepolia.arbiscan.io/tx/${record.approval_tx_hash}`} target="_blank" rel="noopener noreferrer">{shortHash(record.approval_tx_hash)}</a> : <span className="mono text-[12px] text-[var(--body)]">—</span>}</div></div>
            <div className="col-actions flex justify-end">
              <button className="btn btn-ghost !border !border-[var(--ink)] !px-[12px] !py-[6px]" onClick={() => openDocument(record)}>
                View statement
              </button>
            </div>
          </div>
        ))}
      </div>
    </>
  );
}

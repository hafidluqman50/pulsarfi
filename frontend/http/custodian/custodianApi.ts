import client from '@/http/client';

export interface CustodianStats {
  assets_under_custody_idr: string;
  mint_volume_24h_idrx: string;
  mint_count_24h: number;
  burn_count_24h: number;
  pending_requests: {
    total: number;
    mints: number;
    redeems: number;
  };
}

export interface AttestorInfo {
  name: string;
  wallet_address: string;
  type: 'approve' | 'reject';
  tx_hash?: string;
  attested_at: string | null;
}

export interface CustodianRequest {
  id: number;
  on_chain_id: number;
  kind: 'mint' | 'redeem';
  ticker: string;
  stock_name: string;
  idx_ticker: string;
  token_amount: string;
  idrx_amount?: string;
  fee_idrx?: string;
  user_address?: string;
  requester_address?: string;
  approve_initiator_address?: string;
  reject_initiator_address?: string;
  source: 'retail' | 'institutional';
  destination?: 'operator_wallet' | 'liquidity_pool';
  approval_count: number;
  reject_count: number;
  status: string;
  request_tx_hash?: string;
  attestors?: AttestorInfo[];
  created_at: string;
}

export interface CustodianRequests {
  items: CustodianRequest[];
}

export interface ReserveStock {
  id: number;
  ticker: string;
  stock_name: string;
  idx_ticker: string;
  sector?: string | null;
  contract_address?: string | null;
}

export interface ReserveEntry {
  stock: ReserveStock;
  custodian_holdings: string;
  on_chain_supply: string;
  peg_ratio: string;
  peg_status: 'pegged' | 'depegged' | 'unknown';
  last_attested_at: string;
  attestation_hash?: string | null;
}

export interface WalletVerification {
  id: number;
  wallet_address: string;
  type: 'retail' | 'institution';
  status: 'pending' | 'approved' | 'rejected';
  full_name?: string | null;
  email?: string | null;
  document_ref?: string | null;
  approval_tx_hash?: string | null;
  submitted_at: string;
  verified_at?: string | null;
  verified_by?: number | null;
}

export interface CustodianStock {
  id: number;
  ticker: string;
  stock_name: string;
  idx_ticker: string;
  sector?: string | null;
  contract_address?: string | null;
}

interface ApiResponse<T> {
  status_code: number;
  message: string;
  data: T;
}

export async function getCustodianStats(): Promise<CustodianStats> {
  const { data } = await client.get<ApiResponse<CustodianStats>>('/custodian/stats');
  return data.data;
}

export async function getCustodianRequests(): Promise<CustodianRequests> {
  const { data } = await client.get<ApiResponse<CustodianRequests>>('/custodian/requests');
  return data.data;
}

export async function getCustodianStocks(): Promise<CustodianStock[]> {
  const { data } = await client.get<ApiResponse<CustodianStock[]>>('/custodian/stocks');
  return Array.isArray(data.data) ? data.data : [];
}

export async function getReserves(): Promise<ReserveEntry[]> {
  const { data } = await client.get<ApiResponse<ReserveEntry[]>>('/public/reserves');
  return data.data;
}

export async function getWalletVerifications(): Promise<WalletVerification[]> {
  const { data } = await client.get<ApiResponse<WalletVerification[]>>('/custodian/wallet-verifications');
  return data.data;
}

export async function createWalletVerification(input: FormData): Promise<WalletVerification> {
  const { data } = await client.post<ApiResponse<WalletVerification>>('/custodian/wallet-verifications', input, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
  return data.data;
}

export async function getWalletVerificationDocumentUrl(id: number): Promise<string> {
  const { data } = await client.get<ApiResponse<{ url: string; expires_in: number }>>(`/custodian/wallet-verifications/${id}/document-url`);
  return data.data.url;
}

export async function recordMintRequest(input: {
  on_chain_id: number;
  ticker: string;
  token_amount: string;
  idrx_amount: string;
  attestation_hash: string;
  destination: 'operator_wallet' | 'liquidity_pool';
  tx_hash: string;
}) {
  await client.post('/custodian/mint-proposals', input);
}

export async function recordMintApproval(onChainId: number, txHash: string) {
  await client.post('/custodian/mint-proposals/approve', { on_chain_id: onChainId, tx_hash: txHash });
}

export async function recordMintRejection(onChainId: number, txHash: string) {
  await client.post('/custodian/mint-proposals/reject', { on_chain_id: onChainId, tx_hash: txHash });
}

export async function recordMintExecution(onChainId: number, txHash: string, contractAddress: string) {
  await client.post('/custodian/mint-proposals/execute', { on_chain_id: onChainId, tx_hash: txHash, contract_address: contractAddress });
}

export async function recordMintRejectExecution(onChainId: number, txHash: string) {
  await client.post('/custodian/mint-proposals/execute-reject', { on_chain_id: onChainId, tx_hash: txHash, contract_address: '' });
}

export async function recordRedeemApproval(onChainId: number, txHash: string) {
  await client.post('/custodian/redeem-proposals/approve', { on_chain_id: onChainId, tx_hash: txHash });
}

export async function recordRedeemRejection(onChainId: number, txHash: string) {
  await client.post('/custodian/redeem-proposals/reject', { on_chain_id: onChainId, tx_hash: txHash });
}

export async function recordRedeemExecution(onChainId: number, txHash: string, blockNumber: number) {
  await client.post('/custodian/redeem-proposals/execute', { on_chain_id: onChainId, tx_hash: txHash, block_number: blockNumber });
}

export async function recordRedeemRejectionExecution(onChainId: number, txHash: string, blockNumber: number) {
  await client.post('/custodian/redeem-proposals/execute-reject', { on_chain_id: onChainId, tx_hash: txHash, block_number: blockNumber });
}

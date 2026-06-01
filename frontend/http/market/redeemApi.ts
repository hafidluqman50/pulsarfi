import client from '@/http/client';

export interface RecordRedeemInput {
  on_chain_id: number;
  ticker: string;
  token_amount: string;
  fee_idrx: string;
  user_address: string;
  attestation_hash: string;
  tx_hash: string;
  block_number: number;
}

export async function recordRedeemRequest(input: RecordRedeemInput): Promise<void> {
  await client.post('/public/redeem-requests', input);
}

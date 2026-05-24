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

interface ApiResponse<T> {
  status_code: number;
  message: string;
  data: T;
}

export async function getCustodianStats(): Promise<CustodianStats> {
  const { data } = await client.get<ApiResponse<CustodianStats>>('/custodian/stats');
  return data.data;
}

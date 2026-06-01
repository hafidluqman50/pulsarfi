import client from '@/http/client';

export interface ProtocolStats {
  volume_24h:   number;
  tvl_idrx:     number;
  pair_count:   number;
  idr_usd_rate: number;
}

export async function getProtocolStats(): Promise<ProtocolStats> {
  const res = await client.get('/public/stats');
  return res.data.data;
}

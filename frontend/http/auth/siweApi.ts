import client from '@/http/client';

interface NonceResponse {
  status_code: number;
  message: string;
  data: { nonce: string };
}

interface VerifyResponse {
  status_code: number;
  message: string;
  data: { access_token: string };
}

export async function fetchNonce(address: string): Promise<string> {
  const { data } = await client.get<NonceResponse>('/auth/nonce', {
    params: { address },
  });
  return data.data.nonce;
}

export function buildSiweMessage(address: string, nonce: string): string {
  const domain = window.location.host;
  const issuedAt = new Date().toISOString();
  return [
    `${domain} wants you to sign in with your Ethereum account:`,
    address,
    '',
    'Sign in to PulsarFi',
    '',
    `URI: ${window.location.origin}`,
    'Version: 1',
    'Chain ID: 421614',
    `Nonce: ${nonce}`,
    `Issued At: ${issuedAt}`,
  ].join('\n');
}

export async function verifySignature(
  address: string,
  message: string,
  signature: string,
  nonce: string,
): Promise<string> {
  const { data } = await client.post<VerifyResponse>('/auth/verify', {
    address,
    message,
    signature,
    nonce,
  });
  return data.data.access_token;
}

import axios from 'axios';

console.log(process.env.NEXT_PUBLIC_BACKEND_URL)

const client = axios.create({
  baseURL: `${process.env.NEXT_PUBLIC_BACKEND_URL}/api/v1`,
  headers: { 'Content-Type': 'application/json' },
});

client.interceptors.request.use((config) => {
  const token = typeof window !== 'undefined' ? localStorage.getItem('access_token') : null;
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

export default client;

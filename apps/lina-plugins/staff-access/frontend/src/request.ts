import type { PluginHostMountContext } from './host-context';

type Envelope<T> = { code?: number | string; data?: T };
export function createRequest(current: () => PluginHostMountContext) {
  return async <T>(path: string, init: RequestInit & { body?: string } = {}): Promise<T> => {
    const response = await fetch(new URL(path, location.origin), { ...init, credentials: 'same-origin', headers: { Accept: 'application/json', ...(init.body ? { 'Content-Type': 'application/json' } : {}), ...(current().accessToken ? { Authorization: `Bearer ${current().accessToken}` } : {}), ...init.headers } });
    const payload = await response.json() as Envelope<T> | T;
    if (!response.ok) throw new Error('request_failed');
    if (typeof payload === 'object' && payload !== null && 'data' in payload) {
      if (payload.data === undefined || (payload.code !== undefined && String(payload.code) !== '0')) throw new Error('request_failed');
      return payload.data;
    }
    return payload as T;
  };
}

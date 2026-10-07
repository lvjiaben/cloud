import type { PluginHostMountContext } from '../host-context';

type ResponseEnvelope<T> = {
  code?: number | string;
  data?: T;
  message?: string;
};

export class PluginRequestError extends Error {
  constructor(
    readonly status: number,
    readonly code = '',
  ) {
    super(`Plugin request failed (${status}).`);
    this.name = 'PluginRequestError';
  }
}

export type PluginRequestOptions = Omit<RequestInit, 'body'> & {
  body?: unknown;
  timeoutMs?: number;
};

export function createPluginRequest(
  currentContext: () => PluginHostMountContext,
) {
  return async function request<T>(
    path: string,
    options: PluginRequestOptions = {},
  ): Promise<T> {
    const controller = new AbortController();
    const timeout = globalThis.setTimeout(
      () => controller.abort(),
      options.timeoutMs ?? 20_000,
    );
    const context = currentContext();
    const token = context.accessToken.trim();
    const { body, headers, timeoutMs: _timeoutMs, ...init } = options;

    try {
      const response = await fetch(new URL(path, globalThis.location.origin), {
        ...init,
        body: body === undefined ? undefined : JSON.stringify(body),
        credentials: 'same-origin',
        headers: {
          Accept: 'application/json',
          ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          ...headers,
        },
        signal: controller.signal,
      });

      let payload: ResponseEnvelope<T> | T | undefined;
      try {
        payload = (await response.json()) as ResponseEnvelope<T> | T;
      } catch {
        payload = undefined;
      }

      const envelope = payload as ResponseEnvelope<T> | undefined;
      const code =
        envelope && typeof envelope === 'object' && 'code' in envelope
          ? String(envelope.code ?? '')
          : '';
      if (!response.ok) {
        throw new PluginRequestError(response.status, code);
      }
      if (envelope && typeof envelope === 'object' && 'data' in envelope) {
        if (
          envelope.data === undefined ||
          (envelope.code !== undefined && String(envelope.code) !== '0')
        ) {
          throw new PluginRequestError(response.status, code);
        }
        return envelope.data;
      }
      if (payload === undefined) {
        throw new PluginRequestError(response.status, code);
      }
      return payload as T;
    } catch (error) {
      if (error instanceof PluginRequestError) {
        throw error;
      }
      if (controller.signal.aborted) {
        throw new PluginRequestError(408, 'timeout');
      }
      throw new PluginRequestError(0, 'network');
    } finally {
      globalThis.clearTimeout(timeout);
    }
  };
}

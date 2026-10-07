import type { AxiosAdapter, InternalAxiosRequestConfig } from '@vben/request';

import {
  authenticateResponseInterceptor,
  AxiosError,
  AxiosHeaders,
  defaultResponseInterceptor,
  RequestClient,
} from '@vben/request';
import { describe, expect, it, vi } from 'vitest';

import { requestWithHttpStatus } from './request-with-http-status';

type Reply = { status: number; body: unknown } | Error;

function httpReply(config: InternalAxiosRequestConfig, reply: Reply) {
  if (reply instanceof Error) return Promise.reject(reply);
  const response = {
    config,
    data: JSON.stringify(reply.body),
    headers: new AxiosHeaders(),
    status: reply.status,
    statusText: String(reply.status),
  };
  return reply.status >= 400
    ? Promise.reject(
        new AxiosError(
          'HTTP request failed',
          undefined,
          config,
          undefined,
          response,
        ),
      )
    : Promise.resolve(response);
}

function makeClient(replies: Reply[], refresh = async () => 'next-token') {
  let accessToken = 'initial-token';
  const calls: InternalAxiosRequestConfig[] = [];
  const adapter: AxiosAdapter = (config) => {
    calls.push(config);
    const reply = replies.shift();
    if (!reply) throw new Error('Unexpected request');
    return httpReply(config, reply);
  };
  const client = new RequestClient({ adapter, responseReturn: 'data' });
  const doReAuthenticate = vi.fn(async () => undefined);
  client.addRequestInterceptor({
    fulfilled(config) {
      config.headers.Authorization = `Bearer ${accessToken}`;
      config.headers['X-Tenant-Code'] = 'customer-tenant';
      config.headers['Accept-Language'] = 'zh-CN';
      return config;
    },
  });
  client.addResponseInterceptor(
    defaultResponseInterceptor({
      codeField: 'code',
      dataField: 'data',
      successCode: 0,
    }),
  );
  client.addResponseInterceptor(
    authenticateResponseInterceptor({
      client,
      doReAuthenticate,
      doRefreshToken: async () => {
        accessToken = await refresh();
        return accessToken;
      },
      enableRefreshToken: true,
      formatToken: (token) => (token ? `Bearer ${token}` : null),
    }),
  );
  return { client, calls, doReAuthenticate };
}

describe('customer request transport status', () => {
  it('restores first-response 403 without reading a misleading business code', async () => {
    const payload = { code: 12345, messageKey: 'original.error' };
    const { client } = makeClient([{ status: 403, body: payload }]);
    await expect(
      requestWithHttpStatus(client, '/credit'),
    ).rejects.toMatchObject({
      cause: payload,
      response: { status: 403, data: payload },
    });
  });

  it('preserves 403 after the real official token-refresh replay', async () => {
    const refresh = vi.fn(async () => 'next-token');
    const { client, calls } = makeClient(
      [
        { status: 401, body: { code: 9001 } },
        { status: 403, body: { code: 9002 } },
      ],
      refresh,
    );
    await expect(
      requestWithHttpStatus(client, '/credit'),
    ).rejects.toMatchObject({
      response: { status: 403, data: { code: 9002 } },
    });
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(calls).toHaveLength(2);
    expect(calls[1]?.headers.Authorization).toBe('Bearer next-token');
  });

  it('does not reuse 401 when the refreshed attempt fails without a response', async () => {
    const networkFailure = new AxiosError('Network Error', 'ERR_NETWORK');
    const { client } = makeClient([
      { status: 401, body: { code: 401 } },
      networkFailure,
    ]);
    await expect(requestWithHttpStatus(client, '/credit')).rejects.toBe(
      networkFailure,
    );
    expect(networkFailure.response).toBeUndefined();
  });

  it('preserves 503 instead of misclassifying its payload as an access denial', async () => {
    const { client } = makeClient([{ status: 503, body: { code: 403 } }]);
    await expect(
      requestWithHttpStatus(client, '/credit'),
    ).rejects.toMatchObject({
      response: { status: 503, data: { code: 403 } },
    });
  });

  it('returns already-unwrapped data unchanged and retains transforms and request headers', async () => {
    const data = { data: { nested: true }, total: 123 };
    const { client, calls } = makeClient([
      { status: 200, body: { code: 0, data } },
    ]);
    await expect(
      requestWithHttpStatus(client, '/credit', {
        method: 'POST',
        data: { amount: '100' },
        params: { page: 2 },
      }),
    ).resolves.toEqual(data);
    expect(calls[0]?.data).toBe('{"amount":"100"}');
    expect(calls[0]?.params).toEqual({ page: 2 });
    expect(calls[0]?.headers['Accept-Language']).toBe('zh-CN');
    expect(calls[0]?.headers['X-Tenant-Code']).toBe('customer-tenant');
  });

  it('does not turn a 200 business rejection into an HTTP 403', async () => {
    const { client } = makeClient([{ status: 200, body: { code: 403 } }]);
    await expect(
      requestWithHttpStatus(client, '/credit'),
    ).rejects.toMatchObject({
      response: { status: 200 },
    });
  });

  it('keeps the second 401 authoritative when refresh succeeds but authentication is rejected', async () => {
    const { client, calls, doReAuthenticate } = makeClient([
      { status: 401, body: { code: 'expired' } },
      { status: 401, body: { code: 'revoked' } },
    ]);
    await expect(
      requestWithHttpStatus(client, '/credit'),
    ).rejects.toMatchObject({
      response: { status: 401, data: { code: 'revoked' } },
    });
    expect(calls).toHaveLength(2);
    expect(doReAuthenticate).toHaveBeenCalledTimes(1);
  });

  it('does not modify a caller-provided transform array', async () => {
    const { client } = makeClient([
      { status: 200, body: { code: 0, data: 'ok' } },
    ]);
    const transformResponse = [vi.fn((text: string) => JSON.parse(text))];
    await expect(
      requestWithHttpStatus(client, '/credit', { transformResponse }),
    ).resolves.toBe('ok');
    expect(transformResponse).toHaveLength(1);
    expect(transformResponse[0]).toHaveBeenCalledTimes(1);
  });

  it('keeps capture state local to concurrent requests sharing the client', async () => {
    const { client } = makeClient([]);
    let finishDenied!: () => void;
    client.instance.defaults.adapter = (config) =>
      config.url === '/denied'
        ? new Promise((resolve, reject) => {
            finishDenied = () =>
              httpReply(config, { status: 403, body: { code: 'denied' } }).then(
                resolve,
                reject,
              );
          })
        : httpReply(config, { status: 503, body: { code: 'unavailable' } });
    const denied = requestWithHttpStatus(client, '/denied');
    const deniedCheck = expect(denied).rejects.toMatchObject({
      response: { status: 403 },
    });
    await expect(
      requestWithHttpStatus(client, '/unavailable'),
    ).rejects.toMatchObject({ response: { status: 503 } });
    finishDenied();
    await deniedCheck;
  });
});

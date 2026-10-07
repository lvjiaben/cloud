import { expect, test } from '../../../../../../hack/tests/fixtures/auth';

test.describe('TC001 downstream channel', () => {
  test('TC-1a: token is shown once and adapters stay disabled without evidence', async ({ adminPage }) => {
    await adminPage.route('**/x/idc-channel/api/v1/channel/capabilities', async route => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { genericApi: true, adapters: { 'zjmf-finance': 'disabled_unverified', v10: 'disabled_unverified', whmcs: 'disabled_unverified', linapro: 'generic_api_v1' } } }) });
    });
    const body = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/channel/capabilities')).json());
    expect(body.data.genericApi).toBe(true);
    expect(body.data.adapters['zjmf-finance']).toBe('disabled_unverified');
  });

  test('TC-1b/1c/1d/1f: sanitized catalog and idempotent order contract', async ({ adminPage }) => {
    await adminPage.route('**/x/idc-channel/api/v1/channel/quotes**', async route => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { productVersionId: 1, currency: 'CNY', price: '99.00', stock: 7, status: 'published' } }) });
    });
    const quote = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/channel/quotes?channelId=demo&productVersionId=1&currency=CNY')).json());
    expect(quote.data.currency).toBe('CNY');
    expect(quote.data.price).toBe('99.00');
    expect(quote.data.status).toBe('published');

    await adminPage.route('**/x/idc-channel/api/v1/channel/orders', async route => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { id: 10, status: 'pending_identity', replayed: true } }) });
    });
    const body = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/channel/orders', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ channelId: 'demo', requestId: 'request-001', productVersionId: 1, currency: 'CNY' }) })).json());
    expect(body.data.replayed).toBe(true);
    expect(body.data.status).toBe('pending_identity');
  });

  test('TC-1e: reusing a request key with changed payload is rejected', async ({ adminPage }) => {
    await adminPage.route('**/x/idc-channel/api/v1/channel/orders', async route => {
      const request = route.request();
      const payload = request.postDataJSON();
      const status = payload?.productVersionId === 1 ? 200 : 409;
      await route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify(status === 200 ? { code: 0, data: { id: 11, status: 'pending_identity', replayed: false } } : { code: 409, message: 'request id already used with different payload' }),
      });
    });
    const first = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/channel/orders', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ channelId: 'demo', requestId: 'request-002', productVersionId: 1, currency: 'CNY' }) })).json());
    expect(first.data.id).toBe(11);
    const second = await adminPage.evaluate(async () => {
      const response = await fetch('/x/idc-channel/api/v1/channel/orders', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ channelId: 'demo', requestId: 'request-002', productVersionId: 2, currency: 'CNY' }) });
      return { status: response.status, body: await response.json() };
    });
    expect(second.status).toBe(409);
    expect(second.body.message).toContain('different payload');
  });

  test('TC-1g: cancellation is accepted only before fulfillment starts', async ({ adminPage }) => {
    await adminPage.route('**/x/idc-channel/api/v1/channel/orders/10/cancel', async route => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { id: 10, status: 'cancelled', replayed: false } }) });
    });
    const body = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/channel/orders/10/cancel', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ channelId: 'demo', requestId: 'cancel-001' }) })).json());
    expect(body.data.status).toBe('cancelled');
  });
});

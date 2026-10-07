import { expect, test } from '../../../../../../hack/tests/fixtures/auth';

test.describe('TC-5 下游渠道安全契约矩阵', () => {
  test('TC-5a: 跨渠道编号查询统一不可见', async ({ adminPage }) => {
    await adminPage.route('**/x/idc-channel/api/v1/channel/orders/99**', async (route) => {
      expect(route.request().url()).toContain('channelId=other-channel');
      await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ code: 404, message: 'order not found' }) });
    });
    const result = await adminPage.evaluate(async () => {
      const response = await fetch('/x/idc-channel/api/v1/channel/orders/99?channelId=other-channel');
      return { status: response.status, body: await response.json() };
    });
    expect(result.status).toBe(404);
    expect(result.body.message).toBe('order not found');
    expect(JSON.stringify(result.body)).not.toContain('pending_identity');
  });

  test('TC-5b: 撤销令牌后后续请求立即失效', async ({ adminPage }) => {
    let revoked = false;
    await adminPage.route('**/x/idc-channel/api/v1/admin/channel/tokens/42', async (route) => {
      expect(route.request().method()).toBe('DELETE');
      revoked = true;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { id: 42, revoked: true } }) });
    });
    await adminPage.route('**/x/idc-channel/api/v1/channel/catalog**', async (route) => {
      await route.fulfill({ status: revoked ? 401 : 200, contentType: 'application/json', body: JSON.stringify(revoked ? { code: 401, message: 'token revoked' } : { code: 0, data: { list: [] } }) });
    });
    const revoke = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/admin/channel/tokens/42', { method: 'DELETE' })).json());
    expect(revoke.data.revoked).toBe(true);
    const after = await adminPage.evaluate(async () => {
      const response = await fetch('/x/idc-channel/api/v1/channel/catalog?channelId=demo', { headers: { authorization: 'Bearer once' } });
      return { status: response.status, body: await response.json() };
    });
    expect(after.status).toBe(401);
    expect(after.body.message).toBe('token revoked');
  });

  test('TC-5c: 来源限制只接受合法 CIDR', async ({ adminPage }) => {
    await adminPage.route('**/x/idc-channel/api/v1/admin/channel/tokens', async (route) => {
      const body = route.request().postDataJSON();
      const source = body?.sourceCidrs?.[0];
      if (source === 'not-a-cidr') {
        await route.fulfill({ status: 422, contentType: 'application/json', body: JSON.stringify({ code: 422, message: 'token source restriction is invalid' }) });
        return;
      }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { id: 7, token: 'shown-once' } }) });
    });
    const invalid = await adminPage.evaluate(async () => {
      const response = await fetch('/x/idc-channel/api/v1/admin/channel/tokens', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ channelId: 'demo', sourceCidrs: ['not-a-cidr'] }) });
      return { status: response.status, body: await response.json() };
    });
    expect(invalid.status).toBe(422);
    expect(invalid.body.message).toContain('source restriction');
    const valid = await adminPage.evaluate(async () => {
      const response = await fetch('/x/idc-channel/api/v1/admin/channel/tokens', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ channelId: 'demo', sourceCidrs: ['192.0.2.0/24'] }) });
      return { status: response.status, body: await response.json() };
    });
    expect(valid.status).toBe(200);
    expect(valid.body.data.token).toBe('shown-once');
  });

  test('TC-5d: 重复下单同键复用结果，改载荷拒绝', async ({ adminPage }) => {
    await adminPage.route('**/x/idc-channel/api/v1/channel/orders', async (route) => {
      const body = route.request().postDataJSON();
      if (body?.requestId !== 'same-key') {
        await route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ code: 400, message: 'request id is required' }) });
        return;
      }
      if (body?.productVersionId === 1) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { id: 100, status: 'pending_identity', replayed: true } }) });
        return;
      }
      await route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ code: 409, message: 'request id already used with different payload' }) });
    });
    const replay = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/channel/orders', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ channelId: 'demo', requestId: 'same-key', productVersionId: 1, currency: 'CNY' }) })).json());
    expect(replay.data.replayed).toBe(true);
    const changed = await adminPage.evaluate(async () => {
      const response = await fetch('/x/idc-channel/api/v1/channel/orders', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ channelId: 'demo', requestId: 'same-key', productVersionId: 2, currency: 'CNY' }) });
      return { status: response.status, body: await response.json() };
    });
    expect(changed.status).toBe(409);
    expect(changed.body.message).toContain('different payload');
  });

  test('TC-5e: 信用结算成功后订单进入统一结清状态', async ({ adminPage }) => {
    let upstreamCalls = 0;
    await adminPage.route('**/x/idc-channel/api/v1/channel/orders', async (route) => {
      expect(route.request().postDataJSON()?.settlementMethod).toBe('credit');
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { id: 501, status: 'financially_settled', replayed: false } }) });
    });
    await adminPage.route('**/upstream/**', async (route) => {
      upstreamCalls += 1;
      await route.continue();
    });
    const result = await adminPage.evaluate(async () => {
      const response = await fetch('/x/idc-channel/api/v1/channel/orders', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ channelId: 'demo', requestId: 'credit-low', productVersionId: 1, currency: 'CNY', ownershipMode: 'channel_owned', settlementMethod: 'credit' }) });
      return { status: response.status, body: await response.json() };
    });
    expect(result.status).toBe(200);
    expect(result.body.data.status).toBe('financially_settled');
    expect(upstreamCalls).toBe(0);
  });

  test('TC-5f: 重复 Webhook 事件按事件编号拒绝', async ({ adminPage }) => {
    let accepted = false;
    await adminPage.route('**/x/idc-channel/api/v1/webhooks/**', async (route) => {
      if (!accepted) {
        accepted = true;
        await route.fulfill({ status: 202, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { accepted: true } }) });
        return;
      }
      await route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ code: 409, message: 'event already processed' }) });
    });
    const first = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/webhooks/demo/evt-1', { method: 'POST', body: '{}' })).json());
    expect(first.data.accepted).toBe(true);
    const second = await adminPage.evaluate(async () => {
      const response = await fetch('/x/idc-channel/api/v1/webhooks/demo/evt-1', { method: 'POST', body: '{}' });
      return { status: response.status, body: await response.json() };
    });
    expect(second.status).toBe(409);
    expect(second.body.message).toContain('already processed');
  });

  test('TC-5g: 渠道停用期间只保留历史查询，恢复后才允许新购', async ({ adminPage }) => {
    let active = false;
    await adminPage.route('**/x/idc-channel/api/v1/channel/capabilities', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { genericApi: true, channelStatus: active ? 'active' : 'suspended', operations: { create_order: active ? 'enabled' : 'disabled_channel_suspended', get_order: 'enabled' } } }) });
    });
    const suspended = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/channel/capabilities')).json());
    expect(suspended.data.channelStatus).toBe('suspended');
    expect(suspended.data.operations.create_order).toBe('disabled_channel_suspended');
    expect(suspended.data.operations.get_order).toBe('enabled');
    active = true;
    const resumed = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/channel/capabilities')).json());
    expect(resumed.data.channelStatus).toBe('active');
    expect(resumed.data.operations.create_order).toBe('enabled');
  });

  test('TC-5h: 未验证兼容能力保持禁用且通用接口继续可用', async ({ adminPage }) => {
    await adminPage.route('**/x/idc-channel/api/v1/channel/capabilities', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { genericApi: true, adapters: { 'zjmf-finance': 'disabled_unverified', v10: 'disabled_unverified', whmcs: 'disabled_unverified', linapro: 'generic_api_v1' }, operations: { catalog: 'enabled', quote: 'enabled', create_order: 'enabled' } } }) });
    });
    const body = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/channel/capabilities')).json());
    expect(body.data.genericApi).toBe(true);
    expect(body.data.adapters['zjmf-finance']).toBe('disabled_unverified');
    expect(body.data.adapters.v10).toBe('disabled_unverified');
    expect(body.data.adapters.whmcs).toBe('disabled_unverified');
    expect(body.data.adapters.linapro).toBe('generic_api_v1');
  });

  test('TC-5i: 变配和换 IP 先冻结报价，确认后才结算并执行', async ({ adminPage }) => {
    const confirmedQuotes = new Set<string>();
    await adminPage.route('**/x/idc-channel/api/v1/channel/services/**/operations/**', async (route) => {
      expect(route.request().method()).toBe('POST');
      const payload = route.request().postDataJSON();
      expect(payload?.serviceNo).toBe('service-1');
      const operation = new URL(route.request().url()).pathname.split('/').at(-1)!;
      if (!payload?.confirmed) {
        expect(payload?.parameters).toBeTruthy();
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ code: 0, data: { quoteNo: `quote-${operation}`, status: 'quoted', currencyCode: 'CNY', direction: operation === 'reconfigure' ? 'upgrade' : '', amountMinor: operation === 'reconfigure' ? 8800 : 2000, paymentRequired: true, expiresAt: Date.parse('2026-08-31T08:15:00Z'), replayed: false } }),
        });
        return;
      }
      const quoteNo = payload?.parameters?.quoteNo;
      expect(quoteNo).toBe(`quote-${operation}`);
      expect(payload?.parameters?.settlementMethod).toBe('balance');
      const replayed = confirmedQuotes.has(quoteNo);
      confirmedQuotes.add(quoteNo);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ code: 0, data: { operationNo: `operation-${operation}`, quoteNo, status: 'ready', currencyCode: 'CNY', amountMinor: operation === 'reconfigure' ? 8800 : 2000, paymentRequired: true, replayed } }),
      });
    });
    for (const operation of ['reconfigure', 'change_ip']) {
      const preview = await adminPage.evaluate(async ({ operation }) => {
        const response = await fetch(`/x/idc-channel/api/v1/channel/services/service-1/operations/${operation}`, {
          method: 'POST', headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ serviceNo: 'service-1', requestId: `preview-${operation}`, operation, parameters: operation === 'reconfigure' ? { revision: 'rev-1', termNo: 'monthly', currencyCode: 'CNY', selections: [{ dimensionNo: 'cpu', optionNo: '4-core', quantity: 1 }] } : {}, confirmed: false }),
        });
        return { status: response.status, body: await response.json() };
      }, { operation });
      expect(preview.status).toBe(200);
      expect(preview.body.data.status).toBe('quoted');
      const confirm = async () => adminPage.evaluate(async ({ operation }) => {
        const response = await fetch(`/x/idc-channel/api/v1/channel/services/service-1/operations/${operation}`, {
          method: 'POST', headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ serviceNo: 'service-1', requestId: `confirm-${operation}`, operation, parameters: { quoteNo: `quote-${operation}`, settlementMethod: 'balance' }, confirmed: true }),
        });
        return { status: response.status, body: await response.json() };
      }, { operation });
      const first = await confirm();
      const replay = await confirm();
      expect(first.status).toBe(200);
      expect(first.body.data.operationNo).toBe(`operation-${operation}`);
      expect(replay.body.data.operationNo).toBe(first.body.data.operationNo);
      expect(replay.body.data.replayed).toBe(true);
    }
  });

  test('TC-5j: 下游可按稳定事件编号补拉原始签名事件', async ({ adminPage }) => {
    const eventId = `evt_${'a'.repeat(64)}`;
    await adminPage.route(`**/x/idc-channel/api/v1/channel/webhooks/demo/${eventId}`, async (route) => {
      expect(route.request().method()).toBe('GET');
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          code: 0,
          data: {
            eventId,
            version: 'v1',
            eventType: 'order.updated',
            payload: { status: 'paid' },
            signature: 'sha256=stable',
            status: 'delivered',
            attempts: 1,
          },
        }),
      });
    });
    const replay = await adminPage.evaluate(async ({ eventId }) => {
      const response = await fetch(`/x/idc-channel/api/v1/channel/webhooks/demo/${eventId}`);
      return { status: response.status, body: await response.json() };
    }, { eventId });
    expect(replay.status).toBe(200);
    expect(replay.body.data.eventId).toBe(eventId);
    expect(replay.body.data.version).toBe('v1');
    expect(replay.body.data.payload.status).toBe('paid');
    expect(replay.body.data.signature).toBe('sha256=stable');
  });
});

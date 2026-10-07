import { expect, test } from '../../../../../../hack/tests/fixtures/auth';

test('TC-2a/2b/2c/2e/2f: unverified compatibility adapters are explicitly disabled', async ({ adminPage }) => {
  await adminPage.route('**/x/idc-channel/api/v1/channel/capabilities', async route => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { genericApi: true, adapters: { 'zjmf-finance': 'disabled_unverified', v10: 'disabled_unverified', whmcs: 'disabled_unverified', linapro: 'generic_api_v1' }, operations: { catalog: 'enabled', quote: 'enabled', create_order: 'enabled', cancel_order: 'enabled_pending_identity_only', renew: 'disabled_lifecycle_unwired', reconfigure: 'disabled_lifecycle_unwired', reinstall: 'disabled_lifecycle_unwired', console: 'disabled_lifecycle_unwired', change_ip: 'disabled_lifecycle_unwired', terminate: 'disabled_lifecycle_unwired' } } }) });
  });
  const body = await adminPage.evaluate(async () => (await fetch('/x/idc-channel/api/v1/channel/capabilities')).json());
  expect(Object.values(body.data.adapters)).not.toContain('enabled');
  expect(body.data.operations.quote).toBe('enabled');
  expect(body.data.operations.renew).toBe('disabled_lifecycle_unwired');
  expect(body.data.operations.terminate).toBe('disabled_lifecycle_unwired');
});

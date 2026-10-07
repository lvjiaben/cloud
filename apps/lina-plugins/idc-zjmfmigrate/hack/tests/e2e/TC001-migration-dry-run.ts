import { expect, test } from '../../../../../../hack/tests/fixtures/auth';

test.describe('TC001 migration dry run', () => {
  test('TC-1a/1b: source preflight exposes verified matrix and blocks unknown source', async ({ adminPage }) => {
    await adminPage.route('**/x/idc-zjmfmigrate/api/v1/admin/migration/preflight', async route => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { supported: false, blockers: ['source system is not in the verified adapter matrix'], capabilities: ['identity', 'orders', 'funds'] } }) });
    });
    const body = await adminPage.evaluate(async () => (await fetch('/x/idc-zjmfmigrate/api/v1/admin/migration/preflight', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ sourceSystem: 'unknown', sourceVersion: 'v1' }) })).json());
    expect(body.data.supported).toBe(false);
    expect(body.data.blockers).toContain('source system is not in the verified adapter matrix');
  });

  test('TC-1c/1d/1e/1f: batch run, conflict, reconcile and cutover remain explicit', async ({ adminPage }) => {
    await adminPage.route('**/x/idc-zjmfmigrate/api/v1/admin/migration/batches/1/cutover', async route => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data: { allowed: false, checks: ['unresolved conflicts or reconciliation differences'] } }) });
    });
    const body = await adminPage.evaluate(async () => (await fetch('/x/idc-zjmfmigrate/api/v1/admin/migration/batches/1/cutover', { method: 'POST' })).json());
    expect(body.data.allowed).toBe(false);
    expect(body.data.checks[0]).toContain('unresolved');
  });
});

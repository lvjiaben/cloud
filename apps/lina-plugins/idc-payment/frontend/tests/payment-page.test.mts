// Verify pagination state without contacting the host, database, or supplier.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { usePaymentPage } from '../src/payment-page.ts';

test('confirmed rows and total survive failure; the same requested page is retried', async () => {
  const calls: number[][] = [];
  let reject = false;
  const page = usePaymentPage(async (number, size) => {
    calls.push([number, size]);
    if (reject) throw { response: { status: 503 } };
    return { items: [{ id: number }], total: 121 };
  }, () => undefined);
  await page.load(2, 20);
  reject = true;
  assert.equal(await page.load(3, 20), false);
  assert.deepEqual([page.state.currentPage, page.state.total, page.state.items[0]?.id], [2, 121, 2]);
  reject = false;
  await page.load();
  assert.deepEqual(calls.at(-1), [3, 20]);
  await page.load(3, 100);
  assert.deepEqual(calls.at(-1), [1, 100]);
});

test('late successful response cannot restore rows after a newer refusal', async () => {
  let release: (value: { items: { id: number }[]; total: number }) => void = () => {};
  let calls = 0;
  let cleared = 0;
  const page = usePaymentPage(async () => {
    calls += 1;
    if (calls === 1) return { items: [{ id: 1 }], total: 121 };
    if (calls === 2) return new Promise(resolve => { release = resolve; });
    throw { response: { status: 403 } };
  }, () => undefined, () => { cleared += 1; });
  await page.load();
  const slow = page.load(2);
  await page.load(3);
  release({ items: [{ id: 2 }], total: 121 });
  await slow;
  assert.equal(cleared, 1);
  assert.deepEqual([page.state.items, page.state.total, page.state.currentPage], [[], 0, 1]);
});

test('empty filtered page has authoritative zero; missing totals never fabricate counts', async () => {
  let valid = true;
  const page = usePaymentPage(async () => valid ? { items: [{ id: 1 }], total: 121 } : { items: [], total: undefined as unknown as number }, () => undefined);
  await page.load(); valid = false;
  assert.equal(await page.load(2), false);
  assert.equal(page.state.total, 121);
  page.reset();
  assert.deepEqual([page.state.items, page.state.total], [[], 0]);
});

test('opaque request refusal codes clear rows while ordinary outages retain them', async () => {
  for (const code of [401, 403, 61, 'NotAuthorized']) {
    let fail = false;
    const page = usePaymentPage(async () => { if (fail) throw { code }; return { items: [{ id: 1 }], total: 1 }; }, () => undefined);
    await page.load(); fail = true; await page.load();
    assert.equal(page.state.total, 0);
    assert.equal(page.state.items.length, 0);
  }
});

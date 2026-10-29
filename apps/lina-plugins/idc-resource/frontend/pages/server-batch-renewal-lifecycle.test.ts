import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { nextTick } from 'vue';
import Renewal from './server-batch-renewal.vue';

const state = vi.hoisted(() => ({
  access: undefined as unknown as { safe: boolean; view: boolean },
  modals: [] as Array<{ options: { onConfirm: () => unknown }; open: ReturnType<typeof vi.fn>; close: ReturnType<typeof vi.fn> }>,
  list: vi.fn(), preview: vi.fn(), confirm: vi.fn(),
}));
vi.mock('./server-batch-renewal-client', () => ({ listServerRenewalCandidates: state.list, previewServerRenewals: state.preview, confirmServerRenewals: state.confirm }));
vi.mock('./plugin-access', async () => {
  const { reactive } = await import('vue');
  state.access = reactive({ safe: true, view: true });
  return { useAccess: () => ({ hasAccessByCodes: (codes: string[]) => codes.some(code => code.endsWith(':safe') ? state.access.safe : state.access.view) }) };
});
vi.mock('@vben/preferences', () => ({ preferences: { app: { locale: 'en-US' } } }));
vi.mock('#/locales', () => ({ $t: (key: string) => key }));
vi.mock('#/utils/time', () => ({ formatTimestamp: String }));
vi.mock('../../../vben-adapter/frontend/use-admin-grid-layout', () => ({ useAdminGridLayout: () => () => {} }));
vi.mock('../../../vben-adapter/frontend/src', () => ({ fixedRightActionColumn: (value: unknown) => value }));
vi.mock('#/adapter/form', async () => {
  const { defineComponent, h } = await import('vue');
  const z = { number: () => ({ int: () => ({ min: () => ({ max: () => ({}) }) }) }) };
  return { z, useVbenForm: () => [defineComponent(() => () => h('div')), { validate: async () => ({ valid: true }), getValues: async () => ({ renewalCycles: 1 }) }] };
});
vi.mock('#/adapter/vxe-table', async () => {
  const { defineComponent, h, ref } = await import('vue');
  return { useVbenVxeGrid: () => {
    const rows = ref<Array<{serverNo: string}>>([]);
    const Grid = defineComponent((_, { slots }) => () => h('div', rows.value.map(row => slots.action?.({ row }))));
    return [Grid, { setGridOptions: (value: {data?: typeof rows.value}) => { if (value.data) rows.value = value.data; }, grid: { recalculate: vi.fn() } }];
  } };
});
vi.mock('@vben/common-ui', async () => {
  const { defineComponent, h } = await import('vue');
  const Page = defineComponent((_, { slots }) => () => h('main', slots.default?.()));
  return { Page, useVbenModal: (options: {onConfirm: () => unknown}) => {
    const modal = { options, open: vi.fn(), close: vi.fn(), lock: vi.fn(), unlock: vi.fn() };
    state.modals.push(modal);
    return [defineComponent((_, { slots }) => () => h('div', slots.default?.())), modal];
  } };
});
vi.mock('ant-design-vue', async () => {
  const { defineComponent, h } = await import('vue');
  const Box = defineComponent((_, { slots }) => () => h('div', [slots.extra?.(), slots.default?.()]));
  const Button = defineComponent((_, { slots, attrs }) => () => h('button', attrs, slots.default?.()));
  const Checkbox = defineComponent({ props: ['checked'], emits: ['update:checked'], setup: (props, { emit }) => () => h('input', { type: 'checkbox', checked: props.checked, onChange: () => emit('update:checked', true) }) });
  return { Card: Box, Space: Box, Alert: Box, Tag: Box, Button, Checkbox, message: { warning: vi.fn(), error: vi.fn() } };
});
const row = (key: string) => ({ serverNo: `SVR-${key.repeat(32)}`, productNameZhCN: key, productNameEnUS: key });
const quote = (key: string) => ({ requestKey: `request-${key}`, renewalCycles: 1, confirmationDigest: 'a'.repeat(64), items: [row(key)] });
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(done => { resolve = done; }); return { promise, resolve }; }
let wrapper: ReturnType<typeof mount> | undefined;
beforeEach(() => {
  state.modals.length = 0; state.access.safe = true; state.access.view = true;
  state.list.mockReset().mockImplementation(async (input: {entitlementNo: string}) => ({ items: [row(input.entitlementNo === 'ENT_A' ? 'A' : 'B')], total: 1, nextCursor: '' }));
  state.preview.mockReset().mockResolvedValue(quote('A')); state.confirm.mockReset().mockResolvedValue({items: []});
});
afterEach(() => { wrapper?.unmount(); wrapper = undefined; });
async function open() { wrapper = mount(Renewal, { props: { entitlementNo: 'ENT_A' } }); await flushPromises(); return wrapper; }
async function previewOne() { await wrapper!.findAll('button').find(button => button.text().endsWith('.previewOne'))!.trigger('click'); await flushPromises(); }

describe('renewal ownership lifecycle', () => {
  it('does not reopen an old preview or send confirmation after leaving its product', async () => {
    const pending = deferred<ReturnType<typeof quote>>(); state.preview.mockReturnValue(pending.promise);
    await open(); await previewOne(); expect(state.preview).toHaveBeenCalledOnce();
    const oldModal = state.modals[0]!; wrapper!.unmount(); wrapper = undefined;
    pending.resolve(quote('A')); await flushPromises();
    expect(oldModal.open).not.toHaveBeenCalled(); await oldModal.options.onConfirm();
    expect(state.confirm).not.toHaveBeenCalled();
  });
  it('rejects an out-of-order preview and binds the next confirmation to the replacement product', async () => {
    const pending = deferred<ReturnType<typeof quote>>(); state.preview.mockReturnValueOnce(pending.promise).mockResolvedValue(quote('B'));
    await open(); await previewOne(); await wrapper!.setProps({ entitlementNo: 'ENT_B' }); await flushPromises();
    pending.resolve(quote('A')); await flushPromises(); expect(state.modals[0]!.open).not.toHaveBeenCalled();
    await previewOne(); expect(state.preview).toHaveBeenLastCalledWith(expect.objectContaining({entitlementNo: 'ENT_B', serverNos: [row('B').serverNo]}));
    expect(state.modals[0]!.open).toHaveBeenCalledOnce();
    await wrapper!.find('input[type=checkbox]').setValue(true); await state.modals[0]!.options.onConfirm();
    expect(state.confirm).toHaveBeenCalledWith(expect.objectContaining({entitlementNo: 'ENT_B', serverNos: [row('B').serverNo]}));
  });
  it('rechecks a revoked safe grant before accepting an already-open confirmation', async () => {
    await open(); await previewOne(); expect(state.modals[0]!.open).toHaveBeenCalledOnce();
    await wrapper!.find('input[type=checkbox]').setValue(true);
    state.access.safe = false; await nextTick(); await flushPromises(); await state.modals[0]!.options.onConfirm();
    expect(state.confirm).not.toHaveBeenCalled(); expect(state.modals[0]!.close).toHaveBeenCalled();
  });
});

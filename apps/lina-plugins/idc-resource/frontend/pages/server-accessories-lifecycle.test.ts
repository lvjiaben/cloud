import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { nextTick } from 'vue';
import Accessories from './server-accessories.vue';
const state = vi.hoisted(() => ({ get: vi.fn(), create: vi.fn(), success: vi.fn(), permissions: undefined as unknown as Set<string> }));
vi.mock('./server-accessory-client', () => ({ getStaffServerAccessories: state.get, createStaffServerAccessoryOperation: state.create, listStaffServerServices: vi.fn(), previewStaffServerBatch: vi.fn(), confirmStaffServerBatch: vi.fn(), executeStaffServerBatch: vi.fn() }));
vi.mock('./plugin-access', async () => { const {reactive} = await import('vue'); state.permissions = reactive(new Set<string>()); return { useAccess: () => ({ hasAccessByCodes: (codes: string[]) => codes.some(code => state.permissions.has(code)) }) }; });
vi.mock('vue-router', () => ({ useRoute: () => ({query: {}}) }));
vi.mock('@vben/preferences', () => ({preferences: {app: {locale: 'en-US'}}}));
vi.mock('@vben/locales', () => ({$te: () => false}));
vi.mock('#/locales', () => ({$t: (key: string) => key}));
vi.mock('#/utils/time', () => ({formatTimestamp: String}));
vi.mock('../../../vben-adapter/frontend/use-admin-grid-layout', () => ({useAdminGridLayout: () => () => {}}));
vi.mock('../../../idc-core/frontend/pages/components/operations-detail-panel.vue', () => ({default: {render: () => null}}));
vi.mock('./components/traffic-package-purchase.vue', () => ({default: {render: () => null}}));
vi.mock('./components/vben-drawer-compat.vue', () => ({default: {render: () => null}}));
vi.mock('#/adapter/vxe-table', async () => { const {defineComponent,h} = await import('vue'); return {useVbenVxeGrid: () => [defineComponent(() => () => h('div')), {setGridOptions: vi.fn(), query: vi.fn(), grid: {recalculate: vi.fn()}}]}; });
vi.mock('@vben/common-ui', async () => { const {defineComponent,h} = await import('vue'); const Box = defineComponent((_,{slots}) => () => h('div',slots.default?.())); return {Page: Box, useVbenDrawer: () => [Box,{close: vi.fn(),setData: vi.fn(),open:vi.fn()}]}; });
vi.mock('./components/vben-modal-compat.vue', async () => {
 const {defineComponent,h} = await import('vue');
 return {default: defineComponent({props:['open'], emits:['ok'], setup: (props,{slots,emit}) => () => h('section', props.open ? [slots.default?.(),h('button',{'data-testid':'test-confirm',onClick:()=>emit('ok')},'Confirm')] : [])})};
});
vi.mock('ant-design-vue', async () => {
 const {defineComponent,h} = await import('vue');
 const Box=defineComponent((_,{slots})=>()=>h('div',slots.default?.()));
 const Button=defineComponent((_,{slots,attrs})=>()=>h('button',attrs,slots.default?.()));
 const Input=defineComponent({props:['value'],emits:['update:value'],setup:(props,{emit,attrs})=>()=>h('input',{...attrs,value:props.value,onInput:(event:Event)=>emit('update:value',(event.target as HTMLInputElement).value)})});
 return {Alert:Box,Button,Card:Box,Checkbox:Box,Descriptions:Box,DescriptionsItem:Box,Empty:Box,Form:Box,FormItem:Box,Input,InputNumber:Input,Select:Box,SelectOption:Box,Space:Box,Spin:Box,Tag:Box,Textarea:Input,message:{success:state.success,warning:vi.fn()}};
});
function detail(serviceNo: string) { return {detail:{serviceNo,customerNo:'CUS_A',accessories:[],availableActions:['snapshot_create'],accessoriesExpiresAt:Date.now()+60_000,activeOperationNo:'',entitlementStatus:'active',runtimeStatus:'running',providerTaskStatus:'succeeded',productNameZhCN:serviceNo,productNameEnUS:serviceNo}}; }
function deferred<T>() {let resolve!: (value:T)=>void;const promise=new Promise<T>(done=>{resolve=done;});return {promise,resolve};}
beforeEach(()=>{state.permissions.clear();state.permissions.add('idc:admin:resource:server-accessory:snapshot:create');state.get.mockReset().mockImplementation(async (serviceNo:string)=>detail(serviceNo));state.create.mockReset();state.success.mockReset();});
describe('scoped accessory context',()=>{
 it('keeps the replacement product when old detail arrives last',async()=>{
  const pending=deferred<ReturnType<typeof detail>>();state.get.mockImplementationOnce(()=>pending.promise);
  const wrapper=mount(Accessories,{props:{scopedServiceNo:'SVC_A'}});await wrapper.setProps({scopedServiceNo:'SVC_B'});await flushPromises();
  expect(wrapper.text()).toContain('SVC_B');pending.resolve(detail('SVC_A'));await flushPromises();
  expect(wrapper.text()).toContain('SVC_B');expect(wrapper.text()).not.toContain('SVC_A');wrapper.unmount();
 });
 it('does not issue a follow-up read or success toast when an operation finishes after unmount',async()=>{
  const pending=deferred<{paymentRequired:boolean}>();state.create.mockReturnValue(pending.promise);
  const wrapper=mount(Accessories,{props:{scopedServiceNo:'SVC_A'}});await flushPromises();
  await wrapper.get('[data-testid=staff-accessory-action-snapshot_create]').trigger('click');await wrapper.get('[data-testid=staff-accessory-name]').setValue('before-leaving');await wrapper.get('[data-testid=test-confirm]').trigger('click');await flushPromises();
  expect(state.create).toHaveBeenCalledWith('SVC_A',expect.objectContaining({action:'snapshot_create'}));wrapper.unmount();pending.resolve({paymentRequired:false});await flushPromises();
  expect(state.get).toHaveBeenCalledTimes(1);expect(state.success).not.toHaveBeenCalled();
 });
 it('does not send a confirmed operation after its exact action permission is revoked',async()=>{
  const wrapper=mount(Accessories,{props:{scopedServiceNo:'SVC_A'}});await flushPromises();
  await wrapper.get('[data-testid=staff-accessory-action-snapshot_create]').trigger('click');await wrapper.get('[data-testid=staff-accessory-name]').setValue('permission-revoked');
  state.permissions.clear();await nextTick();await wrapper.get('[data-testid=test-confirm]').trigger('click');await flushPromises();
  expect(state.create).not.toHaveBeenCalled();wrapper.unmount();
 });
});

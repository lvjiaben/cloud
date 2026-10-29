<script setup lang="ts">
import { ref } from 'vue';
import { useVbenModal } from '@vben/common-ui';
import { Alert, Button, message } from 'ant-design-vue';
import { useVbenForm, z } from '#/adapter/form';
import { $t } from '#/locales';
import { getPaymentPublicAddresses, savePaymentPublicAddresses } from '../payment-client';

const version = ref(0);
const loadFailed = ref(false);
const tr = (key: string) => $t(`plugin.idc-payment.publicAddresses.${key}`);

function schema() {
  return [
    { fieldName: 'notificationOrigin', component: 'Input' as const, label: tr('notification'), help: tr('notificationHelp'), componentProps: { placeholder: 'https://notify.example.com', maxlength: 2048 }, rules: z.string().max(2048).optional() },
    { fieldName: 'customerReturnOrigin', component: 'Input' as const, label: tr('customerReturn'), help: tr('returnHelp'), componentProps: { placeholder: 'https://customer.example.com', maxlength: 2048 }, rules: z.string().max(2048).optional() },
  ];
}
const [AddressForm, formApi] = useVbenForm({ layout: 'vertical', showDefaultActions: false, wrapperClass: 'grid-cols-1', schema: [] });
const [AddressModal, modalApi] = useVbenModal({
  onOpenChange: async (open: boolean) => {
    if (open) await load();
    else { version.value = 0; await formApi.resetForm(); }
  },
  onConfirm: async () => {
    if (loadFailed.value || !(await formApi.validate()).valid) return;
    const values = await formApi.getValues();
    modalApi.setState({confirmLoading: true});
    try {
      const result = await savePaymentPublicAddresses({notificationOrigin: String(values.notificationOrigin || '').trim(), customerReturnOrigin: String(values.customerReturnOrigin || '').trim(), version: version.value});
      version.value = result.addresses.version;
      message.success(tr('saved'));
      modalApi.close();
    } finally { modalApi.setState({confirmLoading: false}); }
  },
});
async function load() {
  loadFailed.value = false;
  modalApi.setState({loading: true, confirmDisabled: true, title: tr('title')});
  formApi.setState({schema: schema()});
  try {
    const result = await getPaymentPublicAddresses();
    version.value = result.addresses.version;
    await formApi.setValues(result.addresses);
    modalApi.setState({confirmDisabled: false});
  } catch { loadFailed.value = true; }
  finally { modalApi.setState({loading: false}); }
}
</script>

<template>
  <Button v-access:code="['idc:admin:payment:channel:manage']" data-testid="payment-public-addresses-open" @click="modalApi.open()">{{ tr('title') }}</Button>
  <AddressModal class="w-[min(96vw,680px)]" data-testid="payment-public-addresses-modal" :confirm-text="tr('save')">
    <Alert class="mb-4" type="info" show-icon :message="tr('help')" :description="tr('emptyHelp')" />
    <Alert v-if="loadFailed" class="mb-4" type="error" show-icon :message="$t('plugin.idc-payment.loadFailed')"><template #action><Button size="small" @click="load">{{ $t('plugin.idc-payment.refresh') }}</Button></template></Alert>
    <AddressForm />
  </AddressModal>
</template>

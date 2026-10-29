<script setup lang="ts">
import type { PaymentChannel } from '../payment-client';
import { useVbenModal } from '@vben/common-ui';
import { Alert, Button, message } from 'ant-design-vue';
import { useVbenForm, z } from '#/adapter/form';
import { $t } from '#/locales';
import { savePaymentChannelModes } from '../payment-client';
const props = defineProps<{channel: PaymentChannel; availableModes: string[]}>();
let editedChannel: PaymentChannel | undefined;
const emit = defineEmits<{updated: []}>();
const tr = (key: string) => $t(`plugin.idc-payment.channels.modes.${key}`);
const [ModesForm, formApi] = useVbenForm({schema: [], showDefaultActions: false, layout: 'vertical', wrapperClass: 'grid-cols-1', commonConfig: {componentProps: {class: 'w-full'}}});
const [ModesModal, modalApi] = useVbenModal({
  onOpenChange: async (open) => {
    if (!open) {editedChannel = undefined; await formApi.resetForm(); return;}
    editedChannel = {...props.channel, modes: [...props.channel.modes]};
    modalApi.setState({title: tr('title'), confirmText: tr('save')});
    formApi.setState({schema: [{fieldName: 'enabledModes', label: tr('label'), component: 'Select', componentProps: {mode: 'multiple', options: props.availableModes.map(value => ({value, label: $t(`plugin.idc-payment.channels.mode.${value}`)})), 'data-testid': 'payment-channel-enabled-modes'}, rules: z.array(z.string()).min(1, tr('required')).max(16, tr('maximum'))}]});
    await formApi.setValues({enabledModes: [...props.channel.modes]});
  },
  onConfirm: async () => {
    if (!editedChannel || !(await formApi.validate()).valid) return;
    const values = await formApi.getValues();
    modalApi.setState({confirmLoading: true});
    try {
      await savePaymentChannelModes(editedChannel, values.enabledModes);
      message.success(tr('saved'));
      modalApi.close();
      emit('updated');
    } finally {modalApi.setState({confirmLoading: false});}
  },
});
</script>
<template>
  <Button v-access:code="['idc:admin:payment:channel:manage']" size="small" :disabled="!availableModes.length" :data-testid="`payment-channel-modes-${channel.channelNo}`" @click="modalApi.open()">{{ tr('action') }}</Button>
  <ModesModal class="w-[min(96vw,620px)]">
    <Alert class="mb-4" type="info" show-icon :message="tr('help')" />
    <ModesForm />
  </ModesModal>
</template>

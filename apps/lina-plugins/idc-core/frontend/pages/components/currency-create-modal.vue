<script setup lang="ts">
import { ref } from 'vue';
import { useVbenModal } from '@vben/common-ui';
import { Alert, message } from 'ant-design-vue';
import { useVbenForm } from '#/adapter/form';
import { $t } from '#/locales';
import { preferences } from '@vben/preferences';
import { currencyCreate, currencyLabel } from '../currency-client';
const emit = defineEmits<{ saved: [] }>();
const error = ref('');
const busy = ref(false);
const tr = (key: string) => $t(`plugin.idc-core.currency.${key}`);
const [Form, form] = useVbenForm({ commonConfig: { labelWidth: 90, componentProps: { class: 'w-full' } }, schema: [], showDefaultActions: false });
const [Modal, modal] = useVbenModal({
  async onConfirm() {
    if (busy.value) return;
    const { valid } = await form.validate(); if (!valid) return;
    const values = await form.getValues<{ currencyCode: string }>();
    busy.value = true; error.value = ''; modal.lock(true);
    try { await currencyCreate(values.currencyCode); message.success(tr('saved')); emit('saved'); await modal.close(); }
    catch { error.value = tr('createFailed'); }
    finally { busy.value = false; modal.lock(false); }
  },
  async onOpenChange(open) {
    if (!open) return;
    error.value = ''; busy.value = false; await form.resetForm();
    const codes = Intl.supportedValuesOf('currency').filter(code => code !== 'XXX');
    await form.setState({ schema: [{ fieldName: 'currencyCode', component: 'Select', label: tr('name'), rules: 'required', componentProps: { 'data-testid': 'currency-code', showSearch: true, optionFilterProp: 'label', options: codes.map(code => ({ value: code, label: currencyLabel(code, preferences.app.locale) })) } }] });
  },
});
</script>
<template>
  <Modal :title="tr('add')" class="w-[440px]" :fullscreen-button="false">
    <div class="flex flex-col gap-3" data-testid="currency-create-modal"><p class="text-muted-foreground text-sm">{{ tr('createHelp') }}</p><Alert v-if="error" :message="error" type="error" show-icon /><Form /></div>
  </Modal>
</template>

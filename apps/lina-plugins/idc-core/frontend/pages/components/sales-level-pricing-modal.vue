<script setup lang="ts">
import { ref } from 'vue';
import { useVbenModal } from '@vben/common-ui';
import { Alert, message } from 'ant-design-vue';
import { useVbenForm } from '#/adapter/form';
import { $t } from '#/locales';
import { salesLevelPricing, salesSaveLevelPricing } from '../sales-management-client';

const emit = defineEmits<{ saved: [] }>();
const tr = (key: string) => $t(`plugin.idc-core.salesManagement.${key}`);
const name = ref('');
const error = ref('');
const ready = ref(false);
const busy = ref(false);
let levelNo = '';
let version = 0;
let opening = 0;
const [Form, form] = useVbenForm({ commonConfig: { labelWidth: 130, componentProps: { class: 'w-full' } }, schema: [], showDefaultActions: false });

async function load() {
  if (busy.value) return;
  const sequence = opening;
  busy.value = true; ready.value = false; error.value = ''; modal.lock(true);
  try {
    const { item } = await salesLevelPricing(levelNo);
    if (sequence !== opening) return;
    version = item.version;
    await form.setValues({ percentage: item.percentageBasisPoints === null ? '' : String(item.percentageBasisPoints / 100) });
    ready.value = true;
  } catch { if (sequence === opening) error.value = tr('loadFailed'); }
  finally { if (sequence === opening) { busy.value = false; modal.lock(false); } }
}
async function save() {
  if (!ready.value || busy.value) return;
  let points: number | null = null;
  {
    const values = await form.getValues();
    const value = String(values.percentage ?? '').trim();
    if (value && !/^\d{1,3}(?:\.\d{1,2})?$/.test(value)) { error.value = tr('globalPercentageInvalid'); return; }
    if (value) {
      const [integer, fraction = ''] = value.split('.');
      points = Number(integer) * 100 + Number(fraction.padEnd(2, '0'));
    }
    if (points !== null && points > 10000) { error.value = tr('globalPercentageInvalid'); return; }
  }
  busy.value = true; error.value = ''; modal.lock(true);
  try {
    await salesSaveLevelPricing(levelNo, points, version);
    message.success(tr('saved')); emit('saved'); modal.close();
  } catch { error.value = tr('globalPercentageSaveFailed'); }
  finally { busy.value = false; modal.lock(false); }
}
const [Modal, modal] = useVbenModal({
  onConfirm: () => save(),
  async onOpenChange(open) {
    opening++;
    if (!open) return;
    const data = modal.getData<{ levelNo: string; name: string }>();
    levelNo = data.levelNo; name.value = data.name; error.value = ''; ready.value = false; busy.value = false;
    await form.resetForm();
    await form.setState({ schema: [{ fieldName: 'percentage', component: 'Input', label: tr('fields.percentageBasisPoints'), componentProps: { suffix: '%', placeholder: '80', 'data-testid': 'sales-level-percentage' } }] });
    await load();
  },
});
</script>
<template>
  <Modal :title="`${tr('globalPercentage')} · ${name}`" class="w-[440px]" :fullscreen-button="false">
    <div class="flex flex-col gap-3" data-testid="sales-level-pricing-modal">
      <p class="text-sm text-muted-foreground">{{ tr('globalPercentageHelp') }}</p>
      <Alert v-if="error" type="error" show-icon :message="error" />
      <Form />
    </div>
  </Modal>
</template>

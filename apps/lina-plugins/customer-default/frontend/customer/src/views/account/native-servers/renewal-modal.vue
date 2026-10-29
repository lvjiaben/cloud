<script setup lang="ts">
import type { ServiceRenewalQuote } from '#/api/idc/customer-renewals';
import { onBeforeUnmount, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { preferences } from '@vben/preferences';
import { Alert, Button, Descriptions, DescriptionsItem } from 'ant-design-vue';
import { useVbenForm, z } from '#/adapter/form';
import { confirmServiceRenewal, quoteServiceRenewal } from '#/api/idc/customer-renewals';
import VbenModalCompat from '#/components/vben-modal-compat.vue';
import { $t } from '#/locales';
import { formatMinorAmount } from '#/utils/format-amount';
const props = defineProps<{ open: boolean; entitlementNo: string }>();
const emit = defineEmits<{ 'update:open': [value: boolean]; settled: [] }>();
const router = useRouter();
const quote = ref<ServiceRenewalQuote>();
const quoting = ref(false);
const confirming = ref(false);
const failed = ref(false);
let generation = 0;
let quoteKey = '';
let confirmKey = '';
const tr = (key: string) => $t(`serverRenewal.${key}`);
const [Form, formApi] = useVbenForm({ showDefaultActions: false, wrapperClass: 'grid-cols-1', commonConfig: {labelWidth: 110} });
function invalidate() { generation++; quote.value = undefined; quoteKey = ''; confirmKey = ''; failed.value = false; }
function schema() { return [{ fieldName: 'cycles', label: tr('cycles'), component: 'InputNumber' as const, componentProps: () => ({ min: 1, max: 120, precision: 0, disabled: confirming.value, onChange: invalidate, 'data-testid': 'native-renewal-cycles' }), rules: z.number().int().min(1).max(120) }]; }
function amount(value: ServiceRenewalQuote) { return formatMinorAmount(value.totalAmountMinor,value.currencyCode); }
function date(value: number) { return new Date(value).toLocaleString(preferences.app.locale); }
async function loadQuote() {
  if (quoting.value || confirming.value || !(await formApi.validate()).valid) return;
  const values = await formApi.getValues<{cycles: number}>();
  const current = generation;
  if (!quoteKey) quoteKey = crypto.randomUUID();
  quoting.value = true; failed.value = false;
  try {
    const result = await quoteServiceRenewal(props.entitlementNo, values.cycles, quoteKey);
    if (current !== generation || !props.open) return;
    quote.value = result.quote; confirmKey = crypto.randomUUID();
  } catch { if (current === generation) failed.value = true; }
  finally { if (current === generation) quoting.value = false; }
}
async function confirm() {
  if (!quote.value || quoting.value || confirming.value) return;
  if (quote.value.expiresAt <= Date.now()) { invalidate(); failed.value = true; return; }
  const current = generation;
  confirming.value = true; failed.value = false;
  try {
    // A failed response retains this quote and key so retries cannot create
    // another invoice or extend the service twice.
    const result = await confirmServiceRenewal(quote.value.quoteNo, confirmKey);
    if (current !== generation || !props.open) return;
    emit('update:open', false);
    if (result.operation.statusCode === 'succeeded') emit('settled');
    else await router.push(`/account/billing/invoices/${encodeURIComponent(result.operation.invoiceNo)}`);
  } catch { if (current === generation) failed.value = true; }
  finally { if (current === generation) confirming.value = false; }
}
watch(() => [props.open, props.entitlementNo] as const, async ([open]) => {
  invalidate(); quoting.value = false; confirming.value = false;
  if (open) { formApi.setState({schema: schema()}); await formApi.setValues({cycles: 1}); }
}, {immediate: true});
onBeforeUnmount(invalidate);
</script>
<template>
  <VbenModalCompat :open="open" :title="tr('title')" :ok-text="tr('confirm')" :cancel-text="tr('cancel')" :confirm-loading="confirming" :ok-button-props="{disabled: !quote || quoting}" @update:open="emit('update:open', $event)" @ok="confirm">
    <section data-testid="native-renewal-modal">
      <p>{{ tr('help') }}</p>
      <Form />
      <Button class="mb-4" :loading="quoting" :disabled="confirming" data-testid="native-renewal-quote" @click="loadQuote">{{ tr('quote') }}</Button>
      <Alert v-if="failed" class="mb-4" type="error" show-icon :message="tr('failed')" />
      <Descriptions v-if="quote" bordered :column="1" data-testid="native-renewal-preview">
        <DescriptionsItem :label="tr('amount')">{{ amount(quote) }}</DescriptionsItem>
        <DescriptionsItem :label="tr('expiry')">{{ date(quote.projectedExpiresAt) }}</DescriptionsItem>
      </Descriptions>
      <p v-if="quote" class="mt-3">{{ tr('payment') }}</p>
    </section>
  </VbenModalCompat>
</template>

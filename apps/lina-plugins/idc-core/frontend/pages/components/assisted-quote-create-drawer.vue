<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { AssistedQuote } from "../assisted-quote-client";
import type { CustomerSummary } from "../customer-client";
import { onBeforeUnmount, ref, watch } from "vue";
import { useVbenDrawer } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Alert, Button, Divider, message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { assistedOrderOptions, createAssistedQuote } from "../assisted-quote-client";
import { majorToMinorAmount } from "../catalog-money";
import AssistedQuoteItem from "./assisted-quote-item.vue";

const emit = defineEmits<{ created: [request: AssistedQuote] }>();
const saving = ref(false);
const error = ref("");
const currencies = ref<string[]>([]);
const currency = ref("");
const customer = ref<Pick<CustomerSummary, "customerNo" | "customerCode" | "displayName"> | null>(null);
const rows = ref<{ id: string }[]>([]);
const itemRefs = new Map<string, InstanceType<typeof AssistedQuoteItem>>();
let openSequence = 0;
let opened = false;
let lastFingerprint = "";
let actionKey = "";
function tr(key: string) { return $t(`plugin.idc-core.assistedCreate.${key}`); }
function field(fieldName: string, label: string, component = "Input") { return { fieldName, label: tr(label), component, componentProps: { "data-testid": `assisted-create-${fieldName}`, disabled: saving.value } }; }
function schema(): VbenFormProps["schema"] {
  return [
    { ...field("currencyCode", "currency", "Select"), rules: "required", componentProps: { "data-testid": "assisted-create-currency", disabled: saving.value, options: currencies.value.map(code => ({ label: code, value: code })), onChange: (code: string) => { currency.value = code; } } },
    { ...field("manualDiscount", "discount"), defaultValue: "0", rules: z.string().refine(value => { try { return BigInt(majorToMinorAmount(value, currency.value)) <= 9_000_000_000_000_000n; } catch { return false; } }, tr("discountInvalid")) },
    { ...field("paymentDueAt", "validity", "DatePicker"), rules: "required", componentProps: { "data-testid": "assisted-create-deadline", disabled: saving.value, showTime: true, valueFormat: "x", format: "YYYY-MM-DD HH:mm", allowClear: false } },
    { ...field("reason", "reason", "Textarea"), componentProps: { "data-testid": "assisted-create-reason", disabled: saving.value, rows: 2, maxlength: 1000, showCount: true }, formItemClass: "md:col-span-2" },
  ];
}
const [Form, formApi] = useVbenForm({ commonConfig: { componentProps: { class: "w-full" }, labelWidth: 115 }, schema: [], showDefaultActions: false, wrapperClass: "grid-cols-1 md:grid-cols-2" });
function syncSchema() { void formApi.setState({ schema: schema() }); }
const [Drawer, drawerApi] = useVbenDrawer({
  closeOnClickModal: false,
  onConfirm: submit,
  onOpenChange(isOpen) {
    opened = isOpen;
    if (isOpen) void initialize();
    else { ++openSequence; }
  },
});
async function initialize() {
  const current = ++openSequence;
  saving.value = false; error.value = "";
  customer.value = drawerApi.getData<CustomerSummary>() || null;
  lastFingerprint = ""; actionKey = ""; currencies.value = []; currency.value = "";
  rows.value = [{ id: crypto.randomUUID() }]; itemRefs.clear();
  syncSchema();
  await formApi.resetForm();
  await formApi.setValues({ currencyCode: null, manualDiscount: "0", paymentDueAt: String(Date.now() + 86400000), reason: "" }, false);
  if (current !== openSequence) return;
  await loadCurrencies();
}
async function loadCurrencies() {
  const current = openSequence;
  if (!customer.value?.customerNo) { error.value = tr("chooseCustomer"); return; }
  try {
    const configured = await assistedOrderOptions(customer.value.customerNo);
    if (current !== openSequence || !opened) return;
    currencies.value = configured.currencies;
    currency.value = currencies.value[0] || "";
    await formApi.setValues({ currencyCode: currency.value || null }, false);
    error.value = currency.value ? "" : tr("currencyFailed");
    syncSchema();
  } catch { if (current === openSequence) error.value = tr("currencyFailed"); }
}

function setItemRef(id: string, element: unknown) {
  if (element) itemRefs.set(id, element as InstanceType<typeof AssistedQuoteItem>);
  else itemRefs.delete(id);
}
function addItem() { rows.value = [...rows.value, { id: crypto.randomUUID() }]; }
function removeItem(id: string) { rows.value = rows.value.filter(row => row.id !== id); }
async function submit() {
  if (saving.value || !customer.value?.customerNo) return;
  error.value = "";
  if (!(await formApi.validate()).valid) return;
  if (!rows.value.length) { error.value = tr("addAtLeastOne"); return; }
  saving.value = true; syncSchema(); drawerApi.lock();
  try {
    const values = await formApi.getValues<{ currencyCode: string; manualDiscount: string; paymentDueAt: string; reason?: string }>();
    const paymentDueAt = Number(values.paymentDueAt);
    const remaining = paymentDueAt - Date.now();
    if (!Number.isSafeInteger(paymentDueAt) || remaining < 300000 || remaining > 604800000) throw new Error(tr("deadlineInvalid"));
    const items = [];
    for (const row of rows.value) {
      const component = itemRefs.get(row.id);
      if (!component) throw new Error(tr("checkForm"));
      items.push(await component.getInput());
    }
    const payload = {
      customerNo: customer.value.customerNo, currencyCode: values.currencyCode,
      manualDiscountMinor: majorToMinorAmount(values.manualDiscount, values.currencyCode),
      paymentDueAt,
      contractPolicyCode: "inherit" as const, contractTemplateNo: "",
      reason: values.reason?.trim() || "", internalNote: "", items,
    };
    const fingerprint = JSON.stringify(payload);
    if (lastFingerprint !== fingerprint) { actionKey = crypto.randomUUID(); lastFingerprint = fingerprint; }
    const request = await createAssistedQuote({ ...payload, idempotencyKey: actionKey });
    message.success(tr("created"));
    emit("created", request);
    drawerApi.unlock(); drawerApi.close();
  } catch (cause) {
    error.value = submissionError(cause);
  } finally { saving.value = false; drawerApi.unlock(); syncSchema(); }
}
function submissionError(cause: unknown) {
  // Only our known translated validation messages or a resolvable backend
  // message key are safe to display; transport/technical details stay out.
  const known = ["chooseProduct", "fileRequired", "invalidConfiguration", "checkForm", "configurationFailed", "deadlineInvalid"].map(tr);
  if (cause instanceof Error && known.includes(cause.message)) return cause.message;
  if (cause && typeof cause === "object") {
    const failure = cause as { messageKey?: string; messageParams?: Record<string, string>; response?: { data?: { messageKey?: string; messageParams?: Record<string, string> } } };
    const data = failure.response?.data || failure;
    if (typeof data.messageKey === "string") {
      const translated = $t(data.messageKey, data.messageParams || {});
      if (translated !== data.messageKey) return translated;
    }
  }
  return tr("submitFailed");
}
watch(() => preferences.app.locale, syncSchema);
onBeforeUnmount(() => { ++openSequence; });
</script>
<template>
  <Drawer :title="tr('title')" :confirm-text="tr('submit')" :show-confirm-button="!!customer" class="w-[min(96vw,1040px)]">
    <div data-testid="assisted-create-drawer" class="space-y-4">
      <Alert :message="tr('guide')" show-icon type="info" />
      <div v-if="customer" class="rounded-lg bg-muted p-3 font-medium" data-testid="assisted-create-selected-customer">{{ tr("customer") }}：{{ customer.displayName }} · {{ customer.customerCode }}</div>
      <Form />
      <p class="text-sm text-muted-foreground" data-testid="assisted-create-help">{{ tr("discountHint") }} {{ tr("validityHint") }}</p>
      <Divider orientation="left">{{ tr('products') }}</Divider>
      <AssistedQuoteItem v-for="(row, index) in rows" :key="row.id" :ref="element => setItemRef(row.id, element)" :index="index" :currency="currency" :busy="saving" @remove="removeItem(row.id)" />
      <Button block :disabled="saving || rows.length >= 8 || !currency" data-testid="assisted-create-add-item" @click="addItem">{{ tr('addProduct') }} ({{ rows.length }}/8)</Button>
      <Alert v-if="error" data-testid="assisted-create-error" :message="error" show-icon type="error"><template v-if="!currency" #action><Button size="small" @click="loadCurrencies">{{ tr('retry') }}</Button></template></Alert>
    </div>
  </Drawer>
</template>

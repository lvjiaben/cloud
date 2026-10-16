<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { AssistedQuoteCreateItem } from "../assisted-quote-client";
import type { CatalogConfigurationValues } from "../catalog-configuration-state";
import type { AssistedProduct, PurchaseConfiguration, PurchaseEvaluation, PurchaseTerm } from "../assisted-purchase-client";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Alert, Button, Card, Spin } from "ant-design-vue";
import { preferences } from "@vben/preferences";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { assistedProducts, evaluatePurchase, purchaseConfiguration, purchaseDeclaration, purchaseDraft, purchaseValues, quotePurchase } from "../assisted-purchase-client";
import { minorToMajorAmount } from "../catalog-money";
import CatalogConfigurationForm from "./catalog-configuration-form.vue";

const props = defineProps<{ currency: string; index: number; busy: boolean; allowedProducts?: AssistedProduct[]; removable?: boolean }>();
const emit = defineEmits<{ remove: [] }>();
const products = ref<AssistedProduct[]>([]);
const nextCursor = ref("");
const keyword = ref("");
const searching = ref(false);
const loading = ref(false);
const evaluating = ref(false);
const error = ref("");
const view = ref<PurchaseConfiguration>();
const evaluation = ref<PurchaseEvaluation>();
const draft = ref<CatalogConfigurationValues>({});
const terms = ref<PurchaseTerm[]>([]);
const selectedSlug = ref("");
const selectedTerm = ref("");
const configForm = ref<InstanceType<typeof CatalogConfigurationForm>>();
let sequence = 0;
let selectionSequence = 0;
let searchSequence = 0;
let timer: ReturnType<typeof setTimeout> | undefined;
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let normalizedFingerprint = "";
const declaration = computed(() => view.value ? purchaseDeclaration(view.value) : undefined);
const states = computed(() => Object.fromEntries((view.value?.fields || []).map(field => [field.fieldToken, { visible: field.visible, required: field.required, disabled: field.disabled || field.type === "file" }])));
const issues = computed(() => (evaluation.value?.issues || []).map(issue => ({ fieldKey: issue.fieldToken, issueCode: issue.issueCode })));
const hasFile = computed(() => view.value?.fields.some(field => field.type === "file" && field.visible));
const needsFile = computed(() => view.value?.fields.some(field => field.type === "file" && field.visible && field.required && !field.disabled));
const price = computed(() => terms.value.find(term => term.billingTermToken === selectedTerm.value));
function tr(key: string) { return $t(`plugin.idc-core.assistedCreate.${key}`); }
function localized(item: { nameZhCN?: string; nameEnUS?: string; labelZhCN?: string; labelEnUS?: string }) {
  return preferences.app.locale === "zh-CN" ? item.nameZhCN || item.labelZhCN || item.nameEnUS || item.labelEnUS : item.nameEnUS || item.labelEnUS || item.nameZhCN || item.labelZhCN;
}
function schema(): VbenFormProps["schema"] {
  return [
    { fieldName: "publicSlug", component: "Select", label: tr("product"), rules: "required", componentProps: {
      "data-testid": `assisted-item-product-${props.index}`, showSearch: true, filterOption: false, loading: searching.value,
      options: products.value.map(product => ({ value: product.publicSlug, label: localized(product), disabled: product.availability !== "available" })),
      onSearch: search, onChange: chooseProduct, disabled: props.busy || !props.currency,
    } },
    { fieldName: "billingTermToken", component: "Select", label: tr("billingTerm"), rules: "required", componentProps: {
      "data-testid": `assisted-item-term-${props.index}`, disabled: props.busy || evaluating.value || !terms.value.length,
      options: terms.value.map(term => ({ value: term.billingTermToken, label: localized(term) })), onChange: chooseTerm,
    } },
    { fieldName: "quantity", component: "InputNumber", label: tr("quantity"), defaultValue: 1, rules: z.number().int().min(1).max(1000), componentProps: { "data-testid": `assisted-item-quantity-${props.index}`, min: 1, max: 1000, precision: 0, disabled: props.busy } },
    { fieldName: "purchaseCycles", component: "InputNumber", label: tr("cycles"), defaultValue: 1, rules: z.number().int().min(price.value?.minimumPurchaseCycles || 1).max(120), componentProps: { "data-testid": `assisted-item-cycles-${props.index}`, min: price.value?.minimumPurchaseCycles || 1, max: 120, precision: 0, disabled: props.busy } },
  ];
}
const [Form, formApi] = useVbenForm({ commonConfig: { componentProps: { class: "w-full" }, labelWidth: 105 }, schema: [], showDefaultActions: false, wrapperClass: "grid-cols-1 md:grid-cols-2" });
function syncSchema() { void formApi.setState({ schema: schema() }); }
function search(value: string) {
  keyword.value = value;
  ++searchSequence;
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(() => void loadProducts(false), 250);
}
async function loadProducts(append = false) {
  if (props.allowedProducts) {
    products.value = props.allowedProducts;
    nextCursor.value = "";
    error.value = products.value.length ? "" : tr("noProducts");
    syncSchema();
    return;
  }
  if (!props.currency || (append && !nextCursor.value)) return;
  const current = ++searchSequence;
  searching.value = true; syncSchema();
  try {
    const result = await assistedProducts({ currency: props.currency, namePrefix: keyword.value.trim(), cursor: append ? nextCursor.value : undefined });
    if (current !== searchSequence) return;
    const selected = products.value.find(product => product.publicSlug === selectedSlug.value);
    const all = [...(append ? products.value : selected ? [selected] : []), ...result.products];
    products.value = [...new Map(all.map(product => [product.publicSlug, product])).values()];
    nextCursor.value = result.nextCursor;
    error.value = products.value.length ? "" : tr("noProducts");
  } catch { if (current === searchSequence) error.value = tr("productsFailed"); }
  finally { if (current === searchSequence) { searching.value = false; syncSchema(); } }
}
async function chooseProduct(slug: string) {
  const current = ++sequence;
  const selection = ++selectionSequence;
  if (timer) clearTimeout(timer);
  evaluating.value = false;
  normalizedFingerprint = "";
  selectedSlug.value = slug; selectedTerm.value = "";
  view.value = undefined; evaluation.value = undefined; terms.value = []; draft.value = {}; error.value = "";
  await formApi.setValues({ billingTermToken: null, purchaseCycles: 1 }, false);
  syncSchema(); loading.value = true;
  try {
    const response = await purchaseConfiguration(slug);
    if (current !== sequence) return;
    view.value = response;
    draft.value = purchaseDraft(response);
    normalizedFingerprint = JSON.stringify(draft.value);
    await evaluateNow();
  } catch { if (current === sequence) error.value = tr("configurationFailed"); }
  finally { if (selection === selectionSequence) loading.value = false; }
}
async function chooseTerm(token: string) {
  selectedTerm.value = token;
  const values = await formApi.getValues<{ purchaseCycles: number }>();
  if ((values.purchaseCycles || 1) < (price.value?.minimumPurchaseCycles || 1)) await formApi.setValues({ purchaseCycles: price.value!.minimumPurchaseCycles }, false);
  syncSchema();
}
async function evaluateNow() {
  if (!view.value || !props.currency) return false;
  if (timer) clearTimeout(timer);
  const current = ++sequence;
  const slug = selectedSlug.value;
  evaluating.value = true; error.value = ""; syncSchema();
  try {
    const result = await evaluatePurchase(slug, view.value.configurationRevision, purchaseValues(view.value, draft.value));
    if (current !== sequence) return false;
    evaluation.value = result; view.value = result;
    if (!result.valid) { terms.value = []; return false; }
    const normalized = purchaseDraft(result);
    normalizedFingerprint = JSON.stringify(normalized);
    draft.value = normalized;
    const quote = await quotePurchase(slug, result.configurationRevision, props.currency, result.values);
    if (current !== sequence) return false;
    configForm.value?.clearValidation();
    terms.value = quote.terms || [];
    if (!terms.value.some(term => term.billingTermToken === selectedTerm.value)) {
      selectedTerm.value = terms.value[0]?.billingTermToken || "";
      await formApi.setValues({ billingTermToken: selectedTerm.value || null }, false);
      if (selectedTerm.value) await chooseTerm(selectedTerm.value);
    }
    return terms.value.length > 0;
  } catch {
    if (current === sequence) { terms.value = []; evaluation.value = undefined; error.value = tr("configurationFailed"); }
    return false;
  } finally { if (current === sequence) { evaluating.value = false; syncSchema(); } }
}
watch(draft, () => {
  if (!view.value || JSON.stringify(draft.value) === normalizedFingerprint) return;
  ++sequence; evaluation.value = undefined; terms.value = []; syncSchema();
  if (timer) clearTimeout(timer);
  timer = setTimeout(() => void evaluateNow(), 300);
}, { deep: true });
watch(() => props.currency, () => {
  ++sequence; ++selectionSequence; ++searchSequence;
  if (timer) clearTimeout(timer);
  if (searchTimer) clearTimeout(searchTimer);
  timer = undefined; searchTimer = undefined;
  products.value = []; nextCursor.value = ""; keyword.value = "";
  loading.value = false; evaluating.value = false; searching.value = false;
  error.value = ""; evaluation.value = undefined; draft.value = {}; normalizedFingerprint = "";
  view.value = undefined; terms.value = []; selectedSlug.value = ""; selectedTerm.value = "";
  // Disable the default value merge: it ignores null and cannot clear Selects.
  void formApi.setValues({ publicSlug: null, billingTermToken: null, purchaseCycles: 1 }, false);
  syncSchema();
  void loadProducts();
});
watch(() => [props.busy, props.index, preferences.app.locale], syncSchema);
onMounted(async () => {
  await formApi.setState({ schema: schema() });
  await formApi.setValues({ quantity: 1, purchaseCycles: 1 }, false);
  void loadProducts();
});
onBeforeUnmount(() => { ++sequence; ++selectionSequence; ++searchSequence; if (timer) clearTimeout(timer); if (searchTimer) clearTimeout(searchTimer); });
async function getInput(): Promise<AssistedQuoteCreateItem> {
  if (!view.value || needsFile.value) throw new Error(needsFile.value ? tr("fileRequired") : tr("chooseProduct"));
  if (!await evaluateNow()) throw new Error(error.value || tr("invalidConfiguration"));
  await configForm.value?.validate();
  if (!(await formApi.validate()).valid) throw new Error(tr("checkForm"));
  const values = await formApi.getValues<{ quantity: number; purchaseCycles: number; billingTermToken: string }>();
  return { publicSlug: selectedSlug.value, configurationRevision: view.value.configurationRevision, billingTermToken: values.billingTermToken, quantity: values.quantity, purchaseCycles: values.purchaseCycles, values: evaluation.value!.values };
}
defineExpose({ getInput });
function displayAmount(value: string) {
  try { return `${props.currency} ${minorToMajorAmount(value, props.currency)}`; } catch { return "—"; }
}
</script>
<template>
  <Card :title="`${tr('item')} ${index + 1}`" size="small" :data-testid="`assisted-item-${index}`">
    <template v-if="removable !== false" #extra><Button :disabled="busy" danger type="link" :data-testid="`assisted-item-remove-${index}`" @click="emit('remove')">{{ tr('remove') }}</Button></template>
    <Form />
    <Button v-if="nextCursor" :loading="searching" :disabled="busy" size="small" data-testid="assisted-product-more" @click="loadProducts(true)">{{ tr('moreProducts') }}</Button>
    <Alert v-if="error" class="mt-3" :message="error" type="error" show-icon><template #action><Button size="small" @click="view ? evaluateNow() : selectedSlug ? chooseProduct(selectedSlug) : loadProducts()">{{ tr('retry') }}</Button></template></Alert>
    <Spin :spinning="loading || evaluating">
      <Alert v-if="hasFile" class="my-3" :message="tr(needsFile ? 'fileRequired' : 'fileNotice')" show-icon :type="needsFile ? 'warning' : 'info'" />
      <CatalogConfigurationForm v-if="declaration" ref="configForm" v-model="draft" :declaration="declaration" :field-states="states" :issues="issues" :readonly="busy" surface="customer" />
      <div v-if="price" class="rounded border p-3" :data-testid="`assisted-item-price-${index}`">
        <strong>{{ tr('referencePrice') }} · {{ localized(price) }}: {{ displayAmount(price.totalAmount) }}</strong>
        <p class="mb-0 mt-1 text-sm text-muted-foreground">{{ tr('priceHint') }}</p>
        <p v-if="price.status !== 'final'" class="mb-0 mt-1 text-sm text-muted-foreground">{{ tr('customerPrice') }}</p>
      </div>
    </Spin>
  </Card>
</template>

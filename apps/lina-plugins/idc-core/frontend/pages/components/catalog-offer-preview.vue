<script setup lang="ts">
// Customer choices use the generic declaration. Saved charges come from the
// read-only server preview; this component never creates an order or payment.
import type { VbenFormSchema } from "#/adapter/form";
import type {
  CatalogConfigurationField,
  CatalogConfigurationCandidate,
  CatalogConfigurationRule,
  CatalogConfigurationValues,
  CatalogConfigurationDeclaration,
} from "../catalog-configuration-state";
import { catalogConfigurationValuesToRows, validateCatalogConfigurationDraftValue } from '../catalog-configuration-state';
import { catalogConfigurationMessage } from '../catalog-configuration-i18n';
import { resolveOfferPreview, type OfferPreviewField } from '../catalog-offer-preview-state';
import { computed, markRaw, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { Alert, Button, Skeleton, Slider } from "ant-design-vue";
import { preferences } from "@vben/preferences";
import { $t } from "#/locales";
import { catalogEditorPricePreview, type CatalogEditorPricePreview } from "../catalog-editor-price-preview-client";
import { minorToMajorAmount } from "../catalog-money";
import { useVbenForm, z } from "#/adapter/form";

const props = defineProps<{ fields: OfferPreviewField[]; conditions?: CatalogConfigurationRule[]; productNo?: string; versionNo?: string; currencies?: string[]; pricingDirty?: boolean }>();
const tr = (key: string) => $t(`plugin.idc-core.catalog.optionPreview.${key}`);
const localized = (label: CatalogConfigurationField["label"]) =>
  preferences.app.locale === "zh-CN" ? label.zhCN || label.enUS : label.enUS || label.zhCN;
let rebuilding = false;
let sequence = 0;
let currentValues: Record<string, unknown> = {};
const priceText = (key: string) => $t(`plugin.idc-core.catalog.editorPricePreview.${key}`);
const priceResult = ref<CatalogEditorPricePreview>();
const priceLoading = ref(false);
const priceFailed = ref(false);
const currency = ref("");
const billingRuleNo = ref("");
const priceReady = computed(() => !!props.productNo && !!props.versionNo && !!currency.value && !props.pricingDirty);
const selectedTerm = computed(() => priceResult.value?.terms.find(term => term.billingRuleNo === billingRuleNo.value));
let priceSequence = 0;
let priceTimer: ReturnType<typeof setTimeout> | undefined;
function amount(value: string | null | undefined) {
  if (value === undefined || value === null) return "—";
  try { return `${minorToMajorAmount(value, currency.value)} ${currency.value}`; } catch { return "—"; }
}
function termLabel(term: NonNullable<CatalogEditorPricePreview>["terms"][number]) {
  const label = term.chargeType === "one_time" ? $t("plugin.idc-core.catalog.detail.code.period.once") : `${term.periodCount} ${$t(`plugin.idc-core.catalog.detail.code.period.${term.periodUnit}`)}${term.cycles > 1 ? ` × ${term.cycles}` : ""}`;
  return term.status === "unavailable" ? `${label} · ${priceText("unavailableLabel")}` : label;
}
const [PriceForm, priceForm] = useVbenForm({
  schema: [], showDefaultActions: false, wrapperClass: "grid-cols-1 sm:grid-cols-2 gap-x-3", commonConfig:{labelWidth:72},
  handleValuesChange(values) {
    const nextCurrency = String(values.currency || "");
    if (nextCurrency !== currency.value) { currency.value = nextCurrency; schedulePrice(); }
    billingRuleNo.value = String(values.billingRuleNo || "");
  },
});
async function renderPriceForm() {
  await priceForm.setState({schema:[
    {fieldName:"currency",component:"Select",label:priceText("currency"),componentProps:{virtual:false,options:(props.currencies || []).map(value=>({value,label:value})),disabled:priceLoading.value || !props.currencies?.length,"data-testid":"catalog-preview-currency"}},
    ...(priceResult.value?.terms.length ? [{fieldName:"billingRuleNo",component:"Select" as const,label:priceText("period"),componentProps:{virtual:false,options:priceResult.value.terms.map(term=>({value:term.billingRuleNo,label:termLabel(term)})),disabled:priceLoading.value,"data-testid":"catalog-preview-period"}}] : []),
  ]});
  await priceForm.setValues({currency:currency.value,billingRuleNo:billingRuleNo.value},false);
}
function schedulePrice() {
  if (priceTimer) clearTimeout(priceTimer);
  priceSequence++;
  priceResult.value = undefined;
  priceFailed.value = false;
  priceLoading.value = false;
  if (!priceReady.value) return;
  priceTimer = setTimeout(() => void loadPrice(), 350);
}
async function loadPrice() {
  if (!priceReady.value) return;
  const request = ++priceSequence;
  priceLoading.value = true; priceFailed.value = false; priceResult.value = undefined;
  try {
    const fields = safeFields.value.filter(field => field.type !== "file");
    const rows = catalogConfigurationValuesToRows({version:"v1",fields,conditions:props.conditions},"customer",currentValues as CatalogConfigurationValues);
    const result = await catalogEditorPricePreview(props.productNo!,props.versionNo!,currency.value,rows);
    if (request !== priceSequence) return;
    priceResult.value = result.preview;
    if (!result.preview.terms.some(term=>term.billingRuleNo===billingRuleNo.value)) billingRuleNo.value = (result.preview.terms.find(term=>term.status==="final") || result.preview.terms.find(term=>term.status!=="unavailable") || result.preview.terms[0])?.billingRuleNo || "";
  } catch { if (request === priceSequence) priceFailed.value = true; }
  finally { if (request === priceSequence) { priceLoading.value = false; await renderPriceForm(); } }
}
watch(() => [props.productNo,props.versionNo,props.pricingDirty,props.currencies,preferences.app.locale], () => {
  if (!props.currencies?.includes(currency.value)) currency.value = props.currencies?.[0] || "";
  void renderPriceForm(); schedulePrice();
}, {immediate:true,deep:true});
onBeforeUnmount(() => { priceSequence++; if (priceTimer) clearTimeout(priceTimer); });
const [Form, formApi] = useVbenForm({
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
  commonConfig: { labelWidth: 135 },
  handleValuesChange: values => {
    if (!rebuilding) void renderPreview(values);
  },
});
const safeFields = computed(() =>
  props.fields.filter((field) => field.type !== "secret"),
);
function hierarchy(
  choices: CatalogConfigurationCandidate[],
  parent = "",
  visited = new Set<string>(),
): Array<{ value: string; label: string; children?: unknown[] }> {
  return choices
    .filter(
      (choice) =>
        (choice.parentValue || "") === parent && !visited.has(choice.value),
    )
    .map((choice) => {
      const children = hierarchy(
        choices,
        choice.value,
        new Set([...visited, choice.value]),
      );
      return {
        value: choice.value,
        label: localized(choice.label),
        ...(children.length ? { children } : {}),
      };
    });
}
function schema(field: OfferPreviewField, declaration: CatalogConfigurationDeclaration, values: CatalogConfigurationValues): VbenFormSchema {
  const choices = field.staticCandidates || [];
  const options = choices.map((choice) => ({
    value: choice.value,
    label: localized(choice.label),
  }));
  let component: VbenFormSchema["component"] = "Input";
  let componentProps: Record<string, unknown> = {
    maxlength: field.maximumLength,
    "data-testid": `catalog-option-preview-${field.key}`,
    disabled: field.disabled,
  };
  if (
    field.type === "integer" &&
    !["radio", "select"].includes(field.displayControl || "")
  ) {
    component =
      field.displayControl === "slider" ? markRaw(Slider) : "InputNumber";
    Object.assign(componentProps, {
      min: Number(field.minimum || "0"),
      max: Number(field.maximum || "9007199254740991"),
      step: Number(field.quantityStep || "1"),
      precision: 0,
    });
  } else if (
    ["single_select", "multi_select", "integer"].includes(field.type)
  ) {
    component = field.displayControl === "radio" ? "RadioGroup" : "Select";
    Object.assign(componentProps, {
      options,
      ...(field.type === "multi_select" ? { mode: "multiple" } : {}),
      ...(component === "RadioGroup"
        ? { optionType: "button", buttonStyle: "solid" }
        : { virtual: false }),
    });
  } else if (["address", "tree_select"].includes(field.type)) {
    component = field.type === "address" ? "Cascader" : "TreeSelect";
    Object.assign(
      componentProps,
      component === "Cascader"
        ? { options: hierarchy(choices) }
        : { treeData: hierarchy(choices), treeCheckable: true },
    );
  } else if (field.type === "boolean") component = "Switch";
  else if (field.type === "multi_line") component = "Textarea";
  else if (["date", "datetime"].includes(field.type)) {
    component = "DatePicker";
    Object.assign(componentProps, {
      showTime: field.type === "datetime",
      valueFormat: field.type === 'datetime' ? 'YYYY-MM-DDTHH:mm:ssZ' : 'YYYY-MM-DD',
    });
  } else if (field.type === "file") {
    componentProps = { disabled: true, placeholder: tr("fileHint") };
  }
  return {
    fieldName: field.key,
    label: `${field.group ? `${field.group} / ` : ''}${localized(field.label)}${field.unit ? `（${field.unit}）` : ""}`,
    component,
    componentProps,
    help: field.description && localized(field.description).trim() ? localized(field.description) : undefined,
    rules: field.type === 'file' ? undefined : z.any().superRefine((value, context) => {
      const issue = validateCatalogConfigurationDraftValue(declaration, field, {
        ...values, [field.key]: typeof value === 'number' ? String(value) : value,
      });
      if (issue) context.addIssue({ code: 'custom', message: catalogConfigurationMessage(issue) });
    }),
    ...(field.displayControl === "slider" ? { modelPropName: "value" } : {}),
  };
}
async function renderPreview(values: Record<string, unknown>) {
  const request = ++sequence;
  rebuilding = true;
  try {
    const declaration = { version: 'v1', fields: safeFields.value, conditions: props.conditions };
    const resolved = resolveOfferPreview(declaration, values);
    currentValues = resolved.values;
    schedulePrice();
    await formApi.setState({ schema: resolved.fields.map(field => schema(field, declaration, resolved.validationValues)) });
    await nextTick();
    if (request === sequence && safeFields.value.length) await formApi.setValues(currentValues, false);
  } finally { if (request === sequence) rebuilding = false; }
}
watch(() => [safeFields.value, props.conditions], () => void renderPreview(currentValues), { immediate: true, deep: true });
</script>

<template>
  <section class="offer-preview" data-testid="catalog-offer-preview">
    <header>
      <div class="preview-heading">
        <h4>{{ tr("title") }}</h4>
        <strong v-if="selectedTerm && selectedTerm.status !== 'unavailable'" class="preview-heading-total" data-testid="catalog-preview-header-total">{{ priceText('total') }} {{ amount(selectedTerm.totalAmountMinor) }}</strong>
      </div>
      <p>{{ priceText("hint") }}</p>
    </header>
    <Form v-if="safeFields.length" />
    <Alert v-else type="info" :message="tr('empty')" />
    <section class="preview-price-summary" data-testid="catalog-preview-price-summary">
      <h4>{{ priceText('title') }}</h4>
      <PriceForm />
      <Alert v-if="pricingDirty" type="info" show-icon data-testid="catalog-preview-price-dirty" :message="priceText('saveFirst')" />
      <Alert v-else-if="!currencies?.length" type="info" :message="priceText('noPrices')" />
      <Skeleton v-else-if="priceLoading" active :paragraph="{rows:3}" />
      <Alert v-else-if="priceFailed" type="warning" show-icon data-testid="catalog-preview-price-failed" :message="priceText('failed')"><template #action><Button size="small" @click="loadPrice">{{ priceText('retry') }}</Button></template></Alert>
      <Alert v-else-if="selectedTerm?.status === 'unavailable'" type="warning" show-icon data-testid="catalog-preview-price-unavailable" :message="priceText('unavailable')">
        <template #description>
          <ul class="preview-price-issues">
            <li v-for="(issue,index) in selectedTerm.issues || []" :key="index">{{ issue.kind === 'base' ? priceText('base') : localized({zhCN:issue.nameZhCN,enUS:issue.nameEnUS}) || issue.fieldKey || priceText('configuration') }}：{{ priceText(issue.reason === 'missing' ? 'missingPrice' : 'invalidPrice') }}</li>
          </ul>
          <p>{{ priceText('configurePrice') }}</p>
        </template>
      </Alert>
      <template v-else-if="selectedTerm">
        <dl class="preview-price-breakdown">
          <div><dt>{{ priceText('base') }}</dt><dd data-testid="catalog-preview-base-amount">{{ amount(selectedTerm.baseAmountMinor) }}</dd></div>
          <div><dt>{{ priceText('configuration') }}</dt><dd data-testid="catalog-preview-configuration-amount">{{ amount(selectedTerm.configurationAmountMinor) }}</dd></div>
          <div><dt>{{ priceText('setup') }}</dt><dd data-testid="catalog-preview-setup-amount">{{ amount(selectedTerm.setupAmountMinor) }}</dd></div>
          <div><dt>{{ priceText(priceResult?.priceIncludesTax ? 'includedTax' : 'tax') }}</dt><dd data-testid="catalog-preview-tax-amount">{{ amount(selectedTerm.taxAmountMinor) }}</dd></div>
          <div class="preview-price-total"><dt>{{ priceText('total') }}</dt><dd data-testid="catalog-preview-total-amount">{{ amount(selectedTerm.totalAmountMinor) }}</dd></div>
        </dl>
        <p v-if="selectedTerm.status !== 'final'">{{ priceText(selectedTerm.status === 'pending_usage' ? 'usage' : 'customerContext') }}</p>
        <p>{{ priceText('savedHint') }}</p>
      </template>
    </section>
  </section>
</template>

<style scoped>
.preview-heading { display:flex; align-items:baseline; justify-content:space-between; flex-wrap:wrap; gap:8px 16px; }
.preview-heading-total { color:hsl(var(--primary)); font-size:16px; font-variant-numeric:tabular-nums; white-space:nowrap; }
.preview-price-summary { border-top: 1px solid hsl(var(--border)); margin-top: 20px; padding-top: 18px; }
.preview-price-issues { margin: 8px 0; padding-left: 18px; list-style: disc; }
.preview-price-summary h4 { margin-bottom: 16px; }
.preview-price-breakdown { margin: 14px 0 0; }
.preview-price-breakdown > div { display: flex; justify-content: space-between; gap: 16px; padding: 6px 0; }
.preview-price-breakdown dd { margin: 0; font-variant-numeric: tabular-nums; white-space: nowrap; }
.preview-price-total { border-top: 1px solid hsl(var(--border)); margin-top: 8px; font-size: 16px; font-weight: 600; }
.offer-preview {
  padding: 20px;
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
  background: hsl(var(--background));
}
.offer-preview header {
  padding-bottom: 16px;
  margin-bottom: 18px;
  border-bottom: 1px solid hsl(var(--border));
}
.offer-preview h4 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}
.offer-preview p {
  margin: 6px 0 0;
  font-size: 12px;
  color: hsl(var(--muted-foreground));
}
</style>

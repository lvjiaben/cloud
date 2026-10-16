<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { CatalogBillingRule, CatalogConfigurationPriceCandidate } from "../catalog-client";

import { ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { Alert, Button, message } from "ant-design-vue";
import { preferences } from "@vben/preferences";

import { $t } from "#/locales";
import { useVbenForm, z } from "#/adapter/form";

import {
  catalogConfigurationPriceCandidates,
  catalogConfigurationPriceDimensionCreate,
  catalogConfigurationPriceScheduleCreate,
} from "../catalog-client";
import { currencyOptions } from "../currency-client";
import { majorToMinorAmount } from "../catalog-money";

const emit = defineEmits<{ success: [] }>();
const versionNo = ref("");
const rules = ref<CatalogBillingRule[]>([]);
const candidates = ref<CatalogConfigurationPriceCandidate[]>([]);
const tierCount = ref(1);
const quantitySelected = ref(false);
let candidateOptions: Array<{label: string; value: string}> = [];
let currencies: string[] = [];
const selectedCandidate = (values: Record<string, unknown>) => candidates.value.find(item => item.candidateToken === values.candidateToken);


function localized(zh: string, en: string) {
  return preferences.app.locale === "zh-CN" ? zh || en : en || zh;
}

function billingRuleLabel(rule: CatalogBillingRule) {
  const charge = $t(
    `plugin.idc-core.catalog.detail.code.charge.${rule.chargeType}`,
  );
  const period = $t(
    `plugin.idc-core.catalog.detail.code.period.${rule.periodUnit}`,
  );
  const phase = $t(
    `plugin.idc-core.catalog.detail.code.pricePhase.${rule.pricePhase}`,
  );
  return `${charge} · ${rule.periodCount} × ${period} · ${phase}`;
}

function priceSchema(candidateOptions: Array<{ label: string; value: string }>, currencyCodes: string[]): VbenFormProps["schema"] {
  const schema: NonNullable<VbenFormProps["schema"]> = [
    { component: "Select", componentProps: { options: candidateOptions, showSearch: true, optionFilterProp: "label", onChange: async (token: string) => { const selected = candidates.value.find(item => item.candidateToken === token); quantitySelected.value = !!selected?.quantity; await formApi.setValues({ pricingMode: selected?.pricingMode || "per_unit" }); } }, fieldName: "candidateToken", label: $t("plugin.idc-core.catalog.configurationPrice.candidate"), rules: z.string().min(1) },
    { component: "Select", componentProps: { options: rules.value.map((rule) => ({ label: billingRuleLabel(rule), value: rule.billingRuleNo })) }, fieldName: "billingRuleNo", label: $t("plugin.idc-core.catalog.configurationPrice.billingRule"), rules: z.string().min(1) },
    { component: "Select", componentProps: { options: currencyCodes.map((value) => ({ label: value, value })), showSearch: true }, fieldName: "currencyCode", label: $t("plugin.idc-core.catalog.configurationPrice.currency"), rules: z.string().regex(/^[A-Z]{3}$/) },
    { component: "Select", componentProps: { "data-testid": "catalog-configuration-price-mode", options: [{ label: $t("plugin.idc-core.catalog.configurationPrice.perUnit"), value: "per_unit" }, { label: $t("plugin.idc-core.catalog.configurationPrice.bandTotal"), value: "band_total" }] }, fieldName: "pricingMode", label: $t("plugin.idc-core.catalog.configurationPrice.mode"), rules: z.string().min(1), dependencies: { triggerFields: ["candidateToken"], if: values => !!selectedCandidate(values)?.quantity, disabled: values => !!selectedCandidate(values)?.dimensionNo } },

  ];
  for (let index = 0; index < tierCount.value; index++) {
    const suffix = index === 0 ? "" : `_${index}`;
    if (index > 0) schema.push({ component: "InputNumber", componentProps: { min: 1, max: 2147483646, precision: 0, "data-testid": `catalog-configuration-price-minimum${suffix}` }, fieldName: `minimum${suffix}`, label: $t("plugin.idc-core.catalog.configurationPrice.bandStart", { number: index + 1 }), rules: z.number().int().min(1).max(2147483646), dependencies: { triggerFields: ["candidateToken"], if: values => !!selectedCandidate(values)?.quantity } });
    for (const field of ["amount", "setupFee", "renewalAmount"] as const) {
      schema.push({ component: "Input", componentProps: { inputmode: "decimal", maxlength: 24, "data-testid": `catalog-configuration-price-${field}${suffix}` }, fieldName: `${field}${suffix}`, label: $t(`plugin.idc-core.catalog.configurationPrice.${field}`), rules: field === "renewalAmount" ? z.union([z.literal(""), z.string().regex(/^\d{1,19}(?:\.\d{1,4})?$/)]) : z.string().regex(/^\d{1,19}(?:\.\d{1,4})?$/), dependencies: { triggerFields: ["candidateToken"], if: values => index === 0 || !!selectedCandidate(values)?.quantity } });
    }
  }
  return schema;
}

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 180 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "idc-plugin-form-surface",
});

const [Modal, modalApi] = useVbenModal({
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid || !versionNo.value) return false;
    const values = await formApi.getValues<{
      amount: string;
      billingRuleNo: string;
      candidateToken: string;
      currencyCode: string;
      renewalAmount: string;
      setupFee: string;
    }>();
    const selectedRule = rules.value.find((item) => item.billingRuleNo === values.billingRuleNo);
    if (!selectedRule || (selectedRule.pricePhase !== "new_purchase" && !values.renewalAmount)) {
      message.error($t("plugin.idc-core.catalog.configurationPrice.renewalRequired"));
      return false;
    }
    const allValues = await formApi.getValues<Record<string, string | number>>();
    const choice = candidates.value.find(item => item.candidateToken === values.candidateToken);
    const mode = choice?.quantity ? choice.pricingMode || String(allValues.pricingMode) : "fixed";
    const tiers: Array<{ amountMinor: string; setupFeeMinor: string; renewalAmountMinor?: string; minimumQuantity: number; maximumQuantity: number | null }> = [];
    try {
      for (let index = 0; index < (choice?.quantity ? tierCount.value : 1); index++) {
        const suffix = index === 0 ? "" : `_${index}`;
        const renewal = String(allValues[`renewalAmount${suffix}`] ?? "");
        if (selectedRule.pricePhase !== "new_purchase" && !renewal) throw new Error("renewal_required");
        const minimum = index === 0 ? 0 : Number(allValues[`minimum${suffix}`]);
        if (!Number.isSafeInteger(minimum) || minimum < 0 || minimum > 2147483646) throw new Error("invalid_band");
        if (index > 0 && minimum <= tiers[index - 1]!.minimumQuantity) throw new Error("invalid_band");
        if (index > 0) tiers[index - 1]!.maximumQuantity = minimum;
        tiers.push({ amountMinor: majorToMinorAmount(String(allValues[`amount${suffix}`]), values.currencyCode), setupFeeMinor: majorToMinorAmount(String(allValues[`setupFee${suffix}`]), values.currencyCode), minimumQuantity: minimum, maximumQuantity: null, ...(renewal ? { renewalAmountMinor: majorToMinorAmount(renewal, values.currencyCode) } : {}) });
      }
    } catch {
      message.error($t("plugin.idc-core.catalog.configurationPrice.invalidAmount"));
      return false;
    }
    modalApi.lock(true);
    try {
      const latest = (await catalogConfigurationPriceCandidates(versionNo.value)).candidates;
      const candidate = latest.find((item) => item.candidateToken === values.candidateToken);
      if (!candidate || (candidate.quantity && candidate.pricingMode && candidate.pricingMode !== mode)) {
        message.error($t("plugin.idc-core.catalog.configurationPrice.changed"));
        return false;
      }
      const dimensionNo = candidate.dimensionNo ||
        (await catalogConfigurationPriceDimensionCreate(versionNo.value, candidate.candidateToken, mode)).dimensionNo;
      await catalogConfigurationPriceScheduleCreate(versionNo.value, values.billingRuleNo, {
        configurationPriceDimensionNo: dimensionNo,
        currencyCode: values.currencyCode,
        status: "active",
        tiers,
      });
      message.success($t("plugin.idc-core.catalog.configurationPrice.saved"));
      emit("success");
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{ billingRules?: CatalogBillingRule[]; currencyCodes?: string[]; versionNo?: string }>();
    versionNo.value = data?.versionNo || "";
    rules.value = data?.billingRules ?? [];
    tierCount.value = 1;
    candidates.value = (await catalogConfigurationPriceCandidates(versionNo.value)).candidates;
    quantitySelected.value = !!candidates.value[0]?.quantity;
    const options = candidates.value.map((candidate) => ({
      value: candidate.candidateToken,
      label: `${localized(candidate.fieldLabelZhCN, candidate.fieldLabelEnUS)} · ${localized(candidate.candidateLabelZhCN, candidate.candidateLabelEnUS)}${candidate.dimensionNo ? ` · ${$t("plugin.idc-core.catalog.configurationPrice.created")}` : ""}`,
    }));
    currencies = (await currencyOptions()).currencies;
    candidateOptions = options;
    await formApi.setState({ schema: priceSchema(options, currencies) });
    await formApi.setValues({
      candidateToken: options[0]?.value,
      pricingMode: candidates.value[0]?.pricingMode || "per_unit",
      billingRuleNo: rules.value[0]?.billingRuleNo,
      currencyCode: currencies[0],
      amount: "0.00",
      setupFee: "0.00",
      renewalAmount: "",
    });
  },
});
async function addBand() {
  if (tierCount.value >= 16) return;
  const values = await formApi.getValues<Record<string, unknown>>();
  if (!selectedCandidate(values)?.quantity) return;
  const index = tierCount.value++;
  const previousMinimum = index === 1 ? 0 : Number(values[`minimum_${index - 1}`] ?? 0);
  await formApi.setState({ schema: priceSchema(candidateOptions, currencies) });
  await formApi.setValues({ ...values, [`minimum_${index}`]: previousMinimum + 100, [`amount_${index}`]: "0.00", [`setupFee_${index}`]: "0.00", [`renewalAmount_${index}`]: "" });
}
async function removeBand() {
  if (tierCount.value <= 1) return;
  const values = await formApi.getValues<Record<string, unknown>>();
  tierCount.value--;
  await formApi.setState({ schema: priceSchema(candidateOptions, currencies) });
  await formApi.setValues(values);
}
</script>

<template>
  <Modal :title="$t('plugin.idc-core.catalog.configurationPrice.title')" class="w-[680px]">
    <div data-testid="catalog-configuration-price-modal">
      <Alert v-if="quantitySelected" class="mb-4" type="info" :message="$t('plugin.idc-core.catalog.configurationPrice.bandHint')" />
      <Form />
      <Button v-if="quantitySelected" class="mr-2" data-testid="catalog-configuration-price-add-band" :disabled="tierCount >= 16" @click="addBand">{{ $t('plugin.idc-core.catalog.configurationPrice.addBand') }}</Button>
      <Button v-if="quantitySelected && tierCount > 1" data-testid="catalog-configuration-price-remove-band" @click="removeBand">{{ $t("plugin.idc-core.catalog.configurationPrice.removeBand") }}</Button>
    </div>
  </Modal>
</template>

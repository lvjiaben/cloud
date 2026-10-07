<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { CatalogBillingRule } from "../catalog-client";

import { ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { message } from "ant-design-vue";
import { preferences } from "@vben/preferences";

import { $t } from "#/locales";
import { useVbenForm, z } from "#/adapter/form";

import {
  catalogConfigurationPriceCandidates,
  catalogConfigurationPriceDimensionCreate,
  catalogConfigurationPriceScheduleCreate,
} from "../catalog-client";
import { majorToMinorAmount } from "../catalog-money";

const emit = defineEmits<{ success: [] }>();
const versionNo = ref("");
const rules = ref<CatalogBillingRule[]>([]);

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
  return [
    { component: "Select", componentProps: { options: candidateOptions, showSearch: true, optionFilterProp: "label" }, fieldName: "candidateToken", label: $t("plugin.idc-core.catalog.configurationPrice.candidate"), rules: z.string().min(1) },
    { component: "Select", componentProps: { options: rules.value.map((rule) => ({ label: billingRuleLabel(rule), value: rule.billingRuleNo })) }, fieldName: "billingRuleNo", label: $t("plugin.idc-core.catalog.configurationPrice.billingRule"), rules: z.string().min(1) },
    { component: "Select", componentProps: { options: currencyCodes.map((value) => ({ label: value, value })), showSearch: true }, fieldName: "currencyCode", label: $t("plugin.idc-core.catalog.configurationPrice.currency"), rules: z.string().regex(/^[A-Z]{3}$/) },
    { component: "Input", componentProps: { inputmode: "decimal", maxlength: 24 }, fieldName: "amount", label: $t("plugin.idc-core.catalog.configurationPrice.amount"), rules: z.string().regex(/^\d{1,19}(?:\.\d{1,4})?$/) },
    { component: "Input", componentProps: { inputmode: "decimal", maxlength: 24 }, fieldName: "setupFee", label: $t("plugin.idc-core.catalog.configurationPrice.setupFee"), rules: z.string().regex(/^\d{1,19}(?:\.\d{1,4})?$/) },
    { component: "Input", componentProps: { inputmode: "decimal", maxlength: 24 }, fieldName: "renewalAmount", label: $t("plugin.idc-core.catalog.configurationPrice.renewalAmount"), rules: z.union([z.literal(""), z.string().regex(/^\d{1,19}(?:\.\d{1,4})?$/)]) },
  ];
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
    const candidates = (await catalogConfigurationPriceCandidates(versionNo.value)).candidates;
    const candidate = candidates.find((item) => item.candidateToken === values.candidateToken);
    if (!candidate) return false;
    const selectedRule = rules.value.find((item) => item.billingRuleNo === values.billingRuleNo);
    if (!selectedRule || (selectedRule.pricePhase !== "new_purchase" && !values.renewalAmount)) {
      message.error($t("plugin.idc-core.catalog.configurationPrice.renewalRequired"));
      return false;
    }
    let dimensionNo = candidate.dimensionNo;
    if (!dimensionNo) {
      dimensionNo = (await catalogConfigurationPriceDimensionCreate(versionNo.value, candidate.candidateToken)).dimensionNo;
    }
    let amountMinor: string;
    let setupFeeMinor: string;
    let renewalAmountMinor: string | undefined;
    try {
      amountMinor = majorToMinorAmount(values.amount, values.currencyCode);
      setupFeeMinor = majorToMinorAmount(values.setupFee, values.currencyCode);
      renewalAmountMinor = values.renewalAmount ? majorToMinorAmount(values.renewalAmount, values.currencyCode) : undefined;
    } catch {
      message.error($t("plugin.idc-core.catalog.configurationPrice.invalidAmount"));
      return false;
    }
    await catalogConfigurationPriceScheduleCreate(versionNo.value, values.billingRuleNo, {
      configurationPriceDimensionNo: dimensionNo,
      currencyCode: values.currencyCode,
      status: "active",
      tiers: [{ amountMinor, setupFeeMinor, minimumQuantity: 0, maximumQuantity: null, ...(renewalAmountMinor ? { renewalAmountMinor } : {}) }],
    });
    message.success($t("plugin.idc-core.catalog.configurationPrice.saved"));
    emit("success");
    modalApi.close();
  },
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{ billingRules?: CatalogBillingRule[]; currencyCodes?: string[]; versionNo?: string }>();
    versionNo.value = data?.versionNo || "";
    rules.value = data?.billingRules ?? [];
    const candidates = (await catalogConfigurationPriceCandidates(versionNo.value)).candidates;
    const options = candidates.map((candidate) => ({
      value: candidate.candidateToken,
      label: `${localized(candidate.fieldLabelZhCN, candidate.fieldLabelEnUS)} · ${localized(candidate.candidateLabelZhCN, candidate.candidateLabelEnUS)}${candidate.dimensionNo ? ` · ${$t("plugin.idc-core.catalog.configurationPrice.created")}` : ""}`,
    }));
    const currencies = data?.currencyCodes?.length ? data.currencyCodes : ["CNY"];
    await formApi.setState({ schema: priceSchema(options, currencies) });
    await formApi.setValues({
      candidateToken: options[0]?.value,
      billingRuleNo: rules.value[0]?.billingRuleNo,
      currencyCode: currencies[0],
      amount: "0.00",
      setupFee: "0.00",
      renewalAmount: "",
    });
  },
});
</script>

<template>
  <Modal :title="$t('plugin.idc-core.catalog.configurationPrice.title')" class="w-[680px]">
    <Form />
  </Modal>
</template>

<script setup lang="ts">
// This form creates a normal billing period or a capability-gated trial period.
import { ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { catalogBillingRuleCreate } from "../catalog-client";

const emit = defineEmits<{ success: [] }>();
let versionNo = "";
const trial = ref(false);
const [Form, formApi] = useVbenForm({
  commonConfig: { labelWidth: 140, componentProps: { class: "w-full" } },
  schema: [], showDefaultActions: false,
});
const [Modal, modalApi] = useVbenModal({
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{ versionNo: string; trial?: boolean }>();
    versionNo = data.versionNo;
    trial.value = data.trial === true;
    await formApi.resetForm();
    await formApi.setState({ schema: [
      { component: "Select", fieldName: "periodUnit", label: $t(trial.value ? "plugin.idc-core.catalog.commerce.trialUnit" : "plugin.idc-core.catalog.commerce.period"), componentProps: { "data-testid": "catalog-billing-period", options: (trial.value ? ["hour", "day", "month", "year"] : ["once", "hour", "day", "month", "year"]).map(value => ({ value, label: $t(`plugin.idc-core.catalog.detail.code.period.${value}`) })) }, rules: trial.value ? z.enum(["hour", "day", "month", "year"]) : z.enum(["once", "hour", "day", "month", "year"]) },
      { component: "InputNumber", fieldName: "periodCount", label: $t(trial.value ? "plugin.idc-core.catalog.commerce.trialDuration" : "plugin.idc-core.catalog.commerce.periodCount"), componentProps: { "data-testid": "catalog-billing-count", min: 1, max: 87600, precision: 0 }, rules: z.number().int().min(1).max(87600), dependencies: { triggerFields: ["periodUnit"], if: values => values.periodUnit !== "once" } },
      { component: "InputNumber", fieldName: "minimumPurchaseCycles", label: $t("plugin.idc-core.catalog.commerce.minimumCycles"), componentProps: { min: 1, max: 120, precision: 0 }, rules: z.number().int().min(1).max(120), dependencies: { triggerFields: ["periodUnit"], if: values => !trial.value && values.periodUnit !== "once" } },
      { component: "InputNumber", fieldName: "maximumRenewalCycles", label: $t("plugin.idc-core.catalog.commerce.maximumRenewal"), componentProps: { min: 1, max: 120, precision: 0 }, rules: z.number().int().min(1).max(120).optional(), dependencies: { triggerFields: ["periodUnit"], if: values => !trial.value && values.periodUnit !== "once" } },
      ...(trial.value ? [{ component: "InputNumber", fieldName: "trialQuantity", label: $t("plugin.idc-core.catalog.commerce.trialLimit"), componentProps: { "data-testid": "catalog-trial-limit", min: 1, max: 100, precision: 0 }, rules: z.number().int().min(1).max(100) }] : []),
    ] });
    await formApi.setValues({ periodUnit: trial.value ? "day" : "month", periodCount: 1, minimumPurchaseCycles: 1, maximumRenewalCycles: undefined, trialQuantity: 1 });
  },
  async onConfirm() {
    if (!(await formApi.validate()).valid) return false;
    const values = await formApi.getValues<{ periodUnit: "once" | "hour" | "day" | "month" | "year"; periodCount: number; minimumPurchaseCycles: number; maximumRenewalCycles?: number; trialQuantity?: number }>();
    const once = values.periodUnit === "once";
    modalApi.lock(true);
    try {
      await catalogBillingRuleCreate(versionNo, {
        ruleCode: `plan_${crypto.randomUUID().replaceAll("-", "")}`,
        chargeType: trial.value ? "trial" : once ? "one_time" : "recurring", periodUnit: values.periodUnit,
        periodCount: once ? 1 : values.periodCount, pricePhase: once || trial.value ? "new_purchase" : "both",
        quantityMode: "fixed", trialQuantity: trial.value ? values.trialQuantity! : 0, minimumPurchaseCycles: once || trial.value ? 1 : values.minimumPurchaseCycles,
        ...(!once && !trial.value && values.maximumRenewalCycles ? { maximumRenewalCycles: values.maximumRenewalCycles } : {}), status: "active",
      });
      message.success($t("plugin.idc-core.catalog.commerce.saved"));
      emit("success"); modalApi.close();
    } finally { modalApi.lock(false); }
  },
});
</script>
<template>
  <Modal :title="$t(trial ? 'plugin.idc-core.catalog.commerce.addTrial' : 'plugin.idc-core.catalog.commerce.addPeriod')" class="w-[560px]">
    <div data-testid="catalog-billing-modal"><p v-if="trial" class="text-muted-foreground mb-4">{{ $t("plugin.idc-core.catalog.commerce.trialHint") }}</p><Form /></div>
  </Modal>
</template>

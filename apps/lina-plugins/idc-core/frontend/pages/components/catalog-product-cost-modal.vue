<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { CatalogProductVersionCost } from "../catalog-client";

import { ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { message } from "ant-design-vue";
import dayjs from "dayjs";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { catalogProductVersionCostCreate, catalogProductVersionCostUpdate } from "../catalog-client";
import { majorToMinorAmount, minorToMajorAmount } from "../catalog-money";

const emit = defineEmits<{ success: [] }>();
const versionNo = ref("");
const editing = ref<CatalogProductVersionCost>();

function schema(currencyCodes: string[]): VbenFormProps["schema"] {
  return [
    { component: "Select", componentProps: { options: currencyCodes.map((value) => ({ label: value, value })), showSearch: true }, fieldName: "currencyCode", label: $t("plugin.idc-core.catalog.cost.currency"), rules: z.string().regex(/^[A-Z]{3}$/) },
    { component: "Select", componentProps: { options: ["once", "hour", "day", "month", "year"].map((value) => ({ label: $t(`plugin.idc-core.catalog.detail.code.period.${value}`), value })) }, fieldName: "periodUnit", label: $t("plugin.idc-core.catalog.cost.periodUnit"), rules: z.enum(["once", "hour", "day", "month", "year"]) },
    { component: "InputNumber", componentProps: { min: 1, precision: 0 }, fieldName: "periodCount", label: $t("plugin.idc-core.catalog.cost.periodCount"), rules: z.coerce.number().int().min(1).max(87600) },
    { component: "Input", componentProps: { "data-testid": "catalog-cost-unit-amount", inputmode: "decimal", maxlength: 24 }, fieldName: "unitCostAmount", label: $t("plugin.idc-core.catalog.cost.unitCost"), rules: z.string().regex(/^\d{1,19}(?:\.\d{1,4})?$/) },
    { component: "DatePicker", componentProps: { showTime: true, valueFormat: "x" }, fieldName: "effectiveFrom", label: $t("plugin.idc-core.catalog.cost.effectiveFrom") },
    { component: "DatePicker", componentProps: { showTime: true, valueFormat: "x" }, fieldName: "effectiveUntil", label: $t("plugin.idc-core.catalog.cost.effectiveUntil") },
  ];
}

const [Form, formApi] = useVbenForm({ commonConfig: { componentProps: { class: "w-full" }, labelWidth: 180 }, schema: [], showDefaultActions: false, wrapperClass: "idc-plugin-form-surface" });
const [Modal, modalApi] = useVbenModal({
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid || !versionNo.value) return false;
    const values = await formApi.getValues<Record<string, any>>();
    let unitCostMinor = "";
    try {
      unitCostMinor = majorToMinorAmount(String(values.unitCostAmount), String(values.currencyCode));
    } catch {
      message.error($t("plugin.idc-core.catalog.cost.invalidAmount"));
      return false;
    }
    const input = { currencyCode: values.currencyCode, periodUnit: values.periodUnit, periodCount: Number(values.periodCount), unitCostMinor, ...(values.effectiveFrom ? { effectiveFrom: Number(values.effectiveFrom) } : {}), ...(values.effectiveUntil ? { effectiveUntil: Number(values.effectiveUntil) } : {}) };
    if (editing.value) await catalogProductVersionCostUpdate(versionNo.value, editing.value.costNo, { ...input, expectedVersion: editing.value.version });
    else await catalogProductVersionCostCreate(versionNo.value, input);
    message.success($t("plugin.idc-core.catalog.cost.saved"));
    emit("success"); modalApi.close();
  },
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{ cost?: CatalogProductVersionCost; currencyCodes?: string[]; versionNo?: string }>();
    versionNo.value = data?.versionNo || ""; editing.value = data?.cost;
    await formApi.setState({ schema: schema(data?.currencyCodes?.length ? data.currencyCodes : ["CNY"]) });
    const currencyCode = data?.cost?.currencyCode || data?.currencyCodes?.[0] || "CNY";
    await formApi.setValues({ currencyCode, periodUnit: data?.cost?.periodUnit || "month", periodCount: data?.cost?.periodCount || 1, unitCostAmount: data?.cost ? minorToMajorAmount(data.cost.unitCostMinor, currencyCode) : "0.00", effectiveFrom: data?.cost ? String(data.cost.effectiveFrom) : String(dayjs().valueOf()), effectiveUntil: data?.cost?.effectiveUntil ? String(data.cost.effectiveUntil) : undefined });
  },
});
</script>

<template><Modal :title="$t(editing ? 'plugin.idc-core.catalog.cost.edit' : 'plugin.idc-core.catalog.cost.add')" class="w-[640px]"><Form /></Modal></template>

<script setup lang="ts">
import type { CatalogTaxPolicy } from "../catalog-client";
import { useVbenModal } from "@vben/common-ui";
import { message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { catalogTaxPolicyCreate } from "../catalog-client";

const emit = defineEmits<{ success: [] }>();
let versionNo = "";
const [Form, formApi] = useVbenForm({
  commonConfig: { labelWidth: 140, componentProps: { class: "w-full" } },
  schema: [], showDefaultActions: false,
});
const label = (key: string) => $t(`plugin.idc-core.catalog.commerce.${key}`);
const [Modal, modalApi] = useVbenModal({
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{ versionNo: string; currencies: string[] }>();
    versionNo = data.versionNo;
    await formApi.resetForm();
    await formApi.setState({ schema: [
      { component: "Select", fieldName: "currencyCode", label: $t("plugin.idc-core.catalog.configurationPrice.currency"), componentProps: { options: data.currencies.map(value => ({ value, label: value })) }, rules: z.string().regex(/^[A-Z]{3}$/) },
      { component: "Select", fieldName: "taxTreatment", label: label("taxTreatment"), componentProps: { "data-testid": "catalog-tax-treatment", options: ["taxable", "zero_rated", "exempt"].map(value => ({ value, label: $t(`plugin.idc-core.catalog.detail.code.taxTreatment.${value}`) })) }, rules: z.enum(["taxable", "zero_rated", "exempt"]) },
      { component: "Input", fieldName: "rate", label: label("taxPercent"), componentProps: { "data-testid": "catalog-tax-rate", inputmode: "decimal", placeholder: "6" }, rules: z.string().regex(/^\d{1,3}(?:\.\d{1,2})?$/).refine(value => Number(value) <= 100), dependencies: { triggerFields: ["taxTreatment"], if: values => values.taxTreatment === "taxable" } },
      { component: "Switch", fieldName: "priceIncludesTax", label: $t("plugin.idc-core.catalog.detail.priceIncludesTax"), componentProps: { class: "w-auto" }, rules: z.boolean() },
      { component: "Input", fieldName: "taxCategoryCode", label: label("taxCategory"), componentProps: { "data-testid": "catalog-tax-category", maxlength: 64 }, rules: z.string().regex(/^[a-z0-9][a-z0-9._-]{0,63}$/) },
      { component: "Select", fieldName: "roundingScope", label: label("roundingScope"), componentProps: { options: ["line", "total"].map(value => ({ value, label: label(`roundingScope_${value}`) })) }, rules: z.enum(["line", "total"]) },
      { component: "Select", fieldName: "roundingMode", label: label("roundingMode"), componentProps: { options: ["half_up", "down", "up"].map(value => ({ value, label: label(`roundingMode_${value}`) })) }, rules: z.enum(["half_up", "down", "up"]) },
    ] });
    await formApi.setValues({ currencyCode: data.currencies[0], priceIncludesTax: false, roundingScope: "line", roundingMode: "half_up" });
  },
  async onConfirm() {
    if (!(await formApi.validate()).valid) return false;
    const values = await formApi.getValues<Omit<CatalogTaxPolicy, "taxPolicyNo" | "taxRateBasisPoints" | "roundingQuantumMinor"> & { rate?: string }>();
    const rate = values.rate ?? "0";
    const [whole = "0", fraction = ""] = rate.split(".");
    modalApi.lock(true);
    try {
      await catalogTaxPolicyCreate(versionNo, {
        currencyCode: values.currencyCode, priceIncludesTax: values.priceIncludesTax,
        taxTreatment: values.taxTreatment, taxCategoryCode: values.taxCategoryCode,
        taxRateBasisPoints: values.taxTreatment === "exempt" ? null : values.taxTreatment === "zero_rated" ? 0 : Number(whole) * 100 + Number(fraction.padEnd(2, "0")),
        roundingScope: values.roundingScope, roundingMode: values.roundingMode, roundingQuantumMinor: "1",
      });
      message.success(label("taxSaved")); emit("success"); modalApi.close();
    } finally { modalApi.lock(false); }
  },
});
</script>
<template>
  <Modal :title="label('addTax')" class="w-[560px]">
    <div data-testid="catalog-tax-modal"><p class="text-muted-foreground mb-4">{{ label('taxHint') }}</p><Form /></div>
  </Modal>
</template>

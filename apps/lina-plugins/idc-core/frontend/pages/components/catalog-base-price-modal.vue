<script setup lang="ts">
import type { CatalogBillingRule } from "../catalog-client";
import { ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { catalogConfigurationPriceScheduleCreate } from "../catalog-client";
import { currencyOptions } from "../currency-client";
import { majorToMinorAmount } from "../catalog-money";

const trial = ref(false);
const emit = defineEmits<{ success: [] }>();
let data: { versionNo: string; rule: CatalogBillingRule; existingCurrencies: string[] };
const amount = () => z.string().regex(/^\d{1,19}(?:\.\d{1,4})?$/);
const [Form, formApi] = useVbenForm({ commonConfig: { labelWidth: 140, componentProps: { class: "w-full" } }, schema: [], showDefaultActions: false });
const [Modal, modalApi] = useVbenModal({
  async onOpenChange(open) {
    if (!open) return;
    modalApi.setState({ confirmDisabled: true });
    data = modalApi.getData();
    trial.value = data.rule.chargeType === "trial";
    await formApi.resetForm();
    const currencies = (await currencyOptions()).currencies.filter(code => !data.existingCurrencies.includes(code));
    modalApi.setState({ confirmDisabled: currencies.length === 0 });
    if (currencies.length === 0) message.warning($t('plugin.idc-core.catalog.commerce.noPriceCurrency'));
    await formApi.setState({ schema: [
      { component: "Select", fieldName: "currencyCode", label: $t("plugin.idc-core.catalog.configurationPrice.currency"), componentProps: { "data-testid": "catalog-base-price-currency", options: currencies.map(value => ({ value, label: value })) }, rules: z.string().regex(/^[A-Z]{3}$/) },
      { component: "Input", fieldName: "amount", label: $t("plugin.idc-core.catalog.configurationPrice.amount"), componentProps: { "data-testid": "catalog-base-price-amount", inputmode: "decimal", disabled: trial.value }, rules: amount() },
      { component: "Input", fieldName: "setupFee", label: $t("plugin.idc-core.catalog.configurationPrice.setupFee"), componentProps: { "data-testid": "catalog-base-price-setup", inputmode: "decimal", disabled: trial.value }, rules: amount() },
      ...(data.rule.pricePhase === "new_purchase" ? [] : [{ component: "Input", fieldName: "renewalAmount", label: $t("plugin.idc-core.catalog.configurationPrice.renewalAmount"), componentProps: { "data-testid": "catalog-base-price-renewal", inputmode: "decimal", disabled: trial.value }, rules: amount() }]),
    ] });
    await formApi.setValues({ currencyCode: currencies[0], amount: trial.value ? "0" : "", setupFee: "0", renewalAmount: "" });
  },
  async onConfirm() {
    if (!(await formApi.validate()).valid) return false;
    const values = await formApi.getValues<{ currencyCode: string; amount: string; setupFee: string; renewalAmount: string }>();
    let tier;
    try {
      tier = { minimumQuantity: 0, maximumQuantity: null, amountMinor: majorToMinorAmount(values.amount, values.currencyCode), setupFeeMinor: majorToMinorAmount(values.setupFee, values.currencyCode), ...(data.rule.pricePhase !== "new_purchase" ? { renewalAmountMinor: majorToMinorAmount(values.renewalAmount, values.currencyCode) } : {}) };
    } catch {
      message.error($t("plugin.idc-core.catalog.configurationPrice.invalidAmount")); return false;
    }
    modalApi.lock(true);
    try {
      await catalogConfigurationPriceScheduleCreate(data.versionNo, data.rule.billingRuleNo, { currencyCode: values.currencyCode, status: "active", tiers: [tier] });
      message.success($t("plugin.idc-core.catalog.commerce.priceSaved")); emit("success"); modalApi.close();
    } finally { modalApi.lock(false); }
  },
});
</script>
<template>
  <Modal :title="$t('plugin.idc-core.catalog.commerce.addBasePrice')" class="w-[560px]">
    <div data-testid="catalog-base-price-modal"><p class="text-muted-foreground mb-4">{{ $t(trial ? 'plugin.idc-core.catalog.commerce.trialPriceHint' : 'plugin.idc-core.catalog.commerce.basePriceHint') }}</p><Form /></div>
  </Modal>
</template>

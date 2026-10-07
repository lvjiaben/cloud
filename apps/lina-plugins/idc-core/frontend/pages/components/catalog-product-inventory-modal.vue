<script setup lang="ts">
import type { CatalogProductVersionInventory } from "../catalog-client";
import type { VbenFormProps } from "@vben/common-ui";

import { ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { catalogProductVersionInventoryUpdate } from "../catalog-client";

const emit = defineEmits<{ success: [CatalogProductVersionInventory] }>();
const versionNo = ref("");
const current = ref<CatalogProductVersionInventory>();

const schema: VbenFormProps["schema"] = [
  { component: "Switch", componentProps: { "data-testid": "catalog-inventory-tracking" }, fieldName: "trackingEnabled", label: $t("plugin.idc-core.catalog.inventory.trackingEnabled") },
  { component: "InputNumber", componentProps: { "data-testid": "catalog-inventory-quantity", min: 0, precision: 0 }, dependencies: { show: (values) => values.trackingEnabled === true, triggerFields: ["trackingEnabled"] }, fieldName: "sellableQuantity", label: $t("plugin.idc-core.catalog.inventory.sellableQuantity"), rules: z.number().int().min(0) },
];

const [Form, formApi] = useVbenForm({ commonConfig: { componentProps: { class: "w-full" }, labelWidth: 150 }, schema, showDefaultActions: false });
const [Modal, modalApi] = useVbenModal({
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid || !current.value) return false;
    const values = await formApi.getValues<{ sellableQuantity: number; trackingEnabled: boolean }>();
    modalApi.lock(true);
    try {
      const result = await catalogProductVersionInventoryUpdate(versionNo.value, { ...values, expectedVersion: current.value.version });
      emit("success", result.inventory);
      message.success($t("plugin.idc-core.catalog.inventory.saved"));
      modalApi.close();
    } finally { modalApi.lock(false); }
  },
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{ inventory: CatalogProductVersionInventory; versionNo: string }>();
    versionNo.value = data.versionNo;
    current.value = data.inventory;
    await formApi.setValues({ trackingEnabled: data.inventory.trackingEnabled, sellableQuantity: data.inventory.sellableQuantity });
  },
});
</script>

<template><Modal :title="$t('plugin.idc-core.catalog.inventory.edit')" class="w-[560px]"><Form /></Modal></template>

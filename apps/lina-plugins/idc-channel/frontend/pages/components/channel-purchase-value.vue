<script setup lang="ts">
import type { VbenFormSchema } from "#/adapter/form";
import type { PurchaseValueDraft } from "../channel-purchase-values";
import { onMounted, watch } from "vue";
import { Button } from "ant-design-vue";
import { preferences } from "@vben/preferences";
import { useVbenForm } from "#/adapter/form";
import { channelText as t } from "../channel-i18n";
import { purchaseValueTypes } from "../channel-purchase-values";

const props = defineProps<{ modelValue: PurchaseValueDraft }>();
const emit = defineEmits<{
  "update:modelValue": [value: PurchaseValueDraft];
  remove: [];
}>();
const [Form, api] = useVbenForm({
  schema: [],
  showDefaultActions: false,
  layout: "vertical",
  wrapperClass: "grid-cols-1 md:grid-cols-3",
  handleValuesChange: (values) =>
    emit("update:modelValue", {
      fieldToken: String(values.fieldToken || ""),
      fieldType: String(values.fieldType || "single_line"),
      fieldValue: String(values.fieldValue || ""),
    }),
});
function schema(): VbenFormSchema[] {
  return [
    {
      fieldName: "fieldToken",
      component: "Input",
      label: t("fieldToken"),
      componentProps: { name: "fieldToken" },
    },
    {
      fieldName: "fieldType",
      component: "Select",
      label: t("fieldType"),
      componentProps: {
        options: purchaseValueTypes.map((value) => ({
          value,
          label: t("valueType." + value),
        })),
      },
    },
    {
      fieldName: "fieldValue",
      component: "Input",
      label: t("fieldValue"),
      componentProps: { name: "fieldValue" },
    },
  ];
}
watch(
  () => preferences.app.locale,
  () => api.setState({ schema: schema() }),
);
onMounted(async () => {
  api.setState({ schema: schema() });
  await api.setValues(props.modelValue);
});
</script>
<template>
  <div class="channel-purchase-value">
    <Form /><Button danger @click="emit('remove')">{{ t("remove") }}</Button>
  </div>
</template>
<style scoped>
.channel-purchase-value {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
}
@media (max-width: 600px) {
  .channel-purchase-value {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>

<script setup lang="ts">
import type { CatalogProductSummary } from "../catalog-client";
import { ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { Alert, message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { catalogProductCopy } from "../catalog-client";

const emit = defineEmits<{ success: [] }>();
const source = ref<CatalogProductSummary>();
const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [], showDefaultActions: false,
});
function schema() {
  return [
    { component: "Input", fieldName: "nameZhCn", label: $t("plugin.idc-core.catalog.create.nameZhCn"), componentProps: { maxlength: 128, "data-testid": "catalog-copy-name-zh" }, rules: z.string().trim().min(1).max(128) },
    { component: "Input", fieldName: "nameEnUs", label: $t("plugin.idc-core.catalog.create.nameEnUs"), componentProps: { maxlength: 128, "data-testid": "catalog-copy-name-en" }, rules: z.string().trim().min(1).max(128) },
    { component: "Input", fieldName: "publicSlug", label: $t("plugin.idc-core.catalog.create.publicSlug"), componentProps: { maxlength: 128, "data-testid": "catalog-copy-slug" }, rules: z.string().trim().min(1).max(128).regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/) },
  ];
}
const [Modal, modalApi] = useVbenModal({
  async onClosed() { source.value = undefined; await formApi.resetForm(); },
  async onOpenChange(open) {
    if (!open) return;
    source.value = modalApi.getData<CatalogProductSummary>();
    await formApi.setState({ schema: schema() });
    if (source.value) await formApi.setValues({ nameZhCn: `${source.value.nameZhCn} ${$t("plugin.idc-core.catalog.copy.suffix")}`.slice(0, 128), nameEnUs: `${source.value.nameEnUs} Copy`.slice(0, 128), publicSlug: `${source.value.publicSlug.slice(0, 123).replace(/-+$/, "")}-copy` });
  },
  async onConfirm() {
    if (!source.value || !(await formApi.validate()).valid) return false;
    const values = await formApi.getValues<{ nameZhCn: string; nameEnUs: string; publicSlug: string }>();
    modalApi.lock(true);
    try {
      await catalogProductCopy(source.value.productNo, { sourceVersionNo: source.value.versionNo, nameZhCn: values.nameZhCn.trim(), nameEnUs: values.nameEnUs.trim(), publicSlug: values.publicSlug.trim() });
      message.success($t("plugin.idc-core.catalog.copy.succeeded"));
      emit("success"); modalApi.close();
    } finally { modalApi.lock(false); }
  },
});
</script>
<template>
  <Modal class="w-[520px]" :title="$t('plugin.idc-core.catalog.copy.title')" :confirm-text="$t('plugin.idc-core.catalog.copy.confirm')">
    <div data-testid="catalog-product-copy-modal">
      <Alert class="mb-4" type="info" :message="$t('plugin.idc-core.catalog.copy.hint')" />
      <Form />
    </div>
  </Modal>
</template>

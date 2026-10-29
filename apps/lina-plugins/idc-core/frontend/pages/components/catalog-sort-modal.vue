<script setup lang="ts">
import type { CatalogCategory, CatalogProductDetailProduct } from "../catalog-client";
import { ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { useVbenForm, z } from "#/adapter/form";
import { Alert, message } from "ant-design-vue";
import { $t } from "#/locales";
import { catalogCategoryUpdate, catalogProductDescriptionUpdate, catalogProductDetail } from "../catalog-client";

const emit = defineEmits<{ success: [] }>();
const category = ref<CatalogCategory>();
const product = ref<CatalogProductDetailProduct>();
const loading = ref(false);
const failed = ref(false);
let target: { category?: CatalogCategory; productNo?: string } = {};
const key = "plugin.idc-core.catalog.screen.";
const [Form, formApi] = useVbenForm({ showDefaultActions: false, schema: [], commonConfig: { labelWidth: 100 } });
async function loadTarget() {
  loading.value = true;
  failed.value = false;
  product.value = undefined;
  category.value = undefined;
  try {
    if (target.category) category.value = target.category;
    else if (target.productNo) product.value = (await catalogProductDetail(target.productNo)).product;
    formApi.setState({ schema: [{ component: "InputNumber", fieldName: "sortOrder", label: $t("plugin.idc-core.catalog.create.sortOrder"), componentProps: { min: -1000000, max: 1000000, precision: 0, class: "w-full", "data-testid": "catalog-sort-value" }, rules: z.number().int().min(-1000000).max(1000000) }] });
    await formApi.setValues({sortOrder: category.value?.sortOrder ?? product.value?.sortOrder ?? 100});
  } catch { failed.value = true; }
  finally { loading.value = false; }
}
const [Modal, modalApi] = useVbenModal({
  async onOpenChange(open) {
    if (!open) return;
    target = modalApi.getData();
    await loadTarget();
  },
  async onConfirm() {
    if (loading.value || failed.value || (!category.value && !product.value)) return false;
    const {valid} = await formApi.validate();
    if (!valid) return false;
    const {sortOrder} = await formApi.getValues<{sortOrder:number}>();
    modalApi.lock(true);
    try {
      if (category.value) {
        const row = category.value;
        await catalogCategoryUpdate(row.categoryNo, { parentCategoryNo: row.parentCategoryNo || undefined, nameZhCn: row.nameZhCn, nameEnUs: row.nameEnUs, sortOrder, version: row.version });
      } else if (product.value) {
        const row = product.value;
        await catalogProductDescriptionUpdate(row.productNo, { version: row.descriptionVersion!, nameZhCn: row.nameZhCn, nameEnUs: row.nameEnUs, summaryZhCn: row.summaryZhCn, summaryEnUs: row.summaryEnUs, sortOrder });
      }
      message.success($t(key + "sortSaved"));
      modalApi.close();
      emit("success");
    } finally { modalApi.lock(false); }
  },
});
</script>
<template>
  <Modal :title="$t(key+'sortTitle')" :loading="loading" :confirm-disabled="loading || failed">
    <div data-testid="catalog-sort-modal">
      <Alert v-if="failed" type="error" :message="$t(key+'sortLoadFailed')"><template #action><a @click="loadTarget">{{ $t('plugin.idc-core.catalog.workspace.retry') }}</a></template></Alert>
      <template v-else><p class="mb-4 text-muted-foreground">{{ $t(key+'sortHint') }}</p><Form /></template>
    </div>
  </Modal>
</template>

<script setup lang="ts">
// The existing modal remains an adapter to the same editor used inline.
import { ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { $t } from "#/locales";
import type { CatalogConfigurationPriceEditorData } from "../catalog-configuration-price-editor";
import CatalogConfigurationPriceForm from "./catalog-configuration-price-form.vue";
const emit = defineEmits<{ success: [] }>();
const data = ref<CatalogConfigurationPriceEditorData>();
const editor = ref<InstanceType<typeof CatalogConfigurationPriceForm>>();
const [Modal, modalApi] = useVbenModal({
  async onConfirm() {
    if (!editor.value) return false;
    modalApi.lock(true);
    try {
      return await editor.value?.save();
    } finally {
      modalApi.lock(false);
    }
  },
  onOpenChange(open) {
    data.value = open
      ? modalApi.getData<CatalogConfigurationPriceEditorData>()
      : undefined;
  },
});
function saved() {
  emit("success");
  modalApi.close();
}
</script>
<template>
  <Modal
    :title="$t('plugin.idc-core.catalog.configurationPrice.title')"
    class="w-[680px]"
  >
    <div data-testid="catalog-configuration-price-modal">
      <CatalogConfigurationPriceForm
        v-if="data"
        ref="editor"
        :data="data"
        @success="saved"
      />
    </div>
  </Modal>
</template>

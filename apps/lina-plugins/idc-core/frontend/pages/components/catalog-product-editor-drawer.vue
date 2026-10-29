<script setup lang="ts">
import { ref } from 'vue';
import { useVbenDrawer } from '@vben/common-ui';
import { $t } from '#/locales';
import { Button, Tooltip } from 'ant-design-vue';
import CatalogProductEditor from './catalog-merchant-workspace.vue';

const emit = defineEmits<{closed: []; updated: []}>();
const productNo = ref('');
const section = ref('basic');
const editor = ref<InstanceType<typeof CatalogProductEditor>>();
const [Drawer, drawerApi] = useVbenDrawer({
  async onBeforeClose() {return editor.value ? editor.value.requestClose() : true;},
  onOpenChange(open) {
    if (!open) return;
    const data = drawerApi.getData<{productNo: string; section?: string}>();
    productNo.value = data.productNo;
    section.value = data.section || 'basic';
  },
  onClosed() { productNo.value = ''; emit('closed'); },
});
</script>

<template>
  <Drawer :title="$t('plugin.idc-core.catalog.drawer.title')" class="catalog-product-drawer" content-class="p-0" :close-on-click-modal="false">
    <CatalogProductEditor v-if="productNo" ref="editor" :key="productNo" :product-no="productNo" :section="section" @updated="emit('updated')" @close="drawerApi.close()" />
    <template #footer>
      <Button :disabled="editor?.footerBusy" data-testid="catalog-editor-back" @click="drawerApi.close()">{{ $t('plugin.idc-core.catalog.drawer.close') }}</Button>
      <Tooltip v-if="editor?.footerCanSave" :open="Boolean(editor?.hasUnsavedChanges && !editor?.footerBusy)" :trigger="[]" placement="topRight">
        <template #title><span role="status" data-testid="catalog-save-unsaved-hint">{{ $t('plugin.idc-core.catalog.editorSafety.saveReminder') }}</span></template>
        <Button type="primary" :loading="editor?.footerBusy" :disabled="editor?.footerDisabled" :data-testid="editor?.footerSaveTestId" @click="editor?.saveActive()">{{ $t('plugin.idc-core.catalog.merchantWorkspace.save') }}</Button>
      </Tooltip>
    </template>
  </Drawer>
</template>

<style scoped>
:global(.catalog-product-drawer){width:min(1520px,97vw)!important;max-width:97vw!important;}
@media(max-width:600px){:global(.catalog-product-drawer){width:100vw!important;max-width:100vw!important;}}
</style>

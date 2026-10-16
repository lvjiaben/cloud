<script lang="ts">
export const pluginPageMeta = { routePath: 'idc/sales-channel', title: 'Agent and sales management' };
</script>
<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { computed, ref, watch } from 'vue';
import { Page } from '@vben/common-ui';
import { Tabs, TabPane } from 'ant-design-vue';
import { $t } from '#/locales';
import { useAccess } from './plugin-access';
import type { SalesSection } from './sales-management-client';
import SalesPerformance from './components/sales-performance.vue';
import SalesManagementPanel from './components/sales-management-panel.vue';
const active = ref<SalesSection>('agents');
const { hasAccessByCodes } = useAccess();
const sections: SalesSection[] = ['agents', 'levels', 'ownership', 'performance', 'commission-rules', 'commission/accounts', 'commission/records', 'commission/payouts'];
function label(section: SalesSection) { return $t(`plugin.idc-core.salesManagement.sections.${section.replace('commission/', '')}`); }
function visible(section: SalesSection) {
  return hasAccessByCodes([section.startsWith('commission/') ? 'idc:admin:sales-channel:commission:view' : 'idc:admin:sales-channel:overview:view']);
}
const visibleSections = computed(() => sections.filter(visible));
watch(visibleSections, (available) => {
  if (!available.includes(active.value) && available[0]) active.value = available[0];
}, { immediate: true });
</script>
<template>
  <Page class="plugin-admin-page plugin-admin-workspace" auto-content-height>
    <div class="flex h-full min-h-0 flex-col" data-testid="sales-channel-workspace">
      <Tabs v-model:active-key="active" class="sales-tabs plugin-admin-tabs" :destroy-inactive-tab-pane="true">
        <TabPane v-for="section in visibleSections" :key="section" :tab="label(section)">
          <SalesPerformance v-if="section === 'performance'" />
          <SalesManagementPanel v-else :section="section" />
        </TabPane>
      </Tabs>
    </div>
  </Page>
</template>
<style scoped>
.sales-tabs { height: 100%; display: flex; flex-direction: column; }
.sales-tabs :deep(.ant-tabs-content-holder), .sales-tabs :deep(.ant-tabs-content), .sales-tabs :deep(.ant-tabs-tabpane) { flex: 1; min-height: 0; height: 100%; }
</style>

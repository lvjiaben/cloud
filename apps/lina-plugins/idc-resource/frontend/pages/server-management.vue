<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/servers",
  title: "Server Management",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { Page } from "@vben/common-ui";
import { Empty, Tabs, TabPane } from "ant-design-vue";
import { $t } from "#/locales";
import { useAccess } from "./plugin-access";
import ServerList from "./server-accessories.vue";
import ServerRenewals from "./server-batch-renewal.vue";

const { hasAccessByCodes } = useAccess();
const route = useRoute();
const tabPreference = `idc-resource:${route.path}:tab`;
const tabs = computed(() => [
  { key: "servers", permission: "server-accessory:view", component: ServerList },
  { key: "renewals", permission: "server-batch:view", component: ServerRenewals },
].filter((tab) => hasAccessByCodes([`idc:admin:resource:${tab.permission}`])));
const entryTab = (route.meta.query as Record<string, unknown> | undefined)?.resourceTab;
const activeTab = ref(String(route.query.resourceTab ?? (entryTab !== "servers" ? entryTab : undefined) ?? sessionStorage.getItem(tabPreference) ?? entryTab ?? ""));
watch(tabs, (items) => {
  if (!items.length) return;
  if (!items.some((tab) => tab.key === activeTab.value)) activeTab.value = items[0]?.key ?? "";
}, { immediate: true });
function rememberTab(key: string | number) {
  sessionStorage.setItem(tabPreference, String(key));
}
watch(() => (route.meta.query as Record<string, unknown> | undefined)?.resourceTab, (key) => {
  if (tabs.value.some((tab) => tab.key === key)) activeTab.value = String(key);
});
</script>

<template>
  <Page class="plugin-admin-page plugin-admin-workspace" auto-content-height data-testid="server-management-page">
    <Tabs v-if="tabs.length" v-model:active-key="activeTab" destroy-inactive-tab-pane class="server-tabs plugin-admin-tabs" @change="rememberTab">
      <TabPane v-for="tab in tabs" :key="tab.key" :tab="$t(`plugin.idc-resource.workspaces.${tab.key}`)">
        <component :is="tab.component" v-if="activeTab === tab.key" />
      </TabPane>
    </Tabs>
    <Empty v-else :description="$t('plugin.idc-resource.workspaces.noAccess')" />
  </Page>
</template>

<style scoped>
.server-tabs { height: 100%; }
.server-tabs :deep(.ant-tabs-nav) { margin: 0 0 12px; }
.server-tabs :deep(.ant-tabs-content-holder) { overflow: hidden; }
</style>

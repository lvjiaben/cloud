<script setup lang="ts">
import { computed, ref } from "vue";
import type { ProductManagementKind } from "../customer-product-management";
import { useVbenDrawer } from "@vben/common-ui";
import { Alert, Tabs } from "ant-design-vue";
import { $t } from "#/locales";
import { useAccess } from "../plugin-access";
import ServerManagement from "../../../../idc-resource/frontend/pages/server-accessories.vue";
import NativeServerDetail from "../../../../idc-resource/frontend/pages/server-entitlement-detail.vue";
import ServerRenewals from "../../../../idc-resource/frontend/pages/server-batch-renewal.vue";
import ProviderTasks from "../../../../idc-resource/frontend/pages/provider-tasks.vue";
import CertificateManagement from "./customer-certificate-management.vue";
const { hasAccessByCodes } = useAccess();
const target = ref<{ serviceNo: string; name: string; kind: ProductManagementKind }>();
const activeTab = ref("management");
const canReadDetail = computed(() => target.value && hasAccessByCodes([target.value.kind === "native-server" ? "idc:admin:resource:server-batch:view" : "idc:admin:resource:server-accessory:view"]));
const text = (key: string) => $t(`plugin.idc-core.customer.workspace.${key}`);
const [Drawer, drawerApi] = useVbenDrawer({
  showConfirmButton: false,
  destroyOnClose: true,
  onOpenChange(open) {
    target.value = open ? drawerApi.getData<{ serviceNo: string; name: string; kind: ProductManagementKind }>() : undefined;
    activeTab.value = canReadDetail.value ? "management" : hasAccessByCodes(["idc:admin:resource:task:list"]) ? "tasks" : "logs";
  },
});
</script>
<template>
  <Drawer :title="target?.name || text('manage')" class="resource-product-management-panel">
    <div v-if="target" :key="target.serviceNo" data-testid="customer-product-management" class="min-h-[600px]">
      <p>{{ target.serviceNo }}</p>
      <CertificateManagement v-if="target.kind === 'ssl'" :service-no="target.serviceNo" />
      <Tabs v-else v-model:active-key="activeTab" destroy-inactive-tab-pane>
        <Tabs.TabPane v-if="canReadDetail" key="management" :tab="text('manage')"><NativeServerDetail v-if="target.kind === 'native-server'" :entitlement-no="target.serviceNo" /><ServerManagement v-else :scoped-service-no="target.serviceNo" /></Tabs.TabPane>
        <Tabs.TabPane v-if="target.kind === 'native-server' && canReadDetail" key="renewals" :tab="text('resourceRenewals')"><ServerRenewals :entitlement-no="target.serviceNo" /></Tabs.TabPane>
        <Tabs.TabPane v-if="hasAccessByCodes(['idc:admin:resource:task:list'])" key="tasks" :tab="text('resourceTasks')"><ProviderTasks :service-no="target.serviceNo" /></Tabs.TabPane>
        <Tabs.TabPane v-if="hasAccessByCodes(['idc:admin:resource:api-log:list'])" key="logs" :tab="text('integrationLogs')"><Alert type="info" :message="text('noLinkedLogs')" /></Tabs.TabPane>
      </Tabs>
    </div>
  </Drawer>
</template>
<style>
.resource-product-management-panel { width: min(1100px, 95vw) !important; }
</style>

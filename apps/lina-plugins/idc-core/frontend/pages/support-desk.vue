<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/support",
  title: "Support Desk",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { Page } from "@vben/common-ui";
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import OperationsDetailPanel from "./components/operations-detail-panel.vue";
import { useAccess } from "./plugin-access";
import { Card } from "ant-design-vue";
import App from "../src/App.vue";
import "../src/styles.css";
import { useSourcePluginContext } from "./source-context";

const context = useSourcePluginContext();
const route = useRoute();
const router = useRouter();
const { hasAccessByCodes } = useAccess();
const operationsDetail = computed(() => route.query.operationsMetric === 'tickets');
async function openTicket(ticket: string) {
  await router.replace({ path: route.path, query: { ticket } });
}
</script>

<template>
  <Page class="plugin-admin-page" :auto-content-height="true">
    <OperationsDetailPanel v-if="operationsDetail" :can-open="hasAccessByCodes(['idc:admin:support:ticket:query'])" @open="openTicket($event.id)" />
    <Card
      v-else
      :bordered="false"
      :style="{ background: 'transparent' }"
      :body-style="{ height: '100%', padding: 0, background: 'transparent' }"
      class="h-full"
    >
      <div class="idc-plugin-shell h-full"><App :context="context" /></div>
    </Card>
  </Page>
</template>

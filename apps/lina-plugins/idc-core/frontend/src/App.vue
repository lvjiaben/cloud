<script setup lang="ts">
import { computed } from "vue";

import type { PluginHostMountContext } from "./host-context";

import ContentWorkspace from "./content/ContentWorkspace.vue";
import FinanceWorkspace from "./finance/FinanceWorkspace.vue";
import IPChangePolicyWorkspace from "./ip-change/IPChangePolicyWorkspace.vue";
import ServerOperationPolicyWorkspace from "./server-operation/ServerOperationPolicyWorkspace.vue";
import ServerBatchWorkspace from "./server-batch/ServerBatchWorkspace.vue";
import SupportSettingsWorkspace from "./support/SupportSettingsWorkspace.vue";
import SupportWorkspace from "./support/SupportWorkspace.vue";
import OperationsWorkspace from "./operations/OperationsWorkspace.vue";
import SalesChannelWorkspace from "./sales-channel/SalesChannelWorkspace.vue";

const props = defineProps<{ context: PluginHostMountContext }>();

const page = computed(() => {
  const path = (
    props.context.routePath ||
    props.context.route.path ||
    ""
  ).toLowerCase();
  if (path.includes("/content")) return "content";
  if (path.includes("/finance")) return "finance";
  if (path.includes("/resource/server-operation-security"))
    return "server-operation-policy";
  if (path.includes("/resource/server-batches")) return "server-batches";
  if (path.includes("/resource/ip-change-policy")) return "ip-change-policy";
  if (path.includes("/support/settings")) return "support-settings";
  if (path.includes("/operations/")) return "operations";
  if (path.includes("/sales-channel")) return "sales-channel";
  return "support";
});
</script>

<template>
  <ContentWorkspace v-if="page === 'content'" :context="context" />
  <FinanceWorkspace v-else-if="page === 'finance'" :context="context" />
  <ServerOperationPolicyWorkspace
    v-else-if="page === 'server-operation-policy'"
    :context="context"
  />
  <ServerBatchWorkspace
    v-else-if="page === 'server-batches'"
    :context="context"
  />
  <IPChangePolicyWorkspace
    v-else-if="page === 'ip-change-policy'"
    :context="context"
  />
  <SupportSettingsWorkspace
    v-else-if="page === 'support-settings'"
    :context="context"
  />
  <OperationsWorkspace v-else-if="page === 'operations'" :context="context" />
  <SalesChannelWorkspace
    v-else-if="page === 'sales-channel'"
    :context="context"
  />
  <SupportWorkspace v-else :context="context" />
</template>

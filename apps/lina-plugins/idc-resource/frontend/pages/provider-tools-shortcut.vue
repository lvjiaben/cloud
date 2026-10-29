<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/provider-tools",
  title: "Supplier Operations",
};
</script>
<script setup lang="ts">
import { onMounted } from "vue";
import { useRouter } from "vue-router";
import { Page } from "@vben/common-ui";
import { useTabs } from "@vben/hooks";
import { Spin } from "ant-design-vue";
const router = useRouter();
const { closeTabByKey } = useTabs();
onMounted(async () => {
  await router.replace({
    path: "/idc/resource/provider-configuration",
    query: { resourceTab: "operations" },
  });
  await closeTabByKey("/idc/resource/provider-tools");
});
</script>
<template>
  <Page auto-content-height><Spin /></Page>
</template>

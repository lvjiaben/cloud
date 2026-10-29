<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/customer-products",
  title: "Server Management",
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
    path: "/idc/customers",
    query: { resourceShortcut: "products" },
  });
  await closeTabByKey("/idc/resource/customer-products");
});
</script>
<template>
  <Page auto-content-height><Spin /></Page>
</template>

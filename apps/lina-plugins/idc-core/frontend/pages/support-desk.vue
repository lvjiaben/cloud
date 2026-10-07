<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/support",
  title: "Support Desk",
};
</script>

<script setup lang="ts">
import { Page } from "@vben/common-ui";
import { Card } from "ant-design-vue";
import App from "../src/App.vue";
import "../src/styles.css";
import { useSourcePluginContext } from "./source-context";

const context = useSourcePluginContext();
</script>

<template>
  <Page :auto-content-height="true">
    <Card
      :bordered="false"
      :body-style="{ height: '100%', padding: 0 }"
      class="h-full"
    >
      <div class="idc-plugin-shell h-full"><App :context="context" /></div>
    </Card>
  </Page>
</template>

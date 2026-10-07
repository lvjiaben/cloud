<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/content",
  title: "Content Management",
};
</script>

<script setup lang="ts">
import { Page } from "@vben/common-ui";
import App from "../src/App.vue";
import "../src/styles.css";
import { useSourcePluginContext } from "./source-context";

const context = useSourcePluginContext();
</script>

<template>
  <Page :auto-content-height="true" content-class="p-0">
    <div class="idc-plugin-shell h-full bg-card">
      <App :context="context" />
    </div>
  </Page>
</template>

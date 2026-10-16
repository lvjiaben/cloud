<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/content",
  title: "Content Management",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { Page } from "@vben/common-ui";
import App from "../src/App.vue";
import "../src/styles.css";
import { useSourcePluginContext } from "./source-context";

const context = useSourcePluginContext();
</script>

<template>
  <Page class="plugin-admin-page" :auto-content-height="true">
    <div class="idc-plugin-shell h-full">
      <App :context="context" />
    </div>
  </Page>
</template>

<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/operations/workbench",
  title: "Operations Workbench",
};
</script>

<script setup lang="ts">
import { Page } from "@vben/common-ui";
import { computed } from "vue";
import App from "../src/App.vue";
import "../src/styles.css";
import { useSourcePluginContext } from "./source-context";

const props = withDefaults(
  defineProps<{
    routePath?: string;
  }>(),
  {
    routePath: "",
  },
);

const sourceContext = useSourcePluginContext();
const context = computed(() => ({
  ...sourceContext.value,
  routePath: props.routePath || sourceContext.value.routePath,
}));
</script>

<template>
  <Page :auto-content-height="true">
    <div class="idc-plugin-shell h-full"><App :context="context" /></div>
  </Page>
</template>

<script lang="ts">
export const pluginPageMeta = { routePath: 'idc/support/settings', title: 'Support Settings' };
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { Page } from '@vben/common-ui';
import SupportSettingsWorkspace from './components/support-settings-workspace.vue';
import '../src/styles.css';
import { useSourcePluginContext } from './source-context';

const context = useSourcePluginContext();
</script>

<template>
  <Page class="plugin-admin-page" :auto-content-height="true">
    <div class="idc-plugin-shell h-full"><SupportSettingsWorkspace :context="context" /></div>
  </Page>
</template>

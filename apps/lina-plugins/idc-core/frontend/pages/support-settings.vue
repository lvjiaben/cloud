<script lang="ts">
export const pluginPageMeta = { routePath: 'idc/support/settings', title: 'Support Settings' };
</script>

<script setup lang="ts">
import { Page } from '@vben/common-ui';
import App from '../src/App.vue';
import '../src/styles.css';
import { useSourcePluginContext } from './source-context';

const context = useSourcePluginContext();
</script>

<template>
  <Page :auto-content-height="true">
    <div class="idc-plugin-shell h-full"><App :context="context" /></div>
  </Page>
</template>

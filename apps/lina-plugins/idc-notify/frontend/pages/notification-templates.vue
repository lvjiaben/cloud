<script lang="ts">
export const pluginPageMeta = { routePath: 'idc/notifications/templates', title: 'Notification Templates' };
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { Page } from '@vben/common-ui';
import App from '../src/App.vue';
import { useSourcePluginContext } from './source-context';

const context = useSourcePluginContext('/idc/notifications/templates');
</script>

<template>
  <Page class="plugin-admin-page" :auto-content-height="true">
    <div class="h-full notify-plugin-shell" data-plugin-frontend-root="idc-notify">
      <App :context="context" />
    </div>
  </Page>
</template>

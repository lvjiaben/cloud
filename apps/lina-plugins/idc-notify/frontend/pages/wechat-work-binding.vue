<script lang="ts">
export const pluginPageMeta = { routePath: 'idc/notifications/wechat-work-binding', title: 'My WeCom Binding' };
</script>

<script setup lang="ts">
import { Page } from '@vben/common-ui';
import { Card } from 'ant-design-vue';
import App from '../src/App.vue';
import { useSourcePluginContext } from './source-context';

const context = useSourcePluginContext();
</script>

<template>
  <Page :auto-content-height="true">
    <Card :bordered="false" class="h-full notify-plugin-shell" data-plugin-frontend-root="idc-notify">
      <App :context="context" />
    </Card>
  </Page>
</template>

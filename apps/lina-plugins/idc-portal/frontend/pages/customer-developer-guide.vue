<script setup lang="ts">
import { computed } from 'vue';
import { useVbenDrawer } from '@vben/common-ui';
import { Button } from 'ant-design-vue';
import { $t } from '#/locales';
import TemplateApiReference from './template-api-reference.vue';
import guide from '../../docs/customer-plugin-development.zh-CN.md?raw';
const [Drawer, api] = useVbenDrawer({ footer: false });
const title = computed(() => $t('plugin.idc-portal.customerCenter.developmentGuide'));
</script>
<template>
  <Button @click="api.open()">{{ title }}</Button>
  <Drawer :title="title" class="w-[900px] max-w-full">
    <article class="p-4"><TemplateApiReference /><a href="/api.json" target="_blank" rel="noopener">OpenAPI</a><pre class="whitespace-pre-wrap break-words font-sans leading-8">{{ guide }}</pre></article>
  </Drawer>
</template>

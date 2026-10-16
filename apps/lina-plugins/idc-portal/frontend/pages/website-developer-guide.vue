<script setup lang="ts">
import { computed } from 'vue';
import { useVbenDrawer } from '@vben/common-ui';
import { Button } from 'ant-design-vue';
import { $t } from '#/locales';
import guide from '../../docs/website-plugin-development.zh-CN.md?raw';
import TemplateApiReference from './template-api-reference.vue';

const [Drawer, drawerApi] = useVbenDrawer({ footer: false });
const title = computed(() => $t('plugin.idc-portal.website.developmentGuide'));
</script>
<template>
 <Button @click="drawerApi.open()">{{ title }}</Button>
 <Drawer :title="title" class="website-development-drawer"><TemplateApiReference public-only /><article class="website-development-guide mt-6"><pre>{{ guide }}</pre></article></Drawer>
</template>
<style scoped>
.website-development-guide pre { white-space:pre-wrap; overflow-wrap:anywhere; font-family:inherit; font-size:14px; line-height:1.8; color:hsl(var(--foreground)); }
:global(.website-development-drawer) { width:min(900px,96vw)!important; max-width:96vw; }
</style>

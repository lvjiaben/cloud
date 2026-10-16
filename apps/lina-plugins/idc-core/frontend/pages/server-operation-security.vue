<script lang="ts">
export const pluginPageMeta = { routePath: 'idc/resource/server-operation-security', title: 'Server Operation Security' };
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { Page } from '@vben/common-ui';
import { Card } from 'ant-design-vue';
import App from '../src/App.vue';
import '../src/styles.css';
import { useSourcePluginContext } from './source-context';

const context = useSourcePluginContext();
</script>

<template><Page class="plugin-admin-page" :auto-content-height="true"><Card :bordered="false" class="h-full"><div class="idc-plugin-shell h-full"><App :context="context" /></div></Card></Page></template>

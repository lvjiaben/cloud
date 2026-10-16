<script setup lang="ts">
import type { NuxtError } from '#app';
import { computed } from 'vue';

const props = defineProps<{ error: NuxtError }>();
const { locale } = useI18n();
const chinese = computed(() => locale.value !== 'en-US');
const isNotFound = computed(() => props.error.statusCode === 404);
const title = computed(() =>
  isNotFound.value
    ? chinese.value
      ? '该内容不存在或已下线'
      : 'This content is unavailable'
    : chinese.value
      ? '页面暂时无法打开'
      : 'This page is temporarily unavailable',
);
const action = computed(() => (chinese.value ? '返回首页' : 'Back to home'));
useSeoMeta({ robots: 'noindex, nofollow', title });
</script>

<template>
  <main class="error-page">
    <p>{{ error.statusCode }}</p>
    <h1>{{ title }}</h1>
    <NuxtLink to="/">{{ action }}</NuxtLink>
  </main>
</template>

<style scoped>
.error-page { display: grid; min-height: 100vh; place-content: center; justify-items: start; padding: 32px; color: #172033; background: #f3f6fa; }
.error-page p { margin: 0 0 8px; color: #1677ff; font-weight: 700; }
.error-page h1 { max-width: 680px; margin: 0 0 28px; font-size: clamp(36px, 7vw, 64px); letter-spacing: -.04em; }
.error-page a { padding: 10px 18px; border-radius: 0; color: #fff; background: #1677ff; }
</style>

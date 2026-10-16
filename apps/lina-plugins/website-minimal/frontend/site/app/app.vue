<script setup lang="ts">
// This Nuxt application owns public website pages only.
const head = useLocaleHead();
useHead(() => ({
  htmlAttrs: head.value.htmlAttrs,
  link: head.value.link,
  meta: head.value.meta,
}));
</script>

<template>
  <PublicSiteShell>
    <NuxtPage />
  </PublicSiteShell>
</template>

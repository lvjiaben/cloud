<script setup lang="ts">const route = useRoute();</script>
<template><PublicContentPage kind="news" :slug="String(route.params.slug || '')" /></template>

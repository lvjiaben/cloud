<script setup lang="ts">const route = useRoute();</script>
<template><PublicContentPage kind="knowledge" :slug="String(route.params.slug || '')" /></template>

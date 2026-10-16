<script setup lang="ts">const route = useRoute();</script>
<template><PublicContentPage kind="downloads" :slug="String(route.params.slug || '')" /></template>

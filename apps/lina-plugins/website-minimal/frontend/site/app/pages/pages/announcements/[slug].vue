<script setup lang="ts">const route = useRoute();</script>
<template><PublicContentPage kind="announcements" :slug="String(route.params.slug || '')" /></template>

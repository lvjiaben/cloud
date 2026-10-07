<script setup lang="ts">
import { watch } from "vue";
import { useVbenDrawer } from "@vben/common-ui";
const props = withDefaults(defineProps<{ open?: boolean; title?: string }>(), { open: false, title: "" });
const emit = defineEmits<{ "after-open-change": [value: boolean]; close: []; "update:open": [value: boolean] }>();
const [Drawer, drawerApi] = useVbenDrawer({ onCancel() { emit("close"); emit("update:open", false); drawerApi.close(); }, onOpenChange(open) { emit("after-open-change", open); if (!open) emit("update:open", false); } });
watch(() => props.open, (open) => (open ? drawerApi.open() : drawerApi.close()), { immediate: true });
</script>
<template><Drawer :footer="false" :title="title" class="w-[min(96vw,1100px)]"><slot /></Drawer></template>

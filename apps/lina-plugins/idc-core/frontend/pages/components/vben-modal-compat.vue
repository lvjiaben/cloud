<script setup lang="ts">
import { watch } from "vue";

import { useVbenModal } from "@vben/common-ui";

const props = withDefaults(
  defineProps<{
    confirmLoading?: boolean;
    footer?: unknown;
    okButtonProps?: { disabled?: boolean };
    okText?: string;
    open?: boolean;
    title?: string;
  }>(),
  { confirmLoading: false, open: false, title: "" },
);

const emit = defineEmits<{
  cancel: [];
  ok: [];
  "update:open": [value: boolean];
}>();

const [Modal, modalApi] = useVbenModal({
  // Closed nested dialogs must leave the popup stack. Keeping their content
  // mounted makes the hidden child consume the next Escape key, so the
  // visible parent dialog cannot be dismissed.
  destroyOnClose: true,
  onCancel() {
    emit("cancel");
    emit("update:open", false);
    modalApi.close();
  },
  onConfirm() {
    if (!props.okButtonProps?.disabled) emit("ok");
  },
  onOpenChange(open) {
    if (!open) emit("update:open", false);
  },
});

watch(
  () => props.open,
  (open) => (open ? modalApi.open() : modalApi.close()),
  { immediate: true },
);

watch(
  () => props.confirmLoading,
  (loading) => (loading ? modalApi.lock() : modalApi.unlock()),
  { immediate: true },
);
</script>

<template>
  <Modal
    :confirm-disabled="okButtonProps?.disabled"
    :confirm-text="okText"
    :footer="footer === null || footer === false ? false : undefined"
    :title="title"
    class="w-[min(96vw,760px)]"
  >
    <slot />
  </Modal>
</template>

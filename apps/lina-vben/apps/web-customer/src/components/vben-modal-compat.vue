<script setup lang="ts">
import { watch } from 'vue';
import { useVbenModal } from '@vben/common-ui';

const props = withDefaults(
  defineProps<{
    cancelText?: string;
    confirmLoading?: boolean;
    footer?: unknown;
    okButtonProps?: { disabled?: boolean };
    okText?: string;
    open?: boolean;
    title?: string;
  }>(),
  { confirmLoading: false, open: false, title: '' },
);
const emit = defineEmits<{
  cancel: [];
  ok: [];
  'update:open': [value: boolean];
}>();
const [Modal, modalApi] = useVbenModal({
  onCancel() {
    emit('cancel');
    emit('update:open', false);
    modalApi.close();
  },
  onConfirm() {
    if (!props.okButtonProps?.disabled) emit('ok');
  },
  onOpenChange(open) {
    if (!open) emit('update:open', false);
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
    :cancel-text="cancelText"
    :confirm-disabled="okButtonProps?.disabled"
    :confirm-text="okText"
    :footer="footer === null || footer === false ? false : undefined"
    :title="title"
    class="w-[min(96vw,760px)]"
    ><slot
  /></Modal>
</template>

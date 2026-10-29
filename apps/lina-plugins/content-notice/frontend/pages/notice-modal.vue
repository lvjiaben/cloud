<script setup lang="ts">
import { computed, ref } from "vue";

import { useVbenDrawer } from "@vben/common-ui";

import {
  Form,
  FormItem,
  Input,
  InputNumber,
  message,
  RadioGroup,
} from "ant-design-vue";

import { noticeAdd, noticeInfo, noticeUpdate } from "./notice-client";
import { $t } from "#/locales";
import { TiptapEditor } from "#/components/tiptap";

const emit = defineEmits<{ reload: [] }>();

interface FormData {
  id?: number;
  title: string;
  status: number;
  sortOrder: number;
  content: string;
}

const defaultValues: FormData = {
  id: undefined,
  title: "",
  status: 0,
  sortOrder: 0,
  content: "",
};

const isEdit = computed(() => !!formData.value.id);
const formData = ref<FormData>({ ...defaultValues });
const ready = ref(false);
const title = computed(() =>
  isEdit.value
    ? $t("plugin.content-notice.drawer.editTitle")
    : $t("plugin.content-notice.drawer.createTitle"),
);

const formRules = computed(() => ({
  title: [
    { message: $t("plugin.content-notice.validation.title"), required: true },
  ],
  status: [
    { message: $t("plugin.content-notice.validation.status"), required: true },
  ],
  content: [
    { message: $t("plugin.content-notice.validation.content"), required: true },
  ],
}));

const { validate, validateInfos, resetFields } = Form.useForm(
  formData,
  formRules,
);

const [Drawer, drawerApi] = useVbenDrawer({
  class: "notice-editor-drawer w-[min(96vw,1100px)]",
  modal: true,
  closeOnClickModal: false,
  contentClass: "overflow-y-auto p-4",
  destroyOnClose: true,
  onConfirm: handleConfirm,
  onOpenChange: async (isOpen: boolean) => {
    if (!isOpen) return;
    ready.value = false;
    const data = drawerApi.getData();
    if (data?.id) {
      drawerApi.setState({ confirmLoading: true });
      try {
        const record = await noticeInfo(data.id);
        formData.value = {
          id: record.id,
          title: record.title,
          status: record.status,
          sortOrder: record.sortOrder,
          content: record.content || "",
        };
        ready.value = true;
      } catch {
        message.error($t("plugin.content-notice.messages.loadFailed"));
        drawerApi.close();
      } finally {
        drawerApi.setState({ confirmLoading: false });
      }
    } else {
      formData.value = { ...defaultValues };
      resetFields();
      ready.value = true;
    }
  },
});

async function handleConfirm() {
  if (!ready.value) return;
  try {
    drawerApi.lock(true);
    await validate();

    const { id, ...submitData } = formData.value;
    if (isEdit.value && id) {
      await noticeUpdate(id, submitData);
      message.success($t("pages.common.updateSuccess"));
    } else {
      await noticeAdd(submitData);
      message.success($t("pages.common.createSuccess"));
    }
    emit("reload");
    drawerApi.close();
  } catch (error) {
    console.error(error);
  } finally {
    drawerApi.lock(false);
  }
}
</script>

<template>
  <Drawer :title="title">
    <Form layout="vertical">
      <FormItem
        :label="$t('plugin.content-notice.fields.title')"
        v-bind="validateInfos.title"
      >
        <Input
          v-model:value="formData.title"
          :placeholder="$t('plugin.content-notice.placeholders.title')"
        />
      </FormItem>
      <div class="grid lg:grid-cols-2 sm:grid-cols-1">
        <FormItem
          :label="$t('plugin.content-notice.fields.status')"
          v-bind="validateInfos.status"
        >
          <RadioGroup
            v-model:value="formData.status"
            button-style="solid"
            option-type="button"
            :options="[
              { label: $t('plugin.content-notice.status.draft'), value: 0 },
              { label: $t('plugin.content-notice.status.published'), value: 1 },
            ]"
          />
        </FormItem>
      </div>
      <FormItem :label="$t('plugin.content-notice.fields.sortOrder')">
        <InputNumber
          v-model:value="formData.sortOrder"
          :min="0"
          :precision="0"
        />
      </FormItem>
      <FormItem
        :label="$t('plugin.content-notice.fields.content')"
        v-bind="validateInfos.content"
      >
        <TiptapEditor
          v-model="formData.content"
          :height="300"
          :placeholder="$t('plugin.content-notice.validation.content')"
          scene="notice_image"
        />
      </FormItem>
    </Form>
  </Drawer>
</template>

<style>
.notice-editor-drawer {
  box-shadow: -12px 0 40px rgb(0 0 0 / 18%);
}
.notice-editor-drawer[data-state="open"] {
  animation: notice-drawer-in 260ms ease-out;
}
.notice-editor-drawer[data-state="closed"] {
  animation: notice-drawer-out 180ms ease-in;
}
@keyframes notice-drawer-in {
  from { transform: translateX(100%); }
  to { transform: translateX(0); }
}
@keyframes notice-drawer-out {
  from { transform: translateX(0); }
  to { transform: translateX(100%); }
}
</style>

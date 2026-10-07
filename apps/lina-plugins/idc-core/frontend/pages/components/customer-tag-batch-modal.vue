<script setup lang="ts">
import type { CustomerSegment, CustomerSummary } from "../customer-client";

import { onBeforeUnmount, reactive, ref } from "vue";

import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";

import { message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { customerSegments, customerTagBatch } from "../customer-client";

const emit = defineEmits<{ success: [] }>();

const selectedRows = ref<CustomerSummary[]>([]);
const idempotencyKey = ref("");
const tagSearchParams = reactive({ namePrefix: "" });
let tagSearchTimer: ReturnType<typeof setTimeout> | undefined;

function localizedSegmentName(item: CustomerSegment) {
  return preferences.app.locale === "zh-CN"
    ? item.nameZhCn || item.nameEnUs || item.code
    : item.nameEnUs || item.nameZhCn || item.code;
}

async function tagOptions(params: { namePrefix?: string }) {
  const result = await customerSegments({
    kind: "tag",
    limit: 25,
    locale: preferences.app.locale === "zh-CN" ? "zh-CN" : "en-US",
    namePrefix: params.namePrefix || "",
  });
  return result.items.map((item) => ({
    label: localizedSegmentName(item),
    value: item.number,
  }));
}

function scheduleTagSearch(value: string) {
  if (tagSearchTimer) clearTimeout(tagSearchTimer);
  tagSearchTimer = setTimeout(() => {
    tagSearchParams.namePrefix = value.trim();
    tagSearchTimer = undefined;
  }, 250);
}

const [Form, formApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: "w-full" },
    labelWidth: 96,
  },
  schema: [
    {
      component: "RadioGroup",
      componentProps: {
        "data-testid": "customer-tag-batch-action",
        buttonStyle: "solid",
        optionType: "button",
        options: [
          {
            label: $t("plugin.idc-core.customer.tagBatch.assign"),
            value: "assign",
          },
          {
            label: $t("plugin.idc-core.customer.tagBatch.revoke"),
            value: "revoke",
          },
        ],
      },
      defaultValue: "assign",
      fieldName: "action",
      label: $t("plugin.idc-core.customer.tagBatch.action"),
      rules: z.enum(["assign", "revoke"]),
    },
    {
      component: "ApiSelect",
      componentProps: {
        "data-testid": "customer-tag-batch-tags",
        api: tagOptions,
        filterOption: false,
        labelField: "label",
        maxCount: 20,
        mode: "multiple",
        onSearch: scheduleTagSearch,
        params: tagSearchParams,
        placeholder: $t(
          "plugin.idc-core.customer.tagBatch.tagsPlaceholder",
        ),
        showSearch: true,
        valueField: "value",
      },
      fieldName: "tagNos",
      label: $t("plugin.idc-core.customer.tagBatch.tags"),
      rules: z
        .array(z.string())
        .min(1, {
          message: $t("plugin.idc-core.customer.tagBatch.tagsRequired"),
        })
        .max(20),
    },
    {
      component: "Textarea",
      componentProps: {
        "data-testid": "customer-tag-batch-reason",
        maxlength: 256,
        placeholder: $t(
          "plugin.idc-core.customer.tagBatch.reasonPlaceholder",
        ),
        rows: 3,
        showCount: true,
      },
      fieldName: "reason",
      formItemClass: "items-start",
      label: $t("plugin.idc-core.customer.tagBatch.reason"),
      rules: z
        .string()
        .trim()
        .min(1, {
          message: $t(
            "plugin.idc-core.customer.tagBatch.reasonRequired",
          ),
        })
        .max(256),
    },
  ],
  showDefaultActions: false,
});

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  async onClosed() {
    selectedRows.value = [];
    idempotencyKey.value = "";
    tagSearchParams.namePrefix = "";
    await formApi.resetForm();
  },
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const values = await formApi.getValues<{
      action: "assign" | "revoke";
      reason: string;
      tagNos: string[];
    }>();
    modalApi.lock(true);
    try {
      await customerTagBatch({
        action: values.action,
        customerNos: selectedRows.value.map((row) => row.customerNo),
        idempotencyKey: idempotencyKey.value,
        reason: values.reason.trim(),
        tagNos: values.tagNos,
      });
      message.success($t("plugin.idc-core.customer.tagBatch.success"));
      emit("success");
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{ rows?: CustomerSummary[] }>();
    selectedRows.value = data?.rows ?? [];
    idempotencyKey.value = `customer-tag-${crypto.randomUUID()}`;
    await formApi.setValues({ action: "assign", reason: "", tagNos: [] });
  },
});

onBeforeUnmount(() => {
  if (tagSearchTimer) clearTimeout(tagSearchTimer);
});
</script>

<template>
  <Modal
    :title="$t('plugin.idc-core.customer.tagBatch.title')"
    class="w-[620px]"
  >
    <div class="batch-summary" data-testid="customer-tag-batch-summary">
      <strong>{{ selectedRows.length }}</strong>
      {{
        $t("plugin.idc-core.customer.tagBatch.selected", {
          count: selectedRows.length,
        })
      }}
    </div>
    <Form />
    <p class="audit-note">
      {{ $t("plugin.idc-core.customer.tagBatch.auditNote") }}
    </p>
  </Modal>
</template>

<style scoped>
.batch-summary {
  background: hsl(var(--muted));
  border-radius: 10px;
  margin-bottom: 18px;
  padding: 12px 14px;
}

.batch-summary strong {
  color: hsl(var(--primary));
  font-size: 18px;
  margin-right: 6px;
}

.audit-note {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  margin: 10px 0 0 96px;
}
</style>

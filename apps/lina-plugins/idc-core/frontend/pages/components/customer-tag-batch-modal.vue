<script setup lang="ts">
import type { CustomerSegment, CustomerSummary } from "../customer-client";

import { onBeforeUnmount, ref } from "vue";

import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";

import { message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { customerSegments, customerTagBatch } from "../customer-client";

const emit = defineEmits<{ success: [] }>();

const selectedRows = ref<CustomerSummary[]>([]);
const idempotencyKey = ref("");
const tagOptionsCache = new Map<string, { label: string; value: string }>();
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
  for (const item of result.items) tagOptionsCache.set(item.number, {label: localizedSegmentName(item), value:item.number});
  return Array.from(tagOptionsCache.values());
}

function scheduleTagSearch(value: string) {
  if (tagSearchTimer) clearTimeout(tagSearchTimer);
  tagSearchTimer = setTimeout(() => {
    void refreshTagOptions(value.trim());
    tagSearchTimer = undefined;
  }, 250);
}

const [Form, formApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: "w-full" },
    labelWidth: 96,
  },
  schema: [],
  showDefaultActions: false,
});

async function refreshTagOptions(namePrefix = "") {
  const options = await tagOptions({ namePrefix });
  await formApi.updateSchema([{fieldName:"tagNos",componentProps:{options}}]);
}
function batchSchema() {
  return [
    { fieldName:"action", component:"RadioGroup", label:$t("plugin.idc-core.customer.tagBatch.action"), defaultValue:"assign", componentProps:{"data-testid":"customer-tag-batch-action",buttonStyle:"solid",optionType:"button",options:[{label:$t("plugin.idc-core.customer.tagBatch.assign"),value:"assign"},{label:$t("plugin.idc-core.customer.tagBatch.revoke"),value:"revoke"}]},rules:z.enum(["assign","revoke"]) },
    { fieldName:"tagNos",component:"Select",label:$t("plugin.idc-core.customer.tagBatch.tags"),componentProps:{"data-testid":"customer-tag-batch-tags",mode:"tags",showSearch:true,filterOption:false,onSearch:scheduleTagSearch,maxCount:20,options:[],placeholder:$t("plugin.idc-core.customer.tagBatch.tagsPlaceholder")},rules:z.array(z.string().trim().min(1).max(128)).min(1,{message:$t("plugin.idc-core.customer.tagBatch.tagsRequired")}).max(20) },
  ];
}

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  async onClosed() {
    if (tagSearchTimer) clearTimeout(tagSearchTimer);
    selectedRows.value = [];
    idempotencyKey.value = "";
    tagOptionsCache.clear();
    await formApi.resetForm();
  },
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const values = await formApi.getValues<{
      action: "assign" | "revoke";
      tagNos: string[];
    }>();
    if (values.action === "revoke" && values.tagNos.some(value => !tagOptionsCache.has(value))) {
      message.warning($t("plugin.idc-core.customer.tagBatch.existingOnly"));
      return false;
    }
    modalApi.lock(true);
    try {
      await customerTagBatch({
        action: values.action,
        customerNos: selectedRows.value.map((row) => row.customerNo),
        idempotencyKey: idempotencyKey.value,
        tagNos: values.tagNos.filter(value => tagOptionsCache.has(value)),
        tagNames: values.tagNos.filter(value => !tagOptionsCache.has(value)).map(value => value.trim()),
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
    await formApi.setState({schema:batchSchema()});
    await formApi.setValues({ action: "assign", tagNos: [] });
    await refreshTagOptions();
  },
});

onBeforeUnmount(() => {
  if (tagSearchTimer) clearTimeout(tagSearchTimer);
});
</script>

<template>
  <Modal
    :title="$t('plugin.idc-core.customer.tagBatch.title')"
    class="w-[min(96vw,620px)]"
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

</style>

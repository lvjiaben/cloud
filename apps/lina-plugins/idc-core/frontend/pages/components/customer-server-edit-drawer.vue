<script setup lang="ts">
import type { VbenFormSchema } from "#/adapter/form";
import { ref } from "vue";
import { useVbenDrawer } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Alert, Button, Descriptions, Modal, message } from "ant-design-vue";
import { useVbenForm } from "#/adapter/form";
import { pluginApiPath, requestClient } from "#/api/request";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import { minorToMajorAmount } from "../catalog-money";

interface Option {
  optionNo: string;
  nameZhCN: string;
  nameEnUS: string;
  minimumQuantity: number;
  maximumQuantity: number;
  defaultQuantity: number;
}
interface Options {
  revision: string;
  currencyCode: string;
  groups: {
    groupNo: string;
    nameZhCN: string;
    nameEnUS: string;
    selectionMode: string;
    minimumSelected: number;
    maximumSelected: number;
    options: Option[];
  }[];
  currentSelections: { optionNo: string; quantity: number }[];
}
interface Quote {
  quoteNo: string;
  direction: string;
  currencyCode: string;
  chargeAmountMinor: number;
  targetRecurringAmountMinor: number;
  effectiveAt: number;
  expiresAt: number;
}
const emit = defineEmits<{ changed: [] }>();
const text = (key: string) => $t(`plugin.idc-core.customer.workspace.${key}`);
const serviceNo = ref("");
const name = ref("");
const version = ref(0);
const options = ref<Options>();
const quote = ref<Quote>();
const busy = ref(false);
const failed = ref(false);
let generation = 0;
let quoteKey = "";
let checkoutKey = "";
const path = (suffix: string) =>
  pluginApiPath(
    "idc-core",
    `/admin/servers/${encodeURIComponent(serviceNo.value)}/${suffix}`,
  );
const [MetadataForm, metadataForm] = useVbenForm({
  commonConfig: { labelWidth: 90, componentProps: { class: "w-full" } },
  schema: [],
  showDefaultActions: false,
});
const [ConfigurationForm, configurationForm] = useVbenForm({
  commonConfig: { labelWidth: 110, componentProps: { class: "w-full" } },
  schema: [],
  showDefaultActions: false,
});
function label(item: { nameZhCN: string; nameEnUS: string }) {
  return preferences.app.locale === "zh-CN"
    ? item.nameZhCN || item.nameEnUS
    : item.nameEnUS || item.nameZhCN;
}
async function load() {
  const current = ++generation;
  failed.value = false;
  options.value = undefined;
  quote.value = undefined;
  drawerApi.setState({ loading: true });
  try {
    const result = await requestClient.get<{
      metadata: { alias: string; remark: string; version: number };
    }>(path("metadata"));
    if (current !== generation) return;
    version.value = result.metadata.version;
    await metadataForm.setState({
      schema: [
        {
          fieldName: "alias",
          label: text("alias"),
          component: "Input",
          componentProps: { maxlength: 128 },
        },
        {
          fieldName: "remark",
          label: text("remark"),
          component: "Textarea",
          componentProps: { maxlength: 1000, rows: 3 },
        },
      ],
    });
    await metadataForm.setValues(result.metadata);
    // Unsupported products can still edit their note; configuration uses only the owner workflow.
    try {
      const available = await requestClient.get<Options>(
        path("resize-options"),
      );
      if (current !== generation) return;
      options.value = available;
      const schema: VbenFormSchema[] = [];
      const values: Record<string, unknown> = {};
      available.groups.forEach((group, index) => {
        const field = `group_${index}`;
        const selected = available.currentSelections.filter((item) =>
          group.options.some((option) => option.optionNo === item.optionNo),
        );
        const multiple = group.selectionMode === "multiple";
        schema.push({
          fieldName: field,
          label: label(group),
          component: "Select",
          componentProps: {
            allowClear: group.minimumSelected === 0,
            mode: multiple ? "multiple" : undefined,
            options: group.options.map((item) => ({
              label: label(item),
              value: item.optionNo,
            })),
          },
          rules: group.minimumSelected > 0 ? "required" : undefined,
        });
        values[field] = multiple
          ? selected.map((item) => item.optionNo)
          : selected[0]?.optionNo;
        group.options.forEach((option, optionIndex) => {
          if (option.maximumQuantity <= 1) return;
          const quantityField = `quantity_${index}_${optionIndex}`;
          schema.push({
            fieldName: quantityField,
            label: `${label(option)} · ${text("quantity")}`,
            component: "InputNumber",
            componentProps: {
              min: Math.max(1, option.minimumQuantity),
              max: option.maximumQuantity,
            },
            dependencies: {
              triggerFields: [field],
              show: (data) =>
                Array.isArray(data[field])
                  ? data[field].includes(option.optionNo)
                  : data[field] === option.optionNo,
            },
          });
          values[quantityField] =
            selected.find((item) => item.optionNo === option.optionNo)
              ?.quantity || Math.max(1, option.defaultQuantity);
        });
      });
      await configurationForm.setState({ schema });
      await configurationForm.setValues(values);
    } catch {
      if (current === generation) options.value = undefined;
    }
  } catch {
    if (current === generation) failed.value = true;
  } finally {
    if (current === generation) drawerApi.setState({ loading: false });
  }
}
async function saveNote() {
  if (busy.value) return;
  const current = generation;
  busy.value = true;
  try {
    const values = await metadataForm.getValues();
    const result = await requestClient.put<{ metadata: { version: number } }>(
      path("metadata"),
      {
        alias: values.alias || "",
        remark: values.remark || "",
        expectedVersion: version.value,
      },
    );
    if (current !== generation) return;
    version.value = result.metadata.version;
    message.success(text("saved"));
    emit("changed");
  } finally {
    busy.value = false;
  }
}
async function preview() {
  if (busy.value || !options.value) return;
  if (!(await configurationForm.validate()).valid) return;
  const current = generation;
  busy.value = true;
  try {
    const values = await configurationForm.getValues();
    const selections: { optionNo: string; quantity: number }[] = [];
    options.value.groups.forEach((group, index) => {
      const selected = values[`group_${index}`];
      const keys = Array.isArray(selected) ? selected : [selected];
      group.options.forEach((option, optionIndex) => {
        if (keys.includes(option.optionNo))
          selections.push({
            optionNo: option.optionNo,
            quantity: Number(values[`quantity_${index}_${optionIndex}`] || 1),
          });
      });
    });
    quoteKey = crypto.randomUUID();
    checkoutKey = crypto.randomUUID();
    const result = await requestClient.post<{ quote: Quote }>(
      path("resize-quotes"),
      {
        revision: options.value.revision,
        currencyCode: options.value.currencyCode,
        selections,
        idempotencyKey: quoteKey,
      },
    );
    if (current === generation) quote.value = result.quote;
  } finally {
    busy.value = false;
  }
}
function confirmChange() {
  const frozen = quote.value;
  const current = generation;
  if (!frozen || busy.value) return;
  Modal.confirm({
    title: text("changeConfirm"),
    content: text(
      frozen.direction === "downgrade" ? "downgradeHint" : "upgradeHint",
    ),
    onOk: async () => {
      if (current !== generation || quote.value !== frozen) return;
      if (Date.now() >= frozen.expiresAt) {
        quote.value = undefined;
        message.warning(text("quoteExpired"));
        return;
      }
      busy.value = true;
      try {
        await requestClient.post(
          path(`resize-quotes/${encodeURIComponent(frozen.quoteNo)}/checkout`),
          { idempotencyKey: checkoutKey },
        );
        if (current !== generation) return;
        message.success(
          text(
            frozen.chargeAmountMinor > 0 ? "billCreated" : "changeScheduled",
          ),
        );
        emit("changed");
        drawerApi.close();
      } finally {
        busy.value = false;
      }
    },
  });
}
const [Drawer, drawerApi] = useVbenDrawer({
  showConfirmButton: false,
  onOpenChange(open) {
    if (open) {
      const data = drawerApi.getData<{ serviceNo: string; name: string }>();
      serviceNo.value = data.serviceNo;
      name.value = data.name;
      void load();
    } else {
      ++generation;
      quote.value = undefined;
      options.value = undefined;
    }
  },
});
</script>
<template>
  <Drawer :title="text('editProduct')" class="w-[640px] max-w-full">
    <Alert v-if="failed" type="error" :message="text('loadFailed')"
      ><template #action
        ><Button @click="load">{{ text("retry") }}</Button></template
      ></Alert
    >
    <div v-else class="server-editor" data-testid="customer-server-edit">
      <div class="product-heading">
        <strong>{{ name }}</strong
        ><span>{{ serviceNo }}</span>
      </div>
      <section>
        <h3>{{ text("displayInformation") }}</h3>
        <MetadataForm /><Button
          type="primary"
          :loading="busy"
          data-testid="customer-server-save-note"
          @click="saveNote"
          >{{ text("save") }}</Button
        >
      </section>
      <section>
        <h3>{{ text("configuration") }}</h3>
        <template v-if="options">
          <div v-show="!quote">
            <ConfigurationForm /><Button
              :loading="busy"
              data-testid="customer-server-preview"
              @click="preview"
              >{{ text("viewFee") }}</Button
            >
          </div>
          <template v-if="quote"
            ><Descriptions bordered :column="1"
              ><Descriptions.Item :label="text('changeFee')"
                >{{ quote.currencyCode }}
                {{
                  minorToMajorAmount(
                    String(quote.chargeAmountMinor),
                    quote.currencyCode,
                  )
                }}</Descriptions.Item
              ><Descriptions.Item :label="text('nextPrice')"
                >{{ quote.currencyCode }}
                {{
                  minorToMajorAmount(
                    String(quote.targetRecurringAmountMinor),
                    quote.currencyCode,
                  )
                }}</Descriptions.Item
              ><Descriptions.Item :label="text('effectiveAt')">{{
                formatTimestamp(quote.effectiveAt)
              }}</Descriptions.Item></Descriptions
            >
            <Alert
              type="info"
              :message="
                text(
                  quote.direction === 'downgrade'
                    ? 'downgradeHint'
                    : 'upgradeHint',
                )
              "
            />
            <div class="edit-actions">
              <Button :disabled="busy" @click="quote = undefined">{{
                text("back")
              }}</Button
              ><Button
                type="primary"
                :loading="busy"
                data-testid="customer-server-confirm-change"
                @click="confirmChange"
                >{{ text("confirmChange") }}</Button
              >
            </div>
          </template>
        </template>
        <p v-else class="muted">{{ text("configurationUnavailable") }}</p>
      </section>
    </div>
  </Drawer>
</template>
<style scoped>
.server-editor {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding: 8px;
}
.product-heading {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.product-heading strong {
  font-size: 17px;
}
.product-heading span,
.muted {
  color: hsl(var(--muted-foreground));
}
section {
  border-top: 1px solid hsl(var(--border));
  padding-top: 20px;
}
h3 {
  margin: 0 0 18px;
  font-size: 15px;
  font-weight: 600;
}
.edit-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 16px;
}
section :deep(.ant-alert) {
  margin-top: 14px;
}
</style>

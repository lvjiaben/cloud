<script setup lang="ts">
// Merchant-authored child values are structured rows. This form never calls a
// supplier, creates a quote, or interprets a value as a server-specific field.
import type { VbenFormSchema } from "#/adapter/form";
import type { CatalogConfigurationCandidate } from "../catalog-configuration-state";
import { computed, nextTick, ref, watch } from "vue";
import { useVbenModal } from "@vben/common-ui";
import VbenDataGrid from "./vben-data-grid.vue";
import { preferences } from "@vben/preferences";
import { Alert, Button, Empty } from "ant-design-vue";
import { useVbenForm } from "#/adapter/form";
import { $t } from "#/locales";

const props = defineProps<{
  value?: CatalogConfigurationCandidate[];
  disabled?: boolean;
  numeric?: boolean;
  multilingual?: boolean;
  upstream?: {
    bindings: Record<string, string>;
    choices?: CatalogConfigurationCandidate[];
    text?: boolean;
  };
}>();
const emit = defineEmits<{
  "update:value": [CatalogConfigurationCandidate[]];
  "update:bindings": [Record<string, string>];
}>();
const rows = ref<
  Array<CatalogConfigurationCandidate & { id: string; upstreamValue?: string }>
>([]);
const activeID = ref("");
let lastEmitted = "";
let sequence = 0;
const tr = (key: string) => $t(`plugin.idc-core.catalog.optionValues.${key}`);
const batchText = ref("");
const batchTr = (key: string, params: Record<string, unknown> = {}) => $t(`plugin.idc-core.catalog.optionBatch.${key}`, params);
const batchColumns = computed(() => [
  { key: "line", title: batchTr("line"), width: 65 },
  { key: "value", title: tr("value"), minWidth: 110 },
  { key: "name", title: tr("name"), minWidth: 160 },
  ...(props.upstream ? [{ key: "upstreamValue", title: tr("upstreamValue"), minWidth: 150 }] : []),
  { key: "error", title: batchTr("validation"), minWidth: 180 },
]);
const batchRows = computed(() => {
  const seen = new Set(rows.value.map(row => row.value));
  return batchText.value.split(/\r?\n/).map((text, index) => ({text, line:index+1})).filter(row => row.text.trim()).slice(0, 513).map(({text, line}) => {
    // Spreadsheet paste uses tabs; commas inside an interface value remain literal.
    const cells = text.split("\t");
    const value = (cells[0] || "").trim();
    const name = (cells[1] || "").trim();
    const upstreamValue = cells[2] || "";
    let error = "";
    if (cells.length > (props.upstream ? 3 : 2)) error = batchTr("tooManyColumns");
    else if (!value || !name) error = batchTr("missing");
    else if (value.length > 256 || name.length > 256 || upstreamValue.length > 256) error = batchTr("tooLong");
    else if (props.numeric && (!/^(0|[1-9]\d*|-[1-9]\d*)$/.test(value) || !Number.isSafeInteger(Number(value)))) error = batchTr("numeric");
    else if (seen.has(value)) error = batchTr("duplicate");
    else if (upstreamValue && props.upstream && (!props.upstream.choices && !props.upstream.text || props.upstream.choices && !props.upstream.choices.some(choice => choice.value === upstreamValue))) error = batchTr("invalidUpstream");
    seen.add(value);
    return {line, value, name, upstreamValue, error};
  });
});
const batchLimit = computed(() => rows.value.length + batchRows.value.length > 512);
const batchValid = computed(() => !!batchRows.value.length && !batchLimit.value && batchRows.value.every(row => !row.error));
const [BatchForm, batchForm] = useVbenForm({
  schema: [], showDefaultActions: false, wrapperClass: "grid-cols-1",
  handleValuesChange: values => { batchText.value = String(values.content || ""); },
});
const [BatchModal, batchModal] = useVbenModal({
  onConfirm: applyBatch,
  async onOpenChange(open) {
    if (!open) return;
    batchText.value = "";
    await batchForm.setState({schema:[{fieldName:"content", component:"Textarea", label:batchTr("paste"), componentProps:{rows:6, maxlength:400000, placeholder:batchTr(props.upstream ? "exampleWithAPI" : "example"), "data-testid":"catalog-option-batch-input"}}]});
    await batchForm.setValues({content:""}, false);
  },
});
async function applyBatch() {
  if (!batchValid.value || props.disabled) return;
  const added = batchRows.value.map(row => ({id:String(++sequence), value:row.value, label:{zhCN:row.name,enUS:""}, upstreamValue:row.upstreamValue}));
  rows.value.push(...added);
  publish();
  if (added[0]) await select(added[0].id);
  await batchModal.close();
}
async function copy() {
  if (props.disabled || !selected.value || rows.value.length >= 512) return;
  const source = selected.value;
  const row = { ...source, id:String(++sequence), value:"", label:{...source.label} };
  rows.value.splice(rows.value.findIndex(item => item.id === source.id) + 1, 0, row);
  publish();
  await select(row.id);
}
const selected = computed(() =>
  rows.value.find((row) => row.id === activeID.value),
);
const [Form, formApi] = useVbenForm({
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
  commonConfig: { labelWidth: 88 },
});

function publish() {
  const value = rows.value.map(({ value, label, parentValue }) => ({
    value,
    label: { ...label },
    ...(parentValue ? { parentValue } : {}),
  }));
  lastEmitted = JSON.stringify(value);
  emit("update:value", value);
  if (props.upstream)
    emit(
      "update:bindings",
      Object.fromEntries(
        rows.value
          .filter((row) => row.upstreamValue)
          .map((row) => [row.value, row.upstreamValue!]),
      ),
    );
}
function change(
  property: "value" | "zhCN" | "enUS" | "upstreamValue",
  value: string,
) {
  const row = selected.value;
  if (!row) return;
  if (property === "value") row.value = value;
  else if (property === "upstreamValue") row.upstreamValue = value;
  else row.label[property] = value;
  publish();
}
async function render() {
  const row = selected.value;
  const schema: VbenFormSchema[] = row
    ? [
        {
          fieldName: "value",
          component: "Input",
          label: tr("value"),
          componentProps: {
            disabled: props.disabled,
            maxlength: 256,
            inputmode: props.numeric ? "numeric" : "text",
            "data-testid": "catalog-option-child-value",
            onChange: (event: Event) =>
              change("value", (event.target as HTMLInputElement).value),
          },
        },
        ...(
          ["zhCN", ...(props.multilingual ? ["enUS"] : [])] as Array<
            "zhCN" | "enUS"
          >
        ).map((language) => ({
          fieldName: language,
          component: "Input" as const,
          label: tr(language === "zhCN" ? "name" : "englishName"),
          componentProps: {
            disabled: props.disabled,
            maxlength: 256,
            "data-testid": `catalog-option-child-${language}`,
            onChange: (event: Event) =>
              change(language, (event.target as HTMLInputElement).value),
          },
        })),
        ...(props.upstream
          ? [
              {
                fieldName: "upstreamValue",
                component: props.upstream.choices
                  ? ("Select" as const)
                  : ("Input" as const),
                label: tr("upstreamValue"),
                componentProps: props.upstream.choices
                  ? {
                      disabled: props.disabled,
                      options: [
                        { value: "", label: tr("sameValue") },
                        ...(props.upstream.choices || []).map((choice) => ({
                          value: choice.value,
                          label: `${preferences.app.locale === "zh-CN" ? choice.label.zhCN || choice.label.enUS : choice.label.enUS || choice.label.zhCN} · ${choice.value}`,
                        })),
                        ...(row.upstreamValue &&
                        !props.upstream.choices.some(
                          (choice) => choice.value === row.upstreamValue,
                        )
                          ? [
                              {
                                value: row.upstreamValue,
                                label: `${tr("savedValue")} · ${row.upstreamValue}`,
                              },
                            ]
                          : []),
                      ],
                      showSearch: true,
                      optionFilterProp: "label",
                      virtual: false,
                      "data-testid": "catalog-option-child-upstream",
                      onChange: (value: string) =>
                        change("upstreamValue", value),
                    }
                  : {
                      disabled: props.disabled || !props.upstream.text,
                      maxlength: 256,
                      placeholder: tr("sameValue"),
                      "data-testid": "catalog-option-child-upstream",
                      onChange: (event: Event) =>
                        change(
                          "upstreamValue",
                          (event.target as HTMLInputElement).value,
                        ),
                    },
              },
            ]
          : []),
      ]
    : [];
  await formApi.setState({ schema });
  await nextTick();
  await formApi.setValues(
    row
      ? {
          value: row.value,
          ...row.label,
          upstreamValue: row.upstreamValue || "",
        }
      : {},
    false,
  );
}
async function select(id: string) {
  activeID.value = id;
  await render();
}
async function add() {
  if (rows.value.length >= 512 || props.disabled) return;
  const row = {
    id: String(++sequence),
    value: "",
    label: { zhCN: "", enUS: "" },
  };
  rows.value.push(row);
  publish();
  await select(row.id);
}
async function remove() {
  const index = rows.value.findIndex((row) => row.id === activeID.value);
  if (index < 0 || props.disabled) return;
  rows.value.splice(index, 1);
  activeID.value = rows.value[Math.min(index, rows.value.length - 1)]?.id || "";
  publish();
  await render();
}
function move(direction: number) {
  const index = rows.value.findIndex((row) => row.id === activeID.value);
  const target = index + direction;
  if (props.disabled || target < 0 || target >= rows.value.length) return;
  [rows.value[index], rows.value[target]] = [
    rows.value[target]!,
    rows.value[index]!,
  ];
  publish();
}
watch(
  () => props.value,
  async (value) => {
    // Vben resets its field binding while applying the saved snapshot. Keep
    // the displayed rows until the replacement value arrives; an explicit
    // empty array still clears the editor.
    if (value === undefined) return;
    if (JSON.stringify(value || []) === lastEmitted) return;
    const previous = selected.value?.value;
    rows.value = (value || []).map((row) => ({
      ...row,
      label: { ...row.label },
      upstreamValue: props.upstream?.bindings[row.value] || "",
      id: String(++sequence),
    }));
    activeID.value =
      rows.value.find((row) => row.value === previous)?.id ||
      rows.value[0]?.id ||
      "";
    await render();
  },
  { immediate: true, deep: true },
);
watch(
  () => [props.multilingual, props.disabled, preferences.app.locale],
  () => void render(),
);
watch(
  () => props.upstream,
  () => {
    for (const row of rows.value)
      row.upstreamValue = props.upstream?.bindings[row.value] || "";
    void render();
  },
  { deep: true },
);
</script>

<template>
  <section class="option-values" data-testid="catalog-option-values-editor">
    <div class="option-values-toolbar">
      <span>{{ tr("items") }}</span>
      <div v-if="!disabled" class="flex flex-wrap gap-2">
      <Button size="small" data-testid="catalog-option-batch-open" :disabled="rows.length >= 512" @click="batchModal.open()">{{ batchTr("add") }}</Button>
      <Button
        v-if="!disabled"
        size="small"
        data-testid="catalog-option-child-add"
        :disabled="rows.length >= 512"
        @click="add"
        >{{ tr("add") }}</Button
      >
      </div>
    </div>
    <Empty
      v-if="!rows.length"
      :description="tr('empty')"
      :image="Empty.PRESENTED_IMAGE_SIMPLE"
    />
    <div v-else class="option-values-layout">
      <div class="option-values-list" role="list" :aria-label="tr('items')">
        <Button
          v-for="(row, index) in rows"
          :key="row.id"
          type="text"
          html-type="button"
          :class="{ selected: row.id === activeID }"
          :data-testid="`catalog-option-child-${index}`"
          :aria-pressed="row.id === activeID"
          @click="select(row.id)"
        >
          <span class="option-value-order">{{ index + 1 }}</span>
          <span class="option-value-name"
            >{{ row.label.zhCN || row.label.enUS || row.value || tr("unnamed")
            }}<small>{{ row.value || tr("valueMissing") }}</small></span
          >
          <span v-if="!disabled" class="option-value-edit">{{ $t("plugin.idc-core.catalog.customerOptions.edit") }}</span>
        </Button>
      </div>
      <div class="option-value-details">
        <Form />
        <div v-if="!disabled" class="option-value-actions">
          <Button size="small" data-testid="catalog-option-child-copy" :disabled="rows.length >= 512" @click="copy">{{ batchTr("copy") }}</Button>
          <Button
            size="small"
            :disabled="rows[0]?.id === activeID"
            :aria-label="tr('moveUp')"
            data-testid="catalog-option-child-up"
            @click="move(-1)"
            >{{ tr("moveUp") }}</Button
          >
          <Button
            size="small"
            :disabled="rows.at(-1)?.id === activeID"
            :aria-label="tr('moveDown')"
            data-testid="catalog-option-child-down"
            @click="move(1)"
            >{{ tr("moveDown") }}</Button
          >
          <Button
            size="small"
            danger
            data-testid="catalog-option-child-remove"
            @click="remove"
            >{{ tr("remove") }}</Button
          >
        </div>
      </div>
    </div>
    <BatchModal :title="batchTr('add')" :confirm-disabled="!batchValid || disabled" :confirm-text="batchTr('apply')" class="w-[900px] max-w-[95vw]" content-class="space-y-4" data-testid="catalog-option-batch-modal">
      <p class="text-sm text-muted-foreground">{{ batchTr(upstream ? 'instructionsWithAPI' : 'instructions') }}</p>
      <BatchForm />
      <Alert v-if="batchLimit" type="error" show-icon :message="batchTr('limit')" />
      <VbenDataGrid :columns="batchColumns" :data="batchRows" row-key="line" :toolbar="false" :height="260" data-testid="catalog-option-batch-preview" />
    </BatchModal>
  </section>
</template>

<style scoped>
.option-values {
  width: 100%;
  min-width: 0;
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
  background: hsl(var(--background));
}
.option-values-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border-bottom: 1px solid hsl(var(--border));
  font-size: 12px;
  color: hsl(var(--muted-foreground));
}
.option-values-layout {
  display: grid;
  grid-template-columns: minmax(130px, 0.7fr) minmax(200px, 1.3fr);
}
.option-values-list {
  max-height: 290px;
  overflow-y: auto;
  border-right: 1px solid hsl(var(--border));
  padding: 6px;
}
.option-values-list button {
  display: flex;
  height: auto;
  min-height: 40px;
  white-space: normal;
  border: 0;
  box-shadow: none;
  align-items: center;
  width: 100%;
  gap: 9px;
  padding: 9px;
  text-align: left;
  border-radius: 5px;
  background: transparent;
}
.option-values-list button:hover {
  background: hsl(var(--muted));
}
.option-values-list button.selected {
  background: hsl(var(--primary) / 0.08);
  color: hsl(var(--primary));
}
.option-values-list button:focus-visible {
  outline: 2px solid hsl(var(--primary));
  outline-offset: -2px;
}
.option-value-order {
  font-size: 11px;
  color: hsl(var(--muted-foreground));
}
.option-value-name {
  min-width: 0;
  overflow-wrap: anywhere;
}
.option-value-name small {
  display: block;
  font-size: 11px;
  color: hsl(var(--muted-foreground));
}
.option-value-details {
  padding: 14px 12px;
  min-width: 0;
}
.option-value-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
@media (max-width: 640px) {
  .option-values-layout {
    grid-template-columns: 1fr;
  }
  .option-values-list {
    max-height: 150px;
    border-right: 0;
    border-bottom: 1px solid hsl(var(--border));
  }
}
</style>

<style scoped>
.option-value-edit { color: hsl(var(--primary)); font-size: 12px; white-space: nowrap; margin-left: auto; }
</style>

<style scoped>
.option-values { container-type: inline-size; width: 100%; min-width: 0; }
@container (max-width: 420px) {
  .option-values-layout { grid-template-columns: minmax(0, 1fr); }
  .option-values-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(135px, 1fr)); gap: 4px; max-height: 160px; border-right: 0; border-bottom: 1px solid hsl(var(--border)); }
}
</style>

<script setup lang="ts">
import type {
  ContractTemplateField,
  ContractTemplateFieldType,
  ContractTemplateParty,
  ContractTemplateScope,
  ContractTemplateVariable,
} from "../electronic-contract-template-client";

import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";

import { Button, Empty, Spin } from "ant-design-vue";

import { GlobalWorkerOptions, getDocument } from "../../vendor/pdf.mjs";
import pdfWorker from "../../vendor/pdf.worker.mjs?url";

import { $t } from "#/locales";

GlobalWorkerOptions.workerSrc = pdfWorker;

const props = defineProps<{
  disabled: boolean;
  modelValue: ContractTemplateField[];
  pdf?: Blob;
  scopeCode: ContractTemplateScope;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: ContractTemplateField[]];
}>();

interface ToolboxItem {
  fieldTypeCode: ContractTemplateFieldType;
  label: string;
  partyCode: ContractTemplateParty;
  variableCode?: ContractTemplateVariable;
}

interface PageView {
  height: number;
  pageNo: number;
  width: number;
}

interface PointerInteraction {
  field: ContractTemplateField;
  index: number;
  mode: "move" | "resize";
  pageBounds: DOMRect;
  pointerX: number;
  pointerY: number;
}

const pages = ref<PageView[]>([]);
const rendering = ref(false);
const renderedFields = ref<ContractTemplateField[]>([]);
const pageElements = new Map<number, HTMLElement>();
const pageCanvases = new Map<number, HTMLCanvasElement>();
let interaction: PointerInteraction | undefined;
let documentTask: ReturnType<typeof getDocument> | undefined;
let renderGeneration = 0;
let latestModelValue: ContractTemplateField[] = [];
const pendingModelSnapshots: string[] = [];

const toolbox = computed<ToolboxItem[]>(() => {
  const variables: ContractTemplateVariable[] = [
    "contract_no",
    "customer_name",
    "merchant_name",
    "order_no",
    "service_name",
    "service_term",
    "amount",
    "currency",
    "signing_date",
  ];
  const items: ToolboxItem[] = variables.map((variableCode) => ({
    fieldTypeCode: "text_variable" as const,
    label: $t(
      `plugin.idc-compliance.contract.template.variables.${variableCode}`,
    ),
    partyCode: (variableCode === "customer_name"
      ? "customer"
      : "merchant") as ContractTemplateParty,
    variableCode,
  }));
  items.push(
    {
      fieldTypeCode: "signature",
      label: $t(
        "plugin.idc-compliance.contract.template.fields.customerSignature",
      ),
      partyCode: "customer",
    },
    {
      fieldTypeCode: "signature",
      label: $t(
        "plugin.idc-compliance.contract.template.fields.merchantSignature",
      ),
      partyCode: "merchant",
    },
    {
      fieldTypeCode: "enterprise_seal",
      label: $t("plugin.idc-compliance.contract.template.fields.merchantSeal"),
      partyCode: "merchant",
    },
    {
      fieldTypeCode: "signing_date",
      label: $t(
        "plugin.idc-compliance.contract.template.fields.customerSigningDate",
      ),
      partyCode: "customer",
    },
    {
      fieldTypeCode: "signing_date",
      label: $t(
        "plugin.idc-compliance.contract.template.fields.merchantSigningDate",
      ),
      partyCode: "merchant",
    },
  );
  if (props.scopeCode === "enterprise") {
    items.splice(11, 0, {
      fieldTypeCode: "enterprise_seal",
      label: $t("plugin.idc-compliance.contract.template.fields.customerSeal"),
      partyCode: "customer",
    });
  }
  return items;
});
const placementDisabled = computed(
  () => props.disabled || rendering.value || pages.value.length === 0,
);

function setPageElement(pageNo: number, value: Element | null) {
  if (value instanceof HTMLElement) pageElements.set(pageNo, value);
  else pageElements.delete(pageNo);
}

function setPageCanvas(pageNo: number, value: Element | null) {
  if (value instanceof HTMLCanvasElement) pageCanvases.set(pageNo, value);
  else pageCanvases.delete(pageNo);
}

function fieldLabel(field: ContractTemplateField) {
  if (field.fieldTypeCode === "text_variable" && field.variableCode) {
    return $t(
      `plugin.idc-compliance.contract.template.variables.${field.variableCode}`,
    );
  }
  const party = field.partyCode === "customer" ? "customer" : "merchant";
  const suffix =
    field.fieldTypeCode === "enterprise_seal"
      ? "Seal"
      : field.fieldTypeCode === "signature"
        ? "Signature"
        : "SigningDate";
  return $t(`plugin.idc-compliance.contract.template.fields.${party}${suffix}`);
}

function startToolDrag(event: DragEvent, item: ToolboxItem) {
  if (placementDisabled.value || !event.dataTransfer) return;
  event.dataTransfer.effectAllowed = "copy";
  event.dataTransfer.setData(
    "application/x-lina-contract-field",
    JSON.stringify(item),
  );
}

function placeTool(item: ToolboxItem) {
  if (placementDisabled.value) return;
  const pageNo = pages.value[0]?.pageNo;
  if (!pageNo) return;
  const size = defaultFieldSize(item.fieldTypeCode);
  const fieldCount = latestModelValue.filter(
    (field) => field.pageNo === pageNo,
  ).length;
  const column = fieldCount % 2;
  const row = Math.floor(fieldCount / 2) % 6;
  appendField(
    item,
    pageNo,
    90_000 + column * 410_000,
    90_000 + row * 125_000,
    size,
  );
}

function dropField(event: DragEvent, pageNo: number) {
  if (placementDisabled.value || !event.dataTransfer) return;
  const raw = event.dataTransfer.getData("application/x-lina-contract-field");
  if (!raw) return;
  let item: ToolboxItem;
  try {
    item = JSON.parse(raw) as ToolboxItem;
  } catch {
    return;
  }
  const bounds = pageElements.get(pageNo)?.getBoundingClientRect();
  if (!bounds || bounds.width <= 0 || bounds.height <= 0) return;
  const size = defaultFieldSize(item.fieldTypeCode);
  const x = clamp(
    Math.round(
      ((event.clientX - bounds.left) / bounds.width) * 1_000_000 -
        size.width / 2,
    ),
    0,
    1_000_000 - size.width,
  );
  const y = clamp(
    Math.round(
      ((event.clientY - bounds.top) / bounds.height) * 1_000_000 -
        size.height / 2,
    ),
    0,
    1_000_000 - size.height,
  );
  appendField(item, pageNo, x, y, size);
}

function appendField(
  item: ToolboxItem,
  pageNo: number,
  x: number,
  y: number,
  size = defaultFieldSize(item.fieldTypeCode),
) {
  updateFields([
    ...latestModelValue,
    {
      fieldTypeCode: item.fieldTypeCode,
      heightPPM: size.height,
      pageNo,
      partyCode: item.partyCode,
      positionXPPM: x,
      positionYPPM: y,
      variableCode: item.variableCode,
      widthPPM: size.width,
    },
  ]);
}

function updateFields(value: ContractTemplateField[]) {
  latestModelValue = value;
  renderedFields.value = value;
  pendingModelSnapshots.push(JSON.stringify(value));
  emit("update:modelValue", value);
}

function defaultFieldSize(type: ContractTemplateFieldType) {
  if (type === "enterprise_seal") return { height: 150_000, width: 150_000 };
  if (type === "signature") return { height: 90_000, width: 280_000 };
  if (type === "signing_date") return { height: 55_000, width: 190_000 };
  return { height: 52_000, width: 270_000 };
}

function startPointer(
  event: PointerEvent,
  field: ContractTemplateField,
  index: number,
  mode: PointerInteraction["mode"],
) {
  if (props.disabled) return;
  const pageBounds = pageElements.get(field.pageNo)?.getBoundingClientRect();
  if (!pageBounds) return;
  event.preventDefault();
  event.stopPropagation();
  interaction = {
    field: { ...field },
    index,
    mode,
    pageBounds,
    pointerX: event.clientX,
    pointerY: event.clientY,
  };
}

function pointerMove(event: PointerEvent) {
  if (!interaction) return;
  const currentInteraction = interaction;
  const deltaX = Math.round(
    ((event.clientX - currentInteraction.pointerX) /
      currentInteraction.pageBounds.width) *
      1_000_000,
  );
  const deltaY = Math.round(
    ((event.clientY - currentInteraction.pointerY) /
      currentInteraction.pageBounds.height) *
      1_000_000,
  );
  const next = latestModelValue.map((field, index) => {
    if (index !== currentInteraction.index) return field;
    const original = currentInteraction.field;
    if (currentInteraction.mode === "move") {
      return {
        ...field,
        positionXPPM: clamp(
          original.positionXPPM + deltaX,
          0,
          1_000_000 - original.widthPPM,
        ),
        positionYPPM: clamp(
          original.positionYPPM + deltaY,
          0,
          1_000_000 - original.heightPPM,
        ),
      };
    }
    return {
      ...field,
      heightPPM: clamp(
        original.heightPPM + deltaY,
        25_000,
        1_000_000 - original.positionYPPM,
      ),
      widthPPM: clamp(
        original.widthPPM + deltaX,
        50_000,
        1_000_000 - original.positionXPPM,
      ),
    };
  });
  updateFields(next);
}

function pointerUp() {
  interaction = undefined;
}

function removeField(index: number) {
  if (props.disabled) return;
  updateFields(latestModelValue.filter((_, current) => current !== index));
}

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(Math.max(value, minimum), maximum);
}

async function renderPDF(blob?: Blob) {
  const generation = ++renderGeneration;
  rendering.value = Boolean(blob);
  pages.value = [];
  pageElements.clear();
  pageCanvases.clear();
  await documentTask?.destroy();
  documentTask = undefined;
  if (!blob) {
    rendering.value = false;
    return;
  }
  try {
    const bytes = new Uint8Array(await blob.arrayBuffer());
    const currentTask = getDocument({ data: bytes });
    documentTask = currentTask;
    const loaded = await currentTask.promise;
    if (generation !== renderGeneration) {
      await currentTask.destroy();
      return;
    }
    const nextPages: PageView[] = [];
    for (let pageNo = 1; pageNo <= loaded.numPages; pageNo += 1) {
      const page = await loaded.getPage(pageNo);
      const viewport = page.getViewport({ scale: 1.25 });
      nextPages.push({
        height: viewport.height,
        pageNo,
        width: viewport.width,
      });
    }
    pages.value = nextPages;
    await nextTick();
    for (const pageView of nextPages) {
      if (generation !== renderGeneration) return;
      const page = await loaded.getPage(pageView.pageNo);
      const viewport = page.getViewport({ scale: 1.25 });
      const canvas = pageCanvases.get(pageView.pageNo);
      const context = canvas?.getContext("2d");
      if (!canvas || !context) continue;
      canvas.width = Math.ceil(viewport.width);
      canvas.height = Math.ceil(viewport.height);
      await page.render({ canvas, canvasContext: context, viewport }).promise;
    }
  } finally {
    if (generation === renderGeneration) rendering.value = false;
  }
}

watch(
  () => props.modelValue,
  (value) => {
    const snapshot = JSON.stringify(value);
    const pendingIndex = pendingModelSnapshots.indexOf(snapshot);
    if (pendingIndex >= 0) {
      pendingModelSnapshots.splice(0, pendingIndex + 1);
      if (pendingModelSnapshots.length > 0) return;
    } else {
      pendingModelSnapshots.length = 0;
    }
    latestModelValue = value;
    renderedFields.value = value;
  },
  { immediate: true },
);
watch(() => props.pdf, renderPDF, { immediate: true });
window.addEventListener("pointermove", pointerMove);
window.addEventListener("pointerup", pointerUp);

onBeforeUnmount(async () => {
  renderGeneration += 1;
  window.removeEventListener("pointermove", pointerMove);
  window.removeEventListener("pointerup", pointerUp);
  await documentTask?.destroy();
});
</script>

<template>
  <div class="placement-shell" data-testid="contract-field-editor">
    <aside v-if="!disabled" class="field-toolbox">
      <h3>
        {{ $t("plugin.idc-compliance.contract.template.fields.toolbox") }}
      </h3>
      <p>
        {{ $t("plugin.idc-compliance.contract.template.fields.toolboxHelp") }}
      </p>
      <Button
        v-for="item in toolbox"
        :key="`${item.fieldTypeCode}:${item.partyCode}:${item.variableCode ?? ''}`"
        class="toolbox-item"
        :data-testid="`contract-field-tool-${item.fieldTypeCode}-${item.partyCode}${item.variableCode ? `-${item.variableCode}` : ''}`"
        :disabled="placementDisabled"
        draggable="true"
        html-type="button"
        @click="placeTool(item)"
        @dragstart="startToolDrag($event, item)"
      >
        <span>{{ item.partyCode === "customer" ? "C" : "M" }}</span>
        {{ item.label }}
      </Button>
    </aside>

    <main class="pdf-stage">
      <Spin :spinning="rendering">
        <Empty
          v-if="!pdf && !rendering"
          :description="
            $t('plugin.idc-compliance.contract.template.fields.noPreview')
          "
        />
        <div
          v-for="page in pages"
          :key="page.pageNo"
          :ref="
            (value: unknown) =>
              setPageElement(page.pageNo, value as Element | null)
          "
          class="pdf-page"
          :style="{ height: `${page.height}px`, width: `${page.width}px` }"
          @dragover.prevent
          @drop.prevent="dropField($event, page.pageNo)"
        >
          <canvas
            :ref="
              (value: unknown) =>
                setPageCanvas(page.pageNo, value as Element | null)
            "
          />
          <div class="page-number">{{ page.pageNo }}</div>
          <div
            v-for="(field, index) in renderedFields"
            v-show="field.pageNo === page.pageNo"
            :key="field.fieldNo ?? `${field.pageNo}:${index}`"
            class="placed-field"
            :class="`placed-field--${field.partyCode}`"
            :style="{
              height: `${field.heightPPM / 10000}%`,
              left: `${field.positionXPPM / 10000}%`,
              top: `${field.positionYPPM / 10000}%`,
              width: `${field.widthPPM / 10000}%`,
            }"
            @pointerdown="startPointer($event, field, index, 'move')"
          >
            <span>{{ fieldLabel(field) }}</span>
            <Button
              v-if="!disabled"
              class="field-delete"
              danger
              shape="circle"
              size="small"
              @pointerdown.stop
              @click.stop="removeField(index)"
              >×</Button
            >
            <i
              v-if="!disabled"
              class="field-resize"
              @pointerdown="startPointer($event, field, index, 'resize')"
            />
          </div>
        </div>
      </Spin>
    </main>
  </div>
</template>

<style scoped>
.placement-shell {
  display: grid;
  gap: 16px;
  grid-template-columns: 220px minmax(0, 1fr);
}
.field-toolbox {
  align-self: start;
  background: hsl(var(--muted) / 52%);
  border-radius: 0;
  display: grid;
  gap: 8px;
  padding: 14px;
  position: sticky;
  top: 12px;
}
.field-toolbox h3 {
  font-size: 15px;
  margin: 0;
}
.field-toolbox p {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
  line-height: 1.6;
  margin: 0 0 4px;
}
.toolbox-item {
  align-items: center;
  background: hsl(var(--card));
  border: 1px solid rgb(148 163 184 / 28%);
  border-radius: 9px;
  color: hsl(var(--foreground));
  cursor: grab;
  display: flex;
  font-size: 12px;
  gap: 8px;
  min-height: 36px;
  padding: 7px 9px;
  text-align: left;
}
.toolbox-item span {
  align-items: center;
  background: rgb(14 165 233 / 12%);
  border-radius: 5px;
  color: rgb(2 132 199);
  display: inline-flex;
  font-size: 9px;
  font-weight: 800;
  height: 20px;
  justify-content: center;
  width: 20px;
}
.pdf-stage {
  background: #334155;
  border-radius: 0;
  min-height: 380px;
  overflow: auto;
  padding: 22px;
}
.pdf-page {
  background: white;
  box-shadow: 0 12px 34px rgb(0 0 0 / 28%);
  margin: 0 auto 22px;
  position: relative;
}
.pdf-page canvas {
  display: block;
  height: 100%;
  width: 100%;
}
.page-number {
  background: #0f172a;
  border-radius: 0 0 0 7px;
  color: white;
  font:
    10px/1 ui-monospace,
    monospace;
  padding: 5px 7px;
  position: absolute;
  right: 0;
  top: 0;
}
.placed-field {
  align-items: center;
  background: rgb(14 165 233 / 18%);
  border: 2px solid #0284c7;
  border-radius: 5px;
  color: #075985;
  cursor: move;
  display: flex;
  font-size: 10px;
  font-weight: 700;
  justify-content: center;
  min-height: 20px;
  position: absolute;
  text-align: center;
  touch-action: none;
  user-select: none;
}
.placed-field--merchant {
  background: rgb(245 158 11 / 19%);
  border-color: #d97706;
  color: #92400e;
}
.field-delete {
  position: absolute;
  right: -12px;
  top: -12px;
}
.field-resize {
  border-bottom: 3px solid currentColor;
  border-right: 3px solid currentColor;
  bottom: 2px;
  cursor: nwse-resize;
  height: 9px;
  position: absolute;
  right: 2px;
  width: 9px;
}
@media (max-width: 900px) {
  .placement-shell {
    grid-template-columns: 1fr;
  }
  .field-toolbox {
    position: static;
  }
}
</style>

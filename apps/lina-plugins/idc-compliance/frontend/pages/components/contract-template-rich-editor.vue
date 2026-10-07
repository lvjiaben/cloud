<script setup lang="ts">
import type { ContractTemplateVariable } from "../electronic-contract-template-client";

import { computed, onBeforeUnmount, ref, watch } from "vue";

import { Button, Input, Select, Space, Tooltip } from "ant-design-vue";

import {
  EditorContent,
  Link,
  mergeAttributes,
  Node,
  Placeholder,
  StarterKit,
  Table,
  TableCell,
  TableHeader,
  TableRow,
  TextAlign,
  Underline,
  useEditor,
} from "../../vendor/contract-editor.mjs";

import { $t } from "#/locales";
import VbenModalCompat from "./vben-modal-compat.vue";

const props = defineProps<{
  disabled: boolean;
  modelValue: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const PageBreak = Node.create({
  atom: true,
  group: "block",
  name: "pageBreak",
  parseHTML: () => [{ tag: "div.page-break" }],
  renderHTML: ({ HTMLAttributes }) => [
    "div",
    mergeAttributes(HTMLAttributes, { class: "page-break" }),
  ],
});

const editor = useEditor({
  content: props.modelValue,
  editable: !props.disabled,
  extensions: [
    StarterKit,
    Underline,
    Link.configure({ openOnClick: false }),
    Placeholder.configure({
      placeholder: $t(
        "plugin.idc-compliance.contract.template.editor.onlinePlaceholder",
      ),
    }),
    TextAlign.configure({ types: ["heading", "paragraph"] }),
    Table.configure({ resizable: false }),
    TableRow,
    TableHeader,
    TableCell,
    PageBreak,
  ],
  onUpdate: ({ editor: current }) =>
    emit("update:modelValue", current.getHTML()),
});

const linkOpen = ref(false);
const linkURL = ref("");
const selectedVariable = ref<ContractTemplateVariable>();

const variableOptions = computed(() =>
  [
    "contract_no",
    "customer_name",
    "merchant_name",
    "order_no",
    "service_name",
    "service_term",
    "amount",
    "currency",
    "signing_date",
  ].map((value) => ({
    label: $t(`plugin.idc-compliance.contract.template.variables.${value}`),
    value,
  })),
);

function chain() {
  return editor.value?.chain().focus();
}

function openLink() {
  linkURL.value = editor.value?.getAttributes("link").href ?? "";
  linkOpen.value = true;
}

function saveLink() {
  const value = linkURL.value.trim();
  if (value) {
    try {
      const parsed = new URL(value);
      if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return;
    } catch {
      return;
    }
    chain()?.extendMarkRange("link").setLink({ href: value }).run();
  } else {
    chain()?.extendMarkRange("link").unsetLink().run();
  }
  linkOpen.value = false;
}

function insertVariable(value: unknown) {
  if (
    typeof value !== "string" ||
    !variableOptions.value.some((item) => item.value === value)
  ) {
    return;
  }
  chain()?.insertContent(`{{${value}}}`).run();
  selectedVariable.value = undefined;
}

watch(
  () => props.modelValue,
  (value) => {
    if (editor.value && value !== editor.value.getHTML()) {
      editor.value.commands.setContent(value, false);
    }
  },
);

watch(
  () => props.disabled,
  (value) => editor.value?.setEditable(!value),
);

onBeforeUnmount(() => editor.value?.destroy());
</script>

<template>
  <div class="contract-rich-editor" data-testid="contract-online-editor">
    <div v-if="!disabled" class="contract-toolbar">
      <Space :size="4" wrap>
        <Tooltip
          :title="$t('plugin.idc-compliance.contract.template.editor.undo')"
        >
          <Button size="small" @click="chain()?.undo().run()">↶</Button>
        </Tooltip>
        <Tooltip
          :title="$t('plugin.idc-compliance.contract.template.editor.redo')"
        >
          <Button size="small" @click="chain()?.redo().run()">↷</Button>
        </Tooltip>
        <Button
          :type="editor?.isActive('bold') ? 'primary' : 'default'"
          size="small"
          @click="chain()?.toggleBold().run()"
          >B</Button
        >
        <Button
          :type="editor?.isActive('italic') ? 'primary' : 'default'"
          size="small"
          @click="chain()?.toggleItalic().run()"
          ><i>I</i></Button
        >
        <Button
          :type="editor?.isActive('underline') ? 'primary' : 'default'"
          size="small"
          @click="chain()?.toggleUnderline().run()"
          ><u>U</u></Button
        >
        <Button size="small" @click="chain()?.toggleHeading({ level: 1 }).run()"
          >H1</Button
        >
        <Button size="small" @click="chain()?.toggleHeading({ level: 2 }).run()"
          >H2</Button
        >
        <Button size="small" @click="chain()?.toggleHeading({ level: 3 }).run()"
          >H3</Button
        >
        <Button size="small" @click="chain()?.setTextAlign('left').run()"
          >⇤</Button
        >
        <Button size="small" @click="chain()?.setTextAlign('center').run()"
          >≡</Button
        >
        <Button size="small" @click="chain()?.setTextAlign('right').run()"
          >⇥</Button
        >
        <Tooltip
          :title="
            $t('plugin.idc-compliance.contract.template.editor.bulletList')
          "
        >
          <Button size="small" @click="chain()?.toggleBulletList().run()"
            >•</Button
          >
        </Tooltip>
        <Tooltip
          :title="
            $t('plugin.idc-compliance.contract.template.editor.orderedList')
          "
        >
          <Button size="small" @click="chain()?.toggleOrderedList().run()"
            >1.</Button
          >
        </Tooltip>
        <Button
          size="small"
          @click="
            chain()
              ?.insertTable({ cols: 3, rows: 3, withHeaderRow: true })
              .run()
          "
        >
          {{ $t("plugin.idc-compliance.contract.template.editor.insertTable") }}
        </Button>
        <Button size="small" @click="openLink">
          {{ $t("plugin.idc-compliance.contract.template.editor.insertLink") }}
        </Button>
        <Button
          size="small"
          @click="chain()?.insertContent({ type: 'pageBreak' }).run()"
        >
          {{ $t("plugin.idc-compliance.contract.template.editor.pageBreak") }}
        </Button>
        <Select
          v-model:value="selectedVariable"
          class="variable-select"
          :options="variableOptions"
          :placeholder="
            $t('plugin.idc-compliance.contract.template.editor.insertVariable')
          "
          @select="insertVariable"
        />
      </Space>
    </div>
    <EditorContent :editor="editor" class="contract-editor-content" />

    <VbenModalCompat
      v-model:open="linkOpen"
      :cancel-text="
        $t('plugin.idc-compliance.contract.template.actions.cancel')
      "
      :ok-text="$t('plugin.idc-compliance.contract.template.actions.confirm')"
      :title="$t('plugin.idc-compliance.contract.template.editor.linkTitle')"
      @ok="saveLink"
    >
      <Input
        v-model:value="linkURL"
        placeholder="https://"
        @press-enter="saveLink"
      />
    </VbenModalCompat>
  </div>
</template>

<style scoped>
.contract-rich-editor {
  border: 1px solid rgb(148 163 184 / 32%);
  border-radius: 0;
  overflow: hidden;
}
.contract-toolbar {
  background: hsl(var(--muted) / 58%);
  border-bottom: 1px solid rgb(148 163 184 / 26%);
  padding: 9px;
}
.variable-select {
  min-width: 190px;
}
.contract-editor-content {
  background: white;
  color: #172033;
  min-height: 430px;
  overflow-y: auto;
  padding: 28px 34px;
}
.contract-editor-content :deep(.tiptap) {
  min-height: 370px;
  outline: none;
}
.contract-editor-content :deep(.tiptap h1) {
  font-size: 2em;
  font-weight: 750;
}
.contract-editor-content :deep(.tiptap h2) {
  font-size: 1.5em;
  font-weight: 720;
}
.contract-editor-content :deep(.tiptap h3) {
  font-size: 1.2em;
  font-weight: 700;
}
.contract-editor-content :deep(.tiptap ul) {
  list-style: disc;
  padding-left: 24px;
}
.contract-editor-content :deep(.tiptap ol) {
  list-style: decimal;
  padding-left: 24px;
}
.contract-editor-content :deep(.tiptap table) {
  border-collapse: collapse;
  width: 100%;
}
.contract-editor-content :deep(.tiptap td),
.contract-editor-content :deep(.tiptap th) {
  border: 1px solid #64748b;
  padding: 7px;
}
.contract-editor-content :deep(.page-break) {
  border-top: 2px dashed #0ea5e9;
  margin: 28px 0;
  position: relative;
}
</style>

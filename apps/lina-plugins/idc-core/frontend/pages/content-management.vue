<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/content",
  title: "Content Management",
};
</script>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from "vue";
import type { VbenFormProps } from "@vben/common-ui";
import { Page, useVbenDrawer, useVbenModal } from "@vben/common-ui";
import { useAccessStore } from "@vben/stores";
import {
  Alert,
  Button,
  Input,
  Popconfirm,
  Progress,
  Space,
  Tag,
  Tree,
  Upload,
  message,
} from "ant-design-vue";
import { useVbenForm } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { TiptapEditor } from "#/components/tiptap";
import { useSourcePluginContext } from "./source-context";
import { createNumberedPage } from "./numbered-page";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import "../../../vben-adapter/frontend/admin.css";

import type {
  ContentCategory,
  ContentDetail,
  ContentSummary,
  ContentUploadPolicy,
  ContentWriteInput,
} from "../src/content/api";

import { createTranslator } from "../src/runtime/i18n";
import { createContentAPI } from "../src/content/api";
import { contentMessages } from "../src/content/messages";
import { registerContentEditorLocale } from "./content-editor-locales";

const context = useSourcePluginContext();
const accessStore = useAccessStore();
const currentContext = () => ({
  ...context.value,
  accessToken: accessStore.accessToken || "",
});
const api = createContentAPI(currentContext);
const tr = createTranslator(currentContext, contentMessages);
registerContentEditorLocale(context.value.locale);

const stableUploadFallback: ContentUploadPolicy = {
  inspectionRequiredAvailable: false,
  inspectionRequiredExtensions: [".zip", ".7z", ".exe", ".msi", ".iso"],
  inspectionRequiredMaxSizeBytes: 0,
  ordinaryExtensions: [
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".webp",
    ".bmp",
    ".pdf",
    ".doc",
    ".docx",
    ".xls",
    ".xlsx",
    ".ppt",
    ".pptx",
  ],
  ordinaryMaxSizeBytes: 5 * 1024 * 1024 * 1024,
};

const filters = reactive({
  categoryNo: "",
  type: "",
});
const editorVisible = ref(false);
const categoryVisible = ref(false);
const initializingEditor = ref(false);
const categorySearch = ref("");
const categoryReady = ref(false);
const categoryLoading = ref(true);
const categories = ref<ContentCategory[]>([]);
const selected = ref<ContentDetail | null>(null);
const detailLoading = ref(false);
const writeLoading = ref(false);
const error = ref("");
const categoryToDelete = ref<ContentCategory | null>(null);
const uploadFile = ref<File | null>(null);
const uploadLoading = ref(false);
const uploadProgress = ref(0);
const uploadPolicy = ref<ContentUploadPolicy>(stableUploadFallback);
const uploadPolicyState = ref<"fallback" | "loading" | "ready">("loading");

const categoryEditor = reactive({
  name: "",
  type: "news",
});

const editor = reactive({
  body: "",
  categoryNo: "",
  contentNo: "",
  summary: "",
  title: "",
  type: "news",
  version: 0,
  visibility: "public",
});

const activeUpload = computed(() => selected.value?.activeUpload);
const acceptedUploadExtensions = computed(() =>
  [
    ...uploadPolicy.value.ordinaryExtensions,
    ...uploadPolicy.value.inspectionRequiredExtensions,
  ].join(","),
);
const inspectionUploadsAvailable = computed(
  () =>
    uploadPolicyState.value === "ready" &&
    uploadPolicy.value.inspectionRequiredAvailable &&
    uploadPolicy.value.inspectionRequiredMaxSizeBytes > 0,
);
function uploadPolicyHelp() {
  const ordinaryLimit = formatBytes(uploadPolicy.value.ordinaryMaxSizeBytes);
  if (uploadPolicyState.value === "loading") {
    return tr("content.file.uploadPolicyLoading");
  }
  if (!inspectionUploadsAvailable.value) {
    return tr("content.file.uploadHelpUnavailable", { ordinaryLimit });
  }
  return tr("content.file.uploadHelp", {
    inspectionLimit: formatBytes(
      uploadPolicy.value.inspectionRequiredMaxSizeBytes,
    ),
    ordinaryLimit,
  });
}
const displayedUploadPercent = computed(() => {
  if (uploadLoading.value || uploadProgress.value > 0)
    return uploadProgress.value;
  const session = activeUpload.value;
  if (!session || session.sizeBytes <= 0) return 0;
  return Math.floor((session.confirmedBytes / session.sizeBytes) * 100);
});

function formatTime(value?: number) {
  if (!value) return "—";
  return new Intl.DateTimeFormat(context.value.locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatBytes(value?: number) {
  if (!value) return "—";
  const units = ["B", "KiB", "MiB", "GiB"];
  let amount = value;
  let unit = 0;
  while (amount >= 1024 && unit < units.length - 1) {
    amount /= 1024;
    unit += 1;
  }
  return `${amount.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

function normalizeUploadExtensions(extensions: string[]) {
  return [
    ...new Set(
      extensions
        .map((extension) => extension.trim().toLowerCase())
        .filter((extension) => /^\.[a-z0-9]+$/.test(extension)),
    ),
  ];
}

function normalizeUploadPolicy(policy: ContentUploadPolicy) {
  const ordinaryExtensions = normalizeUploadExtensions(
    policy.ordinaryExtensions ?? [],
  );
  const inspectionRequiredExtensions = normalizeUploadExtensions(
    policy.inspectionRequiredExtensions ?? [],
  );
  if (
    ordinaryExtensions.length === 0 ||
    inspectionRequiredExtensions.length === 0 ||
    !Number.isSafeInteger(policy.ordinaryMaxSizeBytes) ||
    policy.ordinaryMaxSizeBytes <= 0
  ) {
    throw new Error("invalid_upload_policy");
  }
  return {
    inspectionRequiredAvailable:
      policy.inspectionRequiredAvailable === true &&
      Number.isSafeInteger(policy.inspectionRequiredMaxSizeBytes) &&
      policy.inspectionRequiredMaxSizeBytes > 0,
    inspectionRequiredExtensions,
    inspectionRequiredMaxSizeBytes:
      Number.isSafeInteger(policy.inspectionRequiredMaxSizeBytes) &&
      policy.inspectionRequiredMaxSizeBytes > 0
        ? policy.inspectionRequiredMaxSizeBytes
        : 0,
    ordinaryExtensions,
    ordinaryMaxSizeBytes: policy.ordinaryMaxSizeBytes,
  } satisfies ContentUploadPolicy;
}

function typeLabel(type: string) {
  return tr(`content.type.${type}`);
}

function translatedCode(prefix: string, value?: string) {
  if (!value) return "—";
  const key = `${prefix}.${value}`;
  const label = tr(key);
  return label === key ? value : label;
}

function statusLabel(value?: string) {
  return translatedCode("content.status", value);
}

function scanStatusLabel(value?: string) {
  return translatedCode("content.scan", value);
}

function categoryLabel(categoryNo?: string) {
  if (!categoryNo) return tr("content.category.uncategorized");
  return (
    categories.value.find((item) => item.categoryNo === categoryNo)?.name ?? "—"
  );
}

function chooseDownloadFile(file: File) {
  if (!file) {
    uploadFile.value = null;
    return;
  }
  const extensionPart = file.name.split(".").pop()?.toLowerCase() ?? "";
  const extension = extensionPart ? `.${extensionPart}` : "";
  const ordinaryExtensions = new Set(uploadPolicy.value.ordinaryExtensions);
  const inspectionRequiredExtensions = new Set(
    uploadPolicy.value.inspectionRequiredExtensions,
  );
  const isOrdinary = ordinaryExtensions.has(extension);
  const requiresInspection = inspectionRequiredExtensions.has(extension);
  if (!isOrdinary && !requiresInspection) {
    error.value = tr("content.error.uploadType");
    uploadFile.value = null;
    return;
  }
  if (requiresInspection && !inspectionUploadsAvailable.value) {
    error.value = tr("content.error.uploadInspectionUnavailable");
    uploadFile.value = null;
    return;
  }
  if (
    requiresInspection &&
    file.size > uploadPolicy.value.inspectionRequiredMaxSizeBytes
  ) {
    error.value = tr("content.error.uploadInspectionSize", {
      limit: formatBytes(uploadPolicy.value.inspectionRequiredMaxSizeBytes),
    });
    uploadFile.value = null;
    return;
  }
  if (isOrdinary && file.size > uploadPolicy.value.ordinaryMaxSizeBytes) {
    error.value = tr("content.error.uploadOrdinarySize", {
      limit: formatBytes(uploadPolicy.value.ordinaryMaxSizeBytes),
    });
    uploadFile.value = null;
    return;
  }
  if (
    activeUpload.value &&
    (activeUpload.value.fileName !== file.name ||
      activeUpload.value.sizeBytes !== file.size)
  ) {
    error.value = tr("content.error.uploadMismatch");
    uploadFile.value = null;
    return;
  }
  error.value = "";
  uploadFile.value = file;
}

async function uploadSelectedFile() {
  const detail = selected.value;
  const file = uploadFile.value;
  if (!detail || !file || uploadLoading.value) return;
  uploadLoading.value = true;
  error.value = "";
  try {
    let session = detail.activeUpload
      ? await api.uploadSession(detail.activeUpload.uploadNo)
      : await api.createUploadSession(detail.contentNo, {
          contentVersion: detail.version,
          fileName: file.name,
          sizeBytes: file.size,
        });
    if (session.fileName !== file.name || session.sizeBytes !== file.size) {
      throw new Error("upload_file_mismatch");
    }
    uploadProgress.value = Math.floor(
      (session.confirmedBytes / file.size) * 100,
    );
    while (session.confirmedBytes < file.size) {
      const offset = session.confirmedBytes;
      const end = Math.min(offset + session.chunkSizeBytes, file.size);
      const chunk = await api.uploadChunk(
        session.uploadNo,
        offset,
        file.slice(offset, end),
      );
      session = {
        ...session,
        confirmedBytes: chunk.confirmedBytes,
        version: chunk.version,
      };
      uploadProgress.value = Math.floor(
        (session.confirmedBytes / file.size) * 100,
      );
    }
    await api.completeUpload(session.uploadNo, {
      expectedSizeBytes: file.size,
      version: session.version,
    });
    uploadFile.value = null;
    uploadProgress.value = 100;
    await selectItem(detail.contentNo);
  } catch (uploadError) {
    error.value =
      uploadError instanceof Error &&
      uploadError.message === "upload_file_mismatch"
        ? tr("content.error.uploadMismatch")
        : tr("content.error.write");
  } finally {
    uploadLoading.value = false;
  }
}

async function cancelUpload() {
  const detail = selected.value;
  const session = detail?.activeUpload;
  if (!detail || !session) return;
  uploadLoading.value = true;
  error.value = "";
  try {
    await api.cancelUpload(session.uploadNo, {
      confirmed: true,
      version: session.version,
    });
    uploadFile.value = null;
    uploadProgress.value = 0;
    await selectItem(detail.contentNo);
  } catch {
    error.value = tr("content.error.write");
  } finally {
    uploadLoading.value = false;
  }
}

async function loadCategories() {
  categoryReady.value = false;
  categoryLoading.value = true;
  try {
    const all: ContentCategory[] = [];
    let cursor = "";
    do {
      const result = await api.categories("", cursor);
      all.push(...result.items.filter((item) => !item.synthetic));
      cursor = result.nextCursor;
    } while (cursor);
    categories.value = all;
    categoryReady.value = true;
    formApi.setState({ schema: editorSchema() });
  } catch {
    error.value = tr("content.error.load");
  } finally {
    categoryLoading.value = false;
  }
}

async function loadUploadPolicy() {
  uploadPolicyState.value = "loading";
  try {
    uploadPolicy.value = normalizeUploadPolicy(await api.uploadPolicy());
    uploadPolicyState.value = "ready";
  } catch {
    uploadPolicy.value = stableUploadFallback;
    uploadPolicyState.value = "fallback";
  }
}

async function openCategories() {
  categoryEditor.name = "";
  categoryEditor.type = ["news", "knowledge", "download"].includes(filters.type)
    ? filters.type
    : "news";
  categoryFormApi.setState({ schema: categorySchema() });
  categoryModalApi.open();
  await nextTick();
  await categoryFormApi.resetForm();
  await categoryFormApi.setValues({ ...categoryEditor });
}

async function createCategory() {
  if (writeLoading.value || !(await categoryFormApi.validate()).valid) return;
  Object.assign(categoryEditor, await categoryFormApi.getValues());
  writeLoading.value = true;
  error.value = "";
  try {
    await api.createCategory({
      name: categoryEditor.name.trim(),
      sortOrder: 0,
      type: categoryEditor.type,
    });
    await loadCategories();
    writeLoading.value = false;
    categoryModalApi.close();
    message.success(tr("content.saved"));
  } catch {
    error.value = tr("content.error.write");
  } finally {
    writeLoading.value = false;
  }
}

async function deleteCategory() {
  const category = categoryToDelete.value;
  if (!category || writeLoading.value) return;
  writeLoading.value = true;
  error.value = "";
  try {
    await api.deleteCategory(category.categoryNo, category.version);
    categoryToDelete.value = null;
    filters.categoryNo = "";
    await loadCategories();
    await gridApi.reload();
  } catch {
    error.value = tr("content.error.write");
  } finally {
    writeLoading.value = false;
  }
}

async function loadItems() {
  await gridApi.query();
}

async function selectItem(contentNo: string) {
  detailLoading.value = true;
  error.value = "";
  try {
    selected.value = await api.detail(contentNo);
    if (editor.contentNo === contentNo) editor.version = selected.value.version;
    return selected.value;
  } catch {
    error.value = tr("content.error.load");
  } finally {
    detailLoading.value = false;
  }
}

async function openCreate() {
  if (detailLoading.value || writeLoading.value) return;
  selected.value = null;
  Object.assign(editor, {
    body: "",
    categoryNo: filters.categoryNo,
    contentNo: "",
    summary: "",
    title: "",
    type: filters.type || "news",
    version: 0,
    visibility: "public",
  });
  await showEditor();
}

async function openEdit(contentNo: string) {
  if (detailLoading.value || writeLoading.value) return;
  selected.value = null;
  const detail = await selectItem(contentNo);
  if (!detail) return;
  Object.assign(editor, {
    body: detail.body,
    categoryNo: detail.categoryNo ?? "",
    contentNo: detail.contentNo,
    summary: detail.summary ?? "",
    title: detail.title,
    type: detail.type,
    version: detail.version,
    visibility: detail.visibility,
  });
  await showEditor();
}

async function showEditor() {
  const values = { ...editor };
  initializingEditor.value = true;
  editorDrawerApi.open();
  await nextTick();
  try {
    formApi.setState({ schema: editorSchema() });
    await formApi.resetForm();
    await formApi.setValues(values);
    await nextTick();
  } finally {
    initializingEditor.value = false;
  }
}

function writeInput(saveMode: "draft" | "publish"): ContentWriteInput {
  return {
    body: editor.body,
    categoryNo: editor.categoryNo || undefined,
    status: saveMode === "publish" ? "published" : "draft",
    summary: editor.summary || undefined,
    seoTitle: "",
    seoDescription: "",
    targetCustomerGroupNos: selected.value?.targetCustomerGroupNos ?? [],
    title: editor.title,
    type: editor.type,
    version: editor.contentNo ? editor.version : undefined,
    visibility: editor.visibility,
  };
}

async function save(saveMode: "draft" | "publish") {
  if (writeLoading.value || !(await formApi.validate()).valid) return;
  Object.assign(editor, await formApi.getValues());
  if (
    !editor.body
      .replace(/<[^>]*>/g, "")
      .replace(/&nbsp;/g, " ")
      .trim() &&
    !/<img\b[^>]*\bsrc\s*=/i.test(editor.body)
  ) {
    error.value = tr("content.bodyRequired");
    return;
  }
  writeLoading.value = true;
  error.value = "";
  try {
    let detail = editor.contentNo
      ? await api.update(editor.contentNo, writeInput(saveMode))
      : await api.create(writeInput(saveMode));
    selected.value = detail;
    Object.assign(editor, {
      contentNo: detail.contentNo,
      version: detail.version,
    });
    formApi.setState({ schema: editorSchema() });
    if (saveMode === "publish" && detail.status !== "published") {
      detail = await api.lifecycle(
        detail.contentNo,
        detail.status === "offline" ? "republications" : "publications",
        { contentNo: detail.contentNo, version: detail.version },
      );
    }
    selected.value = detail;
    message.success(tr("content.saved"));
    if (detail.type === "download") {
      Object.assign(editor, {
        contentNo: detail.contentNo,
        version: detail.version,
      });
    } else {
      writeLoading.value = false;
      editorDrawerApi.close();
    }
    await loadItems();
  } catch {
    error.value = tr("content.error.write");
  } finally {
    writeLoading.value = false;
  }
}

async function lifecycle(action: string) {
  if (!selected.value || writeLoading.value) return;
  writeLoading.value = true;
  error.value = "";
  try {
    selected.value = await api.lifecycle(selected.value.contentNo, action, {
      confirmed: true,
      contentNo: selected.value.contentNo,
      version: selected.value.version,
    });
    await loadItems();
  } catch {
    error.value = tr("content.error.write");
  } finally {
    writeLoading.value = false;
  }
}

async function deleteSelected(row: ContentSummary) {
  if (writeLoading.value || detailLoading.value) return;
  const detail = await selectItem(row.contentNo);
  if (!detail || writeLoading.value) return;
  writeLoading.value = true;
  error.value = "";
  try {
    await api.delete(detail.contentNo, detail.version);
    selected.value = null;

    await loadItems();
  } catch {
    error.value = tr("content.error.write");
  } finally {
    writeLoading.value = false;
  }
}

async function candidateAction(action: "activate" | "cancel" | "scan") {
  const detail = selected.value;
  const candidate = detail?.candidateFile;
  const fileNo = candidate?.fileNo;
  if (!detail || !candidate || !fileNo || writeLoading.value) return;
  writeLoading.value = true;
  error.value = "";
  const body = {
    confirmed: action !== "scan",
    contentVersion: detail.version,
    fileVersion: candidate.version,
  };
  try {
    selected.value =
      action === "activate"
        ? await api.activateCandidate(detail.contentNo, fileNo, body)
        : action === "cancel"
          ? await api.cancelCandidate(detail.contentNo, fileNo, body)
          : await api.rescanCandidate(detail.contentNo, fileNo, body);
    if (selected.value) editor.version = selected.value.version;
    await loadItems();
  } catch {
    error.value = tr("content.error.write");
  } finally {
    writeLoading.value = false;
  }
}

const types = ["news", "knowledge", "download"];
const expandedCategoryKeys = ref(types.map((type) => `type:${type}`));
const options = (values: string[], prefix: string) =>
  values.map((value) => ({ value, label: tr(`${prefix}.${value}`) }));
const categoryTree = computed(() =>
  types.map((type) => ({
    key: `type:${type}`,
    title: typeLabel(type),
    children: categories.value
      .filter(
        (item) =>
          item.type === type &&
          item.name.toLowerCase().includes(categorySearch.value.toLowerCase()),
      )
      .map((item) => ({ key: item.categoryNo, title: item.name })),
  })),
);
const activeCategory = computed(() =>
  categories.value.find((item) => item.categoryNo === filters.categoryNo),
);
async function selectCategory(keys: (string | number)[]) {
  const key = String(keys[0] ?? "");
  const category = categories.value.find((item) => item.categoryNo === key);
  filters.categoryNo = category?.categoryNo ?? "";
  filters.type =
    category?.type ?? (key.startsWith("type:") ? key.slice(5) : "");
  await gridApi.formApi.setFieldValue("type", filters.type || undefined);
  const values = await gridApi.formApi.getValues();
  gridApi.formApi.setLatestSubmissionValues(values);
  await gridApi.reload(values);
}
function editorSchema(): VbenFormProps["schema"] {
  return [
    {
      fieldName: "type",
      label: tr("content.field.type"),
      component: "Select",
      rules: "required",
      componentProps: {
        options: options(types, "content.type"),
        disabled: !!editor.contentNo,
        "aria-label": tr("content.field.type"),
      },
    },
    {
      fieldName: "title",
      label: tr("content.field.title"),
      component: "Input",
      rules: "required",
      componentProps: {
        maxlength: 240,
        "aria-label": tr("content.field.title"),
      },
    },
    {
      fieldName: "summary",
      label: tr("content.field.summary"),
      component: "Textarea",
      componentProps: {
        rows: 3,
        maxlength: 2000,
        "aria-label": tr("content.field.summary"),
      },
    },
    {
      fieldName: "categoryNo",
      label: tr("content.field.category"),
      component: "Select",
      componentProps: {
        allowClear: true,
        disabled: !categoryReady.value,
        placeholder: tr("content.category.uncategorized"),
        options: categories.value
          .filter((item) => item.type === editor.type)
          .map((item) => ({ value: item.categoryNo, label: item.name })),
        "aria-label": tr("content.field.category"),
      },
    },
    {
      fieldName: "body",
      label: tr("content.field.body"),
      component: "Input",
      rules: "required",
    },
  ];
}
const [Form, formApi] = useVbenForm({
  schema: [],
  showDefaultActions: false,
  layout: "vertical",
  wrapperClass: "grid-cols-1",
  commonConfig: { componentProps: { class: "w-full" } },
  handleValuesChange(values) {
    if (initializingEditor.value) return;
    const changedType = values.type && values.type !== editor.type;
    Object.assign(editor, values);
    if (changedType) {
      editor.categoryNo = "";
      void formApi.setFieldValue("categoryNo", undefined);
      formApi.setState({ schema: editorSchema() });
    }
  },
});
const [EditorDrawer, editorDrawerApi] = useVbenDrawer({
  onOpenChange(open) {
    editorVisible.value = open;
  },
  onBeforeClose: () => !writeLoading.value && !uploadLoading.value,
  onClosed() {
    selected.value = null;
    error.value = "";
    uploadFile.value = null;
    uploadProgress.value = 0;
  },
});
function categorySchema(): VbenFormProps["schema"] {
  return [
    {
      fieldName: "type",
      label: tr("content.field.type"),
      component: "Select",
      rules: "required",
      componentProps: { options: options(types, "content.type") },
    },
    {
      fieldName: "name",
      label: tr("content.category.name"),
      component: "Input",
      rules: "required",
      componentProps: {
        maxlength: 80,
        "aria-label": tr("content.category.name"),
      },
    },
  ];
}
const [CategoryForm, categoryFormApi] = useVbenForm({
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 100 },
});
const [CategoryModal, categoryModalApi] = useVbenModal({
  onOpenChange(open) {
    categoryVisible.value = open;
  },
  onConfirm: createCategory,
  onBeforeClose: () => !writeLoading.value,
});
const pagination = createNumberedPage<ContentSummary>(
  (state) =>
    gridApi.setGridOptions({
      pagerConfig: {
        currentPage: state.currentPage,
        pageSize: state.pageSize,
        total: state.total,
      },
    }),
  () => editorDrawerApi.close(),
);
function searchSchema(): VbenFormProps["schema"] {
  return [
    {
      fieldName: "query",
      label: tr("content.field.keyword"),
      component: "Input",
      componentProps: {
        allowClear: true,
        "aria-label": tr("content.field.keyword"),
      },
    },
    {
      fieldName: "type",
      label: tr("content.field.type"),
      component: "Select",
      componentProps: {
        allowClear: true,
        options: options(types, "content.type"),
      },
    },
    {
      fieldName: "status",
      label: tr("content.field.status"),
      component: "Select",
      componentProps: {
        allowClear: true,
        options: options(["draft", "published", "offline"], "content.status"),
      },
    },
  ];
}
function columns(): import("#/adapter/vxe-table").VxeGridProps<ContentSummary>["columns"] {
  return [
    {
      field: "title",
      title: tr("content.field.title"),
      minWidth: 240,
      slots: { default: "title" },
    },
    {
      field: "type",
      title: tr("content.field.type"),
      width: 105,
      formatter: ({ cellValue }) => typeLabel(cellValue),
    },
    {
      field: "categoryNo",
      title: tr("content.field.category"),
      width: 130,
      formatter: ({ row }) => categoryLabel(row.categoryNo),
    },
    {
      field: "status",
      title: tr("content.field.status"),
      width: 100,
      slots: { default: "status" },
    },
    {
      field: "updatedAt",
      title: tr("content.updatedAt"),
      width: 180,
      formatter: ({ cellValue }) => formatTime(cellValue),
    },
    {
      field: "action",
      title: tr("content.actions"),
      width: 250,
      fixed: "right",
      slots: { default: "action" },
    },
  ];
}
const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    schema: searchSchema(),
    handleReset: async () => {
      filters.categoryNo = "";
      filters.type = "";
      await gridApi.formApi.resetForm();
      await gridApi.reload({});
    },
  },
  gridOptions: {
    height: "auto",
    minHeight: 300,
    border: false,
    stripe: false,
    keepSource: true,
    rowConfig: { keyField: "contentNo" },
    pagerConfig: { pageSize: 10, pageSizes: [10, 20, 50, 100] },
    toolbarConfig: { refresh: true, zoom: true, custom: true },
    columns: columns(),
    proxyConfig: {
      ajax: {
        query: async (
          { page }: { page: { currentPage: number; pageSize: number } },
          values: Record<string, string>,
        ) =>
          pagination.query(page, async (currentPage, pageSize) => {
            const type = values.type || "";
            if (filters.type !== type) {
              filters.categoryNo = "";
              filters.type = type;
            }
            try {
              const result = await api.list({
                ...values,
                type,
                categoryNo: filters.categoryNo,
                page: currentPage,
                limit: pageSize,
              });
              error.value = "";
              return { items: result.items, total: result.total };
            } catch (failure) {
              error.value = tr("content.error.load");
              const status = (failure as { status?: number }).status;
              throw Object.assign(failure as object, { response: { status } });
            }
          }),
      },
    },
  },
});
const layoutRoot = useAdminGridLayout(gridApi);
async function rowLifecycle(row: ContentSummary, action: string) {
  if (detailLoading.value || writeLoading.value) return;
  selected.value = null;
  await selectItem(row.contentNo);
  await lifecycle(action);
}
watch(
  () => context.value.locale,
  () => {
    registerContentEditorLocale(context.value.locale);
    // Rebuild labels after the host has switched locale.
    gridApi.setGridOptions({ columns: columns() });
    gridApi.formApi.setState({ schema: searchSchema() });
    formApi.setState({ schema: editorSchema() });
  },
);
onMounted(async () => {
  await Promise.all([loadCategories(), loadUploadPolicy()]);
  const contentNo = context.value.query?.content;
  if (contentNo) await openEdit(contentNo);
});
</script>

<template>
  <Page :ref="layoutRoot" class="plugin-admin-page" :auto-content-height="true">
    <div
      class="content-layout"
      data-testid="content-management-page"
      :data-locale="context.locale"
    >
      <aside class="content-categories bg-background rounded-lg border p-4">
        <div class="mb-4 flex items-center justify-between gap-2">
          <strong>{{ tr("content.category.title") }}</strong
          ><Button
            size="small"
            data-testid="content-category-create"
            :disabled="!categoryReady"
            @click="openCategories"
            >{{ tr("content.action.createCategory") }}</Button
          >
        </div>
        <Input
          v-model:value="categorySearch"
          allow-clear
          :placeholder="tr('content.category.name')"
          class="mb-3"
        />
        <Button
          block
          class="mb-2"
          :type="!filters.type && !filters.categoryNo ? 'primary' : 'default'"
          @click="selectCategory([])"
          >{{ tr("content.all") }}</Button
        >
        <Tree
          :tree-data="categoryTree"
          :selected-keys="
            filters.categoryNo
              ? [filters.categoryNo]
              : filters.type
                ? [`type:${filters.type}`]
                : []
          "
          v-model:expanded-keys="expandedCategoryKeys"
          block-node
          @select="selectCategory"
        />
        <div v-if="activeCategory" class="mt-4 border-t pt-3">
          <div class="mb-2 truncate">{{ activeCategory.name }}</div>
          <Popconfirm
            :title="tr('content.category.deleteTitle')"
            :description="tr('content.category.deleteBody')"
            @confirm="
              categoryToDelete = activeCategory!;
              deleteCategory();
            "
            ><Button size="small" danger :disabled="writeLoading">{{
              tr("content.action.delete")
            }}</Button></Popconfirm
          >
        </div>
        <Alert
          v-if="!categoryReady && !categoryLoading"
          type="warning"
          :message="tr('content.error.load')"
          class="mt-3"
          ><template #action
            ><Button size="small" @click="loadCategories">{{
              tr("content.action.refresh")
            }}</Button></template
          ></Alert
        >
      </aside>
      <section
        class="content-grid min-w-0 overflow-hidden rounded-lg border bg-background"
      >
        <Alert
          v-if="error && !editorVisible && !categoryVisible"
          type="error"
          :message="error"
          show-icon
          class="m-3"
          role="alert"
        />
        <Grid>
          <template #toolbar-tools
            ><Button
              type="primary"
              data-testid="content-create"
              :disabled="detailLoading"
              @click="openCreate"
              >{{ tr("content.action.create") }}</Button
            ></template
          >
          <template #title="{ row }"
            ><ghost-button
              :data-testid="`content-item-${row.contentNo}`"
              :disabled="detailLoading"
              @click="openEdit(row.contentNo)"
              >{{ row.title }}</ghost-button
            ></template
          >
          <template #status="{ row }"
            ><Tag
              :color="
                row.status === 'published'
                  ? 'success'
                  : row.status === 'offline'
                    ? 'default'
                    : 'warning'
              "
              >{{ statusLabel(row.status) }}</Tag
            ></template
          >
          <template #action="{ row }">
            <Space :size="8" class="content-row-actions">
              <ghost-button
                :disabled="writeLoading || detailLoading"
                @click="openEdit(row.contentNo)"
                >{{ tr("content.action.edit") }}</ghost-button
              >
              <ghost-button
                v-if="row.status !== 'published'"
                :disabled="writeLoading || detailLoading"
                @click="
                  rowLifecycle(
                    row,
                    row.status === 'offline'
                      ? 'republications'
                      : 'publications',
                  )
                "
                >{{
                  tr(
                    row.status === "offline"
                      ? "content.action.republish"
                      : "content.action.publish",
                  )
                }}</ghost-button
              >
              <ghost-button
                v-else
                :disabled="writeLoading || detailLoading"
                @click="rowLifecycle(row, 'unpublications')"
                >{{ tr("content.action.unpublish") }}</ghost-button
              >
              <Popconfirm
                :title="tr('content.delete.title')"
                :description="tr('content.delete.body')"
                @confirm="deleteSelected(row)"
                ><ghost-button
                  danger
                  :disabled="writeLoading || detailLoading"
                  >{{ tr("content.action.delete") }}</ghost-button
                ></Popconfirm
              >
            </Space>
          </template>
        </Grid>
      </section>
    </div>
    <CategoryModal
      :title="tr('content.action.createCategory')"
      class="content-category-modal"
      :close-on-click-modal="false"
      :confirm-loading="writeLoading"
      ><Alert
        v-if="error"
        type="error"
        :message="error"
        class="mb-4" /><CategoryForm
    /></CategoryModal>
    <EditorDrawer
      :title="
        editor.contentNo
          ? tr('content.action.edit')
          : tr('content.action.create')
      "
      class="content-editor-drawer w-[920px] max-w-full"
      :modal="true"
      :close-on-click-modal="false"
    >
      <div data-testid="content-editor" :data-locale="context.locale">
        <Alert
          v-if="error"
          type="error"
          :message="error"
          show-icon
          class="mb-4"
          role="alert"
        />
        <Form
          ><template #body="{ componentField }"
            ><TiptapEditor
              v-bind="componentField"
              :height="320"
              :placeholder="tr('content.editor.placeholder')"
              :disabled="writeLoading"
              class="w-full"
              scene="other" /></template
        ></Form>
        <section
          v-if="editor.type === 'download'"
          class="space-y-4 rounded-lg border p-4"
          data-testid="content-download-files"
        >
          <strong>{{ tr("content.file.upload") }}</strong>
          <Alert
            v-if="!editor.contentNo"
            type="info"
            :message="tr('content.saveBeforeUpload')"
          />
          <template v-else>
            <p data-testid="content-upload-policy">{{ uploadPolicyHelp() }}</p>
            <Alert
              v-if="
                uploadPolicyState !== 'loading' && !inspectionUploadsAvailable
              "
              data-testid="content-upload-inspection-unavailable"
              type="info"
              :message="tr('content.file.inspectionUnavailable')"
            />
            <template v-if="!selected?.candidateFile">
              <Upload.Dragger
                v-if="!activeUpload || activeUpload.canResume"
                :accept="acceptedUploadExtensions"
                :show-upload-list="false"
                :disabled="uploadLoading || writeLoading"
                :before-upload="
                  (file) => {
                    chooseDownloadFile(file);
                    return false;
                  }
                "
                data-testid="content-upload-file"
                ><p class="p-4">
                  {{
                    uploadFile?.name ||
                    activeUpload?.fileName ||
                    tr("content.action.upload")
                  }}
                </p></Upload.Dragger
              >
              <Progress
                v-if="activeUpload || uploadLoading || uploadProgress"
                :percent="displayedUploadPercent"
              />
              <p v-if="activeUpload" class="text-muted-foreground">
                {{
                  tr("content.upload.expires", {
                    time: formatTime(activeUpload.expiresAt),
                  })
                }}
              </p>
              <Space>
                <Button
                  v-if="uploadFile"
                  type="primary"
                  data-testid="content-upload-start"
                  :loading="uploadLoading"
                  @click="uploadSelectedFile"
                  >{{
                    tr(
                      activeUpload
                        ? "content.action.resumeUpload"
                        : "content.action.startUpload",
                    )
                  }}</Button
                >
                <Popconfirm
                  v-if="activeUpload?.canCancel"
                  :title="tr('content.upload.cancelTitle')"
                  :description="tr('content.upload.cancelBody')"
                  @confirm="cancelUpload"
                  ><Button danger :disabled="uploadLoading">{{
                    tr("content.action.cancelCandidate")
                  }}</Button></Popconfirm
                >
              </Space>
            </template>
            <div class="rounded-lg bg-muted/30 p-3">
              <strong>{{ tr("content.file.current") }}</strong>
              <p>
                {{ selected?.currentFile?.fileName || tr("content.file.none") }}
              </p>
              <p v-if="selected?.currentFile">
                {{ formatBytes(selected.currentFile.sizeBytes) }}
              </p>
            </div>
            <div
              v-if="selected?.candidateFile"
              class="space-y-3 rounded-lg border p-3"
            >
              <strong>{{ tr("content.file.pending") }}</strong>
              <p>
                {{ selected.candidateFile.fileName }} ·
                {{ scanStatusLabel(selected.candidateFile.scanStatus) }}
              </p>
              <p>{{ formatTime(selected.candidateFile.gateExpiresAt) }}</p>
              <Space wrap>
                <Button
                  v-if="
                    selected.candidateFile.canRescan ||
                    selected.availableActions.includes('rescan_candidate')
                  "
                  :loading="writeLoading"
                  @click="candidateAction('scan')"
                  >{{ tr("content.action.rescan") }}</Button
                >
                <Popconfirm
                  v-if="
                    selected.candidateFile.canEnable ||
                    selected.availableActions.includes('activate_candidate')
                  "
                  :title="tr('content.candidate.activateTitle')"
                  :description="tr('content.candidate.activateBody')"
                  @confirm="candidateAction('activate')"
                  ><Button type="primary" :disabled="writeLoading">{{
                    tr("content.action.activate")
                  }}</Button></Popconfirm
                >
                <Popconfirm
                  v-if="selected.candidateFile.canCancel !== false"
                  :title="tr('content.candidate.cancelTitle')"
                  :description="tr('content.candidate.cancelBody')"
                  @confirm="candidateAction('cancel')"
                  ><Button danger :disabled="writeLoading">{{
                    tr("content.action.cancelCandidate")
                  }}</Button></Popconfirm
                >
              </Space>
            </div>
          </template>
        </section>
      </div>
      <template #footer>
        <Button
          :disabled="writeLoading || uploadLoading"
          @click="editorDrawerApi.close()"
          >{{ tr("content.action.cancel") }}</Button
        >
        <Button
          :loading="writeLoading"
          :disabled="uploadLoading"
          @click="save('draft')"
          >{{
            tr(
              editor.contentNo
                ? "content.action.save"
                : "content.action.saveDraft",
            )
          }}</Button
        >
        <Button
          v-if="!selected || selected.status !== 'published'"
          type="primary"
          :loading="writeLoading"
          :disabled="uploadLoading"
          @click="save('publish')"
          >{{ tr("content.action.publish") }}</Button
        >
      </template>
    </EditorDrawer>
  </Page>
</template>

<style scoped>
.content-layout {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 16px;
  height: 100%;
  min-height: 0;
}
.content-categories {
  overflow: auto;
  border-radius: 10px !important;
  background: hsl(var(--card));
}
.content-grid {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-radius: 10px !important;
  background: hsl(var(--card));
}
.content-grid :deep(.vben-vxe-grid) {
  flex: 1;
  min-height: 0;
}
@media (max-width: 900px) {
  .content-layout {
    grid-template-columns: 190px minmax(0, 1fr);
  }
}
</style>

<style>
/* Ant's nested link dialog uses a viewport wrapper; it is not the dialog panel. */
body:has(.content-editor-drawer[data-state="open"])
  .ant-modal-wrap[role="dialog"] {
  background: transparent !important;
  box-shadow: none !important;
  border-radius: 0 !important;
}
.content-editor-drawer {
  box-shadow: -12px 0 40px rgb(0 0 0 / 18%) !important;
}
.content-category-modal {
  width: min(480px, calc(100vw - 32px)) !important;
  box-shadow: 0 16px 48px rgb(0 0 0 / 20%) !important;
  border-radius: 10px !important;
}
/* The installed host skin resets generic surfaces with !important; retain the standard editor shape locally. */
[data-testid="content-editor"] .tiptap-editor {
  width: 100%;
  border-radius: 6px !important;
}
</style>

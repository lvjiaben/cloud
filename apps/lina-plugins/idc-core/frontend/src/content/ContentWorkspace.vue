<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';

import type { PluginHostMountContext } from '../host-context';
import type {
  ContentAudienceCandidate,
  ContentAudienceTarget,
  ContentCategory,
  ContentDetail,
  ContentSummary,
  ContentUploadPolicy,
  ContentWriteInput,
} from './api';

import { createTranslator } from '../runtime/i18n';
import { createContentAPI } from './api';
import { contentMessages } from './messages';

const props = defineProps<{ context: PluginHostMountContext }>();
const currentContext = () => props.context;
const api = createContentAPI(currentContext);
const tr = createTranslator(currentContext, contentMessages);

const stableUploadFallback: ContentUploadPolicy = {
  inspectionRequiredAvailable: false,
  inspectionRequiredExtensions: ['.zip', '.7z', '.exe', '.msi', '.iso'],
  inspectionRequiredMaxSizeBytes: 0,
  ordinaryExtensions: [
    '.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.pdf',
    '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx',
  ],
  ordinaryMaxSizeBytes: 5 * 1024 * 1024 * 1024,
};

const filters = reactive({
  categoryNo: '',
  cursor: '',
  keyword: '',
  limit: 30,
  status: '',
  type: '',
  visibility: '',
});
const items = ref<ContentSummary[]>([]);
const categories = ref<ContentCategory[]>([]);
const selected = ref<ContentDetail | null>(null);
const nextCursor = ref('');
const cursorHistory = ref<string[]>([]);
const loading = ref(false);
const detailLoading = ref(false);
const writeLoading = ref(false);
const error = ref('');
const editorOpen = ref(false);
const deleteOpen = ref(false);
const uploadCancelOpen = ref(false);
const candidateConfirmAction = ref<'' | 'activate' | 'cancel'>('');
const categoryOpen = ref(false);
const categoryToDelete = ref<ContentCategory | null>(null);
const uploadFile = ref<File | null>(null);
const uploadLoading = ref(false);
const uploadProgress = ref(0);
const uploadPolicy = ref<ContentUploadPolicy>(stableUploadFallback);
const uploadPolicyState = ref<'fallback' | 'loading' | 'ready'>('loading');
const audienceQuery = ref('');
const audienceLoading = ref(false);
const audienceCandidates = ref<ContentAudienceCandidate[]>([]);
const selectedAudienceTargets = ref<ContentAudienceTarget[]>([]);

const categoryEditor = reactive({
  name: '',
  type: 'news',
});

const editor = reactive({
  body: '',
  categoryNo: '',
  contentNo: '',
  seoDescription: '',
  seoTitle: '',
  summary: '',
  title: '',
  type: 'news',
  version: 0,
  visibility: 'public',
});

const metrics = computed(() => ({
  downloads: items.value.filter((item) => item.type === 'download').length,
  drafts: items.value.filter((item) => item.status === 'draft').length,
  offline: items.value.filter((item) => item.status === 'offline').length,
  published: items.value.filter((item) => item.status === 'published').length,
}));

const activeUpload = computed(() => selected.value?.activeUpload);
const acceptedUploadExtensions = computed(() =>
  [
    ...uploadPolicy.value.ordinaryExtensions,
    ...uploadPolicy.value.inspectionRequiredExtensions,
  ].join(','),
);
const inspectionUploadsAvailable = computed(
  () =>
    uploadPolicyState.value === 'ready' &&
    uploadPolicy.value.inspectionRequiredAvailable &&
    uploadPolicy.value.inspectionRequiredMaxSizeBytes > 0,
);
function uploadPolicyHelp() {
  const ordinaryLimit = formatBytes(uploadPolicy.value.ordinaryMaxSizeBytes);
  if (uploadPolicyState.value === 'loading') {
    return tr('content.file.uploadPolicyLoading');
  }
  if (!inspectionUploadsAvailable.value) {
    return tr('content.file.uploadHelpUnavailable', { ordinaryLimit });
  }
  return tr('content.file.uploadHelp', {
    inspectionLimit: formatBytes(
      uploadPolicy.value.inspectionRequiredMaxSizeBytes,
    ),
    ordinaryLimit,
  });
}
const displayedUploadPercent = computed(() => {
  if (uploadLoading.value || uploadProgress.value > 0) return uploadProgress.value;
  const session = activeUpload.value;
  if (!session || session.sizeBytes <= 0) return 0;
  return Math.floor((session.confirmedBytes / session.sizeBytes) * 100);
});

const audienceType = computed<ContentAudienceTarget['type'] | ''>(() => {
  if (editor.visibility === 'customer_group') return 'customer_group';
  if (editor.visibility === 'specific_customers') return 'customer';
  if (editor.visibility === 'specific_services') return 'service';
  return '';
});

function formatTime(value?: number) {
  if (!value) return '—';
  return new Intl.DateTimeFormat(props.context.locale, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value));
}

function formatBytes(value?: number) {
  if (!value) return '—';
  const units = ['B', 'KiB', 'MiB', 'GiB'];
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
    throw new Error('invalid_upload_policy');
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

function candidateNumber(candidate: ContentAudienceCandidate) {
  if (audienceType.value === 'customer') return candidate.customerNo ?? '';
  if (audienceType.value === 'service') return candidate.serviceNo ?? '';
  if (audienceType.value === 'customer_group') return candidate.groupNo ?? '';
  return '';
}

function resetAudienceSelection() {
  audienceQuery.value = '';
  audienceCandidates.value = [];
  selectedAudienceTargets.value = [];
}

function handleEditorTypeChange() {
  if (
    editor.type !== 'announcement' &&
    ['specific_customers', 'specific_services'].includes(editor.visibility)
  ) {
    editor.visibility = 'public';
  }
  resetAudienceSelection();
}

async function searchAudience() {
  const kind = audienceType.value;
  if (!kind) return;
  audienceLoading.value = true;
  error.value = '';
  try {
    const keyword = audienceQuery.value.trim();
    const result =
      kind === 'customer'
        ? await api.audienceCustomers(keyword)
        : kind === 'service'
          ? await api.audienceServices(keyword)
          : await api.audienceCustomerGroups(keyword);
    audienceCandidates.value = result.items ?? [];
  } catch {
    error.value = tr('content.error.load');
  } finally {
    audienceLoading.value = false;
  }
}

function addAudience(candidate: ContentAudienceCandidate) {
  const type = audienceType.value;
  const number = candidateNumber(candidate);
  if (!type || !number) return;
  if (
    selectedAudienceTargets.value.some(
      (item) => item.type === type && item.number === number,
    )
  ) {
    return;
  }
  selectedAudienceTargets.value.push({
    label: candidate.label,
    number,
    type,
  });
}

function removeAudience(target: ContentAudienceTarget) {
  selectedAudienceTargets.value = selectedAudienceTargets.value.filter(
    (item) => item.type !== target.type || item.number !== target.number,
  );
}

function typeLabel(type: string) {
  return tr(`content.type.${type}`);
}

function translatedCode(prefix: string, value?: string) {
  if (!value) return '—';
  const key = `${prefix}.${value}`;
  const label = tr(key);
  return label === key ? value : label;
}

function statusLabel(value?: string) {
  return translatedCode('content.status', value);
}

function visibilityLabel(value?: string) {
  return translatedCode('content.visibility', value);
}

function scanStatusLabel(value?: string) {
  return translatedCode('content.scan', value);
}

function categoryLabel(categoryNo?: string) {
  if (!categoryNo) return tr('content.category.uncategorized');
  return categories.value.find((item) => item.categoryNo === categoryNo)?.name ?? '—';
}

function chooseDownloadFile(event: Event) {
  const input = event.currentTarget as HTMLInputElement;
  const file = input.files?.[0] ?? null;
  if (!file) {
    uploadFile.value = null;
    return;
  }
  const extensionPart = file.name.split('.').pop()?.toLowerCase() ?? '';
  const extension = extensionPart ? `.${extensionPart}` : '';
  const ordinaryExtensions = new Set(uploadPolicy.value.ordinaryExtensions);
  const inspectionRequiredExtensions = new Set(
    uploadPolicy.value.inspectionRequiredExtensions,
  );
  const isOrdinary = ordinaryExtensions.has(extension);
  const requiresInspection = inspectionRequiredExtensions.has(extension);
  if (!isOrdinary && !requiresInspection) {
    error.value = tr('content.error.uploadType');
    input.value = '';
    uploadFile.value = null;
    return;
  }
  if (requiresInspection && !inspectionUploadsAvailable.value) {
    error.value = tr('content.error.uploadInspectionUnavailable');
    input.value = '';
    uploadFile.value = null;
    return;
  }
  if (
    requiresInspection &&
    file.size > uploadPolicy.value.inspectionRequiredMaxSizeBytes
  ) {
    error.value = tr('content.error.uploadInspectionSize', {
      limit: formatBytes(uploadPolicy.value.inspectionRequiredMaxSizeBytes),
    });
    input.value = '';
    uploadFile.value = null;
    return;
  }
  if (isOrdinary && file.size > uploadPolicy.value.ordinaryMaxSizeBytes) {
    error.value = tr('content.error.uploadOrdinarySize', {
      limit: formatBytes(uploadPolicy.value.ordinaryMaxSizeBytes),
    });
    input.value = '';
    uploadFile.value = null;
    return;
  }
  if (
    activeUpload.value &&
    (activeUpload.value.fileName !== file.name || activeUpload.value.sizeBytes !== file.size)
  ) {
    error.value = tr('content.error.uploadMismatch');
    input.value = '';
    uploadFile.value = null;
    return;
  }
  error.value = '';
  uploadFile.value = file;
}

async function uploadSelectedFile() {
  const detail = selected.value;
  const file = uploadFile.value;
  if (!detail || !file) return;
  uploadLoading.value = true;
  error.value = '';
  try {
    let session = detail.activeUpload
      ? await api.uploadSession(detail.activeUpload.uploadNo)
      : await api.createUploadSession(detail.contentNo, {
          contentVersion: detail.version,
          fileName: file.name,
          sizeBytes: file.size,
        });
    if (session.fileName !== file.name || session.sizeBytes !== file.size) {
      throw new Error('upload_file_mismatch');
    }
    uploadProgress.value = Math.floor((session.confirmedBytes / file.size) * 100);
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
      uploadProgress.value = Math.floor((session.confirmedBytes / file.size) * 100);
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
      uploadError instanceof Error && uploadError.message === 'upload_file_mismatch'
        ? tr('content.error.uploadMismatch')
        : tr('content.error.write');
  } finally {
    uploadLoading.value = false;
  }
}

async function cancelUpload() {
  const detail = selected.value;
  const session = detail?.activeUpload;
  if (!detail || !session) return;
  uploadLoading.value = true;
  error.value = '';
  try {
    await api.cancelUpload(session.uploadNo, {
      confirmed: true,
      version: session.version,
    });
    uploadCancelOpen.value = false;
    uploadFile.value = null;
    uploadProgress.value = 0;
    await selectItem(detail.contentNo);
  } catch {
    error.value = tr('content.error.write');
  } finally {
    uploadLoading.value = false;
  }
}

async function loadCategories() {
  const result = await api.categories(filters.type);
  categories.value = result.items ?? [];
}

async function loadUploadPolicy() {
  uploadPolicyState.value = 'loading';
  try {
    uploadPolicy.value = normalizeUploadPolicy(await api.uploadPolicy());
    uploadPolicyState.value = 'ready';
  } catch {
    uploadPolicy.value = stableUploadFallback;
    uploadPolicyState.value = 'fallback';
  }
}

function openCategories() {
  categoryEditor.name = '';
  categoryEditor.type = ['news', 'knowledge', 'download'].includes(filters.type)
    ? filters.type
    : 'news';
  categoryOpen.value = true;
}

async function createCategory() {
  if (!categoryEditor.name.trim()) return;
  writeLoading.value = true;
  error.value = '';
  try {
    await api.createCategory({
      name: categoryEditor.name.trim(),
      sortOrder: 0,
      type: categoryEditor.type,
    });
    categoryEditor.name = '';
    await loadCategories();
  } catch {
    error.value = tr('content.error.write');
  } finally {
    writeLoading.value = false;
  }
}

async function deleteCategory() {
  const category = categoryToDelete.value;
  if (!category) return;
  writeLoading.value = true;
  error.value = '';
  try {
    await api.deleteCategory(category.categoryNo, category.version);
    categoryToDelete.value = null;
    await Promise.all([loadCategories(), loadItems(filters.cursor)]);
    if (selected.value) await selectItem(selected.value.contentNo);
  } catch {
    error.value = tr('content.error.write');
  } finally {
    writeLoading.value = false;
  }
}

async function loadItems(cursor = '') {
  loading.value = true;
  error.value = '';
  try {
    const result = await api.list({
      categoryNo: filters.categoryNo,
      cursor,
      limit: filters.limit,
      query: filters.keyword,
      status: filters.status,
      type: filters.type,
      visibility: filters.visibility,
    });
    items.value = result.items ?? [];
    nextCursor.value = result.nextCursor ?? '';
  } catch {
    error.value = tr('content.error.load');
  } finally {
    loading.value = false;
  }
}

async function selectItem(contentNo: string) {
  detailLoading.value = true;
  error.value = '';
  try {
    selected.value = await api.detail(contentNo);
    const url = new URL(globalThis.location.href);
    url.searchParams.set('content', contentNo);
    globalThis.history.replaceState(globalThis.history.state, '', url);
  } catch {
    error.value = tr('content.error.load');
  } finally {
    detailLoading.value = false;
  }
}

function applyFilters() {
  cursorHistory.value = [];
  filters.cursor = '';
  void Promise.all([loadCategories(), loadItems()]);
}

function nextPage() {
  if (!nextCursor.value) return;
  cursorHistory.value.push(filters.cursor);
  filters.cursor = nextCursor.value;
  void loadItems(filters.cursor);
}

function previousPage() {
  filters.cursor = cursorHistory.value.pop() ?? '';
  void loadItems(filters.cursor);
}

function openCreate() {
  Object.assign(editor, {
    body: '',
    categoryNo: '',
    contentNo: '',
    seoDescription: '',
    seoTitle: '',
    summary: '',
    title: '',
    type: filters.type || 'news',
    version: 0,
    visibility: 'public',
  });
  resetAudienceSelection();
  editorOpen.value = true;
}

function openEdit() {
  if (!selected.value) return;
  Object.assign(editor, {
    body: selected.value.body,
    categoryNo: selected.value.categoryNo ?? '',
    contentNo: selected.value.contentNo,
    seoDescription: selected.value.seoDescription ?? '',
    seoTitle: selected.value.seoTitle ?? '',
    summary: selected.value.summary ?? '',
    title: selected.value.title,
    type: selected.value.type,
    version: selected.value.version,
    visibility: selected.value.visibility,
  });
  if (selected.value.targetAudiences?.length) {
    selectedAudienceTargets.value = [...selected.value.targetAudiences];
  } else {
    const targets: ContentAudienceTarget[] = [];
    for (const number of selected.value.targetCustomerNos ?? []) {
      targets.push({ label: tr('content.audience.existing'), number, type: 'customer' });
    }
    for (const number of selected.value.targetServiceNos ?? []) {
      targets.push({ label: tr('content.audience.existing'), number, type: 'service' });
    }
    for (const number of selected.value.targetCustomerGroupNos ?? []) {
      targets.push({ label: tr('content.audience.existing'), number, type: 'customer_group' });
    }
    selectedAudienceTargets.value = targets;
  }
  audienceQuery.value = '';
  audienceCandidates.value = [];
  editorOpen.value = true;
}

function writeInput(saveMode: 'draft' | 'publish'): ContentWriteInput {
  return {
    body: editor.body,
    categoryNo:
      editor.type === 'announcement' ? undefined : editor.categoryNo || undefined,
    seoDescription: editor.seoDescription || undefined,
    seoTitle: editor.seoTitle || undefined,
    status: saveMode === 'publish' ? 'published' : 'draft',
    summary: editor.summary || undefined,
    targetCustomerGroupNos: selectedAudienceTargets.value
      .filter((item) => item.type === 'customer_group')
      .map((item) => item.number),
    targetCustomerNos:
      selectedAudienceTargets.value
        .filter((item) => item.type === 'customer')
        .map((item) => item.number),
    targetServiceNos:
      selectedAudienceTargets.value
        .filter((item) => item.type === 'service')
        .map((item) => item.number),
    title: editor.title,
    type: editor.type,
    version: editor.contentNo ? editor.version : undefined,
    visibility: editor.visibility,
  };
}

async function save(saveMode: 'draft' | 'publish') {
  if (audienceType.value && selectedAudienceTargets.value.length === 0) {
    error.value = tr('content.error.audienceRequired');
    return;
  }
  writeLoading.value = true;
  error.value = '';
  try {
    const detail = editor.contentNo
      ? await api.update(editor.contentNo, writeInput(saveMode))
      : await api.create(writeInput(saveMode));
    editorOpen.value = false;
    selected.value = detail;
    await loadItems(filters.cursor);
  } catch {
    error.value = tr('content.error.write');
  } finally {
    writeLoading.value = false;
  }
}

async function lifecycle(action: string) {
  if (!selected.value) return;
  writeLoading.value = true;
  error.value = '';
  try {
    selected.value = await api.lifecycle(selected.value.contentNo, action, {
      confirmed: true,
      contentNo: selected.value.contentNo,
      version: selected.value.version,
    });
    await loadItems(filters.cursor);
  } catch {
    error.value = tr('content.error.write');
  } finally {
    writeLoading.value = false;
  }
}

async function deleteSelected() {
  if (!selected.value) return;
  writeLoading.value = true;
  error.value = '';
  try {
    await api.delete(selected.value.contentNo, selected.value.version);
    deleteOpen.value = false;
    selected.value = null;
    const url = new URL(globalThis.location.href);
    url.searchParams.delete('content');
    globalThis.history.replaceState(globalThis.history.state, '', url);
    await loadItems(filters.cursor);
  } catch {
    error.value = tr('content.error.write');
  } finally {
    writeLoading.value = false;
  }
}

async function candidateAction(action: 'activate' | 'cancel' | 'scan') {
  const detail = selected.value;
  const candidate = detail?.candidateFile;
  const fileNo = candidate?.fileNo;
  if (!detail || !candidate || !fileNo) return;
  writeLoading.value = true;
  error.value = '';
  const body = {
    confirmed: action !== 'scan',
    contentVersion: detail.version,
    fileVersion: candidate.version,
  };
  try {
    selected.value =
      action === 'activate'
        ? await api.activateCandidate(detail.contentNo, fileNo, body)
        : action === 'cancel'
          ? await api.cancelCandidate(detail.contentNo, fileNo, body)
          : await api.rescanCandidate(detail.contentNo, fileNo, body);
    candidateConfirmAction.value = '';
    await loadItems(filters.cursor);
  } catch {
    error.value = tr('content.error.write');
  } finally {
    writeLoading.value = false;
  }
}

onMounted(async () => {
  await Promise.all([loadCategories(), loadItems(), loadUploadPolicy()]);
  const contentNo = new URL(globalThis.location.href).searchParams.get('content');
  if (contentNo) await selectItem(contentNo);
});
</script>

<template>
  <main class="idc-workspace" :data-locale="context.locale" data-testid="content-management-page">
    <section class="panel workspace-toolbar">
      <div><h2>{{ tr('content.title') }}</h2></div>
      <div class="toolbar-actions"><button class="button secondary" type="button" @click="loadItems(filters.cursor)">{{ tr('content.action.refresh') }}</button><button class="button secondary" type="button" @click="openCategories">{{ tr('content.action.categories') }}</button><button class="button primary" data-testid="content-create" type="button" @click="openCreate">{{ tr('content.action.create') }}</button></div>
    </section>

    <div v-if="error" class="notice error" role="alert">{{ error }}</div>

    <section class="metric-grid" :aria-label="tr('content.title')">
      <article class="metric-card accent-violet"><span>{{ tr('content.metric.drafts') }}</span><strong>{{ metrics.drafts }}</strong></article>
      <article class="metric-card accent-teal"><span>{{ tr('content.metric.published') }}</span><strong>{{ metrics.published }}</strong></article>
      <article class="metric-card accent-amber"><span>{{ tr('content.metric.offline') }}</span><strong>{{ metrics.offline }}</strong></article>
      <article class="metric-card accent-blue"><span>{{ tr('content.metric.downloads') }}</span><strong>{{ metrics.downloads }}</strong></article>
    </section>

    <section class="filter-panel">
      <label class="field wide"><span>{{ tr('content.field.keyword') }}</span><input v-model.trim="filters.keyword" type="search" @keyup.enter="applyFilters" /></label>
      <label class="field"><span>{{ tr('content.field.type') }}</span><select v-model="filters.type" @change="applyFilters"><option value="">—</option><option value="news">{{ tr('content.type.news') }}</option><option value="announcement">{{ tr('content.type.announcement') }}</option><option value="knowledge">{{ tr('content.type.knowledge') }}</option><option value="download">{{ tr('content.type.download') }}</option></select></label>
      <label v-if="filters.type !== 'announcement'" class="field"><span>{{ tr('content.field.category') }}</span><select v-model="filters.categoryNo"><option value="">{{ tr('content.filter.all') }}</option><option v-for="category in categories.filter((item) => !item.synthetic && (!filters.type || item.type === filters.type))" :key="category.categoryNo" :value="category.categoryNo">{{ category.name }}</option></select></label>
      <label class="field"><span>{{ tr('content.field.status') }}</span><select v-model="filters.status"><option value="">—</option><option value="draft">{{ tr('content.status.draft') }}</option><option value="published">{{ tr('content.status.published') }}</option><option value="offline">{{ tr('content.status.offline') }}</option></select></label>
      <label class="field"><span>{{ tr('content.field.visibility') }}</span><select v-model="filters.visibility"><option value="">—</option><option value="public">{{ tr('content.visibility.public') }}</option><option value="all_logged_in">{{ tr('content.visibility.all_logged_in') }}</option><option value="customer_group">{{ tr('content.visibility.customer_group') }}</option><option value="internal">{{ tr('content.visibility.internal') }}</option><option value="specific_customers">{{ tr('content.visibility.specific_customers') }}</option><option value="specific_services">{{ tr('content.visibility.specific_services') }}</option></select></label>
      <button class="button primary filter-submit" type="button" @click="applyFilters">{{ tr('content.action.refresh') }}</button>
    </section>

    <section class="split-workspace">
      <div class="panel content-directory">
        <div class="panel-heading"><div><p class="panel-kicker">{{ tr('content.library.label') }}</p><h2>{{ tr('content.title') }}</h2></div><span class="count-pill">{{ items.length }}</span></div>
        <div v-if="loading" class="empty-state">{{ tr('content.state.loading') }}</div>
        <div v-else-if="items.length === 0" class="empty-state">{{ tr('content.empty') }}</div>
        <button v-for="item in items" v-else :key="item.contentNo" class="content-row" :class="{ active: selected?.contentNo === item.contentNo }" :data-testid="`content-item-${item.contentNo}`" type="button" @click="selectItem(item.contentNo)">
          <div class="content-type-mark" :data-type="item.type">{{ typeLabel(item.type).slice(0, 1) }}</div>
          <div class="content-row-main"><div class="ticket-row-top"><span class="content-type-label">{{ typeLabel(item.type) }}</span><span class="stage-badge">{{ statusLabel(item.status) }}</span></div><strong>{{ item.title }}</strong><p>{{ item.summary || item.contentNo }}</p><time>{{ formatTime(item.updatedAt) }}</time></div>
        </button>
        <footer class="pagination"><button class="button ghost" :disabled="cursorHistory.length === 0 || loading" type="button" @click="previousPage">←</button><button class="button ghost" :disabled="!nextCursor || loading" type="button" @click="nextPage">→</button></footer>
      </div>

      <div class="panel content-detail">
        <div v-if="detailLoading" class="empty-state">{{ tr('content.state.loading') }}</div>
        <div v-else-if="!selected" class="empty-state">{{ tr('content.empty') }}</div>
        <template v-else>
          <div class="detail-heading"><div><p class="panel-kicker">{{ typeLabel(selected.type) }} · {{ selected.contentNo }}</p><h2>{{ selected.title }}</h2><p>{{ selected.summary }}</p></div><span class="stage-badge">{{ statusLabel(selected.status) }}</span></div>
          <div class="action-strip">
            <button v-if="selected.availableActions.some((action) => ['edit', 'update'].includes(action))" class="button secondary" type="button" @click="openEdit">{{ tr('content.action.edit') }}</button>
            <button v-if="selected.status === 'draft' && selected.availableActions.includes('publish')" class="button primary" :disabled="writeLoading" type="button" @click="lifecycle('publications')">{{ tr('content.action.publish') }}</button>
            <button v-if="selected.availableActions.includes('unpublish')" class="button secondary" :disabled="writeLoading" type="button" @click="lifecycle('unpublications')">{{ tr('content.action.unpublish') }}</button>
            <button v-if="selected.status === 'offline' && selected.availableActions.some((action) => ['publish', 'republish'].includes(action))" class="button primary" :disabled="writeLoading" type="button" @click="lifecycle('republications')">{{ tr('content.action.republish') }}</button>
            <button v-if="selected.availableActions.includes('delete')" class="button danger" type="button" @click="deleteOpen = true">{{ tr('content.action.delete') }}</button>
          </div>

          <dl class="fact-grid">
            <div><dt>{{ tr('content.field.visibility') }}</dt><dd>{{ visibilityLabel(selected.visibility) }}</dd></div>
            <div><dt>{{ tr('content.field.category') }}</dt><dd>{{ categoryLabel(selected.categoryNo) }}</dd></div>
            <div><dt>{{ tr('content.field.publicPath') }}</dt><dd class="mono wrap">{{ selected.publicPath || '—' }}</dd></div>
            <div><dt>{{ tr('content.field.status') }}</dt><dd>{{ statusLabel(selected.status) }}</dd></div>
          </dl>

          <section class="detail-section prose-preview"><div class="section-title"><h3>{{ tr('content.field.body') }}</h3></div><p class="content-body">{{ selected.body }}</p></section>

          <section v-if="selected.type === 'download'" class="detail-section">
            <div class="section-title"><h3>{{ tr('content.file.upload') }}</h3><time v-if="activeUpload">{{ tr('content.upload.expires', { time: formatTime(activeUpload.expiresAt) }) }}</time></div>
            <div v-if="!selected.candidateFile" class="upload-card">
              <div><strong>{{ activeUpload?.fileName || tr('content.file.upload') }}</strong><p data-testid="content-upload-policy">{{ uploadPolicyHelp() }}</p><div v-if="uploadPolicyState !== 'loading' && !inspectionUploadsAvailable" class="notice info" data-testid="content-upload-inspection-unavailable" role="status">{{ tr('content.file.inspectionUnavailable') }}</div><div v-if="activeUpload || uploadLoading" class="progress-track" role="progressbar" :aria-valuenow="displayedUploadPercent" aria-valuemin="0" aria-valuemax="100"><span :style="{ width: `${displayedUploadPercent}%` }"></span></div><small v-if="activeUpload || uploadLoading">{{ tr('content.upload.progress', { percent: displayedUploadPercent }) }}</small></div>
              <label v-if="!activeUpload || activeUpload.canResume" class="button secondary file-button"><input :accept="acceptedUploadExtensions" data-testid="content-upload-file" type="file" @change="chooseDownloadFile" />{{ activeUpload ? tr('content.action.resumeUpload') : tr('content.action.upload') }}</label>
              <button v-if="uploadFile" class="button primary" data-testid="content-upload-start" :disabled="uploadLoading" type="button" @click="uploadSelectedFile">{{ uploadLoading ? tr('content.upload.progress', { percent: displayedUploadPercent }) : activeUpload ? tr('content.action.resumeUpload') : tr('content.action.startUpload') }}</button>
              <button v-if="activeUpload?.canCancel" class="button danger" :disabled="uploadLoading" type="button" @click="uploadCancelOpen = true">{{ tr('content.action.cancel') }}</button>
            </div>
            <div class="section-title"><h3>{{ tr('content.file.current') }}</h3></div>
            <article class="file-card"><div><strong>{{ selected.currentFile?.fileName || tr('content.file.none') }}</strong><p>{{ formatBytes(selected.currentFile?.sizeBytes) }} · {{ selected.currentFile?.sha256 || '—' }}</p></div><span class="stage-badge">{{ scanStatusLabel(selected.currentFile?.scanStatus) }}</span></article>
            <template v-if="selected.candidateFile"><div class="section-title compact"><h3>{{ tr('content.file.pending') }}</h3><time>{{ formatTime(selected.candidateFile.gateExpiresAt) }}</time></div><article class="file-card candidate"><div><strong>{{ selected.candidateFile.fileName }}</strong><p>{{ formatBytes(selected.candidateFile.sizeBytes) }} · {{ scanStatusLabel(selected.candidateFile.scanStatus) }}</p></div><div class="action-strip"><button v-if="selected.candidateFile.canRescan || selected.availableActions.includes('rescan_candidate')" class="button secondary" type="button" @click="candidateAction('scan')">{{ tr('content.action.rescan') }}</button><button v-if="selected.candidateFile.canEnable || selected.availableActions.includes('activate_candidate')" class="button primary" type="button" @click="candidateConfirmAction = 'activate'">{{ tr('content.action.activate') }}</button><button v-if="selected.candidateFile.canCancel !== false" class="button ghost" type="button" @click="candidateConfirmAction = 'cancel'">{{ tr('content.action.cancelCandidate') }}</button></div></article></template>
          </section>
        </template>
      </div>
    </section>

    <div v-if="editorOpen" class="modal-backdrop" role="presentation" @click.self="editorOpen = false">
      <form class="modal-card modal-wide" data-testid="content-editor" @submit.prevent="save('draft')">
        <div class="modal-heading"><div><p class="panel-kicker">{{ tr('content.editor.label') }}</p><h2>{{ editor.contentNo ? tr('content.action.edit') : tr('content.action.create') }}</h2></div><button class="icon-button" type="button" @click="editorOpen = false">×</button></div>
        <div class="notice info">{{ tr('content.note.original') }}</div>
        <div class="form-grid"><label class="field"><span>{{ tr('content.field.type') }}</span><select v-model="editor.type" :disabled="Boolean(editor.contentNo)" @change="handleEditorTypeChange"><option value="news">{{ tr('content.type.news') }}</option><option value="announcement">{{ tr('content.type.announcement') }}</option><option value="knowledge">{{ tr('content.type.knowledge') }}</option><option value="download">{{ tr('content.type.download') }}</option></select></label><label class="field"><span>{{ tr('content.field.visibility') }}</span><select v-model="editor.visibility" @change="resetAudienceSelection"><option value="public">{{ tr('content.visibility.public') }}</option><option value="all_logged_in">{{ tr('content.visibility.all_logged_in') }}</option><option value="customer_group">{{ tr('content.visibility.customer_group') }}</option><option value="internal">{{ tr('content.visibility.internal') }}</option><option v-if="editor.type === 'announcement'" value="specific_customers">{{ tr('content.visibility.specific_customers') }}</option><option v-if="editor.type === 'announcement'" value="specific_services">{{ tr('content.visibility.specific_services') }}</option></select></label></div>
        <label v-if="editor.type !== 'announcement'" class="field"><span>{{ tr('content.field.category') }}</span><select v-model="editor.categoryNo"><option value="">{{ tr('content.category.uncategorized') }}</option><option v-for="category in categories.filter((item) => !item.synthetic && item.type === editor.type)" :key="category.categoryNo" :value="category.categoryNo">{{ category.name }}</option></select></label>
        <label class="field wide"><span>{{ tr('content.field.title') }}</span><input v-model.trim="editor.title" required maxlength="240" /></label>
        <label class="field wide"><span>{{ tr('content.field.summary') }}</span><textarea v-model="editor.summary" rows="3" /></label>
        <label class="field wide"><span>{{ tr('content.field.body') }}</span><textarea v-model="editor.body" required rows="12" /></label>
        <div class="notice info">{{ tr('content.note.seo') }}</div>
        <div class="form-grid"><label class="field"><span>{{ tr('content.field.seoTitle') }}</span><input v-model="editor.seoTitle" /></label><label class="field"><span>{{ tr('content.field.seoDescription') }}</span><textarea v-model="editor.seoDescription" rows="2" /></label></div>
        <section v-if="audienceType" class="audience-picker">
          <div class="section-title compact"><h3>{{ tr('content.field.scopeTargets') }}</h3><span class="count-pill">{{ selectedAudienceTargets.length }}</span></div>
          <div class="audience-search"><label class="field"><span>{{ tr('content.audience.search') }}</span><input v-model.trim="audienceQuery" type="search" @keyup.enter.prevent="searchAudience" /></label><button class="button secondary" :disabled="audienceLoading" type="button" @click="searchAudience">{{ audienceLoading ? tr('content.state.loading') : tr('content.action.search') }}</button></div>
          <div v-if="selectedAudienceTargets.length" class="audience-selected" :aria-label="tr('content.audience.selected')"><span v-for="target in selectedAudienceTargets" :key="`${target.type}:${target.number}`" class="audience-chip">{{ target.label }}<button type="button" :aria-label="tr('content.action.remove')" @click="removeAudience(target)">×</button></span></div>
          <p v-else class="audience-help">{{ tr('content.audience.required') }}</p>
          <div class="audience-results"><button v-for="candidate in audienceCandidates" :key="candidateNumber(candidate)" class="audience-option" type="button" @click="addAudience(candidate)"><strong>{{ candidate.label }}</strong><span>{{ tr('content.audience.add') }}</span></button><p v-if="!audienceLoading && audienceCandidates.length === 0" class="audience-help">{{ tr('content.audience.empty') }}</p></div>
        </section>
        <footer class="modal-actions"><button class="button ghost" type="button" @click="editorOpen = false">{{ tr('content.action.cancel') }}</button><button class="button secondary" :disabled="writeLoading" type="submit">{{ tr('content.action.saveDraft') }}</button><button class="button primary" :disabled="writeLoading" type="button" @click="save('publish')">{{ tr('content.action.publish') }}</button></footer>
      </form>
    </div>

    <div v-if="deleteOpen" class="modal-backdrop" role="presentation" @click.self="deleteOpen = false">
      <section class="modal-card confirm-card" role="alertdialog" aria-modal="true"><div class="danger-mark">!</div><h2>{{ tr('content.delete.title') }}</h2><p>{{ tr('content.delete.body') }}</p><footer class="modal-actions"><button class="button ghost" type="button" @click="deleteOpen = false">{{ tr('content.action.cancel') }}</button><button class="button danger" :disabled="writeLoading" type="button" @click="deleteSelected">{{ tr('content.action.delete') }}</button></footer></section>
    </div>

    <div v-if="uploadCancelOpen" class="modal-backdrop" role="presentation" @click.self="uploadCancelOpen = false">
      <section class="modal-card confirm-card" role="alertdialog" aria-modal="true"><div class="danger-mark">!</div><h2>{{ tr('content.upload.cancelTitle') }}</h2><p>{{ tr('content.upload.cancelBody') }}</p><footer class="modal-actions"><button class="button ghost" type="button" @click="uploadCancelOpen = false">{{ tr('content.action.cancel') }}</button><button class="button danger" :disabled="uploadLoading" type="button" @click="cancelUpload">{{ tr('content.action.cancelCandidate') }}</button></footer></section>
    </div>

    <div v-if="candidateConfirmAction" class="modal-backdrop" role="presentation" @click.self="candidateConfirmAction = ''">
      <section class="modal-card confirm-card" role="alertdialog" aria-modal="true"><div class="danger-mark">!</div><h2>{{ tr(`content.candidate.${candidateConfirmAction}Title`) }}</h2><p>{{ tr(`content.candidate.${candidateConfirmAction}Body`) }}</p><footer class="modal-actions"><button class="button ghost" type="button" @click="candidateConfirmAction = ''">{{ tr('content.action.cancel') }}</button><button :class="['button', candidateConfirmAction === 'activate' ? 'primary' : 'danger']" :disabled="writeLoading" type="button" @click="candidateAction(candidateConfirmAction)">{{ candidateConfirmAction === 'activate' ? tr('content.action.activate') : tr('content.action.cancelCandidate') }}</button></footer></section>
    </div>

    <div v-if="categoryOpen" class="modal-backdrop" role="presentation" @click.self="categoryOpen = false">
      <section class="modal-card" role="dialog" aria-modal="true"><div class="modal-heading"><div><p class="panel-kicker">{{ tr('content.action.categories') }}</p><h2>{{ tr('content.category.title') }}</h2></div><button class="icon-button" type="button" @click="categoryOpen = false">×</button></div><form class="category-create" @submit.prevent="createCategory"><label class="field"><span>{{ tr('content.field.type') }}</span><select v-model="categoryEditor.type"><option value="news">{{ tr('content.type.news') }}</option><option value="knowledge">{{ tr('content.type.knowledge') }}</option><option value="download">{{ tr('content.type.download') }}</option></select></label><label class="field"><span>{{ tr('content.category.name') }}</span><input v-model.trim="categoryEditor.name" required maxlength="80" /></label><button class="button primary" :disabled="writeLoading" type="submit">{{ tr('content.action.createCategory') }}</button></form><div class="category-list"><p v-if="categories.filter((item) => !item.synthetic && item.type === categoryEditor.type).length === 0" class="empty-state">{{ tr('content.category.empty') }}</p><article v-for="category in categories.filter((item) => !item.synthetic && item.type === categoryEditor.type)" v-else :key="category.categoryNo"><strong>{{ category.name }}</strong><button class="button danger" type="button" @click="categoryToDelete = category">{{ tr('content.action.delete') }}</button></article></div></section>
    </div>

    <div v-if="categoryToDelete" class="modal-backdrop" role="presentation" @click.self="categoryToDelete = null">
      <section class="modal-card confirm-card" role="alertdialog" aria-modal="true"><div class="danger-mark">!</div><h2>{{ tr('content.category.deleteTitle') }}</h2><p>{{ tr('content.category.deleteBody') }}</p><footer class="modal-actions"><button class="button ghost" type="button" @click="categoryToDelete = null">{{ tr('content.action.cancel') }}</button><button class="button danger" :disabled="writeLoading" type="button" @click="deleteCategory">{{ tr('content.action.delete') }}</button></footer></section>
    </div>
  </main>
</template>

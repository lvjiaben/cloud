<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue';

import type { PluginHostMountContext } from '../host-context';
import type {
  DirectoryItem,
  MergeCandidate,
  SupportCustomFieldDefinition,
  SupportUpstreamForward,
  SupportUpstreamPreview,
  TicketCustomFieldSubmission,
  TicketCustomFieldValue,
  TicketAttachment,
  TicketDetail,
  TicketListQuery,
  TicketMessage,
  TicketSummary,
} from './api';

import { createTranslator } from '../runtime/i18n';
import { createSupportAPI } from './api';
import { supportMessages } from './messages';

const props = defineProps<{ context: PluginHostMountContext }>();
const currentContext = () => props.context;
const api = createSupportAPI(currentContext);
const tr = createTranslator(currentContext, supportMessages);

const filters = reactive<TicketListQuery>({ limit: 30, query: '', stage: '', status: '', departmentNo: '', priority: '' });
const showMoreFilters = ref(false);
const tickets = ref<TicketSummary[]>([]);
const selected = ref<TicketDetail | null>(null);
const selectedTicketNo = ref('');
const nextCursor = ref('');
const pageActions = ref<string[]>([]);
const cursorHistory = ref<string[]>([]);
const loading = ref(false);
const detailLoading = ref(false);
const writeLoading = ref(false);
const error = ref('');
const success = ref('');
const departments = ref<DirectoryItem[]>([]);
const categories = ref<DirectoryItem[]>([]);
const statuses = ref<DirectoryItem[]>([]);
const replyPresets = ref<DirectoryItem[]>([]);
const mergeCandidates = ref<MergeCandidate[]>([]);
const mergeCandidatesLoading = ref(false);
const mergeCandidatesNextCursor = ref('');
const internalCustomFields = ref<SupportCustomFieldDefinition[]>([]);
const upstreamPreview = ref<SupportUpstreamPreview | null>(null);
const upstreamForwards = ref<SupportUpstreamForward[]>([]);
const createCustomFieldValues = reactive<Record<string, string | string[] | undefined>>({});
const createFieldsLoading = ref(false);
const createFieldsFailed = ref(false);
const createTrigger = ref<HTMLButtonElement | null>(null);
const createSubjectInput = ref<HTMLInputElement | null>(null);
let detailRequestSequence = 0;

const baseStages = [
  'new',
  'in_progress',
  'waiting_customer',
  'waiting_upstream',
  'resolved',
  'closed',
];

const createOpen = ref(false);
const createFiles = ref<File[]>([]);
const createForm = reactive({
  body: '',
  categoryNo: '',
  departmentNo: '',
  priority: 'normal',
  subject: '',
});

const composer = reactive({
  body: '',
  categoryNo: '',
  customStatusNo: '',
  departmentNo: '',
  files: [] as File[],
  kind: '',
  presetNo: '',
  reason: '',
  selectedAttachmentNos: [] as string[],
  selectedMergeTicketNos: [] as string[],
  selectedMessageNos: [] as string[],
  targetNo: '',
  targetType: '',
  targetDigest: '',
  targetVersion: 0,
});

const metrics = computed(() => ({
  departmentQueue: tickets.value.filter((item) => item.queue === 'department')
    .length,
  followUp: tickets.value.filter((item) => item.followUp).length,
  publicQueue: tickets.value.filter((item) => item.queue === 'public').length,
  slaRisk: tickets.value.filter((item) => item.slaRisk !== 'none').length,
}));

const selectableAttachments = computed(() =>
  (selected.value?.attachments ?? []).filter(
    (attachment) =>
      !attachment.hidden &&
      ['customer_visible', 'public'].includes(attachment.visibility),
  ),
);

const selectedStatusTransition = computed(() =>
  selected.value?.availableStatusTransitions?.find(
    (item) => item.statusNo === composer.customStatusNo,
  ),
);

function localizedName(item: DirectoryItem) {
  const fallback = item.title || '';
  if (props.context.locale.toLowerCase().startsWith('zh')) {
    return item.nameZhCN || item.nameEnUS || fallback;
  }
  return item.nameEnUS || item.nameZhCN || fallback;
}

function directoryValue(item: DirectoryItem) {
  return (
    item.departmentNo ||
    item.categoryNo ||
    item.statusNo ||
    item.presetNo ||
    ''
  );
}

function departmentLabel(departmentNo?: string) {
  if (!departmentNo) return tr('support.value.none');
  const item = departments.value.find(
    (candidate) => candidate.departmentNo === departmentNo,
  );
  return item ? localizedName(item) : tr('support.value.restricted');
}

function categoryLabel(ticket: TicketSummary) {
  const localized = props.context.locale.toLowerCase().startsWith('zh')
    ? ticket.categoryNameZhCN || ticket.categoryNameEnUS
    : ticket.categoryNameEnUS || ticket.categoryNameZhCN;
  if (localized) return localized;
  const item = categories.value.find(
    (candidate) => candidate.categoryNo === ticket.categoryNo,
  );
  return item ? localizedName(item) : tr('support.value.none');
}

function customStatusLabel(statusNo?: string) {
  if (!statusNo) return '';
  const item = statuses.value.find((candidate) => candidate.statusNo === statusNo);
  return item ? localizedName(item) : tr('support.value.unknown');
}

function translatedCode(prefix: string, value?: string) {
  if (!value) return tr('support.value.none');
  const key = `${prefix}.${value}`;
  const label = tr(key);
  return label === key ? tr('support.value.unknown') : label;
}

function localizedCustomFieldName(
  item: Pick<SupportCustomFieldDefinition | TicketCustomFieldValue, 'nameEnUS' | 'nameZhCN'>,
) {
  return props.context.locale.toLowerCase().startsWith('zh')
    ? item.nameZhCN || item.nameEnUS
    : item.nameEnUS || item.nameZhCN;
}

function localizedCustomFieldDescription(
  item: Pick<SupportCustomFieldDefinition | TicketCustomFieldValue, 'descriptionEnUS' | 'descriptionZhCN'>,
) {
  return props.context.locale.toLowerCase().startsWith('zh')
    ? item.descriptionZhCN || item.descriptionEnUS || ''
    : item.descriptionEnUS || item.descriptionZhCN || '';
}

function customFieldSnapshotValue(item: TicketCustomFieldValue) {
  if (item.selectedOptions?.length) {
    return item.selectedOptions.map(localizedCustomFieldName).join(', ');
  }
  if (item.fieldType === 'boolean') {
    return item.normalizedValue === 'true'
      ? tr('support.value.yes')
      : tr('support.value.no');
  }
  return item.normalizedValue || tr('support.value.none');
}

function relatedEntitlementName(current = false) {
  const entitlement = selected.value?.ticket.relatedEntitlement;
  if (!entitlement) return '';
  const useChinese = props.context.locale.toLowerCase().startsWith('zh');
  if (current && entitlement.currentAvailable) {
    return useChinese
      ? entitlement.currentNameZhCN || entitlement.currentNameEnUS
      : entitlement.currentNameEnUS || entitlement.currentNameZhCN;
  }
  return useChinese
    ? entitlement.snapshotNameZhCN || entitlement.snapshotNameEnUS
    : entitlement.snapshotNameEnUS || entitlement.snapshotNameZhCN;
}

function entitlementStatus(value?: string) {
  return translatedCode('support.entitlement.status', value);
}

function createCustomFieldSubmissions() {
  return internalCustomFields.value.flatMap<TicketCustomFieldSubmission>((field) => {
    const value = createCustomFieldValues[field.fieldNo];
    const empty = value === undefined || value === '' || (Array.isArray(value) && value.length === 0);
    if (empty) return [];
    switch (field.fieldType) {
      case 'boolean':
        return [{ booleanValue: value === 'true', fieldNo: field.fieldNo }];
      case 'date':
        return [{ dateValue: String(value), fieldNo: field.fieldNo }];
      case 'multi_select':
      case 'single_select':
        return [{ fieldNo: field.fieldNo, optionNos: Array.isArray(value) ? value : [String(value)] }];
      case 'number':
        return [{ fieldNo: field.fieldNo, numberValue: String(value).trim() }];
      default:
        return [{ fieldNo: field.fieldNo, textValue: String(value).trim() }];
    }
  });
}

function fileTypeLabel(fileName: string) {
  const extension = fileName.split('.').pop()?.trim().toUpperCase();
  return extension && extension !== fileName.toUpperCase()
    ? tr('support.file.typed', { extension })
    : tr('support.file.generic');
}

function formatBytes(value?: number) {
  if (!value) return '0 B';
  const units = ['B', 'KiB', 'MiB', 'GiB'];
  let amount = value;
  let unit = 0;
  while (amount >= 1024 && unit < units.length - 1) {
    amount /= 1024;
    unit += 1;
  }
  return `${amount.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

function formatTime(value?: number) {
  if (!value) return tr('support.value.none');
  return new Intl.DateTimeFormat(props.context.locale, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value));
}

function messageAuthor(message: TicketMessage) {
  return translatedCode('support.author', message.authorType);
}

function can(action: string) {
  return selected.value?.availableActions?.includes(action) ?? false;
}

async function loadDirectories() {
  const [departmentResult, categoryResult, statusResult] = await Promise.all([
    api.departments(),
    api.issueCategories(),
    api.statuses(),
  ]);
  departments.value = departmentResult.departments ?? [];
  categories.value = categoryResult.issueCategories ?? [];
  statuses.value = statusResult.statuses ?? [];
}

async function loadReplyPresets() {
  try {
    const result = await api.replyPresets();
    replyPresets.value = result.replyPresets ?? [];
  } catch {
    replyPresets.value = [];
  }
}

async function loadTickets(cursor = '') {
  loading.value = true;
  error.value = '';
  try {
    const result = await api.list({ ...filters, cursor });
    tickets.value = result.tickets ?? [];
    pageActions.value = result.pageActions ?? [];
    nextCursor.value = result.nextCursor ?? '';
    if (!pageActions.value.includes('query')) {
      detailRequestSequence += 1;
      selected.value = null;
      selectedTicketNo.value = '';
    }
    if (
      selectedTicketNo.value &&
      !tickets.value.some((item) => item.ticketNo === selectedTicketNo.value)
    ) {
      selected.value = null;
      selectedTicketNo.value = '';
    }
  } catch {
    error.value = tr('support.error.load');
  } finally {
    loading.value = false;
  }
}

async function selectTicket(ticketNo: string) {
  if (!pageActions.value.includes('query')) return;
  const requestSequence = ++detailRequestSequence;
  detailLoading.value = true;
  error.value = '';
  selectedTicketNo.value = ticketNo;
  composer.kind = '';
  try {
    const detail = await api.ticket(ticketNo);
    if (requestSequence !== detailRequestSequence) return;
    selected.value = detail;
    const currentURL = new URL(globalThis.location.href);
    currentURL.searchParams.set('ticket', ticketNo);
    globalThis.history.replaceState(globalThis.history.state, '', currentURL);
    upstreamForwards.value = detail.availableActions.includes('upstream')
      ? await api.ticketUpstreamForwards(ticketNo)
      : [];
    if (requestSequence !== detailRequestSequence) return;
  } catch {
    if (requestSequence !== detailRequestSequence) return;
    error.value = tr('support.error.load');
    selected.value = null;
    upstreamForwards.value = [];
  } finally {
    if (requestSequence === detailRequestSequence) detailLoading.value = false;
  }
}

function resetFilters() {
  Object.assign(filters, { query: '', stage: '', status: '', departmentNo: '', priority: '', followUp: undefined });
  applyFilters();
}

function applyFilters() {
  cursorHistory.value = [];
  filters.cursor = '';
  void loadTickets();
}

function nextPage() {
  if (!nextCursor.value) return;
  cursorHistory.value.push(filters.cursor ?? '');
  filters.cursor = nextCursor.value;
  void loadTickets(nextCursor.value);
}

function previousPage() {
  const cursor = cursorHistory.value.pop() ?? '';
  filters.cursor = cursor;
  void loadTickets(cursor);
}

function openComposer(kind: string) {
  composer.kind = kind;
  composer.body = '';
  composer.categoryNo = '';
  composer.customStatusNo = '';
  composer.departmentNo = '';
  composer.files = [];
  composer.presetNo = '';
  composer.reason = '';
  composer.selectedAttachmentNos = [];
  composer.selectedMergeTicketNos = [];
  composer.selectedMessageNos = [];
  composer.targetNo = '';
  composer.targetType = '';
  composer.targetDigest = '';
  composer.targetVersion = 0;
  upstreamPreview.value = null;
  if (kind === 'public-replies') void loadReplyPresets();
  if (['global-merges', 'merges'].includes(kind)) {
    mergeCandidates.value = [];
    mergeCandidatesNextCursor.value = '';
    void loadMergeCandidates();
  }
}

async function loadMergeCandidates(append = false) {
  if (!selected.value) return;
  mergeCandidatesLoading.value = true;
  try {
    const result = await api.mergeCandidates(
      selected.value.ticket.ticketNo,
      append ? mergeCandidatesNextCursor.value : '',
      composer.kind === 'global-merges',
    );
    mergeCandidates.value = append
      ? [...mergeCandidates.value, ...(result.items ?? [])]
      : (result.items ?? []);
    mergeCandidatesNextCursor.value = result.nextCursor ?? '';
  } catch {
    mergeCandidates.value = [];
    error.value = tr('support.error.load');
  } finally {
    mergeCandidatesLoading.value = false;
  }
}

function openHide(targetType: 'attachment' | 'message', targetNo: string) {
  openComposer('hidden-content');
  composer.targetType = targetType;
  composer.targetNo = targetNo;
}

function openUpstreamRetry(forward: SupportUpstreamForward) {
  openComposer('upstream-retry');
  composer.targetNo = forward.forwardNo;
  composer.targetDigest = forward.previewDigest;
  composer.targetVersion = forward.version;
}

function openUpstreamPublication(messageNo: string) {
  openComposer('upstream-publication');
  composer.targetNo = messageNo;
}

function requiresReason(kind: string) {
  if (kind === 'status-transitions') {
    return selectedStatusTransition.value?.requiresReason ?? false;
  }
  return [
    'closures',
    'escalations',
    'hidden-content',
    'global-merges',
    'merges',
    'reopenings',
    'splits',
    'transfers',
    'upstream',
    'upstream-publication',
    'upstream-retry',
  ].includes(kind);
}

function chooseFiles(event: Event) {
  const input = event.currentTarget as HTMLInputElement;
  const files = [...(input.files ?? [])];
  if (files.some((file) => file.size > 20 * 1024 * 1024)) {
    error.value = tr('support.error.fileSize');
    input.value = '';
    composer.files = [];
    return;
  }
  error.value = '';
  composer.files = files;
}

function chooseCreateFiles(event: Event) {
  const input = event.currentTarget as HTMLInputElement;
  const files = [...(input.files ?? [])];
  if (files.some((file) => file.size > 20 * 1024 * 1024)) {
    error.value = tr('support.error.fileSize');
    input.value = '';
    createFiles.value = [];
    return;
  }
  error.value = '';
  createFiles.value = files;
}

function applyPreset() {
  const preset = replyPresets.value.find(
    (item) => item.presetNo === composer.presetNo,
  );
  if (preset?.body) composer.body = preset.body;
}

async function uploadComposerFiles(ticketNo: string) {
  const context =
    composer.kind === 'internal-notes' ? 'internal_note' : 'public_reply';
  const uploadNos: string[] = [];
  for (const file of composer.files) {
    const upload = await api.createAttachmentUpload(ticketNo, {
      context,
      fileName: file.name,
      sizeBytes: file.size,
    });
    const ready = await api.uploadAttachmentContent(upload.uploadNo, file, context);
    if (!ready.ready) throw new Error('attachment_not_ready');
    uploadNos.push(ready.uploadNo);
  }
  return uploadNos;
}

async function uploadInitialFiles() {
  const uploadNos: string[] = [];
  for (const file of createFiles.value) {
    const upload = await api.createInitialAttachmentUpload({
      fileName: file.name,
      sizeBytes: file.size,
    });
    const ready = await api.uploadAttachmentContent(
      upload.uploadNo,
      file,
      'internal_initial',
    );
    if (!ready.ready) throw new Error('attachment_not_ready');
    uploadNos.push(ready.uploadNo);
  }
  return uploadNos;
}

async function downloadAttachment(attachment: TicketAttachment) {
  error.value = '';
  try {
    const blob = await api.downloadAttachment(attachment.attachmentNo);
    const objectURL = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = objectURL;
    anchor.download = attachment.fileName;
    anchor.click();
    URL.revokeObjectURL(objectURL);
  } catch {
    error.value = tr('support.error.download');
  }
}

async function refreshAfterWrite(ticketNo: string) {
  await Promise.all([loadTickets(filters.cursor ?? ''), selectTicket(ticketNo)]);
  composer.kind = '';
  success.value = tr('support.success.saved');
}

async function claimTicket() {
  if (!selected.value) return;
  writeLoading.value = true;
  error.value = '';
  success.value = '';
  try {
    await api.action(selected.value.ticket.ticketNo, 'claims', {
      ticketVersion: selected.value.ticket.version,
    });
    await refreshAfterWrite(selected.value.ticket.ticketNo);
  } catch {
    error.value = tr('support.error.write');
  } finally {
    writeLoading.value = false;
  }
}

async function submitAction() {
  if (!selected.value || !composer.kind) return;
  const ticketNo = selected.value.ticket.ticketNo;
  const ticketVersion = selected.value.ticket.version;
  writeLoading.value = true;
  error.value = '';
  success.value = '';
  try {
    let body: Record<string, unknown> = { ticketVersion };
    switch (composer.kind) {
      case 'public-replies':
      case 'internal-notes':
        body = {
          ...body,
          body: composer.body,
          uploadNos: await uploadComposerFiles(ticketNo),
        };
        break;
      case 'transfers':
        await api.transfer(ticketNo, {
          ticketVersion,
          departmentNo: composer.departmentNo,
          reason: composer.reason,
        });
        detailRequestSequence += 1;
        selected.value = null;
        selectedTicketNo.value = '';
        composer.kind = '';
        await loadTickets(filters.cursor ?? '');
        success.value = tr('support.success.saved');
        return;
      case 'escalations':
        body = { ...body, reason: composer.reason, toUserId: 0 };
        break;
      case 'status-transitions':
        if (!selectedStatusTransition.value) {
          throw new Error('status_transition_unavailable');
        }
        await api.transitionStatus(ticketNo, {
          customStatusNo: selectedStatusTransition.value.statusNo,
          reason: composer.reason,
          ticketVersion,
        });
        await refreshAfterWrite(ticketNo);
        return;
      case 'splits':
        body = {
          ...body,
          attachmentNos: composer.selectedAttachmentNos,
          categoryNo: composer.categoryNo,
          messageNos: composer.selectedMessageNos,
          reason: composer.reason,
        };
        break;
      case 'upstream':
        if (!upstreamPreview.value) {
          upstreamPreview.value = await api.prepareUpstream(ticketNo, {
            attachmentNos: composer.selectedAttachmentNos,
            messageNos: composer.selectedMessageNos,
            operationNo: `UPP_${globalThis.crypto.randomUUID().replaceAll('-', '')}`,
            ticketVersion,
          });
          return;
        }
        if (!globalThis.confirm(tr('support.upstream.confirmSend'))) return;
        await api.confirmUpstream(upstreamPreview.value.forwardNo, {
          expectedVersion: upstreamPreview.value.version,
          operationNo: `UPS_${globalThis.crypto.randomUUID().replaceAll('-', '')}`,
          previewDigest: upstreamPreview.value.previewDigest,
          reason: composer.reason,
        });
        await refreshAfterWrite(ticketNo);
        return;
      case 'upstream-retry':
        if (!globalThis.confirm(tr('support.upstream.confirmRetry'))) return;
        await api.retryUpstream(composer.targetNo, {
          expectedVersion: composer.targetVersion,
          operationNo: `UPR_${globalThis.crypto.randomUUID().replaceAll('-', '')}`,
          previewDigest: composer.targetDigest,
          reason: composer.reason,
        });
        await refreshAfterWrite(ticketNo);
        return;
      case 'upstream-publication':
        await api.publishUpstreamReply(ticketNo, composer.targetNo, {
          reason: composer.reason,
          ticketVersion,
        });
        await refreshAfterWrite(ticketNo);
        return;
      case 'hidden-content':
        body = {
          ...body,
          reason: composer.reason,
          targetNo: composer.targetNo,
          targetType: composer.targetType,
        };
        break;
      case 'global-merges':
      case 'merges':
        body = {
          mainVersion: ticketVersion,
          reason: composer.reason,
          sources: mergeCandidates.value
            .filter((item) =>
              composer.selectedMergeTicketNos.includes(item.ticketNo),
            )
            .slice(0, 20)
            .map((item) => ({ ticketNo: item.ticketNo, version: item.version })),
        };
        break;
      default:
        body = { ...body, reason: composer.reason };
    }
    await api.action(ticketNo, composer.kind, body);
    await refreshAfterWrite(ticketNo);
  } catch {
    error.value = tr(
      composer.kind === 'status-transitions'
        ? 'support.error.statusTransition'
        : 'support.error.write',
    );
  } finally {
    writeLoading.value = false;
  }
}

async function syncUpstreamReplies(forward: SupportUpstreamForward) {
  if (!selected.value) return;
  writeLoading.value = true;
  error.value = '';
  success.value = '';
  try {
    const result = await api.syncUpstreamReplies(
      forward.forwardNo,
      forward.version,
    );
    await refreshAfterWrite(selected.value.ticket.ticketNo);
    success.value = tr('support.upstream.syncResult', { count: result.added });
  } catch {
    error.value = tr('support.error.write');
  } finally {
    writeLoading.value = false;
  }
}

async function createTicket() {
  writeLoading.value = true;
  error.value = '';
  success.value = '';
  try {
    const result = await api.createTicket({
      ...createForm,
      customFields: createCustomFieldSubmissions(),
      uploadNos: await uploadInitialFiles(),
    });
    closeCreate();
    await loadTickets();
    await selectTicket(result.ticket.ticketNo);
  } catch {
    error.value = tr('support.error.write');
  } finally {
    writeLoading.value = false;
  }
}

async function openCreate() {
  createFiles.value = [];
  for (const key of Object.keys(createCustomFieldValues)) {
    delete createCustomFieldValues[key];
  }
  internalCustomFields.value = [];
  createFieldsFailed.value = false;
  createOpen.value = true;
  createFieldsLoading.value = true;
  try {
    const result = await api.internalCustomFields();
    internalCustomFields.value = result.items ?? [];
  } catch {
    createFieldsFailed.value = true;
  } finally {
    createFieldsLoading.value = false;
  }
  void nextTick(() => createSubjectInput.value?.focus());
}

function closeCreate() {
  createOpen.value = false;
  void nextTick(() => createTrigger.value?.focus());
}

function counterpartTicket(sourceTicketNo: string, targetTicketNo: string) {
  return sourceTicketNo === selected.value?.ticket.ticketNo
    ? targetTicketNo
    : sourceTicketNo;
}

function visibleAttachments(message: TicketMessage) {
  return message.attachments?.filter((item) => !item.hidden) ?? [];
}

function canHideAttachment(attachment: TicketAttachment) {
  return can('hide') && !attachment.hidden;
}

onMounted(async () => {
  loading.value = true;
  try {
    await Promise.all([loadDirectories(), loadTickets()]);
    const requestedTicketNo = new URL(globalThis.location.href).searchParams.get('ticket') ?? '';
    // A dashboard deep link may refer to a ticket outside the first list page.
    // The detail endpoint applies the same ticket visibility authorization.
    if (requestedTicketNo && pageActions.value.includes('query')) {
      await selectTicket(requestedTicketNo);
    }
  } catch {
    error.value = tr('support.error.load');
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <main class="idc-workspace" data-testid="support-desk-page">
    <section class="panel workspace-toolbar">
      <div><h2>{{ tr('support.title') }}</h2></div>
      <div class="toolbar-actions">
        <button class="button secondary" data-testid="support-refresh" type="button" @click="loadTickets(filters.cursor ?? '')">
          {{ tr('support.action.refresh') }}
        </button>
        <button v-if="pageActions.includes('create')" ref="createTrigger" class="button primary" data-testid="support-create" type="button" @click="openCreate">
          {{ tr('support.action.create') }}
        </button>
      </div>
    </section>

    <div v-if="error" class="notice error" role="alert">{{ error }}</div>
    <div v-if="success" class="notice success" role="status">{{ success }}</div>

    <section class="metric-grid" :aria-label="tr('support.title')">
      <article class="metric-card accent-blue"><span>{{ tr('support.metric.publicQueue') }}</span><strong>{{ metrics.publicQueue }}</strong></article>
      <article class="metric-card accent-teal"><span>{{ tr('support.metric.departmentQueue') }}</span><strong>{{ metrics.departmentQueue }}</strong></article>
      <article class="metric-card accent-amber"><span>{{ tr('support.metric.slaRisk') }}</span><strong>{{ metrics.slaRisk }}</strong></article>
      <button class="metric-card accent-rose" type="button" @click="filters.followUp = true; applyFilters()"><span>{{ tr('support.metric.followUp') }}</span><strong>{{ metrics.followUp }}</strong></button>
    </section>

    <section class="filter-panel" :aria-label="tr('support.section.tickets')">
      <label class="field wide"><span>{{ tr('support.field.keyword') }}</span><input :aria-label="tr('support.field.keyword')" v-model.trim="filters.query" data-testid="support-filter-keyword" type="search" @keyup.enter="applyFilters" /></label>
      <label class="field"><span>{{ tr('support.field.stage') }}</span><select :aria-label="tr('support.field.stage')" v-model="filters.stage" data-testid="support-filter-stage"><option value="">{{ tr('support.value.all') }}</option><option v-for="stage in baseStages" :key="stage" :value="stage">{{ translatedCode('support.stage', stage) }}</option></select></label>
      <label class="field"><span>{{ tr('support.field.department') }}</span><select :aria-label="tr('support.field.department')" v-model="filters.departmentNo" data-testid="support-filter-department"><option value="">{{ tr('support.value.all') }}</option><option v-for="item in departments" :key="directoryValue(item)" :value="directoryValue(item)">{{ localizedName(item) }}</option></select></label>
      <label class="field"><span>{{ tr('support.followUp.filter') }}</span><select v-model="filters.followUp" :aria-label="tr('support.followUp.filter')"><option :value="undefined">{{ tr('support.value.all') }}</option><option :value="true">{{ tr('support.followUp.required') }}</option><option :value="false">{{ tr('support.followUp.notRequired') }}</option></select></label>
      <div class="filter-actions">
        <button class="button primary" type="button" @click="applyFilters">{{ tr('support.action.search') }}</button>
        <button class="button secondary" type="button" @click="resetFilters">{{ tr('support.action.reset') }}</button>
        <button class="button ghost" type="button" :aria-expanded="showMoreFilters" @click="showMoreFilters = !showMoreFilters">{{ tr(showMoreFilters ? 'support.action.lessFilters' : 'support.action.moreFilters') }}</button>
      </div>
      <div v-if="showMoreFilters" class="advanced-filters">
      <label class="field"><span>{{ tr('support.field.status') }}</span><select :aria-label="tr('support.field.status')" v-model="filters.status"><option value="">{{ tr('support.value.all') }}</option><option v-for="item in statuses" :key="directoryValue(item)" :value="directoryValue(item)">{{ localizedName(item) }}</option></select></label>
      <label class="field"><span>{{ tr('support.field.priority') }}</span><select :aria-label="tr('support.field.priority')" v-model="filters.priority"><option value="">{{ tr('support.value.all') }}</option><option value="low">{{ tr('support.priority.low') }}</option><option value="normal">{{ tr('support.priority.normal') }}</option><option value="high">{{ tr('support.priority.high') }}</option><option value="urgent">{{ tr('support.priority.urgent') }}</option></select></label>
      </div>
    </section>

    <section class="split-workspace">
      <div class="panel ticket-directory">
        <div class="panel-heading"><div><h2>{{ tr('support.section.tickets') }}</h2></div><span class="count-pill">{{ tickets.length }}</span></div>
        <div v-if="loading" class="empty-state">{{ tr('support.state.loading') }}</div>
        <div v-else-if="tickets.length === 0" class="empty-state">{{ tr('support.empty') }}</div>
        <button v-for="ticket in tickets" v-else :key="ticket.ticketNo" class="ticket-row" :class="{ active: selectedTicketNo === ticket.ticketNo }" :data-testid="`support-ticket-${ticket.ticketNo}`" :disabled="!pageActions.includes('query')" type="button" @click="selectTicket(ticket.ticketNo)">
          <div class="ticket-row-top"><span class="mono">{{ ticket.ticketNo }}</span><span class="stage-badge" :data-stage="ticket.baseStage">{{ customStatusLabel(ticket.customStatusNo) || translatedCode('support.stage', ticket.baseStage) }}</span></div>
          <strong>{{ ticket.subject }}</strong>
          <div class="ticket-meta"><span>{{ categoryLabel(ticket) }}</span><span>{{ translatedCode('support.priority', ticket.priority) }}</span><span v-if="!ticket.slaConfigured" class="risk-dot">{{ tr('support.sla.notConfigured') }}</span><span v-else-if="ticket.slaRisk !== 'none'" class="risk-dot">{{ translatedCode('support.risk', ticket.slaRisk) }}</span></div>
          <time>{{ formatTime(ticket.updatedAt) }}</time>
        </button>
        <footer v-if="nextCursor || cursorHistory.length" class="pagination"><button class="button ghost" :aria-label="tr('support.pagination.previous')" :disabled="cursorHistory.length === 0 || loading" type="button" @click="previousPage">←</button><button class="button ghost" :aria-label="tr('support.pagination.next')" :disabled="!nextCursor || loading" type="button" @click="nextPage">→</button></footer>
      </div>

      <div class="panel ticket-detail">
        <div v-if="detailLoading" class="empty-state">{{ tr('support.state.loading') }}</div>
        <div v-else-if="!selected" class="empty-state">{{ tr('support.state.selectTicket') }}</div>
        <template v-else>
          <div class="detail-heading">
            <div><p class="panel-kicker mono">{{ selected.ticket.ticketNo }}</p><h2>{{ selected.ticket.subject }}</h2></div>
            <div class="detail-badges"><span class="stage-badge">{{ translatedCode('support.stage', selected.ticket.baseStage) }}</span><span v-if="customStatusLabel(selected.ticket.customStatusNo)" class="stage-badge">{{ customStatusLabel(selected.ticket.customStatusNo) }}</span><span class="priority-badge">{{ translatedCode('support.priority', selected.ticket.priority) }}</span></div>
          </div>
          <dl class="fact-grid">
            <div><dt>{{ tr('support.field.customer') }}</dt><dd>{{ selected.ticket.customerDisplayLabel || (selected.ticket.customerNo ? tr('support.value.customerTicket') : tr('support.work.internal')) }}</dd></div>
            <div><dt>{{ tr('support.field.category') }}</dt><dd>{{ categoryLabel(selected.ticket) }}</dd></div>
            <div><dt>{{ tr('support.field.department') }}</dt><dd>{{ departmentLabel(selected.ticket.departmentNo) }}</dd></div>
            <div><dt>{{ tr('support.field.assignee') }}</dt><dd>{{ selected.ticket.assigneeUserId ? tr('support.value.assigned') : tr('support.value.unassigned') }}</dd></div>
            <div><dt>{{ tr('support.field.sla') }}</dt><dd>{{ selected.ticket.slaConfigured ? translatedCode('support.risk', selected.ticket.slaRisk) : tr('support.sla.notConfigured') }}</dd></div>
          </dl>

          <section v-if="selected.ticket.relatedEntitlement || selected.customFields?.length" class="detail-section" data-testid="support-ticket-snapshots">
            <div class="section-title"><h3>{{ tr('support.section.ticketInformation') }}</h3></div>
            <article v-if="selected.ticket.relatedEntitlement" class="sla-card" data-testid="support-related-entitlement">
              <strong>{{ tr('support.field.relatedEntitlement') }}</strong>
              <p>{{ tr('support.entitlement.creationSnapshot') }}：{{ relatedEntitlementName() }} · {{ entitlementStatus(selected.ticket.relatedEntitlement.snapshotStatus) }}</p>
              <p v-if="selected.ticket.relatedEntitlement.currentAvailable">
                {{ tr('support.entitlement.currentProjection') }}：{{ relatedEntitlementName(true) }} · {{ entitlementStatus(selected.ticket.relatedEntitlement.currentStatus) }}
              </p>
              <p v-else>{{ tr('support.entitlement.currentUnavailable') }}</p>
            </article>
            <dl v-if="selected.customFields?.length" class="fact-grid" data-testid="support-custom-field-snapshots">
              <div v-for="field in selected.customFields" :key="field.fieldNo">
                <dt>{{ localizedCustomFieldName(field) }}</dt>
                <dd>{{ customFieldSnapshotValue(field) }}</dd>
                <small v-if="localizedCustomFieldDescription(field)">{{ localizedCustomFieldDescription(field) }}</small>
              </div>
            </dl>
          </section>

          <section class="detail-section">
            <div class="section-title"><h3>{{ tr('support.section.actions') }}</h3></div>
            <div class="action-strip">
              <button v-if="can('claim')" class="button primary" data-testid="support-action-claim" :disabled="writeLoading" type="button" @click="claimTicket">{{ tr('support.action.claim') }}</button>
              <button v-if="can('public_reply')" class="button secondary" data-testid="support-action-public-reply" type="button" @click="openComposer('public-replies')">{{ tr('support.action.publicReply') }}</button>
              <button v-if="can('internal_note')" class="button secondary" data-testid="support-action-internal-note" type="button" @click="openComposer('internal-notes')">{{ tr('support.action.internalNote') }}</button>
              <button v-if="can('transfer')" class="button secondary" data-testid="support-action-transfer" type="button" @click="openComposer('transfers')">{{ tr('support.action.transfer') }}</button>
              <button v-if="can('escalate')" class="button secondary" data-testid="support-action-escalate" type="button" @click="openComposer('escalations')">{{ tr('support.action.markFollowUp') }}</button>
              <button v-if="can('resolve')" class="button secondary" data-testid="support-action-resolve" type="button" @click="openComposer('resolutions')">{{ tr('support.action.resolve') }}</button>
              <button v-if="can('close')" class="button secondary" data-testid="support-action-close" type="button" @click="openComposer('closures')">{{ tr('support.action.close') }}</button>
              <button v-if="can('reopen')" class="button secondary" data-testid="support-action-reopen" type="button" @click="openComposer('reopenings')">{{ tr('support.action.reopen') }}</button>
              <button v-if="can('status_transition') && selected.availableStatusTransitions?.length" class="button secondary" data-testid="support-action-status-transition" type="button" @click="openComposer('status-transitions')">{{ tr('support.action.changeStatus') }}</button>
              <button v-if="can('merge_all')" class="button ghost" data-testid="support-action-global-merge" type="button" @click="openComposer('global-merges')">{{ tr('support.action.mergeGlobal') }}</button>
              <button v-else-if="can('merge')" class="button ghost" data-testid="support-action-merge" type="button" @click="openComposer('merges')">{{ tr('support.action.merge') }}</button>
              <button v-if="can('split')" class="button ghost" data-testid="support-action-split" type="button" @click="openComposer('splits')">{{ tr('support.action.split') }}</button>
              <button v-if="can('upstream')" class="button ghost" data-testid="support-action-upstream" type="button" @click="openComposer('upstream')">{{ tr('support.action.sendUpstream') }}</button>
            </div>
          </section>

          <form v-if="composer.kind" class="composer" data-testid="support-action-composer" @submit.prevent="submitAction">
            <label v-if="['public-replies', 'internal-notes'].includes(composer.kind) && replyPresets.length" class="field"><span>{{ tr('support.field.replyPreset') }}</span><select :aria-label="tr('support.field.replyPreset')" v-model="composer.presetNo" @change="applyPreset"><option value="">{{ tr('support.value.none') }}</option><option v-for="item in replyPresets" :key="item.presetNo" :value="item.presetNo">{{ item.title }}</option></select></label>
            <label v-if="['public-replies', 'internal-notes'].includes(composer.kind)" class="field wide"><span>{{ tr('support.field.body') }}</span><textarea v-model="composer.body" required rows="5" /></label>
            <label v-if="['public-replies', 'internal-notes'].includes(composer.kind)" class="field wide"><span>{{ tr('support.field.attachments') }}</span><input :aria-label="tr('support.field.attachments')" accept=".jpg,.jpeg,.png,.gif,.webp,.pdf,.txt,.log,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.zip" multiple type="file" @change="chooseFiles" /><small>{{ tr('support.field.attachmentsHelp') }}</small></label>
            <label v-if="composer.kind === 'status-transitions'" class="field wide"><span>{{ tr('support.field.statusTarget') }}</span><select :aria-label="tr('support.field.statusTarget')" v-model="composer.customStatusNo" data-testid="support-status-transition-target" required><option value="">{{ tr('support.value.choose') }}</option><option v-for="item in selected.availableStatusTransitions" :key="`${item.statusNo}:${item.version}`" :value="item.statusNo">{{ localizedName(item) }} · {{ translatedCode('support.stage', item.baseStage) }}</option></select><small>{{ tr('support.statusTransition.help') }}</small></label>
            <label v-if="requiresReason(composer.kind) || composer.kind === 'status-transitions'" class="field wide"><span>{{ tr(requiresReason(composer.kind) ? 'support.field.reason' : 'support.field.reasonOptional') }}</span><textarea v-model="composer.reason" :required="requiresReason(composer.kind)" rows="3" /></label>
            <label v-if="composer.kind === 'splits'" class="field"><span>{{ tr('support.target.category') }}</span><select :aria-label="tr('support.target.category')" v-model="composer.categoryNo" required><option value="">{{ tr('support.value.choose') }}</option><option v-for="item in categories" :key="directoryValue(item)" :value="directoryValue(item)">{{ localizedName(item) }}</option></select></label>
            <p v-if="composer.kind === 'transfers'" class="notice info wide">{{ tr('support.transfer.queueHelp') }}</p>
            <p v-if="composer.kind === 'escalations'" class="notice info wide">{{ tr('support.followUp.help') }}</p>
            <label v-if="composer.kind === 'transfers'" class="field"><span>{{ tr('support.field.department') }}</span><select v-model="composer.departmentNo" :aria-label="tr('support.field.department')" required><option value="">{{ tr('support.value.choose') }}</option><option v-for="item in departments" :key="directoryValue(item)" :value="directoryValue(item)">{{ localizedName(item) }}</option></select></label>
            <fieldset v-if="['global-merges', 'merges'].includes(composer.kind)" class="selection-field wide" data-testid="support-merge-candidates"><legend>{{ tr(composer.kind === 'global-merges' ? 'support.merge.globalCandidates' : 'support.merge.candidates') }}</legend><p class="field-help">{{ tr(composer.kind === 'global-merges' ? 'support.merge.globalHelp' : 'support.merge.help') }}</p><p v-if="mergeCandidatesLoading && !mergeCandidates.length">{{ tr('support.state.loading') }}</p><p v-else-if="mergeCandidates.length === 0">{{ tr('support.merge.empty') }}</p><label v-for="candidate in mergeCandidates" v-else :key="candidate.ticketNo" class="selection-option"><input v-model="composer.selectedMergeTicketNos" :data-testid="`support-merge-candidate-${candidate.ticketNo}`" :disabled="composer.selectedMergeTicketNos.length >= 20 && !composer.selectedMergeTicketNos.includes(candidate.ticketNo)" :value="candidate.ticketNo" type="checkbox" /><span><strong>{{ candidate.subject }}</strong><small>{{ candidate.ticketNo }} · {{ translatedCode('support.stage', candidate.stage) }}</small></span></label><small v-if="mergeCandidates.length">{{ tr('support.merge.limit') }}</small><button v-if="mergeCandidatesNextCursor" class="button secondary" :disabled="mergeCandidatesLoading" type="button" @click="loadMergeCandidates(true)">{{ tr('support.merge.more') }}</button></fieldset>
            <fieldset v-if="['splits', 'upstream'].includes(composer.kind) && !upstreamPreview" class="selection-field wide"><legend>{{ tr('support.selection.messages') }}</legend><p v-if="selected.publicMessages.filter((item) => !item.hidden).length === 0">{{ tr('support.selection.emptyMessages') }}</p><label v-for="message in selected.publicMessages.filter((item) => !item.hidden)" v-else :key="message.messageNo" class="selection-option"><input v-model="composer.selectedMessageNos" :data-testid="`support-selection-message-${message.messageNo}`" :value="message.messageNo" type="checkbox" /><span><strong>{{ messageAuthor(message) }}</strong><small>{{ message.body }}</small></span></label></fieldset>
            <fieldset v-if="['splits', 'upstream'].includes(composer.kind) && !upstreamPreview" class="selection-field wide"><legend>{{ tr('support.selection.attachments') }}</legend><p v-if="selectableAttachments.length === 0">{{ tr('support.selection.emptyAttachments') }}</p><label v-for="attachment in selectableAttachments" v-else :key="attachment.attachmentNo" class="selection-option"><input v-model="composer.selectedAttachmentNos" :data-testid="`support-selection-attachment-${attachment.attachmentNo}`" :value="attachment.attachmentNo" type="checkbox" /><span><strong>{{ attachment.fileName }}</strong><small>{{ formatBytes(attachment.sizeBytes) }} · {{ fileTypeLabel(attachment.fileName) }}</small></span></label></fieldset>
            <article v-if="composer.kind === 'upstream' && upstreamPreview" class="notice info" data-testid="support-upstream-preview"><strong>{{ upstreamPreview.title }}</strong><p class="preview-content">{{ upstreamPreview.content }}</p><small>{{ tr('support.upstream.previewLocked') }}</small></article>
            <div class="composer-actions">
              <button class="button primary" data-testid="support-action-submit" :disabled="writeLoading || (composer.kind === 'status-transitions' && !composer.customStatusNo) || (['global-merges', 'merges'].includes(composer.kind) && composer.selectedMergeTicketNos.length === 0) || (['splits', 'upstream'].includes(composer.kind) && !upstreamPreview && composer.selectedMessageNos.length === 0 && composer.selectedAttachmentNos.length === 0)" type="submit">{{ composer.kind === 'upstream' ? (upstreamPreview ? tr('support.upstream.confirmButton') : tr('support.action.preview')) : (writeLoading && composer.files.length ? tr('support.state.uploading') : tr('support.action.submit')) }}</button>
              <button class="button ghost" type="button" @click="composer.kind = ''">{{ tr('support.action.cancel') }}</button>
            </div>
          </form>

          <section v-if="can('upstream') && upstreamForwards.length" class="detail-section" data-testid="support-upstream-forwards">
            <div class="section-title"><h3>{{ tr('support.upstream.history') }}</h3><span>{{ upstreamForwards.length }}</span></div>
            <div class="sla-grid">
              <article v-for="forward in upstreamForwards" :key="forward.forwardNo" class="sla-card">
                <strong>{{ forward.providerTicketReference || forward.forwardNo }}</strong>
                <span>{{ translatedCode('support.upstream.status', forward.status) }}</span>
                <p>{{ translatedCode('support.upstream.result', forward.resultClassification) }}</p>
                <time>{{ formatTime(forward.updatedAt) }}</time>
                <div class="action-strip">
                  <button v-if="forward.status === 'succeeded'" class="button secondary" :disabled="writeLoading" type="button" @click="syncUpstreamReplies(forward)">{{ tr('support.upstream.syncReplies') }}</button>
                  <button v-if="forward.retryEligible" class="button ghost" :disabled="writeLoading" type="button" @click="openUpstreamRetry(forward)">{{ tr('support.upstream.retry') }}</button>
                </div>
              </article>
            </div>
          </section>

          <section class="detail-section">
            <div class="section-title"><h3>{{ tr('support.section.activity') }}</h3><span>{{ selected.publicMessages.length + selected.internalNotes.length }}</span></div>
            <article v-for="message in selected.publicMessages" :key="message.messageNo" class="timeline-entry public-entry">
              <div class="entry-marker"></div>
              <div>
                <div class="entry-top"><strong>{{ messageAuthor(message) }}</strong><time>{{ formatTime(message.createdAt) }}</time></div>
                <p>{{ message.hidden ? tr('support.state.hidden') : message.body }}</p>
                <div v-if="visibleAttachments(message).length" class="preview-list"><ul><li v-for="attachment in visibleAttachments(message)" :key="attachment.attachmentNo">{{ attachment.fileName }} · {{ formatBytes(attachment.sizeBytes) }} <button class="button ghost" type="button" @click="downloadAttachment(attachment)">{{ tr('support.action.download') }}</button><button v-if="canHideAttachment(attachment)" class="button ghost" type="button" @click="openHide('attachment', attachment.attachmentNo)">{{ tr('support.action.hide') }}</button></li></ul></div>
                <div class="action-strip"><span class="visibility-pill">{{ tr('support.visibility.public') }}</span><button v-if="can('hide') && !message.hidden" class="button ghost" type="button" @click="openHide('message', message.messageNo)">{{ tr('support.action.hide') }}</button></div>
              </div>
            </article>
            <article v-for="note in selected.internalNotes" :key="note.messageNo" class="timeline-entry internal-entry">
              <div class="entry-marker"></div>
              <div>
                <div class="entry-top"><strong>{{ messageAuthor(note) }}</strong><time>{{ formatTime(note.createdAt) }}</time></div>
                <p>{{ note.hidden ? tr('support.state.hidden') : note.body }}</p>
                <div v-if="visibleAttachments(note).length" class="preview-list"><ul><li v-for="attachment in visibleAttachments(note)" :key="attachment.attachmentNo">{{ attachment.fileName }} · {{ formatBytes(attachment.sizeBytes) }} <button class="button ghost" type="button" @click="downloadAttachment(attachment)">{{ tr('support.action.download') }}</button><button v-if="canHideAttachment(attachment)" class="button ghost" type="button" @click="openHide('attachment', attachment.attachmentNo)">{{ tr('support.action.hide') }}</button></li></ul></div>
                <div class="action-strip"><span class="visibility-pill">{{ tr('support.visibility.internal') }}</span><button v-if="note.type === 'upstream_reply' && can('upstream') && can('public_reply') && !note.hidden" class="button secondary" type="button" @click="openUpstreamPublication(note.messageNo)">{{ tr('support.upstream.publishReply') }}</button><button v-if="can('hide') && !note.hidden" class="button ghost" type="button" @click="openHide('message', note.messageNo)">{{ tr('support.action.hide') }}</button></div>
              </div>
            </article>
          </section>

          <section class="detail-section">
            <div class="section-title"><h3>{{ tr('support.section.sla') }}</h3></div>
            <p v-if="!selected.ticket.slaConfigured" class="empty-state">{{ tr('support.sla.unconfigured') }}</p>
            <div v-if="selected.ticket.slaConfigured" class="sla-grid"><article v-for="round in selected.slaRounds" :key="round.round" class="sla-card"><strong>#{{ round.round }}</strong><dl><div><dt>{{ tr('support.sla.firstResponse') }}</dt><dd>{{ formatTime(round.firstResponseDueAt) }}</dd></div><div><dt>{{ tr('support.sla.resolution') }}</dt><dd>{{ formatTime(round.resolutionDueAt) }}</dd></div></dl><span class="risk-dot">{{ round.firstResponseBreached || round.resolutionBreached ? tr('support.risk.breached') : tr('support.risk.none') }}</span></article></div>
          </section>

          <section v-if="selected.assignmentHistory.length || selected.relations.length || selected.escalations.length || selected.statusHistory.length" class="detail-section">
            <div class="section-title"><h3>{{ tr('support.section.history') }}</h3></div>
            <div class="sla-grid">
              <article v-for="item in selected.assignmentHistory" :key="`${item.action}-${item.occurredAt}`" class="sla-card"><strong>{{ translatedCode('support.assignment', item.action) }}</strong><span>{{ departmentLabel(item.fromDepartmentNo) }} → {{ departmentLabel(item.toDepartmentNo) }}</span><p v-if="item.reason">{{ item.reason }}</p><time>{{ formatTime(item.occurredAt) }}</time></article>
              <article v-for="item in selected.relations" :key="`${item.type}-${item.createdAt}`" class="sla-card"><strong>{{ translatedCode('support.relation', item.type) }}</strong><span>{{ counterpartTicket(item.sourceTicketNo, item.targetTicketNo) }}</span><p v-if="item.reason">{{ item.reason }}</p><time>{{ formatTime(item.createdAt) }}</time></article>
              <article v-for="item in selected.escalations" :key="item.createdAt" class="sla-card"><strong>{{ tr('support.action.escalate') }}</strong><p>{{ item.reason }}</p><time>{{ formatTime(item.createdAt) }}</time></article>
              <article v-for="item in selected.statusHistory" :key="`${item.toStage}-${item.occurredAt}`" class="sla-card"><strong>{{ tr('support.history.status') }}</strong><span>{{ item.fromStage ? translatedCode('support.stage', item.fromStage) : tr('support.value.none') }} → {{ translatedCode('support.stage', item.toStage) }}</span><span v-if="item.customStatusNo">{{ customStatusLabel(item.customStatusNo) || tr('support.value.restricted') }}</span><p v-if="item.reason">{{ item.reason }}</p><time>{{ formatTime(item.occurredAt) }}</time></article>
            </div>
          </section>
        </template>
      </div>
    </section>

    <div v-if="createOpen" class="modal-backdrop" role="presentation" @click.self="closeCreate" @keydown.esc="closeCreate">
      <form aria-labelledby="support-create-title" aria-modal="true" class="modal-card" data-testid="support-create-modal" role="dialog" @submit.prevent="createTicket">
        <div class="modal-heading"><div><p class="panel-kicker">{{ tr('support.work.internal') }}</p><h2 id="support-create-title">{{ tr('support.action.create') }}</h2></div><button class="icon-button" :aria-label="tr('support.action.cancel')" type="button" @click="closeCreate">×</button></div>
        <label class="field wide"><span>{{ tr('support.field.subject') }}</span><input :aria-label="tr('support.field.subject')" ref="createSubjectInput" v-model.trim="createForm.subject" required maxlength="180" /></label>
        <label class="field wide"><span>{{ tr('support.field.body') }}</span><textarea v-model="createForm.body" required rows="7" /></label>
        <label class="field wide"><span>{{ tr('support.field.attachments') }}</span><input :aria-label="tr('support.field.attachments')" accept=".jpg,.jpeg,.png,.gif,.webp,.pdf,.txt,.log,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.zip" multiple type="file" @change="chooseCreateFiles" /><small>{{ tr('support.field.internalAttachmentsHelp') }}</small></label>
        <div class="form-grid"><label class="field"><span>{{ tr('support.field.category') }}</span><select :aria-label="tr('support.field.category')" v-model="createForm.categoryNo" required><option value="">{{ tr('support.value.choose') }}</option><option v-for="item in categories" :key="directoryValue(item)" :value="directoryValue(item)">{{ localizedName(item) }}</option></select></label><label class="field"><span>{{ tr('support.field.department') }}</span><select :aria-label="tr('support.field.department')" v-model="createForm.departmentNo" required><option value="">{{ tr('support.value.choose') }}</option><option v-for="item in departments" :key="directoryValue(item)" :value="directoryValue(item)">{{ localizedName(item) }}</option></select></label></div>
        <label class="field"><span>{{ tr('support.field.priority') }}</span><select :aria-label="tr('support.field.priority')" v-model="createForm.priority"><option value="low">{{ tr('support.priority.low') }}</option><option value="normal">{{ tr('support.priority.normal') }}</option><option value="high">{{ tr('support.priority.high') }}</option><option value="urgent">{{ tr('support.priority.urgent') }}</option></select></label>
        <section class="selection-field wide" data-testid="support-internal-custom-fields">
          <h3>{{ tr('support.field.additionalInformation') }}</h3>
          <p v-if="createFieldsLoading">{{ tr('support.state.loading') }}</p>
          <div v-else-if="createFieldsFailed" class="notice error" role="alert">{{ tr('support.error.customFieldsLoad') }}</div>
          <p v-else-if="internalCustomFields.length === 0">{{ tr('support.field.noAdditionalInformation') }}</p>
          <div v-for="field in internalCustomFields" v-else :key="field.fieldNo" class="field wide" :data-testid="`support-internal-field-${field.fieldNo}`">
            <label :for="`support-internal-input-${field.fieldNo}`">{{ localizedCustomFieldName(field) }}<template v-if="field.required"> *</template></label>
            <small v-if="localizedCustomFieldDescription(field)">{{ localizedCustomFieldDescription(field) }}</small>
            <textarea v-if="field.fieldType === 'multi_line'" :id="`support-internal-input-${field.fieldNo}`" v-model="createCustomFieldValues[field.fieldNo]" :maxlength="65536" :required="field.required" rows="4" />
            <input v-else-if="field.fieldType === 'single_line'" :id="`support-internal-input-${field.fieldNo}`" v-model="createCustomFieldValues[field.fieldNo]" :maxlength="1024" :required="field.required" />
            <input v-else-if="field.fieldType === 'number'" :id="`support-internal-input-${field.fieldNo}`" v-model="createCustomFieldValues[field.fieldNo]" inputmode="decimal" pattern="-?[0-9]{1,20}(\.[0-9]{1,10})?" :required="field.required" />
            <input v-else-if="field.fieldType === 'date'" :id="`support-internal-input-${field.fieldNo}`" v-model="createCustomFieldValues[field.fieldNo]" :required="field.required" type="date" />
            <select v-else-if="field.fieldType === 'boolean'" :id="`support-internal-input-${field.fieldNo}`" v-model="createCustomFieldValues[field.fieldNo]" :required="field.required"><option value="">{{ tr('support.value.choose') }}</option><option value="true">{{ tr('support.value.yes') }}</option><option value="false">{{ tr('support.value.no') }}</option></select>
            <select v-else-if="field.fieldType === 'single_select'" :id="`support-internal-input-${field.fieldNo}`" v-model="createCustomFieldValues[field.fieldNo]" :required="field.required"><option value="">{{ tr('support.value.choose') }}</option><option v-for="option in field.options.filter((item) => item.status === 'active')" :key="option.optionNo" :value="option.optionNo">{{ localizedCustomFieldName(option) }}</option></select>
            <select v-else :id="`support-internal-input-${field.fieldNo}`" v-model="createCustomFieldValues[field.fieldNo]" multiple :required="field.required"><option v-for="option in field.options.filter((item) => item.status === 'active')" :key="option.optionNo" :value="option.optionNo">{{ localizedCustomFieldName(option) }}</option></select>
          </div>
        </section>
        <footer class="modal-actions"><button class="button ghost" type="button" @click="closeCreate">{{ tr('support.action.cancel') }}</button><button class="button primary" :disabled="writeLoading || createFieldsLoading || createFieldsFailed" type="submit">{{ tr('support.action.submit') }}</button></footer>
      </form>
    </div>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';

import type { PluginHostMountContext } from '../../src/host-context';

import { createTranslator } from '../../src/runtime/i18n';
import { createSupportAPI } from '../../src/support/api';
import type {
  SupportCustomFieldDefinition,
  SupportCustomFieldOptionInput,
  SupportCustomFieldType,
  SupportCustomFieldVisibility,
  SupportCustomStatusSetting,
  SupportDepartmentSetting,
  SupportIssueCategorySetting,
  SupportReplyPresetSetting,
  SupportSLACalendarSetting,
  SupportSLAHolidaySetting,
  SupportSLARuleSetting,
} from '../../src/support/api';
import SupportDepartmentForm from './support-department-form.vue';
import { supportMessages } from '../../src/support/messages';

type SettingSection = 'automation' | 'fields' | 'routing' | 'sla' | 'upstream' | 'writing';
type AssignmentMode = SupportDepartmentSetting['assignmentMode'];
type BaseStage = SupportCustomStatusSetting['baseStage'];
type Priority = SupportIssueCategorySetting['priority'];
type SettingStatus = SupportDepartmentSetting['status'];

const props = defineProps<{ context: PluginHostMountContext }>();
const api = createSupportAPI(() => props.context);
const tr = createTranslator(() => props.context, supportMessages);

const activeSection = ref<SettingSection>('routing');
const loading = ref(true);
const saving = ref(false);
const error = ref('');
const success = ref('');

const departments = ref<SupportDepartmentSetting[]>([]);
const categories = ref<SupportIssueCategorySetting[]>([]);
const statuses = ref<SupportCustomStatusSetting[]>([]);
const presets = ref<SupportReplyPresetSetting[]>([]);
const calendars = ref<SupportSLACalendarSetting[]>([]);
const holidays = ref<SupportSLAHolidaySetting[]>([]);
const rules = ref<SupportSLARuleSetting[]>([]);
const customFields = ref<SupportCustomFieldDefinition[]>([]);
const upstreamChannel = ref<Awaited<ReturnType<typeof api.upstreamChannel>> | null>(null);

const policyForm = reactive({ autoCloseDays: 3, reopenDays: 7, version: 0 });
const departmentForm = reactive({
  assignmentMode: 'manual_claim' as AssignmentMode,
  departmentNo: '',
  nameEnUS: '',
  nameZhCN: '',
  orgDeptId: 0,
  orgDeptName: '',
  parentDepartmentNo: '',
  status: 'active' as SettingStatus,
  version: 0,
});
const categoryForm = reactive({
  categoryNo: '',
  departmentNo: '',
  nameEnUS: '',
  nameZhCN: '',
  priority: 'normal' as Priority,
  sortOrder: 0,
  status: 'active' as SettingStatus,
  version: 0,
});
const statusForm = reactive({
  baseStage: 'in_progress' as BaseStage,
  nameEnUS: '',
  nameZhCN: '',
  sortOrder: 0,
  status: 'active' as SettingStatus,
  statusNo: '',
  version: 0,
});
const presetForm = reactive({
  body: '',
  departmentNo: '',
  locale: 'zh-CN' as SupportReplyPresetSetting['locale'],
  presetNo: '',
  sortOrder: 0,
  status: 'active' as SettingStatus,
  title: '',
  version: 0,
});
const calendarForm = reactive({
  calendarNo: '',
  mode: 'continuous' as SupportSLACalendarSetting['mode'],
  name: '',
  status: 'active' as SettingStatus,
  timezone: 'Asia/Shanghai',
  version: 0,
  workdays: [1, 2, 3, 4, 5] as number[],
  workEnd: '18:00',
  workStart: '09:00',
});
const selectedCalendarNo = ref('');
const holidayForm = reactive({ date: '', name: '' });
const ruleForm = reactive({
  calendarNo: '',
  categoryNo: '',
  departmentNo: '',
  firstResponseMinutes: '' as number | '',
  nearDueMinutes: '',
  priority: 'normal' as Priority,
  resolutionMinutes: '' as number | '',
  ruleNo: '',
  status: 'active' as SettingStatus,
  version: 0,
});
const customFieldForm = reactive({
  appliesCustomer: true,
  appliesInternal: false,
  appliesPresales: false,
  descriptionEnUS: '',
  descriptionZhCN: '',
  fieldNo: '',
  fieldType: 'single_line' as SupportCustomFieldType,
  nameEnUS: '',
  nameZhCN: '',
  options: [] as SupportCustomFieldOptionInput[],
  required: false,
  sortOrder: 0,
  status: 'active' as SettingStatus,
  version: 0,
  visibility: 'customer_visible' as SupportCustomFieldVisibility,
});
const upstreamChannelForm = reactive({
  authMode: 'zjmf_api_login' as 'resource_login' | 'zjmf_api_login',
  baseUrl: '',
  password: '',
  reason: '',
  status: 'enabled' as 'disabled' | 'enabled',
  username: '',
});
const upstreamDepartmentForm = reactive({
  departmentNo: '',
  expectedVersion: 0,
  status: 'active' as 'active' | 'disabled',
  upstreamDepartmentId: 0,
  upstreamDepartmentName: '',
});
const upstreamEntitlementForm = reactive({
  entitlementNo: '',
  expectedVersion: 0,
  status: 'active' as 'active' | 'disabled',
  upstreamReference: '',
});

function operationNo(prefix: string) {
  return `${prefix}_${globalThis.crypto.randomUUID().replaceAll('-', '')}`;
}

const activeDepartments = computed(() =>
  departments.value.filter((item) => item.status === 'active'),
);
const activeCategories = computed(() =>
  categories.value.filter((item) => item.status === 'active'),
);
const activeCalendars = computed(() =>
  calendars.value.filter((item) => item.status === 'active'),
);
const businessCalendars = computed(() =>
  calendars.value.filter(
    (item) => item.status === 'active' && item.mode === 'business_hours',
  ),
);

function displayName(item: { nameEnUS: string; nameZhCN: string }) {
  return props.context.locale.toLowerCase().startsWith('zh')
    ? item.nameZhCN
    : item.nameEnUS;
}

function departmentName(departmentNo?: string) {
  if (!departmentNo) return tr('support.settings.publicQueue');
  const item = departments.value.find(
    (department) => department.departmentNo === departmentNo,
  );
  return item ? displayName(item) : tr('support.value.restricted');
}

function categoryName(categoryNo?: string) {
  if (!categoryNo) return tr('support.settings.departmentDefault');
  const item = categories.value.find(
    (category) => category.categoryNo === categoryNo,
  );
  return item ? displayName(item) : tr('support.value.restricted');
}

function calendarName(calendarNo: string) {
  return (
    calendars.value.find((calendar) => calendar.calendarNo === calendarNo)
      ?.name ?? tr('support.value.restricted')
  );
}

function minuteToTime(value?: number) {
  if (value === undefined) return '';
  return `${String(Math.floor(value / 60)).padStart(2, '0')}:${String(value % 60).padStart(2, '0')}`;
}

function timeToMinute(value: string) {
  const [hours = '0', minutes = '0'] = value.split(':');
  return Number(hours) * 60 + Number(minutes);
}

function optionalMinutes(value: number | '') {
  return value === '' ? undefined : Number(value);
}

function parseNearDueMinutes() {
  return [...new Set(
    ruleForm.nearDueMinutes
      .split(/[，,\s]+/u)
      .filter(Boolean)
      .map(Number)
      .filter((value) => Number.isInteger(value) && value > 0),
  )].sort((left, right) => right - left);
}

function isSelectField(type = customFieldForm.fieldType) {
  return type === 'single_select' || type === 'multi_select';
}

function normalizeFieldVisibility() {
  const internal = customFieldForm.visibility === 'internal_only';
  customFieldForm.appliesInternal = internal;
  if (internal) {
    customFieldForm.appliesCustomer = false;
    customFieldForm.appliesPresales = false;
  }
}

function resetCustomField() {
  Object.assign(customFieldForm, {
    appliesCustomer: true,
    appliesInternal: false,
    appliesPresales: false,
    descriptionEnUS: '',
    descriptionZhCN: '',
    fieldNo: '',
    fieldType: 'single_line',
    nameEnUS: '',
    nameZhCN: '',
    options: [],
    required: false,
    sortOrder: 0,
    status: 'active',
    version: 0,
    visibility: 'customer_visible',
  });
}

function editCustomField(item: SupportCustomFieldDefinition) {
  Object.assign(customFieldForm, {
    ...item,
    descriptionEnUS: item.descriptionEnUS ?? '',
    descriptionZhCN: item.descriptionZhCN ?? '',
    options: item.options.map((option) => ({
      expectedVersion: option.version,
      nameEnUS: option.nameEnUS,
      nameZhCN: option.nameZhCN,
      optionNo: option.optionNo,
      sortOrder: option.sortOrder,
      status: option.status,
    })),
  });
}

function addCustomFieldOption() {
  customFieldForm.options.push({
    expectedVersion: 0,
    nameEnUS: '',
    nameZhCN: '',
    optionNo: '',
    sortOrder: customFieldForm.options.length * 10,
    status: 'active',
  });
}

function removeNewCustomFieldOption(index: number) {
  if (!customFieldForm.options[index]?.optionNo) {
    customFieldForm.options.splice(index, 1);
  }
}

function clearFeedback() {
  error.value = '';
  success.value = '';
}

async function loadAll() {
  loading.value = true;
  clearFeedback();
  try {
    const [
      policy,
      departmentItems,
      categoryItems,
      statusItems,
      presetItems,
      calendarItems,
      ruleItems,
      customFieldItems,
      channel,
    ] = await Promise.all([
      api.automationPolicy(),
      api.departmentSettings(),
      api.issueCategorySettings(),
      api.customStatusSettings(),
      api.replyPresetSettings(),
      api.slaCalendarSettings(),
      api.slaRuleSettings(),
      api.customFieldSettings(),
      api.upstreamChannel(),
    ]);
    Object.assign(policyForm, policy);
    departments.value = departmentItems;
    categories.value = categoryItems;
    statuses.value = statusItems;
    presets.value = presetItems;
    calendars.value = calendarItems;
    rules.value = ruleItems;
    customFields.value = customFieldItems;
    upstreamChannel.value = channel;
    if (channel.configured) {
      Object.assign(upstreamChannelForm, {
        authMode: channel.authMode,
        baseUrl: channel.baseUrl,
        password: '',
        reason: '',
        status: channel.status,
        username: '',
      });
    }
    const selectedStillExists = calendarItems.some(
      (item) =>
        item.calendarNo === selectedCalendarNo.value &&
        item.mode === 'business_hours' &&
        item.status === 'active',
    );
    selectedCalendarNo.value = selectedStillExists
      ? selectedCalendarNo.value
      : (calendarItems.find(
          (item) => item.mode === 'business_hours' && item.status === 'active',
        )?.calendarNo ?? '');
    await loadHolidays();
  } catch {
    error.value = tr('support.error.load');
  } finally {
    loading.value = false;
  }
}

async function runWrite(
  operation: () => Promise<unknown>,
  savedKey = 'support.settings.saved',
  errorKey = 'support.error.write',
) {
  saving.value = true;
  clearFeedback();
  try {
    await operation();
    await loadAll();
    success.value = tr(savedKey);
    return true;
  } catch {
    error.value = tr(errorKey);
    return false;
  } finally {
    saving.value = false;
  }
}

async function saveCustomField() {
  normalizeFieldVisibility();
  if (
    customFieldForm.visibility === 'customer_visible' &&
    !customFieldForm.appliesCustomer &&
    !customFieldForm.appliesPresales
  ) {
    error.value = tr('support.settings.fieldAppliesRequired');
    return;
  }
  const options = isSelectField()
    ? customFieldForm.options.map((option) => ({
        ...option,
        nameEnUS: option.nameEnUS.trim(),
        nameZhCN: option.nameZhCN.trim(),
        sortOrder: Number(option.sortOrder),
      }))
    : [];
  if (isSelectField() && !options.some((option) => option.status === 'active')) {
    error.value = tr('support.settings.fieldOptionRequired');
    return;
  }
  const body = {
    appliesCustomer: customFieldForm.appliesCustomer,
    appliesInternal: customFieldForm.appliesInternal,
    appliesPresales: customFieldForm.appliesPresales,
    descriptionEnUS: customFieldForm.descriptionEnUS.trim(),
    descriptionZhCN: customFieldForm.descriptionZhCN.trim(),
    fieldType: customFieldForm.fieldType,
    nameEnUS: customFieldForm.nameEnUS.trim(),
    nameZhCN: customFieldForm.nameZhCN.trim(),
    options,
    required: customFieldForm.required,
    sortOrder: Number(customFieldForm.sortOrder),
    status: customFieldForm.status,
    visibility: customFieldForm.visibility,
  };
  const saved = await runWrite(
    () =>
      customFieldForm.fieldNo
        ? api.updateCustomField(customFieldForm.fieldNo, {
            ...body,
            expectedVersion: customFieldForm.version,
          })
        : api.createCustomField(body),
    'support.settings.fieldSaved',
    'support.settings.fieldSaveConflict',
  );
  if (saved) resetCustomField();
}

async function disableCustomField(item: SupportCustomFieldDefinition) {
  if (!globalThis.confirm(tr('support.settings.disableFieldConfirm'))) return;
  await runWrite(
    () => api.disableCustomField(item.fieldNo, item.version),
    'support.settings.fieldDisabled',
    'support.settings.fieldSaveConflict',
  );
}

async function savePolicy() {
  await runWrite(async () => {
    Object.assign(
      policyForm,
      await api.updateAutomationPolicy({
        autoCloseDays: Number(policyForm.autoCloseDays),
        expectedVersion: policyForm.version,
        reopenDays: Number(policyForm.reopenDays),
      }),
    );
  });
}

async function saveUpstreamChannel() {
  await runWrite(
    () =>
      api.configureUpstreamChannel({
        ...upstreamChannelForm,
        expectedVersion: upstreamChannel.value?.version ?? 0,
        operationNo: operationNo('UPC'),
      }),
    'support.upstream.channelSaved',
    'support.upstream.reloginError',
  );
}

async function saveUpstreamDepartmentMap() {
  await runWrite(async () => {
    const item = await api.saveUpstreamDepartmentMap(upstreamDepartmentForm.departmentNo, {
      expectedVersion: upstreamDepartmentForm.expectedVersion,
      status: upstreamDepartmentForm.status,
      upstreamDepartmentId: Number(upstreamDepartmentForm.upstreamDepartmentId),
      upstreamDepartmentName: upstreamDepartmentForm.upstreamDepartmentName,
    });
    upstreamDepartmentForm.expectedVersion = item.version;
  });
}

async function saveUpstreamEntitlementMap() {
  await runWrite(async () => {
    const item = await api.saveUpstreamEntitlementMap(upstreamEntitlementForm.entitlementNo, {
      expectedVersion: upstreamEntitlementForm.expectedVersion,
      status: upstreamEntitlementForm.status,
      upstreamReference: upstreamEntitlementForm.upstreamReference,
    });
    upstreamEntitlementForm.expectedVersion = item.version;
  });
}

async function loadUpstreamDepartmentMap() {
  const departmentNo = upstreamDepartmentForm.departmentNo;
  if (!departmentNo) return;
  const item = await api.upstreamDepartmentMap(departmentNo);
  Object.assign(upstreamDepartmentForm, item ?? {
    departmentNo, expectedVersion: 0, status: 'active',
    upstreamDepartmentId: 0, upstreamDepartmentName: '',
  }, item ? { expectedVersion: item.version } : {});
}

async function loadUpstreamEntitlementMap() {
  const entitlementNo = upstreamEntitlementForm.entitlementNo.trim();
  if (!entitlementNo) return;
  const item = await api.upstreamEntitlementMap(entitlementNo);
  Object.assign(upstreamEntitlementForm, item ?? {
    entitlementNo, expectedVersion: 0, status: 'active', upstreamReference: '',
  }, item ? { expectedVersion: item.version } : {});
}

function resetDepartment() {
  Object.assign(departmentForm, {
    assignmentMode: 'manual_claim', departmentNo: '', nameEnUS: '',
    nameZhCN: '', orgDeptId: 0, orgDeptName: '', parentDepartmentNo: '', status: 'active', version: 0,
  });
}

function editDepartment(item: SupportDepartmentSetting) {
  Object.assign(departmentForm, item, {
    parentDepartmentNo: item.parentDepartmentNo ?? '',
  });
}

async function saveDepartment(body: import('../../src/support/api').SupportDepartmentSettingInput) {
  const saved = await runWrite(() =>
    departmentForm.departmentNo
      ? api.updateDepartment(departmentForm.departmentNo, {
          ...body,
          expectedVersion: departmentForm.version,
        })
      : api.createDepartment(body),
  );
  if (saved) resetDepartment();
}

async function disableDepartment(item: SupportDepartmentSetting) {
  if (!globalThis.confirm(tr('support.settings.disableConfirm'))) return;
  await runWrite(() => api.disableDepartment(item.departmentNo, item.version));
}

function resetCategory() {
  Object.assign(categoryForm, {
    categoryNo: '', departmentNo: '', nameEnUS: '', nameZhCN: '',
    priority: 'normal', sortOrder: 0, status: 'active', version: 0,
  });
}

function editCategory(item: SupportIssueCategorySetting) {
  Object.assign(categoryForm, item, { departmentNo: item.departmentNo ?? '' });
}

async function saveCategory() {
  const body = {
    departmentNo: categoryForm.departmentNo,
    nameEnUS: categoryForm.nameEnUS.trim(),
    nameZhCN: categoryForm.nameZhCN.trim(),
    priority: categoryForm.priority,
    sortOrder: Number(categoryForm.sortOrder),
    status: categoryForm.status,
  };
  const saved = await runWrite(() =>
    categoryForm.categoryNo
      ? api.updateIssueCategory(categoryForm.categoryNo, {
          ...body,
          expectedVersion: categoryForm.version,
        })
      : api.createIssueCategory(body),
  );
  if (saved) resetCategory();
}

async function disableCategory(item: SupportIssueCategorySetting) {
  if (!globalThis.confirm(tr('support.settings.disableConfirm'))) return;
  await runWrite(() => api.disableIssueCategory(item.categoryNo, item.version));
}

function resetStatus() {
  Object.assign(statusForm, {
    baseStage: 'in_progress', nameEnUS: '', nameZhCN: '', sortOrder: 0,
    status: 'active', statusNo: '', version: 0,
  });
}

function editStatus(item: SupportCustomStatusSetting) {
  Object.assign(statusForm, item);
}

async function saveStatus() {
  const body = {
    baseStage: statusForm.baseStage,
    nameEnUS: statusForm.nameEnUS.trim(),
    nameZhCN: statusForm.nameZhCN.trim(),
    sortOrder: Number(statusForm.sortOrder),
    status: statusForm.status,
  };
  const saved = await runWrite(() =>
    statusForm.statusNo
      ? api.updateCustomStatus(statusForm.statusNo, {
          ...body,
          expectedVersion: statusForm.version,
        })
      : api.createCustomStatus(body),
  );
  if (saved) resetStatus();
}

async function disableStatus(item: SupportCustomStatusSetting) {
  if (!globalThis.confirm(tr('support.settings.disableConfirm'))) return;
  await runWrite(() => api.disableCustomStatus(item.statusNo, item.version));
}

function resetPreset() {
  Object.assign(presetForm, {
    body: '', departmentNo: activeDepartments.value[0]?.departmentNo ?? '',
    locale: 'zh-CN', presetNo: '', sortOrder: 0, status: 'active', title: '', version: 0,
  });
}

function editPreset(item: SupportReplyPresetSetting) {
  Object.assign(presetForm, item);
}

async function savePreset() {
  const body = {
    body: presetForm.body.trim(),
    departmentNo: presetForm.departmentNo,
    locale: presetForm.locale,
    sortOrder: Number(presetForm.sortOrder),
    status: presetForm.status,
    title: presetForm.title.trim(),
  };
  const saved = await runWrite(() =>
    presetForm.presetNo
      ? api.updateReplyPreset(presetForm.presetNo, {
          ...body,
          expectedVersion: presetForm.version,
        })
      : api.createReplyPreset(body),
  );
  if (saved) resetPreset();
}

async function disablePreset(item: SupportReplyPresetSetting) {
  if (!globalThis.confirm(tr('support.settings.disableConfirm'))) return;
  await runWrite(() => api.disableReplyPreset(item.presetNo, item.version));
}

function resetCalendar() {
  Object.assign(calendarForm, {
    calendarNo: '', mode: 'continuous', name: '', status: 'active',
    timezone: 'Asia/Shanghai', version: 0, workdays: [1, 2, 3, 4, 5],
    workEnd: '18:00', workStart: '09:00',
  });
}

function editCalendar(item: SupportSLACalendarSetting) {
  Object.assign(calendarForm, item, {
    timezone: item.timezone ?? 'Asia/Shanghai',
    workEnd: minuteToTime(item.workEndMinute) || '18:00',
    workStart: minuteToTime(item.workStartMinute) || '09:00',
    workdays: [...item.workdays],
  });
}

async function saveCalendar() {
  const businessHours = calendarForm.mode === 'business_hours';
  const body = {
    mode: calendarForm.mode,
    name: calendarForm.name.trim(),
    status: calendarForm.status,
    timezone: businessHours ? calendarForm.timezone.trim() : '',
    workdays: businessHours ? [...calendarForm.workdays].sort() : [],
    workEndMinute: businessHours ? timeToMinute(calendarForm.workEnd) : undefined,
    workStartMinute: businessHours ? timeToMinute(calendarForm.workStart) : undefined,
  };
  const saved = await runWrite(() =>
    calendarForm.calendarNo
      ? api.updateSLACalendar(calendarForm.calendarNo, {
          ...body,
          expectedVersion: calendarForm.version,
        })
      : api.createSLACalendar(body),
  );
  if (saved) resetCalendar();
}

async function disableCalendar(item: SupportSLACalendarSetting) {
  if (!globalThis.confirm(tr('support.settings.disableConfirm'))) return;
  await runWrite(() => api.disableSLACalendar(item.calendarNo, item.version));
}

async function loadHolidays() {
  holidays.value = selectedCalendarNo.value
    ? await api.slaHolidays(selectedCalendarNo.value)
    : [];
}

async function saveHoliday() {
  const saved = await runWrite(
    () =>
      api.putSLAHoliday(
        selectedCalendarNo.value,
        holidayForm.date,
        holidayForm.name.trim(),
      ),
    'support.settings.holidaySaved',
  );
  if (saved) Object.assign(holidayForm, { date: '', name: '' });
}

async function deleteHoliday(item: SupportSLAHolidaySetting) {
  if (!globalThis.confirm(tr('support.settings.deleteHolidayConfirm'))) return;
  await runWrite(
    () => api.deleteSLAHoliday(item.calendarNo, item.date),
    'support.settings.holidayDeleted',
  );
}

function resetRule() {
  Object.assign(ruleForm, {
    calendarNo: activeCalendars.value[0]?.calendarNo ?? '', categoryNo: '',
    departmentNo: activeDepartments.value[0]?.departmentNo ?? '',
    firstResponseMinutes: '', nearDueMinutes: '', priority: 'normal',
    resolutionMinutes: '', ruleNo: '', status: 'active', version: 0,
  });
}

function editRule(item: SupportSLARuleSetting) {
  Object.assign(ruleForm, item, {
    categoryNo: item.categoryNo ?? '',
    firstResponseMinutes: item.firstResponseMinutes ?? '',
    nearDueMinutes: item.nearDueMinutes.join(', '),
    resolutionMinutes: item.resolutionMinutes ?? '',
  });
}

async function saveRule() {
  if (ruleForm.firstResponseMinutes === '' && ruleForm.resolutionMinutes === '') {
    error.value = tr('support.settings.slaTargetRequired');
    return;
  }
  const body = {
    calendarNo: ruleForm.calendarNo,
    categoryNo: ruleForm.categoryNo,
    departmentNo: ruleForm.departmentNo,
    firstResponseMinutes: optionalMinutes(ruleForm.firstResponseMinutes),
    nearDueMinutes: parseNearDueMinutes(),
    priority: ruleForm.priority,
    resolutionMinutes: optionalMinutes(ruleForm.resolutionMinutes),
    status: ruleForm.status,
  };
  const saved = await runWrite(() =>
    ruleForm.ruleNo
      ? api.updateSLARule(ruleForm.ruleNo, {
          ...body,
          expectedVersion: ruleForm.version,
        })
      : api.createSLARule(body),
  );
  if (saved) resetRule();
}

async function disableRule(item: SupportSLARuleSetting) {
  if (!globalThis.confirm(tr('support.settings.disableConfirm'))) return;
  await runWrite(() => api.disableSLARule(item.ruleNo, item.version));
}

onMounted(loadAll);
</script>

<template>
  <main class="idc-workspace settings-workspace" data-testid="support-settings-page">
    <section class="panel workspace-toolbar">
      <div><h2>{{ tr('support.settings.title') }}</h2></div>
      <button class="button secondary" :disabled="loading || saving" type="button" @click="loadAll">
        {{ tr('support.action.refresh') }}
      </button>
    </section>

    <div v-if="error" class="notice error" role="alert">{{ error }}</div>
    <div v-if="success" class="notice success" role="status">{{ success }}</div>
    <div v-if="loading" class="panel empty-state">{{ tr('support.settings.loading') }}</div>

    <template v-else>
      <nav class="settings-tabs" :aria-label="tr('support.settings.navigation')">
        <button
          v-for="section in (['routing', 'fields', 'writing', 'sla', 'upstream', 'automation'] as SettingSection[])"
          :key="section"
          class="settings-tab"
          :class="{ active: activeSection === section }"
          :aria-pressed="activeSection === section"
          :data-testid="`support-settings-tab-${section}`"
          type="button"
          @click="activeSection = section"
        >
          {{ tr(`support.settings.tab.${section}`) }}
        </button>
      </nav>

      <section v-if="activeSection === 'routing'" class="settings-section" data-testid="support-settings-routing">
        <div class="notice info" role="note">
          <strong>{{ tr('support.settings.safeAssignmentTitle') }}</strong>
          {{ tr('support.settings.safeAssignmentHelp') }}
          <p class="mt-2">{{ tr('support.settings.roleHelp') }}</p>
        </div>

        <article class="panel settings-card">
          <div class="panel-heading">
            <div><p class="panel-kicker">{{ tr('support.settings.routing') }}</p><h2>{{ tr('support.settings.departments') }}</h2></div>
            <button class="button ghost" type="button" @click="resetDepartment">{{ tr('support.settings.newDepartment') }}</button>
          </div>
          <div class="settings-layout">
            <div class="settings-list" data-testid="support-settings-department-list">
              <article v-for="item in departments" :key="item.departmentNo" class="settings-row">
                <div>
                  <strong>{{ displayName(item) }}</strong>
                  <p :class="{ 'department-unlinked': !item.orgDeptId }">{{ item.orgDeptName || tr('support.settings.organizationUnlinked') }}</p>
                  <p>{{ tr('support.settings.assignment.manual_claim') }} · {{ item.status === 'active' ? tr('support.settings.active') : tr('support.settings.disabled') }}</p>
                </div>
                <div class="row-actions">
                  <button class="button ghost" :data-testid="`support-settings-edit-department-${item.departmentNo}`" type="button" @click="editDepartment(item)">{{ tr('support.settings.edit') }}</button>
                  <button v-if="item.status === 'active'" class="button danger" type="button" @click="disableDepartment(item)">{{ tr('support.settings.disable') }}</button>
                </div>
              </article>
              <p v-if="departments.length === 0" class="empty-state">{{ tr('support.settings.empty') }}</p>
            </div>
            <SupportDepartmentForm :item="departmentForm" :departments="activeDepartments" :context="context" :saving="saving" @save="saveDepartment" @cancel="resetDepartment" />
          </div>
        </article>

        <article class="panel settings-card">
          <div class="panel-heading"><div><p class="panel-kicker">{{ tr('support.settings.routing') }}</p><h2>{{ tr('support.settings.categories') }}</h2><p>{{ tr('support.settings.categoryHelp') }}</p></div><button class="button ghost" type="button" @click="resetCategory">{{ tr('support.settings.newCategory') }}</button></div>
          <div class="settings-layout">
            <div class="settings-list" data-testid="support-settings-category-list">
              <article v-for="item in categories" :key="item.categoryNo" class="settings-row"><div><strong>{{ displayName(item) }}</strong><p>{{ departmentName(item.departmentNo) }} · {{ tr(`support.priority.${item.priority}`) }}</p></div><div class="row-actions"><button class="button ghost" type="button" @click="editCategory(item)">{{ tr('support.settings.edit') }}</button><button v-if="item.status === 'active'" class="button danger" type="button" @click="disableCategory(item)">{{ tr('support.settings.disable') }}</button></div></article>
              <p v-if="categories.length === 0" class="empty-state">{{ tr('support.settings.empty') }}</p>
            </div>
            <form class="settings-form" data-testid="support-settings-category-form" @submit.prevent="saveCategory">
              <h3>{{ categoryForm.categoryNo ? tr('support.settings.editCategory') : tr('support.settings.newCategory') }}</h3>
              <div class="form-grid">
                <label class="field"><span>{{ tr('support.settings.nameZhCN') }}</span><input v-model="categoryForm.nameZhCN" required /></label>
                <label class="field"><span>{{ tr('support.settings.nameEnUS') }}</span><input v-model="categoryForm.nameEnUS" required /></label>
                <label class="field"><span>{{ tr('support.settings.mappedDepartment') }}</span><select v-model="categoryForm.departmentNo" data-testid="support-settings-category-department"><option value="">{{ tr('support.settings.publicQueue') }}</option><option v-for="item in activeDepartments" :key="item.departmentNo" :value="item.departmentNo">{{ displayName(item) }}</option></select></label>
                <label class="field"><span>{{ tr('support.field.priority') }}</span><select v-model="categoryForm.priority"><option v-for="value in (['low', 'normal', 'high', 'urgent'] as Priority[])" :key="value" :value="value">{{ tr(`support.priority.${value}`) }}</option></select></label>
                <label class="field"><span>{{ tr('support.settings.sortOrder') }}</span><input v-model.number="categoryForm.sortOrder" min="0" type="number" /></label>
                <label class="field"><span>{{ tr('support.settings.settingState') }}</span><select v-model="categoryForm.status"><option value="active">{{ tr('support.settings.active') }}</option><option value="disabled">{{ tr('support.settings.disabled') }}</option></select></label>
              </div>
              <div class="composer-actions"><button v-if="categoryForm.categoryNo" class="button ghost" type="button" @click="resetCategory">{{ tr('support.action.cancel') }}</button><button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button></div>
            </form>
          </div>
        </article>
      </section>

      <section v-else-if="activeSection === 'fields'" class="settings-section" data-testid="support-settings-fields">
        <div class="notice info" role="note">
          {{ tr('support.settings.fieldSnapshotHelp') }}
        </div>
        <article class="panel settings-card">
          <div class="panel-heading">
            <div>
              <p class="panel-kicker">{{ tr('support.settings.ticketDisplay') }}</p>
              <h2>{{ tr('support.settings.customFields') }}</h2>
              <p>{{ tr('support.settings.customFieldsHelp') }}</p>
            </div>
            <button class="button ghost" type="button" @click="resetCustomField">
              {{ tr('support.settings.newField') }}
            </button>
          </div>
          <div class="settings-layout">
            <div class="settings-list" data-testid="support-settings-field-list">
              <article v-for="item in customFields" :key="item.fieldNo" class="settings-row">
                <div>
                  <strong>{{ displayName(item) }}</strong>
                  <p>
                    {{ tr(`support.settings.fieldType.${item.fieldType}`) }} ·
                    {{ tr(`support.settings.fieldVisibility.${item.visibility}`) }} ·
                    {{ item.required ? tr('support.settings.required') : tr('support.settings.optional') }} ·
                    {{ item.status === 'active' ? tr('support.settings.active') : tr('support.settings.disabled') }}
                  </p>
                </div>
                <div class="row-actions">
                  <button class="button ghost" :data-testid="`support-settings-edit-field-${item.fieldNo}`" type="button" @click="editCustomField(item)">
                    {{ tr('support.settings.edit') }}
                  </button>
                  <button v-if="item.status === 'active'" class="button danger" type="button" @click="disableCustomField(item)">
                    {{ tr('support.settings.disable') }}
                  </button>
                </div>
              </article>
              <p v-if="customFields.length === 0" class="empty-state">{{ tr('support.settings.empty') }}</p>
            </div>

            <form class="settings-form" data-testid="support-settings-field-form" @submit.prevent="saveCustomField">
              <h3>{{ customFieldForm.fieldNo ? tr('support.settings.editField') : tr('support.settings.newField') }}</h3>
              <div class="form-grid">
                <label class="field"><span>{{ tr('support.settings.nameZhCN') }}</span><input v-model="customFieldForm.nameZhCN" maxlength="128" required /></label>
                <label class="field"><span>{{ tr('support.settings.nameEnUS') }}</span><input v-model="customFieldForm.nameEnUS" maxlength="128" required /></label>
                <label class="field"><span>{{ tr('support.settings.fieldType') }}</span><select v-model="customFieldForm.fieldType" data-testid="support-settings-field-type"><option v-for="value in (['single_line', 'multi_line', 'number', 'single_select', 'multi_select', 'date', 'boolean'] as SupportCustomFieldType[])" :key="value" :value="value">{{ tr(`support.settings.fieldType.${value}`) }}</option></select></label>
                <label class="field"><span>{{ tr('support.settings.fieldVisibility') }}</span><select v-model="customFieldForm.visibility" data-testid="support-settings-field-visibility" @change="normalizeFieldVisibility"><option value="customer_visible">{{ tr('support.settings.fieldVisibility.customer_visible') }}</option><option value="internal_only">{{ tr('support.settings.fieldVisibility.internal_only') }}</option></select></label>
                <label class="field"><span>{{ tr('support.settings.sortOrder') }}</span><input v-model.number="customFieldForm.sortOrder" min="0" type="number" /></label>
                <label class="field"><span>{{ tr('support.settings.settingState') }}</span><select v-model="customFieldForm.status"><option value="active">{{ tr('support.settings.active') }}</option><option value="disabled">{{ tr('support.settings.disabled') }}</option></select></label>
              </div>
              <label class="field"><span>{{ tr('support.settings.descriptionZhCN') }}</span><textarea v-model="customFieldForm.descriptionZhCN" maxlength="512" rows="2" /></label>
              <label class="field"><span>{{ tr('support.settings.descriptionEnUS') }}</span><textarea v-model="customFieldForm.descriptionEnUS" maxlength="512" rows="2" /></label>

              <fieldset class="weekday-picker">
                <legend>{{ tr('support.settings.appliesTo') }}</legend>
                <template v-if="customFieldForm.visibility === 'customer_visible'">
                  <label><input v-model="customFieldForm.appliesPresales" type="checkbox" />{{ tr('support.settings.appliesPresales') }}</label>
                  <label><input v-model="customFieldForm.appliesCustomer" type="checkbox" />{{ tr('support.settings.appliesCustomer') }}</label>
                </template>
                <label v-else><input checked disabled type="checkbox" />{{ tr('support.settings.appliesInternal') }}</label>
              </fieldset>
              <label class="selection-option"><input v-model="customFieldForm.required" type="checkbox" /><span><strong>{{ tr('support.settings.required') }}</strong><small>{{ tr('support.settings.requiredHelp') }}</small></span></label>

              <fieldset v-if="isSelectField()" class="selection-field" data-testid="support-settings-field-options">
                <legend>{{ tr('support.settings.fieldOptions') }}</legend>
                <p class="field-help">{{ tr('support.settings.fieldOptionsHelp') }}</p>
                <div v-for="(option, index) in customFieldForm.options" :key="option.optionNo || `new-${index}`" class="form-grid">
                  <label class="field"><span>{{ tr('support.settings.optionNameZhCN') }}</span><input v-model="option.nameZhCN" maxlength="128" required /></label>
                  <label class="field"><span>{{ tr('support.settings.optionNameEnUS') }}</span><input v-model="option.nameEnUS" maxlength="128" required /></label>
                  <label class="field"><span>{{ tr('support.settings.sortOrder') }}</span><input v-model.number="option.sortOrder" min="0" type="number" /></label>
                  <label class="field"><span>{{ tr('support.settings.settingState') }}</span><select v-model="option.status"><option value="active">{{ tr('support.settings.active') }}</option><option value="disabled">{{ tr('support.settings.disabled') }}</option></select></label>
                  <button v-if="!option.optionNo" class="button ghost" type="button" @click="removeNewCustomFieldOption(index)">{{ tr('support.settings.removeUnsavedOption') }}</button>
                </div>
                <button class="button secondary" data-testid="support-settings-add-field-option" type="button" @click="addCustomFieldOption">{{ tr('support.settings.addFieldOption') }}</button>
              </fieldset>

              <div class="composer-actions">
                <button v-if="customFieldForm.fieldNo" class="button ghost" type="button" @click="resetCustomField">{{ tr('support.action.cancel') }}</button>
                <button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button>
              </div>
            </form>
          </div>
        </article>
      </section>

      <section v-else-if="activeSection === 'writing'" class="settings-section" data-testid="support-settings-writing">
        <article class="panel settings-card">
          <div class="panel-heading"><div><p class="panel-kicker">{{ tr('support.settings.ticketDisplay') }}</p><h2>{{ tr('support.settings.customStatuses') }}</h2><p>{{ tr('support.settings.statusHelp') }}</p></div><button class="button ghost" type="button" @click="resetStatus">{{ tr('support.settings.newStatus') }}</button></div>
          <div class="settings-layout">
            <div class="settings-list"><article v-for="item in statuses" :key="item.statusNo" class="settings-row"><div><strong>{{ displayName(item) }}</strong><p>{{ tr(`support.stage.${item.baseStage}`) }}</p></div><div class="row-actions"><button class="button ghost" type="button" @click="editStatus(item)">{{ tr('support.settings.edit') }}</button><button v-if="item.status === 'active'" class="button danger" type="button" @click="disableStatus(item)">{{ tr('support.settings.disable') }}</button></div></article><p v-if="statuses.length === 0" class="empty-state">{{ tr('support.settings.empty') }}</p></div>
            <form class="settings-form" data-testid="support-settings-status-form" @submit.prevent="saveStatus"><h3>{{ statusForm.statusNo ? tr('support.settings.editStatus') : tr('support.settings.newStatus') }}</h3><div class="form-grid"><label class="field"><span>{{ tr('support.settings.nameZhCN') }}</span><input v-model="statusForm.nameZhCN" required /></label><label class="field"><span>{{ tr('support.settings.nameEnUS') }}</span><input v-model="statusForm.nameEnUS" required /></label><label class="field"><span>{{ tr('support.settings.baseStage') }}</span><select v-model="statusForm.baseStage"><option v-for="value in (['new', 'in_progress', 'waiting_customer', 'waiting_upstream', 'resolved', 'closed'] as BaseStage[])" :key="value" :value="value">{{ tr(`support.stage.${value}`) }}</option></select></label><label class="field"><span>{{ tr('support.settings.sortOrder') }}</span><input v-model.number="statusForm.sortOrder" min="0" type="number" /></label><label class="field"><span>{{ tr('support.settings.settingState') }}</span><select v-model="statusForm.status"><option value="active">{{ tr('support.settings.active') }}</option><option value="disabled">{{ tr('support.settings.disabled') }}</option></select></label></div><div class="composer-actions"><button v-if="statusForm.statusNo" class="button ghost" type="button" @click="resetStatus">{{ tr('support.action.cancel') }}</button><button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button></div></form>
          </div>
        </article>

        <article class="panel settings-card">
          <div class="panel-heading"><div><p class="panel-kicker">{{ tr('support.settings.writing') }}</p><h2>{{ tr('support.settings.replyPresets') }}</h2><p>{{ tr('support.settings.presetHelp') }}</p></div><button class="button ghost" type="button" @click="resetPreset">{{ tr('support.settings.newPreset') }}</button></div>
          <div class="settings-layout">
            <div class="settings-list"><article v-for="item in presets" :key="item.presetNo" class="settings-row"><div><strong>{{ item.title }}</strong><p>{{ departmentName(item.departmentNo) }} · {{ item.locale }}</p></div><div class="row-actions"><button class="button ghost" type="button" @click="editPreset(item)">{{ tr('support.settings.edit') }}</button><button v-if="item.status === 'active'" class="button danger" type="button" @click="disablePreset(item)">{{ tr('support.settings.disable') }}</button></div></article><p v-if="presets.length === 0" class="empty-state">{{ tr('support.settings.empty') }}</p></div>
            <form class="settings-form" data-testid="support-settings-preset-form" @submit.prevent="savePreset"><h3>{{ presetForm.presetNo ? tr('support.settings.editPreset') : tr('support.settings.newPreset') }}</h3><div class="form-grid"><label class="field"><span>{{ tr('support.field.department') }}</span><select v-model="presetForm.departmentNo" required><option disabled value="">{{ tr('support.value.choose') }}</option><option v-for="item in activeDepartments" :key="item.departmentNo" :value="item.departmentNo">{{ displayName(item) }}</option></select></label><label class="field"><span>{{ tr('support.settings.locale') }}</span><select v-model="presetForm.locale"><option value="zh-CN">{{ tr('support.settings.locale.zhCN') }}</option><option value="en-US">{{ tr('support.settings.locale.enUS') }}</option></select></label><label class="field"><span>{{ tr('support.settings.presetTitle') }}</span><input v-model="presetForm.title" required /></label><label class="field"><span>{{ tr('support.settings.sortOrder') }}</span><input v-model.number="presetForm.sortOrder" min="0" type="number" /></label><label class="field"><span>{{ tr('support.settings.settingState') }}</span><select v-model="presetForm.status"><option value="active">{{ tr('support.settings.active') }}</option><option value="disabled">{{ tr('support.settings.disabled') }}</option></select></label></div><label class="field"><span>{{ tr('support.field.body') }}</span><textarea v-model="presetForm.body" rows="5" required /></label><div class="composer-actions"><button v-if="presetForm.presetNo" class="button ghost" type="button" @click="resetPreset">{{ tr('support.action.cancel') }}</button><button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button></div></form>
          </div>
        </article>
      </section>

      <section v-else-if="activeSection === 'sla'" class="settings-section" data-testid="support-settings-sla">
        <div class="notice info" role="note">{{ tr('support.settings.slaNoDefault') }}</div>
        <article class="panel settings-card">
          <div class="panel-heading"><div><h2>{{ tr('support.settings.calendars') }}</h2></div><button class="button ghost" type="button" @click="resetCalendar">{{ tr('support.settings.newCalendar') }}</button></div>
          <div class="settings-layout">
            <div class="settings-list"><article v-for="item in calendars" :key="item.calendarNo" class="settings-row"><div><strong>{{ item.name }}</strong><p>{{ tr(`support.settings.calendar.${item.mode}`) }}<template v-if="item.timezone"> · {{ item.timezone }}</template></p></div><div class="row-actions"><button class="button ghost" type="button" @click="editCalendar(item)">{{ tr('support.settings.edit') }}</button><button v-if="item.status === 'active'" class="button danger" type="button" @click="disableCalendar(item)">{{ tr('support.settings.disable') }}</button></div></article><p v-if="calendars.length === 0" class="empty-state">{{ tr('support.settings.empty') }}</p></div>
            <form class="settings-form" data-testid="support-settings-calendar-form" @submit.prevent="saveCalendar"><h3>{{ calendarForm.calendarNo ? tr('support.settings.editCalendar') : tr('support.settings.newCalendar') }}</h3><div class="form-grid"><label class="field"><span>{{ tr('support.settings.calendarName') }}</span><input v-model="calendarForm.name" required /></label><label class="field"><span>{{ tr('support.settings.calendarMode') }}</span><select v-model="calendarForm.mode" data-testid="support-settings-calendar-mode"><option value="continuous">{{ tr('support.settings.calendar.continuous') }}</option><option value="business_hours">{{ tr('support.settings.calendar.business_hours') }}</option></select></label><label class="field"><span>{{ tr('support.settings.settingState') }}</span><select v-model="calendarForm.status"><option value="active">{{ tr('support.settings.active') }}</option><option value="disabled">{{ tr('support.settings.disabled') }}</option></select></label></div><template v-if="calendarForm.mode === 'business_hours'"><label class="field"><span>{{ tr('support.settings.timezone') }}</span><input v-model="calendarForm.timezone" placeholder="Asia/Shanghai" required /><small>{{ tr('support.settings.timezoneHelp') }}</small></label><fieldset class="weekday-picker"><legend>{{ tr('support.settings.workdays') }}</legend><label v-for="day in 7" :key="day"><input v-model="calendarForm.workdays" :value="day" type="checkbox" />{{ tr(`support.settings.weekday.${day}`) }}</label></fieldset><div class="form-grid"><label class="field"><span>{{ tr('support.settings.workStart') }}</span><input v-model="calendarForm.workStart" required type="time" /></label><label class="field"><span>{{ tr('support.settings.workEnd') }}</span><input v-model="calendarForm.workEnd" required type="time" /></label></div></template><div class="composer-actions"><button v-if="calendarForm.calendarNo" class="button ghost" type="button" @click="resetCalendar">{{ tr('support.action.cancel') }}</button><button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button></div></form>
          </div>
        </article>

        <article class="panel settings-card">
          <div class="panel-heading"><div><h2>{{ tr('support.settings.holidays') }}</h2><p>{{ tr('support.settings.holidayHelp') }}</p></div></div>
          <label class="field settings-calendar-select"><span>{{ tr('support.settings.calendarName') }}</span><select v-model="selectedCalendarNo" data-testid="support-settings-holiday-calendar" @change="loadHolidays"><option v-for="item in businessCalendars" :key="item.calendarNo" :value="item.calendarNo">{{ item.name }}</option></select></label>
          <div v-if="selectedCalendarNo" class="settings-layout"><div class="settings-list"><article v-for="item in holidays" :key="item.date" class="settings-row"><div><strong>{{ item.name }}</strong><p>{{ item.date }}</p></div><button class="button danger" type="button" @click="deleteHoliday(item)">{{ tr('support.settings.deleteHoliday') }}</button></article><p v-if="holidays.length === 0" class="empty-state">{{ tr('support.settings.noHolidays') }}</p></div><form class="settings-form" data-testid="support-settings-holiday-form" @submit.prevent="saveHoliday"><h3>{{ tr('support.settings.addHoliday') }}</h3><label class="field"><span>{{ tr('support.settings.holidayDate') }}</span><input v-model="holidayForm.date" required type="date" /></label><label class="field"><span>{{ tr('support.settings.holidayName') }}</span><input v-model="holidayForm.name" required /></label><div class="composer-actions"><button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button></div></form></div>
          <p v-else class="empty-state">{{ tr('support.settings.saveCalendarFirst') }}</p>
        </article>

        <article class="panel settings-card">
          <div class="panel-heading"><div><h2>{{ tr('support.settings.slaRules') }}</h2><p>{{ tr('support.settings.slaRuleHelp') }}</p></div><button class="button ghost" type="button" @click="resetRule">{{ tr('support.settings.newRule') }}</button></div>
          <div class="settings-layout">
            <div class="settings-list"><article v-for="item in rules" :key="item.ruleNo" class="settings-row"><div><strong>{{ departmentName(item.departmentNo) }} · {{ tr(`support.priority.${item.priority}`) }}</strong><p>{{ categoryName(item.categoryNo) }} · {{ calendarName(item.calendarNo) }}</p></div><div class="row-actions"><button class="button ghost" type="button" @click="editRule(item)">{{ tr('support.settings.edit') }}</button><button v-if="item.status === 'active'" class="button danger" type="button" @click="disableRule(item)">{{ tr('support.settings.disable') }}</button></div></article><p v-if="rules.length === 0" class="empty-state">{{ tr('support.settings.empty') }}</p></div>
            <form class="settings-form" data-testid="support-settings-rule-form" @submit.prevent="saveRule"><h3>{{ ruleForm.ruleNo ? tr('support.settings.editRule') : tr('support.settings.newRule') }}</h3><div class="form-grid"><label class="field"><span>{{ tr('support.field.department') }}</span><select v-model="ruleForm.departmentNo" required><option disabled value="">{{ tr('support.value.choose') }}</option><option v-for="item in activeDepartments" :key="item.departmentNo" :value="item.departmentNo">{{ displayName(item) }}</option></select></label><label class="field"><span>{{ tr('support.settings.categoryOverride') }}</span><select v-model="ruleForm.categoryNo"><option value="">{{ tr('support.settings.departmentDefault') }}</option><option v-for="item in activeCategories" :key="item.categoryNo" :value="item.categoryNo">{{ displayName(item) }}</option></select></label><label class="field"><span>{{ tr('support.field.priority') }}</span><select v-model="ruleForm.priority"><option v-for="value in (['low', 'normal', 'high', 'urgent'] as Priority[])" :key="value" :value="value">{{ tr(`support.priority.${value}`) }}</option></select></label><label class="field"><span>{{ tr('support.settings.calendarName') }}</span><select v-model="ruleForm.calendarNo" required><option disabled value="">{{ tr('support.value.choose') }}</option><option v-for="item in activeCalendars" :key="item.calendarNo" :value="item.calendarNo">{{ item.name }}</option></select></label><label class="field"><span>{{ tr('support.settings.firstResponseMinutes') }}</span><input v-model.number="ruleForm.firstResponseMinutes" min="1" type="number" /></label><label class="field"><span>{{ tr('support.settings.resolutionMinutes') }}</span><input v-model.number="ruleForm.resolutionMinutes" min="1" type="number" /></label><label class="field"><span>{{ tr('support.settings.settingState') }}</span><select v-model="ruleForm.status"><option value="active">{{ tr('support.settings.active') }}</option><option value="disabled">{{ tr('support.settings.disabled') }}</option></select></label></div><label class="field"><span>{{ tr('support.settings.nearDueMinutes') }}</span><input v-model="ruleForm.nearDueMinutes" placeholder="60, 30" /><small>{{ tr('support.settings.nearDueHelp') }}</small></label><div class="composer-actions"><button v-if="ruleForm.ruleNo" class="button ghost" type="button" @click="resetRule">{{ tr('support.action.cancel') }}</button><button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button></div></form>
          </div>
        </article>
      </section>

      <section v-else-if="activeSection === 'upstream'" class="settings-section" data-testid="support-settings-upstream">
        <div class="notice info" role="note">{{ tr('support.upstream.channelHelp') }}</div>
        <article class="panel settings-card">
          <div class="panel-heading"><div><p class="panel-kicker">{{ tr('support.upstream.isolated') }}</p><h2>{{ tr('support.upstream.channel') }}</h2><p>{{ upstreamChannel?.configured ? tr('support.upstream.configured') : tr('support.upstream.notConfigured') }}<template v-if="upstreamChannel?.credentialMaskedTail"> · {{ upstreamChannel.credentialMaskedTail }}</template></p></div></div>
          <form class="settings-form" data-testid="support-upstream-channel-form" @submit.prevent="saveUpstreamChannel">
            <div class="form-grid">
              <label class="field"><span>{{ tr('support.upstream.baseUrl') }}</span><input v-model.trim="upstreamChannelForm.baseUrl" placeholder="https://support.example.com" required type="url" /></label>
              <label class="field"><span>{{ tr('support.upstream.authMode') }}</span><select v-model="upstreamChannelForm.authMode"><option value="zjmf_api_login">{{ tr('support.upstream.authMode.zjmf') }}</option><option value="resource_login">{{ tr('support.upstream.authMode.resource') }}</option></select></label>
              <label class="field"><span>{{ tr('support.upstream.username') }}</span><input v-model.trim="upstreamChannelForm.username" autocomplete="off" required /></label>
              <label class="field"><span>{{ tr('support.upstream.password') }}</span><input v-model="upstreamChannelForm.password" autocomplete="new-password" required type="password" /></label>
              <label class="field"><span>{{ tr('support.settings.settingState') }}</span><select v-model="upstreamChannelForm.status"><option value="enabled">{{ tr('support.settings.active') }}</option><option value="disabled">{{ tr('support.settings.disabled') }}</option></select></label>
            </div>
            <label class="field"><span>{{ tr('support.field.reason') }}</span><textarea v-model.trim="upstreamChannelForm.reason" minlength="3" required rows="3" /></label>
            <div class="composer-actions"><button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button></div>
          </form>
        </article>
        <article class="panel settings-card">
          <div class="panel-heading"><div><h2>{{ tr('support.upstream.departmentMap') }}</h2><p>{{ tr('support.upstream.departmentMapHelp') }}</p></div></div>
          <form class="settings-form" data-testid="support-upstream-department-form" @submit.prevent="saveUpstreamDepartmentMap">
            <div class="form-grid">
              <label class="field"><span>{{ tr('support.field.department') }}</span><select v-model="upstreamDepartmentForm.departmentNo" required @change="loadUpstreamDepartmentMap"><option value="">{{ tr('support.value.choose') }}</option><option v-for="item in activeDepartments" :key="item.departmentNo" :value="item.departmentNo">{{ displayName(item) }}</option></select></label>
              <label class="field"><span>{{ tr('support.upstream.departmentId') }}</span><input v-model.number="upstreamDepartmentForm.upstreamDepartmentId" min="1" required type="number" /></label>
              <label class="field"><span>{{ tr('support.upstream.departmentName') }}</span><input v-model.trim="upstreamDepartmentForm.upstreamDepartmentName" /></label>
            </div>
            <div class="composer-actions"><button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button></div>
          </form>
        </article>
        <article class="panel settings-card">
          <div class="panel-heading"><div><h2>{{ tr('support.upstream.entitlementMap') }}</h2><p>{{ tr('support.upstream.entitlementMapHelp') }}</p></div></div>
          <form class="settings-form" data-testid="support-upstream-entitlement-form" @submit.prevent="saveUpstreamEntitlementMap">
            <div class="form-grid">
              <label class="field"><span>{{ tr('support.upstream.localEntitlement') }}</span><input v-model.trim="upstreamEntitlementForm.entitlementNo" required @blur="loadUpstreamEntitlementMap" /></label>
              <label class="field"><span>{{ tr('support.upstream.reference') }}</span><input v-model.trim="upstreamEntitlementForm.upstreamReference" required /></label>
            </div>
            <div class="composer-actions"><button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button></div>
          </form>
        </article>
      </section>

      <section v-else class="settings-section" data-testid="support-settings-automation">
        <form class="panel composer settings-policy" @submit.prevent="savePolicy">
          <div class="panel-heading"><div><p class="panel-kicker">{{ tr('support.settings.automation') }}</p><h2>{{ tr('support.settings.lifecycle') }}</h2><p>{{ tr('support.settings.lifecycleHelp') }}</p></div></div>
          <label class="field"><span>{{ tr('support.settings.autoCloseDays') }}</span><input v-model.number="policyForm.autoCloseDays" data-testid="support-settings-auto-close" max="365" min="1" required type="number" /><small>{{ tr('support.settings.autoCloseHelp') }}</small></label>
          <label class="field"><span>{{ tr('support.settings.reopenDays') }}</span><input v-model.number="policyForm.reopenDays" data-testid="support-settings-reopen" max="365" min="1" required type="number" /><small>{{ tr('support.settings.reopenHelp') }}</small></label>
          <div class="composer-actions"><button class="button primary" :disabled="saving" type="submit">{{ tr('support.settings.save') }}</button></div>
        </form>
      </section>
    </template>
  </main>
</template>

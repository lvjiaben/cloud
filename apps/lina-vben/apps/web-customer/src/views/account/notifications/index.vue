<script lang="ts" setup>
import type {
  CustomerNotificationContactPreference,
  CustomerNotificationDelivery,
  CustomerNotificationMarketingConsent,
  CustomerNotificationPreferences,
  CustomerNotificationQuietHours,
  NotificationDeliveryStatus,
} from '#/api/idc/notifications';

import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  Spin,
  Switch,
  TabPane,
  Tabs,
  Tag,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  getCustomerNotificationPreferences,
  listCustomerNotificationDeliveries,
  updateCustomerNotificationPreference,
} from '#/api/idc/notifications';
import { $t } from '#/locales';

const items = ref<CustomerNotificationDelivery[]>([]);
const cursor = ref('');
const nextCursor = ref('');
const cursorHistory = ref<string[]>([]);
const loading = ref(false);
const failed = ref(false);
const preferencesSnapshot = ref<CustomerNotificationPreferences>();
const preferencesLoading = ref(false);
const preferencesFailed = ref(false);
const preferenceDrafts = ref<Record<string, boolean>>({});
const preferenceFeedback = ref<
  Record<string, 'error' | 'retry' | 'saved' | undefined>
>({});
const preferenceOperationKeys = ref<Record<string, string>>({});
const marketingDrafts = ref<Record<string, boolean>>({});
const marketingFeedback = ref<
  Record<string, 'error' | 'retry' | 'saved' | undefined>
>({});
const marketingOperationKeys = ref<Record<string, string>>({});

interface QuietHoursDraft {
  enabled: boolean;
  endTime: string;
  startTime: string;
  timezoneName: string;
}

const quietHoursDraft = ref<QuietHoursDraft>();
const quietHoursFeedback = ref<'error' | 'retry' | 'saved'>();
const quietHoursOperationKey = ref('');
const savingPreferenceKey = ref('');
const pageNumber = computed(() => cursorHistory.value.length + 1);

async function loadDeliveries() {
  loading.value = true;
  failed.value = false;
  try {
    const result = await listCustomerNotificationDeliveries({
      cursor: cursor.value || undefined,
      limit: 10,
    });
    items.value = result.items || [];
    nextCursor.value = result.nextCursor || '';
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

function preferenceKey(item: CustomerNotificationContactPreference) {
  return `${item.contactNo}:${item.topic}:${item.channelCode}`;
}

function serverPreferenceEnabled(item: CustomerNotificationContactPreference) {
  return item.topic === 'security' ? true : item.enabled;
}

function preferenceEnabled(item: CustomerNotificationContactPreference) {
  return (
    preferenceDrafts.value[preferenceKey(item)] ?? serverPreferenceEnabled(item)
  );
}

function isPreferenceEditable(item: CustomerNotificationContactPreference) {
  return item.topic !== 'marketing' && item.topic !== 'security';
}

function preferenceDisabled(item: CustomerNotificationContactPreference) {
  return (
    !isPreferenceEditable(item) ||
    preferencesFailed.value ||
    preferencesLoading.value ||
    Boolean(savingPreferenceKey.value)
  );
}

function newPreferenceOperationKey() {
  return `pref-${globalThis.crypto.randomUUID()}`;
}

function marketingKey(item: CustomerNotificationMarketingConsent) {
  return `${item.contactNo}:${item.channelCode}`;
}

function isMarketingConsentComplete(
  item: CustomerNotificationMarketingConsent,
) {
  return Boolean(
    item.contactNo &&
    item.policyReference &&
    item.policyVersion &&
    (item.locale === 'en-US' || item.locale === 'zh-CN') &&
    (item.status === 'opted_in' || item.status === 'opted_out'),
  );
}

function marketingEnabled(item: CustomerNotificationMarketingConsent) {
  return (
    marketingDrafts.value[marketingKey(item)] ?? item.status === 'opted_in'
  );
}

function marketingDisabled(item: CustomerNotificationMarketingConsent) {
  return (
    !isMarketingConsentComplete(item) ||
    preferencesFailed.value ||
    preferencesLoading.value ||
    Boolean(savingPreferenceKey.value)
  );
}

const quietHoursUnavailableReason = computed(() => {
  if (!preferencesSnapshot.value?.quietHours) {
    return 'notifications.quietHoursMissing';
  }
  return '';
});

function applyMarketingSnapshot(
  snapshot: CustomerNotificationPreferences,
  preserveDrafts: boolean,
) {
  const previousDrafts = marketingDrafts.value;
  const previousFeedback = marketingFeedback.value;
  const nextDrafts: Record<string, boolean> = {};
  const nextFeedback: Record<string, 'error' | 'retry' | 'saved' | undefined> =
    {};
  const nextOperationKeys: Record<string, string> = {};

  for (const item of snapshot.marketingConsents) {
    const key = marketingKey(item);
    const serverEnabled = item.status === 'opted_in';
    const feedback = previousFeedback[key];
    const keepFailedDraft =
      preserveDrafts && (feedback === 'error' || feedback === 'retry');
    const draftEnabled = keepFailedDraft
      ? (previousDrafts[key] ?? serverEnabled)
      : serverEnabled;
    nextDrafts[key] = draftEnabled;

    if (!preserveDrafts) continue;
    if (feedback === 'error' || feedback === 'retry') {
      if (draftEnabled === serverEnabled) {
        nextFeedback[key] = 'saved';
      } else {
        nextFeedback[key] = 'retry';
        nextOperationKeys[key] = newPreferenceOperationKey();
      }
    }
  }

  marketingDrafts.value = nextDrafts;
  marketingFeedback.value = nextFeedback;
  marketingOperationKeys.value = nextOperationKeys;
}

function quietHoursToDraft(
  quietHours: CustomerNotificationQuietHours,
): QuietHoursDraft {
  return {
    enabled: quietHours.enabled,
    endTime: formatMinute(quietHours.endMinute),
    startTime: formatMinute(quietHours.startMinute),
    timezoneName: quietHours.timezoneName,
  };
}

function emptyQuietHoursDraft(): QuietHoursDraft {
  return {
    enabled: false,
    endTime: '',
    startTime: '',
    timezoneName: '',
  };
}

function quietHoursDraftMatches(
  draft: QuietHoursDraft,
  quietHours: CustomerNotificationQuietHours,
) {
  return (
    draft.enabled === quietHours.enabled &&
    draft.endTime === formatMinute(quietHours.endMinute) &&
    draft.startTime === formatMinute(quietHours.startMinute) &&
    draft.timezoneName.trim() === quietHours.timezoneName
  );
}

function applyQuietHoursSnapshot(
  snapshot: CustomerNotificationPreferences,
  preserveDrafts: boolean,
) {
  const current = snapshot.quietHours;
  if (!current) {
    const previousDraft = quietHoursDraft.value;
    const previousFeedback = quietHoursFeedback.value;
    const keepFailedDraft =
      preserveDrafts &&
      previousDraft &&
      (previousFeedback === 'error' || previousFeedback === 'retry');
    quietHoursDraft.value = keepFailedDraft
      ? previousDraft
      : emptyQuietHoursDraft();
    quietHoursFeedback.value = keepFailedDraft ? 'retry' : undefined;
    quietHoursOperationKey.value = keepFailedDraft
      ? newPreferenceOperationKey()
      : '';
    return;
  }

  const previousDraft = quietHoursDraft.value;
  const previousFeedback = quietHoursFeedback.value;
  const keepFailedDraft =
    preserveDrafts &&
    previousDraft &&
    (previousFeedback === 'error' || previousFeedback === 'retry');
  quietHoursDraft.value = keepFailedDraft
    ? previousDraft
    : quietHoursToDraft(current);
  quietHoursFeedback.value = undefined;
  quietHoursOperationKey.value = '';

  if (!keepFailedDraft || !quietHoursDraft.value) return;
  if (quietHoursDraftMatches(quietHoursDraft.value, current)) {
    quietHoursFeedback.value = 'saved';
  } else {
    quietHoursFeedback.value = 'retry';
    quietHoursOperationKey.value = newPreferenceOperationKey();
  }
}

function applyPreferenceSnapshot(
  snapshot: CustomerNotificationPreferences,
  preserveDrafts: boolean,
) {
  const previousDrafts = preferenceDrafts.value;
  const previousFeedback = preferenceFeedback.value;
  const nextDrafts: Record<string, boolean> = {};
  const nextFeedback: Record<string, 'error' | 'retry' | 'saved' | undefined> =
    {};
  const nextOperationKeys: Record<string, string> = {};

  for (const item of snapshot.preferences) {
    const key = preferenceKey(item);
    const serverEnabled = serverPreferenceEnabled(item);
    const feedback = previousFeedback[key];
    const keepFailedDraft =
      preserveDrafts && (feedback === 'error' || feedback === 'retry');
    const draftEnabled = keepFailedDraft
      ? (previousDrafts[key] ?? serverEnabled)
      : serverEnabled;
    nextDrafts[key] = draftEnabled;

    if (!preserveDrafts) continue;
    if (feedback === 'error' || feedback === 'retry') {
      if (draftEnabled === serverEnabled) {
        nextFeedback[key] = 'saved';
      } else {
        nextFeedback[key] = 'retry';
        nextOperationKeys[key] = newPreferenceOperationKey();
      }
    }
  }

  preferenceDrafts.value = nextDrafts;
  preferenceFeedback.value = nextFeedback;
  preferenceOperationKeys.value = nextOperationKeys;
  applyMarketingSnapshot(snapshot, preserveDrafts);
  applyQuietHoursSnapshot(snapshot, preserveDrafts);
  preferencesSnapshot.value = snapshot;
}

async function loadPreferences(preserveDrafts = false) {
  preferencesLoading.value = true;
  preferencesFailed.value = false;
  try {
    const snapshot = await getCustomerNotificationPreferences();
    applyPreferenceSnapshot(snapshot, preserveDrafts);
  } catch {
    preferencesFailed.value = true;
  } finally {
    preferencesLoading.value = false;
  }
}

async function savePreference(item: CustomerNotificationContactPreference) {
  if (
    !preferencesSnapshot.value ||
    !isPreferenceEditable(item) ||
    preferencesFailed.value ||
    preferencesLoading.value ||
    savingPreferenceKey.value
  ) {
    return;
  }

  const key = preferenceKey(item);
  const enabled = preferenceEnabled(item);
  const operationKey =
    preferenceOperationKeys.value[key] || newPreferenceOperationKey();
  preferenceOperationKeys.value[key] = operationKey;
  savingPreferenceKey.value = key;
  preferenceFeedback.value[key] = undefined;
  try {
    const result = await updateCustomerNotificationPreference({
      channelCode: item.channelCode,
      contactNo: item.contactNo,
      enabled,
      expectedVersion: preferencesSnapshot.value.version,
      idempotencyKey: operationKey,
      topic: item.topic,
    });
    item.enabled = enabled;
    preferencesSnapshot.value.version = result.version;
    delete preferenceOperationKeys.value[key];
    preferenceFeedback.value[key] = 'saved';
  } catch {
    preferenceFeedback.value[key] = 'error';
  } finally {
    savingPreferenceKey.value = '';
  }
}

async function togglePreference(
  item: CustomerNotificationContactPreference,
  checked: boolean | number | string,
) {
  const key = preferenceKey(item);
  preferenceDrafts.value[key] = checked === true;
  preferenceOperationKeys.value[key] = newPreferenceOperationKey();
  await savePreference(item);
}

async function saveMarketingConsent(
  item: CustomerNotificationMarketingConsent,
) {
  if (
    !preferencesSnapshot.value ||
    !isMarketingConsentComplete(item) ||
    preferencesFailed.value ||
    preferencesLoading.value ||
    savingPreferenceKey.value
  ) {
    return;
  }

  const key = marketingKey(item);
  const enabled = marketingEnabled(item);
  const operationKey =
    marketingOperationKeys.value[key] || newPreferenceOperationKey();
  marketingOperationKeys.value[key] = operationKey;
  savingPreferenceKey.value = `marketing:${key}`;
  marketingFeedback.value[key] = undefined;

  try {
    const result = await updateCustomerNotificationPreference({
      channelCode: item.channelCode,
      contactNo: item.contactNo,
      enabled,
      expectedVersion: preferencesSnapshot.value.version,
      idempotencyKey: operationKey,
      ...(enabled
        ? {
            locale: item.locale,
            policyReference: item.policyReference,
            policyVersion: item.policyVersion,
          }
        : {}),
      marketingConsentAction: enabled ? 'granted' : 'withdrawn',
      topic: 'marketing',
    });
    item.status = enabled ? 'opted_in' : 'opted_out';
    preferencesSnapshot.value.version = result.version;
    const preference = preferencesSnapshot.value.preferences.find(
      (candidate) =>
        candidate.contactNo === item.contactNo &&
        candidate.channelCode === item.channelCode &&
        candidate.topic === 'marketing',
    );
    if (preference) preference.enabled = enabled;
    delete marketingOperationKeys.value[key];
    marketingFeedback.value[key] = 'saved';
  } catch {
    marketingFeedback.value[key] = 'error';
  } finally {
    savingPreferenceKey.value = '';
  }
}

async function toggleMarketingConsent(
  item: CustomerNotificationMarketingConsent,
  checked: boolean | number | string,
) {
  const key = marketingKey(item);
  marketingDrafts.value[key] = checked === true;
  marketingOperationKeys.value[key] = newPreferenceOperationKey();
  await saveMarketingConsent(item);
}

function parseMinute(value: string) {
  const match = /^(\d{2}):(\d{2})$/u.exec(value);
  if (!match) return undefined;
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  if (hours > 23 || minutes > 59) return undefined;
  return hours * 60 + minutes;
}

function isValidTimezone(value: string) {
  const timezoneName = value.trim();
  if (!timezoneName || timezoneName.length > 64) return false;
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: timezoneName }).format();
    return true;
  } catch {
    return false;
  }
}

const quietHoursInputValid = computed(() => {
  const draft = quietHoursDraft.value;
  const startMinute = draft ? parseMinute(draft.startTime) : undefined;
  const endMinute = draft ? parseMinute(draft.endTime) : undefined;
  return Boolean(
    draft &&
    isValidTimezone(draft.timezoneName) &&
    startMinute !== undefined &&
    endMinute !== undefined &&
    (!draft.enabled || startMinute !== endMinute),
  );
});

const quietHoursDirty = computed(() => {
  const snapshot = preferencesSnapshot.value?.quietHours;
  const draft = quietHoursDraft.value;
  return Boolean(
    draft && (!snapshot || !quietHoursDraftMatches(draft, snapshot)),
  );
});

const quietHoursDisabled = computed(
  () =>
    preferencesFailed.value ||
    preferencesLoading.value ||
    Boolean(savingPreferenceKey.value),
);

function updateQuietHoursDraft(
  field: keyof QuietHoursDraft,
  value: boolean | string,
) {
  if (!quietHoursDraft.value) return;
  quietHoursDraft.value = { ...quietHoursDraft.value, [field]: value };
  quietHoursFeedback.value = undefined;
  quietHoursOperationKey.value = newPreferenceOperationKey();
}

async function saveQuietHours() {
  const snapshot = preferencesSnapshot.value;
  const draft = quietHoursDraft.value;
  const startMinute = draft ? parseMinute(draft.startTime) : undefined;
  const endMinute = draft ? parseMinute(draft.endTime) : undefined;
  if (
    !snapshot ||
    !draft ||
    startMinute === undefined ||
    endMinute === undefined ||
    !quietHoursInputValid.value ||
    preferencesFailed.value ||
    preferencesLoading.value ||
    savingPreferenceKey.value
  ) {
    return;
  }

  const operationKey =
    quietHoursOperationKey.value || newPreferenceOperationKey();
  quietHoursOperationKey.value = operationKey;
  quietHoursFeedback.value = undefined;
  savingPreferenceKey.value = 'quiet-hours';
  try {
    const result = await updateCustomerNotificationPreference({
      expectedVersion: snapshot.version,
      idempotencyKey: operationKey,
      quietHours: {
        enabled: draft.enabled,
        endMinute,
        startMinute,
        timezoneName: draft.timezoneName.trim(),
      },
    });
    snapshot.quietHours = {
      crossesMidnight: draft.enabled && startMinute > endMinute,
      enabled: draft.enabled,
      endMinute,
      startMinute,
      timezoneName: draft.timezoneName.trim(),
      version: result.version,
    };
    snapshot.version = result.version;
    quietHoursDraft.value = quietHoursToDraft(snapshot.quietHours);
    quietHoursOperationKey.value = '';
    quietHoursFeedback.value = 'saved';
  } catch {
    quietHoursFeedback.value = 'error';
  } finally {
    savingPreferenceKey.value = '';
  }
}

async function refreshPreferences() {
  await loadPreferences(true);
}

function nextPage() {
  if (!nextCursor.value) return;
  cursorHistory.value.push(cursor.value);
  cursor.value = nextCursor.value;
  void loadDeliveries();
}

function previousPage() {
  const previous = cursorHistory.value.pop();
  cursor.value = previous || '';
  void loadDeliveries();
}

function statusColor(status: NotificationDeliveryStatus) {
  if (status === 'delivered' || status === 'succeeded') return 'success';
  if (status === 'failed' || status === 'template_error') return 'error';
  if (
    status === 'manual_review' ||
    status === 'result_unknown' ||
    status === 'retry_wait'
  ) {
    return 'warning';
  }
  if (status === 'cancelled' || status === 'preference_blocked') {
    return 'default';
  }
  return 'processing';
}

function formatTime(timestamp?: number) {
  return timestamp ? dayjs(timestamp).format('YYYY-MM-DD HH:mm') : '';
}

function formatMinute(value: number) {
  const hours = Math.floor(value / 60)
    .toString()
    .padStart(2, '0');
  const minutes = (value % 60).toString().padStart(2, '0');
  return `${hours}:${minutes}`;
}

function consentStatus(value: string) {
  if (value === 'opted_in') return $t('notifications.consentOptedIn');
  if (value === 'opted_out') return $t('notifications.consentOptedOut');
  return $t('notifications.consentUnknown');
}

onMounted(() => {
  void loadDeliveries();
  void loadPreferences();
});
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('notifications.menu')">
    <Tabs default-active-key="deliveries">
      <TabPane key="deliveries" :tab="$t('notifications.deliveryHistory')">
        <Card :bordered="false">
          <div class="mb-5 text-sm text-muted-foreground">
            {{ $t('notifications.pageNumber', { page: pageNumber }) }}
          </div>
          <Alert
            v-if="failed"
            class="mb-4"
            :message="$t('notifications.loadFailed')"
            show-icon
            type="error"
          >
            <template #action>
              <Button size="small" @click="loadDeliveries">
                {{ $t('notifications.retry') }}
              </Button>
            </template>
          </Alert>
          <Spin :spinning="loading">
            <Empty
              v-if="items.length === 0 && !loading"
              :description="$t('notifications.empty')"
            />
            <div v-else class="grid gap-3" data-testid="notification-list">
              <article
                v-for="item in items"
                :key="item.taskNo"
                class="notification-card"
              >
                <div class="min-w-0 flex-1">
                  <strong class="block">
                    {{ $t(`notifications.categories.${item.topic}`) }}
                  </strong>
                  <span class="mt-1 block text-sm text-muted-foreground">
                    {{ $t(`notifications.channels.${item.channelCode}`) }}
                    · {{ $t('notifications.queuedAt') }}
                    {{ formatTime(item.queuedAt) }}
                  </span>
                  <span
                    v-if="item.completedAt"
                    class="mt-1 block text-xs text-muted-foreground"
                  >
                    {{ $t('notifications.completedAt') }}
                    {{ formatTime(item.completedAt) }}
                  </span>
                </div>
                <Tag :color="statusColor(item.status)">
                  {{ $t(`notifications.statuses.${item.status}`) }}
                </Tag>
              </article>
            </div>
          </Spin>
          <div
            v-if="cursorHistory.length > 0 || nextCursor"
            class="mt-5 flex justify-end gap-3"
          >
            <Button
              :disabled="cursorHistory.length === 0"
              @click="previousPage"
            >
              {{ $t('notifications.previousPage') }}
            </Button>
            <Button :disabled="!nextCursor" @click="nextPage">
              {{ $t('notifications.nextPage') }}
            </Button>
          </div>
        </Card>
      </TabPane>

      <TabPane key="preferences" :tab="$t('notifications.preferences')">
        <Card :bordered="false">
          <Alert
            class="mb-5"
            :description="$t('notifications.preferencesDescription')"
            :message="$t('notifications.preferencesTitle')"
            show-icon
            type="info"
          />
          <Alert
            v-if="preferencesFailed"
            class="mb-4"
            :message="$t('notifications.preferencesLoadFailed')"
            show-icon
            type="error"
          >
            <template #action>
              <Button size="small" @click="refreshPreferences">
                {{ $t('notifications.retry') }}
              </Button>
            </template>
          </Alert>
          <Spin :spinning="preferencesLoading">
            <template v-if="preferencesSnapshot">
              <div class="mb-4 flex justify-end">
                <Button
                  data-testid="notification-preferences-refresh"
                  :disabled="Boolean(savingPreferenceKey)"
                  :loading="preferencesLoading"
                  @click="refreshPreferences"
                >
                  {{ $t('notifications.refreshLatest') }}
                </Button>
              </div>
              <div class="mb-5 grid gap-3 sm:grid-cols-2">
                <div class="preference-summary">
                  <span>{{ $t('notifications.preferredLocale') }}</span>
                  <strong>{{ preferencesSnapshot.preferredLocale }}</strong>
                </div>
                <div class="preference-summary">
                  <span>{{ $t('notifications.preferenceVersion') }}</span>
                  <strong>{{ preferencesSnapshot.version }}</strong>
                </div>
              </div>

              <h3 class="mb-3 text-base font-semibold">
                {{ $t('notifications.channelChoices') }}
              </h3>
              <Empty
                v-if="preferencesSnapshot.preferences.length === 0"
                :description="$t('notifications.preferencesEmpty')"
              />
              <div
                v-else
                class="grid gap-3"
                data-testid="notification-preferences"
              >
                <article
                  v-for="item in preferencesSnapshot.preferences"
                  :key="`${item.contactNo}:${item.topic}:${item.channelCode}`"
                  class="notification-card preference-card items-center"
                  :data-testid="`notification-preference-${item.topic}-${item.channelCode}`"
                >
                  <div class="preference-card__summary">
                    <strong>
                      {{ $t(`notifications.categories.${item.topic}`) }}
                    </strong>
                    <span class="mt-1 block text-sm text-muted-foreground">
                      {{ $t(`notifications.channels.${item.channelCode}`) }}
                    </span>
                    <span
                      v-if="item.topic === 'security'"
                      class="mt-2 block text-xs text-muted-foreground"
                    >
                      {{ $t('notifications.securityRequired') }}
                    </span>
                    <span
                      v-else-if="item.topic === 'marketing'"
                      class="mt-2 block text-xs text-muted-foreground"
                    >
                      {{ $t('notifications.marketingEditingUnavailable') }}
                    </span>
                  </div>
                  <Switch
                    :aria-label="
                      $t('notifications.preferenceToggleLabel', {
                        category: $t(`notifications.categories.${item.topic}`),
                        channel: $t(
                          `notifications.channels.${item.channelCode}`,
                        ),
                      })
                    "
                    :checked="preferenceEnabled(item)"
                    :disabled="preferenceDisabled(item)"
                    :loading="savingPreferenceKey === preferenceKey(item)"
                    @change="(checked) => togglePreference(item, checked)"
                  />
                  <div
                    v-if="preferenceFeedback[preferenceKey(item)]"
                    aria-live="polite"
                    class="preference-feedback"
                  >
                    <span
                      v-if="preferenceFeedback[preferenceKey(item)] === 'saved'"
                      class="text-success"
                    >
                      {{ $t('notifications.preferenceSaved') }}
                    </span>
                    <template v-else>
                      <span class="text-error">
                        {{
                          $t(
                            preferenceFeedback[preferenceKey(item)] === 'retry'
                              ? 'notifications.preferenceRetryReady'
                              : 'notifications.preferenceSaveFailed',
                          )
                        }}
                      </span>
                      <div class="mt-2 flex flex-wrap gap-2">
                        <Button size="small" @click="refreshPreferences">
                          {{ $t('notifications.refreshLatest') }}
                        </Button>
                        <Button
                          v-if="
                            preferenceFeedback[preferenceKey(item)] === 'retry'
                          "
                          size="small"
                          type="primary"
                          @click="savePreference(item)"
                        >
                          {{ $t('notifications.saveAgain') }}
                        </Button>
                      </div>
                    </template>
                  </div>
                </article>
              </div>

              <h3 class="mt-6 mb-3 text-base font-semibold">
                {{ $t('notifications.marketingConsents') }}
              </h3>
              <template v-if="preferencesSnapshot.marketingConsents.length > 0">
                <Alert
                  class="mb-3"
                  :description="$t('notifications.marketingDescription')"
                  :message="$t('notifications.marketingTitle')"
                  show-icon
                  type="info"
                />
                <div class="grid gap-3" data-testid="notification-consents">
                  <article
                    v-for="item in preferencesSnapshot.marketingConsents"
                    :key="`${item.contactNo}:${item.channelCode}`"
                    class="notification-card preference-card items-center"
                    :data-testid="`notification-consent-${item.channelCode}-${item.contactNo}`"
                  >
                    <div class="preference-card__summary">
                      <strong>
                        {{ $t(`notifications.channels.${item.channelCode}`) }}
                      </strong>
                      <span class="mt-1 block text-sm text-muted-foreground">
                        {{ consentStatus(item.status) }}
                      </span>
                      <span
                        v-if="!isMarketingConsentComplete(item)"
                        class="mt-2 block text-xs text-muted-foreground"
                      >
                        {{ $t('notifications.marketingEvidenceMissing') }}
                      </span>
                    </div>
                    <Switch
                      :aria-label="
                        $t('notifications.marketingToggleLabel', {
                          channel: $t(
                            `notifications.channels.${item.channelCode}`,
                          ),
                        })
                      "
                      :checked="marketingEnabled(item)"
                      :disabled="marketingDisabled(item)"
                      :loading="
                        savingPreferenceKey ===
                        `marketing:${marketingKey(item)}`
                      "
                      @change="
                        (checked) => toggleMarketingConsent(item, checked)
                      "
                    />
                    <div
                      v-if="marketingFeedback[marketingKey(item)]"
                      aria-live="polite"
                      class="preference-feedback"
                    >
                      <span
                        v-if="marketingFeedback[marketingKey(item)] === 'saved'"
                        class="text-success"
                      >
                        {{ $t('notifications.preferenceSaved') }}
                      </span>
                      <template v-else>
                        <span class="text-error">
                          {{
                            $t(
                              marketingFeedback[marketingKey(item)] === 'retry'
                                ? 'notifications.preferenceRetryReady'
                                : 'notifications.preferenceSaveFailed',
                            )
                          }}
                        </span>
                        <div class="mt-2 flex flex-wrap gap-2">
                          <Button size="small" @click="refreshPreferences">
                            {{ $t('notifications.refreshLatest') }}
                          </Button>
                          <Button
                            v-if="
                              marketingFeedback[marketingKey(item)] === 'retry'
                            "
                            size="small"
                            type="primary"
                            @click="saveMarketingConsent(item)"
                          >
                            {{ $t('notifications.saveAgain') }}
                          </Button>
                        </div>
                      </template>
                    </div>
                  </article>
                </div>
              </template>
              <Empty
                v-else
                :description="$t('notifications.marketingEvidenceMissing')"
              />

              <h3 class="mt-6 mb-3 text-base font-semibold">
                {{ $t('notifications.quietHours') }}
              </h3>
              <template v-if="quietHoursDraft">
                <Alert
                  v-if="quietHoursUnavailableReason"
                  class="mb-4"
                  :message="$t(quietHoursUnavailableReason)"
                  show-icon
                  type="warning"
                />
                <div
                  class="quiet-hours-card"
                  data-testid="notification-quiet-hours"
                >
                  <div class="quiet-hours-fields">
                    <label for="notification-quiet-hours-timezone">
                      {{ $t('notifications.quietHoursTimezone') }}
                    </label>
                    <Input
                      id="notification-quiet-hours-timezone"
                      data-testid="notification-quiet-hours-timezone"
                      :disabled="quietHoursDisabled"
                      :value="quietHoursDraft.timezoneName"
                      @update:value="
                        (value) => updateQuietHoursDraft('timezoneName', value)
                      "
                    />
                    <label for="notification-quiet-hours-start">
                      {{ $t('notifications.quietHoursStart') }}
                    </label>
                    <Input
                      id="notification-quiet-hours-start"
                      data-testid="notification-quiet-hours-start"
                      :disabled="quietHoursDisabled"
                      type="time"
                      :value="quietHoursDraft.startTime"
                      @update:value="
                        (value) => updateQuietHoursDraft('startTime', value)
                      "
                    />
                    <label for="notification-quiet-hours-end">
                      {{ $t('notifications.quietHoursEnd') }}
                    </label>
                    <Input
                      id="notification-quiet-hours-end"
                      data-testid="notification-quiet-hours-end"
                      :disabled="quietHoursDisabled"
                      type="time"
                      :value="quietHoursDraft.endTime"
                      @update:value="
                        (value) => updateQuietHoursDraft('endTime', value)
                      "
                    />
                  </div>
                  <div class="mt-4 flex flex-wrap items-center gap-3">
                    <Switch
                      :aria-label="$t('notifications.quietHoursEnabledLabel')"
                      :checked="quietHoursDraft.enabled"
                      :disabled="quietHoursDisabled"
                      @change="
                        (checked) =>
                          updateQuietHoursDraft('enabled', checked === true)
                      "
                    />
                    <span class="text-sm">
                      {{
                        $t(
                          quietHoursDraft.enabled
                            ? 'notifications.enabled'
                            : 'notifications.disabled',
                        )
                      }}
                    </span>
                    <Button
                      data-testid="notification-quiet-hours-save"
                      :disabled="
                        quietHoursDisabled ||
                        !quietHoursDirty ||
                        !quietHoursInputValid
                      "
                      :loading="savingPreferenceKey === 'quiet-hours'"
                      type="primary"
                      @click="saveQuietHours"
                    >
                      {{ $t('notifications.saveQuietHours') }}
                    </Button>
                  </div>
                  <span
                    v-if="!quietHoursInputValid"
                    class="text-error mt-3 block text-sm"
                  >
                    {{ $t('notifications.quietHoursInvalid') }}
                  </span>
                  <div
                    v-if="quietHoursFeedback"
                    aria-live="polite"
                    class="mt-3 text-sm"
                  >
                    <span
                      v-if="quietHoursFeedback === 'saved'"
                      class="text-success"
                    >
                      {{ $t('notifications.preferenceSaved') }}
                    </span>
                    <template v-else>
                      <span class="text-error">
                        {{
                          $t(
                            quietHoursFeedback === 'retry'
                              ? 'notifications.preferenceRetryReady'
                              : 'notifications.preferenceSaveFailed',
                          )
                        }}
                      </span>
                      <div class="mt-2 flex flex-wrap gap-2">
                        <Button size="small" @click="refreshPreferences">
                          {{ $t('notifications.refreshLatest') }}
                        </Button>
                        <Button
                          v-if="quietHoursFeedback === 'retry'"
                          size="small"
                          type="primary"
                          @click="saveQuietHours"
                        >
                          {{ $t('notifications.saveAgain') }}
                        </Button>
                      </div>
                    </template>
                  </div>
                </div>
              </template>
              <Alert
                v-else
                :message="$t('notifications.quietHoursMissing')"
                show-icon
                type="warning"
              />
            </template>
          </Spin>
        </Card>
      </TabPane>
    </Tabs>
  </Page>
</template>

<style scoped>
.notification-card {
  display: flex;
  align-items: flex-start;
  gap: 0.875rem;
  padding: 1rem;
  background: color-mix(in srgb, hsl(var(--background)) 97%, hsl(var(--primary)) 3%);
  border: 1px solid hsl(var(--border));
  border-radius: 0.75rem;
}

.preference-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem;
  background: hsl(var(--muted));
  border-radius: 0.75rem;
}

.preference-feedback {
  flex: 0 0 100%;
  font-size: 0.8125rem;
}

.preference-card {
  flex-wrap: wrap;
}

.preference-card__summary {
  flex: 1 1 16rem;
  min-width: 0;
}

.quiet-hours-card {
  padding: 1rem;
  background: color-mix(in srgb, hsl(var(--background)) 97%, hsl(var(--primary)) 3%);
  border: 1px solid hsl(var(--border));
  border-radius: 0.75rem;
}

.quiet-hours-fields {
  display: grid;
  grid-template-columns: minmax(8rem, auto) minmax(0, 1fr);
  gap: 0.75rem 1rem;
  align-items: center;
}

@media (max-width: 640px) {
  .notification-card {
    align-items: flex-start;
    flex-direction: column;
  }

  .quiet-hours-fields {
    grid-template-columns: minmax(0, 1fr);
    gap: 0.5rem;
  }
}
</style>

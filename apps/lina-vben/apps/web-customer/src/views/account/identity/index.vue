<script setup lang="ts">
import type { VbenFormSchema } from '#/adapter/form';
import type {
  PersonalVerificationItem,
  PersonalVerificationStatus,
} from '#/api/idc/personal-verification';

import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

import { Page } from '@vben/common-ui';
import { preferences } from '@vben/preferences';

import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  DescriptionsItem,
  Skeleton,
  Space,
  Tag,
} from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { resumePaymentPreparation } from '#/api/idc/customer-commerce';
import {
  createPersonalVerificationConsent,
  getPersonalVerification,
  refreshPersonalVerification,
  startPersonalVerification,
} from '#/api/idc/personal-verification';
import { $t } from '#/locales';

type FlowContext = {
  paymentContext: string;
  returnTo: string;
};

const identityFlowStorageKey =
  'lina-web-customer.personal-verification-flow.v1';
const consentStoragePrefix = 'lina-web-customer.personal-verification-consent.';

const props = withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
});
const pageShell = computed(() => (props.embedded ? 'div' : Page));

const route = useRoute();
const loading = ref(true);
const busy = ref(false);
const loadFailed = ref(false);
const forbidden = ref(false);
const invalidInput = ref(false);
const accepted = ref(false);
const policyVersion = ref('');
const verification = ref<PersonalVerificationItem>();
const providerReturnChecked = ref(false);
const paymentReady = ref<boolean>();
const paymentResumeFailed = ref(false);
const paymentResumeAttempted = ref(false);
const flowContext = ref(readFlowContext());

const locale = computed<'en-US' | 'zh-CN'>(() =>
  preferences.app.locale === 'en-US' ? 'en-US' : 'zh-CN',
);
const browserReturned = computed(() => route.query.verification_return === '1');
const status = computed<PersonalVerificationStatus>(
  () => verification.value?.statusCode || 'unverified',
);
const showForm = computed(() =>
  ['expired', 'rejected', 'unverified'].includes(status.value),
);
const canStart = computed(
  () => accepted.value && Boolean(policyVersion.value) && !busy.value,
);
const continueHref = computed(
  () => flowContext.value.returnTo || '/account/orders',
);

function createIdentitySchema(): VbenFormSchema[] {
  return [
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        'data-testid': 'identity-name',
        maxlength: 64,
        placeholder: $t('identity.form.namePlaceholder'),
      },
      fieldName: 'certName',
      label: $t('identity.form.name'),
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        'data-testid': 'identity-number',
        maxlength: 18,
        placeholder: $t('identity.form.numberPlaceholder'),
      },
      fieldName: 'certNo',
      label: $t('identity.form.number'),
      rules: 'required',
    },
  ];
}

const [IdentityForm, formApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    labelWidth: 120,
  },
  schema: [],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1',
});

function safeReturnTo(value: null | string) {
  if (!value || value.length > 180 || /[?#\u0000-\u001f]/u.test(value)) {
    return '';
  }
  if (value === '/account/cart' || value === '/account/orders') return value;
  return /^\/account\/orders\/[A-Za-z0-9_-]{1,64}$/u.test(value) ? value : '';
}

function safePaymentContext(value: null | string) {
  return value && /^[A-Za-z0-9_-]{1,64}$/u.test(value) ? value : '';
}

function readFlowContext(): FlowContext {
  const queryReturnTo = safeReturnTo(
    typeof route.query.return_to === 'string' ? route.query.return_to : null,
  );
  const queryPaymentContext = safePaymentContext(
    typeof route.query.payment_context === 'string'
      ? route.query.payment_context
      : null,
  );
  if (queryReturnTo || queryPaymentContext) {
    const value = {
      paymentContext: queryPaymentContext,
      returnTo: queryReturnTo || '/account/orders',
    };
    sessionStorage.setItem(identityFlowStorageKey, JSON.stringify(value));
    return value;
  }
  try {
    const stored = JSON.parse(
      sessionStorage.getItem(identityFlowStorageKey) || '{}',
    ) as Partial<FlowContext>;
    return {
      paymentContext: safePaymentContext(stored.paymentContext || null),
      returnTo: safeReturnTo(stored.returnTo || null) || '/account/orders',
    };
  } catch {
    sessionStorage.removeItem(identityFlowStorageKey);
    return { paymentContext: '', returnTo: '/account/orders' };
  }
}

function persistFlowContext() {
  if (
    flowContext.value.paymentContext ||
    flowContext.value.returnTo !== '/account/orders'
  ) {
    sessionStorage.setItem(
      identityFlowStorageKey,
      JSON.stringify(flowContext.value),
    );
  }
}

function clearFlowContext() {
  sessionStorage.removeItem(identityFlowStorageKey);
}

function consentActionKey(version: string) {
  const storageKey = `${consentStoragePrefix}${version}`;
  const existing = sessionStorage.getItem(storageKey);
  if (existing) return existing;
  const value = `personal-consent-${crypto.randomUUID()}`;
  sessionStorage.setItem(storageKey, value);
  return value;
}

function clearConsentActionKey(version: string) {
  sessionStorage.removeItem(`${consentStoragePrefix}${version}`);
}

function responseStatus(error: unknown) {
  return (error as { response?: { status?: number } })?.response?.status || 0;
}

function handleError(error: unknown) {
  forbidden.value = responseStatus(error) === 403;
  loadFailed.value = true;
}

function statusColor(value: PersonalVerificationStatus) {
  if (value === 'verified') return 'success';
  if (value === 'review_required' || value === 'verifying') return 'warning';
  if (value === 'rejected') return 'error';
  return 'default';
}

function formatTime(value?: number) {
  if (!value) return '—';
  return new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value));
}

function validIdentityInput(certName: string, certNo: string) {
  return (
    certName.trim().length > 0 && /^[0-9]{17}[0-9Xx]$/u.test(certNo.trim())
  );
}

function safeContinuation(value: string) {
  try {
    const parsed = new URL(value);
    return parsed.protocol === 'https:' && Boolean(parsed.hostname)
      ? parsed.toString()
      : '';
  } catch {
    return '';
  }
}

async function loadLocalState() {
  const state = await getPersonalVerification();
  verification.value = state.item || undefined;
  policyVersion.value = state.policyVersion;
}

async function maybeResumePayment() {
  if (
    verification.value?.statusCode !== 'verified' ||
    !flowContext.value.paymentContext ||
    paymentResumeAttempted.value
  ) {
    return;
  }
  paymentResumeAttempted.value = true;
  try {
    const result = await resumePaymentPreparation(
      flowContext.value.paymentContext,
    );
    paymentReady.value = result.preparation.ready;
  } catch {
    paymentResumeFailed.value = true;
  }
}

async function load() {
  loading.value = true;
  loadFailed.value = false;
  forbidden.value = false;
  try {
    await loadLocalState();
    if (
      browserReturned.value &&
      verification.value?.statusCode === 'verifying' &&
      verification.value.attemptNo
    ) {
      busy.value = true;
      const result = await refreshPersonalVerification(
        verification.value.attemptNo,
      );
      verification.value = result.item;
      providerReturnChecked.value = true;
    }
    await maybeResumePayment();
  } catch (error) {
    handleError(error);
  } finally {
    busy.value = false;
    loading.value = false;
  }
}

async function queryAuthoritativeState() {
  const attemptNo = verification.value?.attemptNo;
  if (!attemptNo || busy.value) return;
  busy.value = true;
  loadFailed.value = false;
  try {
    const result = await refreshPersonalVerification(attemptNo);
    verification.value = result.item;
    providerReturnChecked.value = true;
    await maybeResumePayment();
  } catch (error) {
    handleError(error);
  } finally {
    busy.value = false;
  }
}

async function start() {
  if (!canStart.value) return;
  const { valid } = await formApi.validate();
  const values = await formApi.getValues<{
    certName: string;
    certNo: string;
  }>();
  if (!valid || !validIdentityInput(values.certName, values.certNo)) {
    invalidInput.value = true;
    return;
  }

  invalidInput.value = false;
  loadFailed.value = false;
  busy.value = true;
  let submittedName = values.certName.trim();
  let submittedNumber = values.certNo.trim().toUpperCase();
  await formApi.resetForm();
  try {
    const actionKey = consentActionKey(policyVersion.value);
    const consent = await createPersonalVerificationConsent({
      idempotencyKey: actionKey,
      locale: locale.value,
      policyVersion: policyVersion.value,
    });
    const result = await startPersonalVerification({
      certName: submittedName,
      certNo: submittedNumber,
      consentNo: consent.consentNo,
    });
    verification.value = result.item;
    clearConsentActionKey(policyVersion.value);
    accepted.value = false;
    persistFlowContext();
    const continuation = safeContinuation(result.item.continuationUrl);
    if (!continuation) {
      throw new Error('unsafe personal verification continuation');
    }
    window.location.assign(continuation);
  } catch (error) {
    handleError(error);
    try {
      await loadLocalState();
    } catch {
      // Keep the original bounded customer-facing failure state.
    }
  } finally {
    submittedName = '';
    submittedNumber = '';
    busy.value = false;
  }
}

onMounted(() => {
  formApi.setState({ schema: createIdentitySchema() });
  void load();
});
</script>

<template>
  <component
    :is="pageShell"
    data-testid="identity-center"
    :description="$t('identity.lead')"
    :title="$t('identity.title')"
  >
    <div class="identity-shell">
      <Card class="identity-status" :bordered="false">
        <div class="identity-status__main">
          <div>
            <div class="identity-eyebrow">{{ $t('identity.account') }}</div>
            <h2>{{ $t('identity.status.title') }}</h2>
          </div>
          <Tag :color="statusColor(status)" data-testid="identity-status">
            {{ $t(`identity.status.${status}`) }}
          </Tag>
        </div>
        <p>{{ $t('identity.status.hint') }}</p>
      </Card>

      <Alert
        v-if="browserReturned && busy"
        :message="$t('identity.returnChecking')"
        show-icon
        type="info"
      />
      <Alert
        v-else-if="
          browserReturned && providerReturnChecked && status === 'verifying'
        "
        :message="$t('identity.returnPending')"
        show-icon
        type="warning"
      />
      <Alert
        v-if="invalidInput"
        :message="$t('identity.form.invalid')"
        show-icon
        type="error"
      />
      <Alert
        v-if="loadFailed"
        :message="
          forbidden ? $t('identity.personalOnly') : $t('identity.loadFailed')
        "
        show-icon
        type="error"
      >
        <template #action>
          <Button size="small" @click="load">
            {{ $t('identity.reload') }}
          </Button>
        </template>
      </Alert>

      <Skeleton v-if="loading" active :paragraph="{ rows: 10 }" />

      <template v-else-if="!loadFailed">
        <Card
          v-if="status === 'verified'"
          class="identity-result identity-result--verified"
          :bordered="false"
          data-testid="identity-verified"
        >
          <div class="identity-result__heading">
            <div>
              <div class="identity-eyebrow">
                {{ $t('identity.status.verified') }}
              </div>
              <h2>{{ $t('identity.verified.title') }}</h2>
              <p>{{ $t('identity.verified.description') }}</p>
            </div>
            <span aria-hidden="true" class="identity-check">✓</span>
          </div>
          <Descriptions :column="{ md: 3, xs: 1 }" size="small">
            <DescriptionsItem :label="$t('identity.verified.name')">
              {{ verification?.maskedName || '—' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('identity.verified.number')">
              {{ verification?.maskedIdentityNo || '—' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('identity.verified.time')">
              {{ formatTime(verification?.verifiedAt) }}
            </DescriptionsItem>
          </Descriptions>
          <Alert
            v-if="paymentResumeFailed"
            class="mt-5"
            :message="$t('identity.payment.failed')"
            show-icon
            type="warning"
          />
          <Alert
            v-else-if="paymentReady !== undefined"
            class="mt-5"
            :message="
              paymentReady
                ? $t('identity.payment.ready')
                : $t('identity.payment.pending')
            "
            show-icon
            :type="paymentReady ? 'success' : 'warning'"
          />
          <Space class="mt-5" wrap>
            <Button
              :href="continueHref"
              type="primary"
              @click="clearFlowContext"
            >
              {{
                flowContext.paymentContext
                  ? $t('identity.payment.continue')
                  : $t('identity.payment.orders')
              }}
            </Button>
          </Space>
        </Card>

        <Card
          v-else-if="status === 'verifying'"
          class="identity-result"
          :bordered="false"
        >
          <h2>{{ $t('identity.verifying.title') }}</h2>
          <p>{{ $t('identity.returnPending') }}</p>
          <Button
            :loading="busy"
            type="primary"
            @click="queryAuthoritativeState"
          >
            {{
              busy
                ? $t('identity.verifying.querying')
                : $t('identity.verifying.query')
            }}
          </Button>
        </Card>

        <Alert
          v-else-if="status === 'review_required'"
          :description="$t('identity.review.description')"
          :message="$t('identity.review.title')"
          show-icon
          type="warning"
        />

        <template v-else-if="showForm">
          <Alert
            v-if="status === 'rejected' || status === 'expired'"
            :description="$t('identity.retry.description')"
            :message="$t('identity.retry.title')"
            show-icon
            type="info"
          />
          <div class="identity-grid">
            <Card :bordered="false">
              <h2>{{ $t('identity.form.title') }}</h2>
              <p class="identity-muted">{{ $t('identity.form.lead') }}</p>
              <IdentityForm />
              <Alert
                :message="$t('identity.form.safety')"
                show-icon
                type="info"
              />
            </Card>

            <Card :bordered="false">
              <div class="identity-consent-heading">
                <div>
                  <h2>{{ $t('identity.consent.title') }}</h2>
                  <span>
                    {{ $t('identity.consent.version') }} {{ policyVersion }}
                  </span>
                </div>
                <span aria-hidden="true">01</span>
              </div>
              <dl class="identity-notice">
                <div>
                  <dt>{{ $t('identity.consent.purposeTitle') }}</dt>
                  <dd>{{ $t('identity.consent.purpose') }}</dd>
                </div>
                <div>
                  <dt>{{ $t('identity.consent.dataTitle') }}</dt>
                  <dd>{{ $t('identity.consent.data') }}</dd>
                </div>
                <div>
                  <dt>{{ $t('identity.consent.recipientTitle') }}</dt>
                  <dd>{{ $t('identity.consent.recipient') }}</dd>
                </div>
                <div>
                  <dt>{{ $t('identity.consent.storageTitle') }}</dt>
                  <dd>{{ $t('identity.consent.storage') }}</dd>
                </div>
                <div>
                  <dt>{{ $t('identity.consent.rightsTitle') }}</dt>
                  <dd>{{ $t('identity.consent.rights') }}</dd>
                </div>
              </dl>
              <Checkbox
                v-model:checked="accepted"
                class="identity-consent"
                data-testid="identity-consent"
              >
                {{ $t('identity.consent.accept') }}
              </Checkbox>
              <Button
                block
                data-testid="identity-start"
                :disabled="!canStart"
                :loading="busy"
                type="primary"
                @click="start"
              >
                {{
                  busy
                    ? $t('identity.form.starting')
                    : $t('identity.form.start')
                }}
              </Button>
            </Card>
          </div>
        </template>
      </template>
    </div>
  </component>
</template>

<style scoped>
.identity-shell {
  display: grid;
  gap: 16px;
  max-width: 100%;
  margin: 0 auto;
}

.identity-status {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
}

.identity-status__main,
.identity-result__heading,
.identity-consent-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.identity-status h2,
.identity-result h2,
.identity-grid h2 {
  margin: 4px 0 8px;
  font-size: 18px;
  font-weight: 650;
}

.identity-status p,
.identity-result p,
.identity-muted {
  margin: 0;
  color: hsl(var(--muted-foreground));
  line-height: 1.65;
}

.identity-eyebrow {
  color: hsl(var(--primary));
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.identity-result--verified {
  border-left: 4px solid #16a34a;
}

.identity-check {
  display: grid;
  width: 44px;
  height: 44px;
  flex: 0 0 auto;
  place-items: center;
  color: #fff;
  font-size: 24px;
  border-radius: 50%;
  background: #16a34a;
}

.identity-grid {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
  gap: 16px;
}

.identity-consent-heading > span {
  color: hsl(var(--primary));
  font-size: 24px;
  font-weight: 750;
  line-height: 1;
}

.identity-consent-heading div > span {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.identity-notice {
  display: grid;
  gap: 12px;
  margin: 18px 0;
}

.identity-notice div {
  padding-left: 12px;
  border-left: 2px solid hsl(var(--primary) / 30%);
}

.identity-notice dt {
  margin-bottom: 3px;
  font-size: 13px;
  font-weight: 650;
}

.identity-notice dd {
  margin: 0;
  color: hsl(var(--muted-foreground));
  font-size: 13px;
  line-height: 1.65;
}

.identity-consent {
  align-items: flex-start;
  margin-bottom: 16px;
  line-height: 1.65;
}

@media (max-width: 900px) {
  .identity-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 520px) {
  .identity-status__main,
  .identity-result__heading {
    flex-direction: column;
  }

  .identity-check {
    order: -1;
  }
}
</style>

<script setup lang="ts">
import type { VbenFormSchema } from '#/adapter/form';
import type {
  EnterpriseCaseStatus,
  EnterpriseMaterial,
  EnterpriseState,
  EnterpriseSubmissionInput,
} from '#/api/idc/enterprise-verification';
import type { UploadProps } from 'ant-design-vue';

import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';
import { preferences } from '@vben/preferences';

import {
  Alert,
  Button,
  Card,
  Checkbox,
  CheckboxGroup,
  Descriptions,
  DescriptionsItem,
  Select,
  Skeleton,
  Space,
  Steps,
  Tag,
  Upload,
  message,
} from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import {
  createEnterpriseLegalChange,
  createEnterpriseVerificationConsent,
  createEnterpriseVerificationDraft,
  getEnterpriseVerification,
  submitEnterpriseVerification,
  uploadEnterpriseMaterial,
} from '#/api/idc/enterprise-verification';
import { $t } from '#/locales';

const UploadDragger = Upload.Dragger;
const actionStoragePrefix = 'lina-web-customer.enterprise-verification-action.';
const maxMaterialBytes = 5 * 1024 * 1024;
const acceptedMaterialTypes = ['application/pdf', 'image/jpeg', 'image/png'];
const evidenceTypes = [
  'business_license',
  'legal_representative_id',
  'agent_id',
  'authorization_letter',
  'organization_certificate',
  'other',
] as const;

const props = withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
});
const pageShell = computed(() => (props.embedded ? 'div' : Page));

const loading = ref(true);
const personalCustomer = ref(false);
const failed = ref(false);
const forbidden = ref(false);
const busy = ref('');
const uploading = ref('');
const invalid = ref(false);
const accepted = ref(false);
const state = ref<EnterpriseState>();
const changeFields = ref<string[]>([]);
const changeReason = ref<string>();
const applicantRole = ref<'authorized_agent' | 'legal_representative'>(
  'legal_representative',
);

const locale = computed<'en-US' | 'zh-CN'>(() =>
  preferences.app.locale === 'en-US' ? 'en-US' : 'zh-CN',
);
const profileVerified = computed(
  () => state.value?.profile?.verificationStatusCode === 'verified',
);
const currentStatus = computed<EnterpriseCaseStatus | 'unverified'>(
  () =>
    state.value?.case?.statusCode ||
    (profileVerified.value ? 'verified' : 'unverified'),
);
const draft = computed(() => currentStatus.value === 'draft');
const underReview = computed(() =>
  ['pending', 'review_required', 'verifying'].includes(currentStatus.value),
);
const beginAvailable = computed(() => {
  if (draft.value || underReview.value) return false;
  if (profileVerified.value) return Boolean(state.value?.activeLegalChange);
  return ['expired', 'rejected', 'unverified'].includes(currentStatus.value);
});
const materialByType = computed(() => {
  const result = new Map<string, EnterpriseMaterial>();
  for (const item of state.value?.case?.materials || []) {
    if (item.statusCode === 'available') {
      result.set(item.materialTypeCode, item);
    }
  }
  return result;
});
const stepIndex = computed(() => {
  if (underReview.value || currentStatus.value === 'verified') return 2;
  return draft.value ? 1 : 0;
});

function createLegalSchema(): VbenFormSchema[] {
  return [
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        'data-testid': 'enterprise-legal-name',
        maxlength: 128,
      },
      fieldName: 'legalName',
      label: $t('enterpriseVerification.form.legalName'),
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        'data-testid': 'enterprise-registration-id',
        maxlength: 18,
      },
      fieldName: 'registrationIdentifier',
      help: $t('enterpriseVerification.form.registrationHint'),
      label: $t('enterpriseVerification.form.registrationId'),
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        'data-testid': 'enterprise-representative-name',
        maxlength: 128,
      },
      fieldName: 'legalRepresentativeName',
      label: $t('enterpriseVerification.form.representative'),
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        'data-testid': 'enterprise-representative-id',
        maxlength: 128,
      },
      fieldName: 'legalRepresentativeIdentityNumber',
      label: $t('enterpriseVerification.form.representativeId'),
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        maxlength: 64,
      },
      fieldName: 'industryCode',
      label: $t('enterpriseVerification.form.industry'),
    },
    {
      component: 'Select',
      componentProps: {
        'data-testid': 'enterprise-applicant-role',
        onChange: (value: 'authorized_agent' | 'legal_representative') => {
          applicantRole.value = value;
        },
        options: [
          {
            label: $t('enterpriseVerification.form.representativeRole'),
            value: 'legal_representative',
          },
          {
            label: $t('enterpriseVerification.form.agentRole'),
            value: 'authorized_agent',
          },
        ],
      },
      defaultValue: 'legal_representative',
      fieldName: 'applicantRoleCode',
      label: $t('enterpriseVerification.form.applicantRole'),
      rules: 'selectRequired',
    },
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        'data-testid': 'enterprise-agent-name',
        maxlength: 128,
      },
      dependencies: {
        show: (values) => values.applicantRoleCode === 'authorized_agent',
        triggerFields: ['applicantRoleCode'],
      },
      fieldName: 'agentName',
      label: $t('enterpriseVerification.form.agentName'),
    },
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        'data-testid': 'enterprise-agent-id',
        maxlength: 128,
      },
      dependencies: {
        show: (values) => values.applicantRoleCode === 'authorized_agent',
        triggerFields: ['applicantRoleCode'],
      },
      fieldName: 'agentIdentityNumber',
      label: $t('enterpriseVerification.form.agentId'),
    },
  ];
}

const [LegalForm, formApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    labelWidth: 132,
  },
  schema: [],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1 md:grid-cols-2',
});

function safeAction(value: string) {
  return value.replace(/[^A-Za-z0-9_.:-]/gu, '-').slice(0, 100);
}

function actionKey(action: string) {
  const storageKey = `${actionStoragePrefix}${safeAction(action)}`;
  const existing = sessionStorage.getItem(storageKey);
  if (existing) return existing;
  const value = `enterprise-${crypto.randomUUID()}`;
  sessionStorage.setItem(storageKey, value);
  return value;
}

function clearActionKey(action: string) {
  sessionStorage.removeItem(`${actionStoragePrefix}${safeAction(action)}`);
}

function responseStatus(error: unknown) {
  return (error as { response?: { status?: number } })?.response?.status || 0;
}

function handleError(error: unknown) {
  forbidden.value = responseStatus(error) === 403;
  failed.value = true;
}

function statusColor(status: EnterpriseCaseStatus | 'unverified') {
  if (status === 'verified') return 'success';
  if (status === 'verifying') return 'processing';
  if (['draft', 'pending', 'review_required'].includes(status))
    return 'warning';
  if (['expired', 'rejected'].includes(status)) return 'error';
  return 'default';
}

function formatTime(value?: number) {
  if (!value) return '—';
  return new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value));
}

function materialRequired(type: string, applicantRoleCode: string) {
  return (
    type === 'business_license' ||
    type === 'legal_representative_id' ||
    (applicantRoleCode === 'authorized_agent' &&
      (type === 'agent_id' || type === 'authorization_letter'))
  );
}

async function load() {
  // Keep the already-rendered form mounted during upload and retry refreshes so
  // sensitive values remain in memory for the current page session only.
  loading.value = !state.value;
  failed.value = false;
  forbidden.value = false;
  personalCustomer.value = false;
  try {
    state.value = await getEnterpriseVerification();
  } catch (error) {
    handleError(error);
  } finally {
    loading.value = false;
  }
}

async function beginDraft() {
  if (
    failed.value ||
    !accepted.value ||
    !state.value?.policyVersion ||
    busy.value
  )
    return;
  busy.value = 'begin';
  failed.value = false;
  const consentAction = `consent:${state.value.policyVersion}`;
  const changeNo = state.value.activeLegalChange?.requestNo || '';
  const draftAction = `draft:${changeNo || 'initial'}`;
  try {
    const consent = await createEnterpriseVerificationConsent({
      idempotencyKey: actionKey(consentAction),
      locale: locale.value,
      policyVersion: state.value.policyVersion,
    });
    await createEnterpriseVerificationDraft({
      consentNo: consent.consentNo,
      idempotencyKey: actionKey(draftAction),
      legalChangeRequestNo: changeNo,
    });
    clearActionKey(consentAction);
    clearActionKey(draftAction);
    accepted.value = false;
    message.success($t('enterpriseVerification.notice.started'));
    await load();
  } catch (error) {
    handleError(error);
  } finally {
    busy.value = '';
  }
}

async function openLegalChange() {
  const profile = state.value?.profile;
  if (
    !profile ||
    failed.value ||
    !changeFields.value.length ||
    !changeReason.value ||
    busy.value
  ) {
    return;
  }
  busy.value = 'change';
  failed.value = false;
  const action = `legal-change:${profile.profileVersion}:${[
    ...changeFields.value,
  ]
    .sort()
    .join('+')}:${changeReason.value}`;
  try {
    await createEnterpriseLegalChange({
      expectedProfileVersion: profile.profileVersion,
      fieldCodes: [...changeFields.value],
      idempotencyKey: actionKey(action),
      reasonCode: changeReason.value,
    });
    clearActionKey(action);
    message.success($t('enterpriseVerification.change.created'));
    await load();
  } catch (error) {
    handleError(error);
  } finally {
    busy.value = '';
  }
}

async function uploadMaterial(file: File, materialType: string) {
  const caseNo = state.value?.case?.caseNo;
  if (!caseNo || failed.value || uploading.value) return;
  if (
    file.size <= 0 ||
    file.size > maxMaterialBytes ||
    !acceptedMaterialTypes.includes(file.type)
  ) {
    message.error($t('enterpriseVerification.evidence.invalid'));
    return;
  }
  uploading.value = materialType;
  failed.value = false;
  const action = `material:${caseNo}:${materialType}`;
  try {
    await uploadEnterpriseMaterial(
      caseNo,
      materialType,
      file,
      actionKey(action),
    );
    clearActionKey(action);
    message.success($t('enterpriseVerification.evidence.uploaded'));
    await load();
  } catch (error) {
    handleError(error);
  } finally {
    uploading.value = '';
  }
}

function beforeUpload(materialType: string): UploadProps['beforeUpload'] {
  return (file) => {
    void uploadMaterial(file as File, materialType);
    return Upload.LIST_IGNORE;
  };
}

async function submit() {
  const currentCase = state.value?.case;
  if (!currentCase || failed.value || busy.value) return;
  const { valid } = await formApi.validate();
  const values = await formApi.getValues<{
    agentIdentityNumber?: string;
    agentName?: string;
    applicantRoleCode: 'authorized_agent' | 'legal_representative';
    industryCode?: string;
    legalName: string;
    legalRepresentativeIdentityNumber: string;
    legalRepresentativeName: string;
    registrationIdentifier: string;
  }>();
  const requiredMaterialsReady = evidenceTypes
    .filter((type) => materialRequired(type, values.applicantRoleCode))
    .every((type) => materialByType.value.has(type));
  const agentReady =
    values.applicantRoleCode !== 'authorized_agent' ||
    (Boolean(values.agentName?.trim()) &&
      Boolean(values.agentIdentityNumber?.trim()));
  const registrationIdentifier = values.registrationIdentifier
    .trim()
    .toUpperCase();
  if (
    !valid ||
    !/^[0-9A-HJ-NPQRTUWXY]{18}$/u.test(registrationIdentifier) ||
    !agentReady ||
    !requiredMaterialsReady
  ) {
    invalid.value = true;
    return;
  }

  invalid.value = false;
  failed.value = false;
  busy.value = 'submit';
  const action = `submit:${currentCase.caseNo}`;
  const input: EnterpriseSubmissionInput = {
    agentIdentityNumber: values.agentIdentityNumber?.trim() || '',
    agentName: values.agentName?.trim() || '',
    applicantRoleCode: values.applicantRoleCode,
    idempotencyKey: actionKey(action),
    industryCode: values.industryCode?.trim() || '',
    legalName: values.legalName.trim(),
    legalRepresentativeIdentityNumber:
      values.legalRepresentativeIdentityNumber.trim(),
    legalRepresentativeName: values.legalRepresentativeName.trim(),
    materialNos: [...materialByType.value.values()].map(
      (item) => item.materialNo,
    ),
    registrationCountryCode: 'CN',
    registrationIdentifier,
  };
  try {
    await submitEnterpriseVerification(currentCase.caseNo, input);
    clearActionKey(action);
    await formApi.resetForm();
    message.success($t('enterpriseVerification.form.submitted'));
    await load();
  } catch (error) {
    handleError(error);
  } finally {
    input.agentIdentityNumber = '';
    input.agentName = '';
    input.legalName = '';
    input.legalRepresentativeIdentityNumber = '';
    input.legalRepresentativeName = '';
    input.registrationIdentifier = '';
    busy.value = '';
  }
}

onMounted(() => {
  formApi.setState({ schema: createLegalSchema() });
  void load();
});
</script>

<template>
  <component
    :is="pageShell"
    data-testid="enterprise-verification-center"
    :description="$t('enterpriseVerification.lead')"
    :title="$t('enterpriseVerification.title')"
  >
    <div class="enterprise-shell">
      <Card class="enterprise-status" :bordered="false">
        <div class="enterprise-status__main">
          <div>
            <div class="enterprise-eyebrow">
              {{ $t('enterpriseVerification.account') }}
            </div>
            <h2>{{ $t('enterpriseVerification.status.title') }}</h2>
          </div>
          <Tag
            :color="statusColor(currentStatus)"
            data-testid="enterprise-status"
          >
            {{ $t(`enterpriseVerification.status.${currentStatus}`) }}
          </Tag>
        </div>
        <p>{{ $t('enterpriseVerification.status.hint') }}</p>
      </Card>

      <Alert
        v-if="personalCustomer"
        :message="$t('enterpriseVerification.enterpriseOnly')"
        show-icon
        type="info"
      >
        <template #action>
          <Button href="/account/identity" size="small">
            {{ $t('enterpriseVerification.personal') }}
          </Button>
        </template>
      </Alert>

      <Alert
        v-if="failed"
        :message="
          forbidden
            ? $t('enterpriseVerification.enterpriseOnly')
            : $t('enterpriseVerification.failed')
        "
        show-icon
        type="error"
      >
        <template #action>
          <Button size="small" @click="load">
            {{ $t('enterpriseVerification.reload') }}
          </Button>
        </template>
      </Alert>

      <Skeleton v-if="loading" active :paragraph="{ rows: 10 }" />

      <template v-else-if="state">
        <Steps
          class="enterprise-steps"
          :current="stepIndex"
          :items="[
            { title: $t('enterpriseVerification.steps.consent') },
            { title: $t('enterpriseVerification.steps.materials') },
            { title: $t('enterpriseVerification.steps.review') },
          ]"
          responsive
        />

        <Alert
          :description="$t('enterpriseVerification.security.description')"
          :message="$t('enterpriseVerification.security.title')"
          show-icon
          type="info"
        />

        <Card
          v-if="profileVerified"
          :bordered="false"
          data-testid="enterprise-current-profile"
          :title="$t('enterpriseVerification.profile.title')"
        >
          <Descriptions :column="{ md: 2, xs: 1 }" size="small">
            <DescriptionsItem
              :label="$t('enterpriseVerification.form.legalName')"
            >
              {{ state.profile?.legalName || '—' }}
            </DescriptionsItem>
            <DescriptionsItem
              :label="$t('enterpriseVerification.form.registrationId')"
            >
              {{ state.profile?.registrationIdentifierMasked || '—' }}
            </DescriptionsItem>
            <DescriptionsItem
              :label="$t('enterpriseVerification.form.industry')"
            >
              {{ state.profile?.industryCode || '—' }}
            </DescriptionsItem>
            <DescriptionsItem
              :label="$t('enterpriseVerification.profile.verifiedAt')"
            >
              {{ formatTime(state.profile?.verifiedAt) }}
            </DescriptionsItem>
          </Descriptions>
        </Card>

        <Alert
          v-if="underReview"
          data-testid="enterprise-under-review"
          :description="
            state.case?.reverification
              ? $t('enterpriseVerification.review.changeDescription')
              : $t('enterpriseVerification.review.description')
          "
          :message="$t('enterpriseVerification.review.title')"
          show-icon
          type="warning"
        />

        <Alert
          v-if="currentStatus === 'rejected'"
          data-testid="enterprise-rejected"
          :description="
            $t(
              `enterpriseVerification.reason.${state.case?.decisionReasonCode || 'other'}`,
            )
          "
          :message="$t('enterpriseVerification.rejected.title')"
          show-icon
          type="error"
        />

        <Card
          v-if="profileVerified && !state.activeLegalChange && !underReview"
          :bordered="false"
          data-testid="enterprise-legal-change"
          :title="$t('enterpriseVerification.change.title')"
        >
          <p class="enterprise-muted">
            {{ $t('enterpriseVerification.change.lead') }}
          </p>
          <div class="enterprise-change-grid">
            <CheckboxGroup
              v-model:value="changeFields"
              data-testid="enterprise-change-fields"
              :options="[
                {
                  label: $t('enterpriseVerification.change.legalName'),
                  value: 'legal_name',
                },
                {
                  label: $t('enterpriseVerification.change.legalIdentifier'),
                  value: 'legal_identifier',
                },
              ]"
            />
            <Select
              v-model:value="changeReason"
              data-testid="enterprise-change-reason"
              :options="[
                {
                  label: $t('enterpriseVerification.change.reasonLegalName'),
                  value: 'legal_name_change',
                },
                {
                  label: $t('enterpriseVerification.change.reasonCorrection'),
                  value: 'identifier_correction',
                },
                {
                  label: $t('enterpriseVerification.change.reasonRegistration'),
                  value: 'enterprise_registration_change',
                },
                {
                  label: $t('enterpriseVerification.change.reasonOther'),
                  value: 'other',
                },
              ]"
              :placeholder="$t('enterpriseVerification.change.reason')"
            />
            <Button
              data-testid="enterprise-create-change"
              :disabled="failed || !changeFields.length || !changeReason"
              :loading="busy === 'change'"
              type="primary"
              @click="openLegalChange"
            >
              {{ $t('enterpriseVerification.change.create') }}
            </Button>
          </div>
        </Card>

        <Card
          v-if="beginAvailable"
          class="enterprise-notice"
          :bordered="false"
          data-testid="enterprise-consent"
          :title="$t('enterpriseVerification.notice.title')"
        >
          <div class="enterprise-notice__points">
            <p>{{ $t('enterpriseVerification.notice.purpose') }}</p>
            <p>{{ $t('enterpriseVerification.notice.data') }}</p>
            <p>{{ $t('enterpriseVerification.notice.recipient') }}</p>
            <p>{{ $t('enterpriseVerification.notice.retention') }}</p>
            <p>{{ $t('enterpriseVerification.notice.rights') }}</p>
          </div>
          <Checkbox
            v-model:checked="accepted"
            data-testid="enterprise-consent-checkbox"
          >
            {{ $t('enterpriseVerification.notice.accept') }}
          </Checkbox>
          <div class="enterprise-notice__action">
            <span>
              {{ $t('enterpriseVerification.notice.policy') }}
              {{ state.policyVersion }}
            </span>
            <Button
              data-testid="enterprise-begin"
              :disabled="failed || !accepted"
              :loading="busy === 'begin'"
              type="primary"
              @click="beginDraft"
            >
              {{ $t('enterpriseVerification.notice.begin') }}
            </Button>
          </div>
        </Card>

        <template v-if="draft && state.case">
          <Alert
            :description="$t('enterpriseVerification.draft.description')"
            :message="$t('enterpriseVerification.draft.title')"
            show-icon
            type="warning"
          />
          <Card
            :bordered="false"
            data-testid="enterprise-legal-form"
            :title="$t('enterpriseVerification.form.title')"
          >
            <p class="enterprise-muted">
              {{ $t('enterpriseVerification.form.lead') }}
            </p>
            <LegalForm />
          </Card>

          <Card
            :bordered="false"
            data-testid="enterprise-evidence"
            :title="$t('enterpriseVerification.evidence.title')"
          >
            <p class="enterprise-muted">
              {{ $t('enterpriseVerification.evidence.lead') }}
            </p>
            <div class="enterprise-evidence-grid">
              <div
                v-for="item in evidenceTypes"
                :key="item"
                class="enterprise-evidence-item"
                :data-testid="`enterprise-evidence-${item}`"
              >
                <div class="enterprise-evidence-item__heading">
                  <div>
                    <strong>
                      {{ $t(`enterpriseVerification.evidence.${item}`) }}
                    </strong>
                    <span v-if="materialByType.has(item)">
                      {{ materialByType.get(item)?.mediaType }} ·
                      {{ materialByType.get(item)?.byteSize }} B
                    </span>
                    <span v-else>
                      {{ $t('enterpriseVerification.evidence.notUploaded') }}
                    </span>
                  </div>
                  <Tag v-if="materialByType.has(item)" color="success">
                    {{ $t('enterpriseVerification.evidence.ready') }}
                  </Tag>
                </div>
                <UploadDragger
                  :accept="acceptedMaterialTypes.join(',')"
                  :before-upload="beforeUpload(item)"
                  :data-testid="`enterprise-upload-${item}`"
                  :disabled="failed || Boolean(uploading)"
                  :show-upload-list="false"
                >
                  <p class="ant-upload-text">
                    {{
                      uploading === item
                        ? $t('enterpriseVerification.evidence.uploading')
                        : $t('enterpriseVerification.evidence.choose')
                    }}
                  </p>
                  <p class="ant-upload-hint">
                    {{
                      $t('enterpriseVerification.evidence.hint', {
                        required: materialRequired(item, applicantRole)
                          ? $t('enterpriseVerification.evidence.required')
                          : $t('enterpriseVerification.evidence.optional'),
                      })
                    }}
                  </p>
                </UploadDragger>
              </div>
            </div>
            <Alert
              v-if="invalid"
              class="mt-5"
              :message="$t('enterpriseVerification.form.invalid')"
              show-icon
              type="error"
            />
            <div class="enterprise-submit">
              <p>{{ $t('enterpriseVerification.form.submitHint') }}</p>
              <Button
                data-testid="enterprise-submit"
                :disabled="failed"
                :loading="busy === 'submit'"
                size="large"
                type="primary"
                @click="submit"
              >
                {{ $t('enterpriseVerification.form.submit') }}
              </Button>
            </div>
          </Card>
        </template>

        <Space v-if="!draft" wrap>
          <Button href="/account/identity">
            {{ $t('enterpriseVerification.personal') }}
          </Button>
          <Button @click="load">
            {{ $t('enterpriseVerification.reload') }}
          </Button>
        </Space>
      </template>
    </div>
  </component>
</template>

<style scoped>
.enterprise-shell {
  display: grid;
  gap: 16px;
  max-width: 100%;
  margin: 0 auto;
}

.enterprise-status {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
}

.enterprise-status__main,
.enterprise-evidence-item__heading,
.enterprise-notice__action,
.enterprise-submit {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.enterprise-status h2 {
  margin: 4px 0 8px;
  font-size: 18px;
  font-weight: 650;
}

.enterprise-status p,
.enterprise-muted,
.enterprise-submit p {
  margin: 0;
  color: hsl(var(--muted-foreground));
  line-height: 1.65;
}

.enterprise-eyebrow {
  color: hsl(var(--primary));
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.enterprise-steps {
  padding: 10px 4px;
}

.enterprise-change-grid {
  display: grid;
  grid-template-columns: 1.3fr minmax(220px, 1fr) auto;
  gap: 16px;
  align-items: center;
  margin-top: 18px;
}

.enterprise-notice {
  border-top: 4px solid hsl(var(--primary));
}

.enterprise-notice__points {
  display: grid;
  gap: 8px;
  padding: 18px;
  margin-bottom: 18px;
  color: hsl(var(--foreground));
  border-radius: 10px;
  background: hsl(var(--muted) / 55%);
}

.enterprise-notice__points p {
  margin: 0;
  line-height: 1.65;
}

.enterprise-notice__action {
  align-items: center;
  padding-top: 18px;
  margin-top: 18px;
  border-top: 1px solid hsl(var(--border));
}

.enterprise-notice__action span {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.enterprise-evidence-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin-top: 18px;
}

.enterprise-evidence-item {
  min-width: 0;
  padding: 14px;
  border: 1px solid hsl(var(--border));
  border-radius: 10px;
}

.enterprise-evidence-item__heading {
  margin-bottom: 12px;
}

.enterprise-evidence-item__heading div {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.enterprise-evidence-item__heading span {
  overflow: hidden;
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.enterprise-submit {
  align-items: center;
  margin-top: 22px;
}

.enterprise-submit p {
  max-width: 720px;
}

@media (max-width: 900px) {
  .enterprise-change-grid,
  .enterprise-evidence-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 520px) {
  .enterprise-status__main,
  .enterprise-notice__action,
  .enterprise-submit {
    flex-direction: column;
  }

  .enterprise-notice__action button,
  .enterprise-submit button {
    width: 100%;
  }
}
</style>

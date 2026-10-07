<script setup lang="ts">
import type {
  ContractSignerHumanChallengeItem,
  ContractSignerItem,
  ContractSignerPhoneCodeItem,
} from '#/api/idc/contract-signers';

import { computed, onMounted, reactive, ref } from 'vue';
import { useRoute } from 'vue-router';

import { Page } from '@vben/common-ui';
import { preferences } from '@vben/preferences';

import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  message,
  Modal as AntModal,
  Radio,
  Space,
  Spin,
  Steps,
  Tag,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  checkContractSignerIdentity,
  completeContractSignerPhoneChallenge,
  createContractSigner,
  createContractSignerPhoneChallenge,
  disableContractSigner,
  listContractSigners,
  sendContractSignerPhoneCode,
  startContractSignerIdentity,
  verifyContractSignerPhone,
} from '#/api/idc/contract-signers';
import { getCustomerProfile } from '#/api/idc/customer-account';
import { $t } from '#/locales';
import { useVbenForm } from '#/adapter/form';
import VbenModalCompat from '#/components/vben-modal-compat.vue';

const route = useRoute();
const props = withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
});
const pageShell = computed(() => (props.embedded ? 'div' : Page));
const loading = ref(false);
const loadFailed = ref(false);
const enterpriseCustomer = ref<boolean>();
const signers = ref<ContractSignerItem[]>([]);
const createOpen = ref(false);
const creating = ref(false);
const identityBusy = ref('');
const phoneOpen = ref(false);
const phoneSigner = ref<ContractSignerItem>();
const phoneChallenge = ref<ContractSignerHumanChallengeItem>();
const phoneChallengeKind = ref<'accessible' | 'image'>('accessible');
const phoneChallengeAnswer = ref('');
const phoneChallengeLoading = ref(false);
const phoneSending = ref(false);
const phoneVerifying = ref(false);
const phoneCode = ref('');
const phoneCodeResult = ref<ContractSignerPhoneCodeItem>();
const phoneFlowUncertain = ref(false);
const phoneBrowserNonce = ref('');

const createForm = reactive({
  certName: '',
  certNo: '',
  consentAccepted: false,
  phone: '',
});

const locale = computed<'en-US' | 'zh-CN'>(() =>
  preferences.app.locale === 'en-US' ? 'en-US' : 'zh-CN',
);
const returnedFromIdentity = computed(
  () => route.query.verification_return === '1',
);
const canCreate = computed(
  () =>
    createForm.certName.trim().length > 0 &&
    createForm.certNo.trim().length > 0 &&
    createForm.phone.trim().length > 0 &&
    createForm.consentAccepted,
);
const phoneChallengeReady = computed(
  () =>
    Boolean(phoneChallenge.value) &&
    (phoneChallenge.value?.kind === 'accessible' ||
      phoneChallenge.value?.kind === 'image') &&
    phoneChallengeAnswer.value.trim().length > 0,
);

const [SignerForm, signerFormApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    labelWidth: 120,
  },
  handleValuesChange(values) {
    createForm.certName = String(values.certName || '');
    createForm.certNo = String(values.certNo || '');
    createForm.phone = String(values.phone || '');
    createForm.consentAccepted = Boolean(values.consentAccepted);
  },
  schema: [
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        'data-testid': 'contract-signer-name',
        maxlength: 64,
      },
      fieldName: 'certName',
      label: $t('contracts.signers.create.name'),
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'off',
        'data-testid': 'contract-signer-identity',
        maxlength: 64,
      },
      fieldName: 'certNo',
      label: $t('contracts.signers.create.identityNo'),
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: {
        autocomplete: 'tel',
        'data-testid': 'contract-signer-phone',
        maxlength: 16,
      },
      fieldName: 'phone',
      label: $t('contracts.signers.create.phone'),
      rules: 'required',
    },
    {
      component: 'Checkbox',
      componentProps: {
        'data-testid': 'contract-signer-consent',
      },
      fieldName: 'consentAccepted',
      label: $t('contracts.signers.create.consent'),
      rules: 'selectRequired',
    },
  ],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1',
});

function randomValue(prefix: string) {
  const value =
    typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : Array.from(crypto.getRandomValues(new Uint8Array(24)), (item) =>
          item.toString(16).padStart(2, '0'),
        ).join('');
  return `${prefix}-${value}`;
}

function formatTime(value?: number) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—';
}

function replaceSigner(item: ContractSignerItem) {
  const index = signers.value.findIndex(
    (candidate) => candidate.signerNo === item.signerNo,
  );
  if (index >= 0) signers.value[index] = item;
  else signers.value.unshift(item);
}

function isCandidate(item: ContractSignerItem) {
  return (
    item.statusCode === 'active' &&
    item.identityStatusCode === 'verified' &&
    item.phoneStatusCode === 'verified'
  );
}

function identityStep(item: ContractSignerItem) {
  return item.identityStatusCode === 'verified' ? 1 : 0;
}

function phoneStep(item: ContractSignerItem) {
  return item.phoneStatusCode === 'verified' ? 1 : 0;
}

function statusColor(value: string) {
  if (value === 'verified' || value === 'active') return 'success';
  if (value === 'review_required' || value === 'verifying') return 'warning';
  if (value === 'disabled' || value === 'expired') return 'default';
  return 'processing';
}

function statusText(scope: 'identity' | 'phone' | 'signer', value: string) {
  return $t(`contracts.signers.status.${scope}.${value}`);
}

async function load() {
  loading.value = true;
  loadFailed.value = false;
  try {
    const profile = await getCustomerProfile();
    enterpriseCustomer.value = profile.item.type === 'enterprise';
    if (!enterpriseCustomer.value) {
      signers.value = [];
      return;
    }
    const result = await listContractSigners();
    signers.value = result.items || [];
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
}

function clearCreateForm() {
  createForm.certName = '';
  createForm.certNo = '';
  createForm.phone = '';
  createForm.consentAccepted = false;
  signerFormApi.setValues({ ...createForm });
}

async function submitCreate() {
  if (!canCreate.value || creating.value) return;
  creating.value = true;
  try {
    const result = await createContractSigner({
      certName: createForm.certName.trim(),
      certNo: createForm.certNo.trim(),
      certType: 'IDENTITY_CARD',
      consentAccepted: true,
      consentPolicyVersion: '1',
      idempotencyKey: randomValue('signer-create'),
      phone: createForm.phone.trim(),
    });
    replaceSigner(result.item);
    clearCreateForm();
    createOpen.value = false;
    message.success($t('contracts.signers.messages.created'));
  } finally {
    creating.value = false;
  }
}

async function startIdentity(item: ContractSignerItem) {
  if (identityBusy.value) return;
  const externalWindow = window.open('about:blank', '_blank');
  if (!externalWindow) {
    message.warning($t('contracts.signers.messages.popupBlocked'));
    return;
  }
  externalWindow.opener = null;
  identityBusy.value = item.signerNo;
  try {
    const result = await startContractSignerIdentity(item.signerNo);
    externalWindow.location.replace(result.item.continuationUrl);
    await load();
  } catch (error) {
    externalWindow.close();
    throw error;
  } finally {
    identityBusy.value = '';
  }
}

async function checkIdentity(item: ContractSignerItem) {
  if (!item.identityAttemptNo || identityBusy.value) return;
  identityBusy.value = item.signerNo;
  try {
    const result = await checkContractSignerIdentity(
      item.signerNo,
      item.identityAttemptNo,
    );
    replaceSigner(result.item);
    message.success(
      result.item.identityStatusCode === 'verified'
        ? $t('contracts.signers.messages.identityVerified')
        : $t('contracts.signers.messages.identityChecked'),
    );
  } finally {
    identityBusy.value = '';
  }
}

function resetPhoneFlow() {
  phoneChallenge.value = undefined;
  phoneChallengeAnswer.value = '';
  phoneCodeResult.value = undefined;
  phoneCode.value = '';
  phoneFlowUncertain.value = false;
  phoneBrowserNonce.value = randomValue('signer-browser');
}

async function openPhone(item: ContractSignerItem) {
  phoneSigner.value = item;
  phoneOpen.value = true;
  phoneChallengeKind.value = 'accessible';
  resetPhoneFlow();
  await loadPhoneChallenge();
}

async function loadPhoneChallenge(
  kind: 'accessible' | 'image' = phoneChallengeKind.value,
) {
  if (!phoneSigner.value || phoneChallengeLoading.value) return;
  phoneChallengeKind.value = kind;
  phoneChallengeLoading.value = true;
  phoneFlowUncertain.value = false;
  phoneChallenge.value = undefined;
  phoneChallengeAnswer.value = '';
  try {
    const result = await createContractSignerPhoneChallenge(
      phoneSigner.value.signerNo,
      {
        browserNonce: phoneBrowserNonce.value,
        locale: locale.value,
        presentationKind: kind,
      },
    );
    phoneChallenge.value = result.item;
  } catch {
    phoneFlowUncertain.value = true;
  } finally {
    phoneChallengeLoading.value = false;
  }
}

function changePhoneChallengeKind(event: {
  target?: { value?: unknown };
}) {
  const kind = event.target?.value;
  if (kind === 'accessible' || kind === 'image') {
    void loadPhoneChallenge(kind);
  }
}

async function completeChallengeAndSendCode() {
  const signer = phoneSigner.value;
  const challenge = phoneChallenge.value;
  if (!signer || !challenge || !phoneChallengeReady.value || phoneSending.value)
    return;
  phoneSending.value = true;
  phoneFlowUncertain.value = false;
  try {
    const proof = await completeContractSignerPhoneChallenge(signer.signerNo, {
      answer: phoneChallengeAnswer.value.trim(),
      browserNonce: phoneBrowserNonce.value,
      challengeNo: challenge.challengeNo,
      locale: locale.value,
    });
    const sent = await sendContractSignerPhoneCode(signer.signerNo, {
      browserNonce: phoneBrowserNonce.value,
      idempotencyKey: randomValue('signer-code'),
      locale: locale.value,
      proofToken: proof.item.proofToken,
    });
    phoneCodeResult.value = sent.item;
    phoneChallenge.value = undefined;
    phoneChallengeAnswer.value = '';
  } catch {
    phoneChallenge.value = undefined;
    phoneChallengeAnswer.value = '';
    phoneFlowUncertain.value = true;
  } finally {
    phoneSending.value = false;
  }
}

async function verifyPhone() {
  const signer = phoneSigner.value;
  const sent = phoneCodeResult.value;
  const code = phoneCode.value.trim();
  if (!signer || !sent || !/^\d{6}$/u.test(code) || phoneVerifying.value)
    return;
  phoneVerifying.value = true;
  try {
    const result = await verifyContractSignerPhone(signer.signerNo, {
      browserNonce: phoneBrowserNonce.value,
      code,
      locale: locale.value,
      verificationNo: sent.verificationNo,
    });
    replaceSigner(result.item);
    phoneOpen.value = false;
    resetPhoneFlow();
    message.success($t('contracts.signers.messages.phoneVerified'));
  } finally {
    phoneVerifying.value = false;
  }
}

function confirmDisable(item: ContractSignerItem) {
  AntModal.confirm({
    cancelText: $t('contracts.signers.disable.cancel'),
    content: $t('contracts.signers.disable.description'),
    okButtonProps: { danger: true },
    okText: $t('contracts.signers.disable.confirm'),
    title: $t('contracts.signers.disable.title'),
    async onOk() {
      const result = await disableContractSigner(item.signerNo, {
        expectedVersion: item.version,
        idempotencyKey: randomValue('signer-disable'),
      });
      replaceSigner(result.item);
      message.success($t('contracts.signers.messages.disabled'));
    },
  });
}

onMounted(load);
</script>

<template>
  <component :is="pageShell" :title="$t('contracts.signers.title')">
    <div class="signer-page" data-testid="contract-signer-page">
      <Card :bordered="false" class="page-toolbar">
        <div class="page-toolbar-content">
          <div>
            <strong>{{ $t('contracts.signers.heading') }}</strong>
            <p>{{ $t('contracts.signers.subtitle') }}</p>
          </div>
          <Button
            v-if="enterpriseCustomer"
            data-testid="contract-signer-add"
            size="large"
            type="primary"
            @click="createOpen = true"
          >
            {{ $t('contracts.signers.add') }}
          </Button>
        </div>
      </Card>

      <Alert
        v-if="enterpriseCustomer === false"
        :description="$t('contracts.signers.enterpriseOnlyDescription')"
        :message="$t('contracts.signers.enterpriseOnlyTitle')"
        show-icon
        type="info"
      />
      <Alert
        v-if="enterpriseCustomer"
        :description="$t('contracts.signers.separationDescription')"
        :message="$t('contracts.signers.separationTitle')"
        show-icon
        type="info"
      />
      <Alert
        v-if="enterpriseCustomer && returnedFromIdentity"
        data-testid="contract-signer-return-notice"
        :description="$t('contracts.signers.returnDescription')"
        :message="$t('contracts.signers.returnTitle')"
        show-icon
        type="warning"
      />
      <Alert
        v-if="enterpriseCustomer && loadFailed"
        :message="$t('contracts.signers.loadFailed')"
        show-icon
        type="error"
      >
        <template #action>
          <Button size="small" @click="load">{{ $t('contracts.signers.retry') }}</Button>
        </template>
      </Alert>

      <Spin v-if="enterpriseCustomer" :spinning="loading">
        <Empty
          v-if="!loading && !loadFailed && signers.length === 0"
          :description="$t('contracts.signers.empty')"
        />
        <div v-else class="signer-grid" data-testid="contract-signer-list">
          <Card
            v-for="item in signers"
            :key="item.signerNo"
            class="signer-card"
            :class="{ 'signer-card--ready': isCandidate(item) }"
          >
            <div class="signer-heading">
              <div>
                <div class="signer-name">{{ item.maskedName }}</div>
              </div>
              <Tag :color="isCandidate(item) ? 'success' : statusColor(item.statusCode)">
                {{
                  isCandidate(item)
                    ? $t('contracts.signers.candidateReady')
                    : statusText('signer', item.statusCode)
                }}
              </Tag>
            </div>

            <dl class="masked-profile">
              <div>
                <dt>{{ $t('contracts.signers.fields.identity') }}</dt>
                <dd>{{ item.maskedIdentityNo }}</dd>
              </div>
              <div>
                <dt>{{ $t('contracts.signers.fields.phone') }}</dt>
                <dd>{{ item.maskedPhone }}</dd>
              </div>
              <div>
                <dt>{{ $t('contracts.signers.fields.updated') }}</dt>
                <dd>{{ formatTime(item.updatedAt) }}</dd>
              </div>
            </dl>

            <div class="verification-block">
              <div>
                <strong>{{ $t('contracts.signers.identity.title') }}</strong>
                <Tag :color="statusColor(item.identityStatusCode)">
                  {{ statusText('identity', item.identityStatusCode) }}
                </Tag>
              </div>
              <Steps
                :current="identityStep(item)"
                :items="[
                  { title: $t('contracts.signers.identity.pending') },
                  { title: $t('contracts.signers.identity.done') },
                ]"
                size="small"
              />
            </div>

            <div class="verification-block">
              <div>
                <strong>{{ $t('contracts.signers.phone.title') }}</strong>
                <Tag :color="statusColor(item.phoneStatusCode)">
                  {{ statusText('phone', item.phoneStatusCode) }}
                </Tag>
              </div>
              <Steps
                :current="phoneStep(item)"
                :items="[
                  { title: $t('contracts.signers.phone.pending') },
                  { title: $t('contracts.signers.phone.done') },
                ]"
                size="small"
              />
            </div>

            <Space v-if="item.statusCode === 'active'" wrap>
              <Button
                v-if="item.identityStatusCode !== 'verified' && item.identityStatusCode !== 'verifying'"
                :loading="identityBusy === item.signerNo"
                @click="startIdentity(item)"
              >
                {{ $t('contracts.signers.identity.start') }}
              </Button>
              <Button
                v-else-if="item.identityStatusCode === 'verifying'"
                :loading="identityBusy === item.signerNo"
                @click="checkIdentity(item)"
              >
                {{ $t('contracts.signers.identity.check') }}
              </Button>
              <Button
                v-if="item.phoneStatusCode !== 'verified'"
                data-testid="contract-signer-phone-start"
                @click="openPhone(item)"
              >
                {{ $t('contracts.signers.phone.start') }}
              </Button>
              <Button danger type="text" @click="confirmDisable(item)">
                {{ $t('contracts.signers.disable.action') }}
              </Button>
            </Space>
          </Card>
        </div>
      </Spin>
    </div>

    <VbenModalCompat
      v-model:open="createOpen"
      :cancel-text="$t('contracts.signers.create.cancel')"
      :confirm-loading="creating"
      :ok-button-props="{ disabled: !canCreate }"
      :ok-text="$t('contracts.signers.create.submit')"
      :title="$t('contracts.signers.create.title')"
      @cancel="clearCreateForm"
      @ok="submitCreate"
    >
      <Alert
        class="modal-alert"
        :description="$t('contracts.signers.create.description')"
        :message="$t('contracts.signers.create.notice')"
        show-icon
        type="warning"
      />
      <SignerForm />
    </VbenModalCompat>

    <VbenModalCompat
      v-model:open="phoneOpen"
      :footer="null"
      :title="$t('contracts.signers.phone.modalTitle')"
      @after-close="resetPhoneFlow"
    >
      <Alert
        class="modal-alert"
        :description="$t('contracts.signers.phone.channelDescription')"
        :message="$t('contracts.signers.phone.channelTitle')"
        show-icon
        type="info"
      />

      <template v-if="!phoneCodeResult">
        <div class="challenge-heading">
          <strong>{{ $t('contracts.signers.phone.humanCheck') }}</strong>
          <Radio.Group
            :value="phoneChallengeKind"
            button-style="solid"
            size="small"
            @change="changePhoneChallengeKind"
          >
            <Radio.Button value="image">{{ $t('contracts.signers.phone.image') }}</Radio.Button>
            <Radio.Button value="accessible">{{ $t('contracts.signers.phone.accessible') }}</Radio.Button>
          </Radio.Group>
        </div>
        <Spin :spinning="phoneChallengeLoading">
          <Alert
            v-if="phoneFlowUncertain"
            data-testid="contract-signer-phone-uncertain"
            :description="$t('contracts.signers.phone.uncertainDescription')"
            :message="$t('contracts.signers.phone.uncertainTitle')"
            show-icon
            type="warning"
          >
            <template #action>
              <Button size="small" @click="loadPhoneChallenge()">
                {{ $t('contracts.signers.phone.newChallenge') }}
              </Button>
            </template>
          </Alert>
          <template v-else-if="phoneChallenge">
            <img
              v-if="phoneChallenge.kind === 'image' && phoneChallenge.imageDataUrl"
              :alt="$t('contracts.signers.phone.imageAlt')"
              class="challenge-image"
              :src="phoneChallenge.imageDataUrl"
            />
            <p
              v-else-if="phoneChallenge.kind === 'accessible'"
              class="challenge-prompt"
              data-testid="contract-signer-phone-prompt"
            >
              {{ phoneChallenge.accessiblePrompt }}
            </p>
            <Alert
              v-else
              :message="$t('contracts.signers.phone.unsupportedChallenge')"
              show-icon
              type="warning"
            />
            <Input
              v-model:value="phoneChallengeAnswer"
              class="challenge-answer"
              data-testid="contract-signer-phone-answer"
              :placeholder="$t('contracts.signers.phone.answer')"
              @press-enter="completeChallengeAndSendCode"
            />
            <Button
              block
              :disabled="!phoneChallengeReady"
              :loading="phoneSending"
              type="primary"
              @click="completeChallengeAndSendCode"
            >
              {{ $t('contracts.signers.phone.sendCode') }}
            </Button>
          </template>
        </Spin>
      </template>

      <template v-else>
        <Alert
          class="modal-alert"
          :description="
            $t('contracts.signers.phone.codeDescription', {
              destination: phoneCodeResult.maskedDestination,
              expires: formatTime(phoneCodeResult.expiresAt),
            })
          "
          :message="$t('contracts.signers.phone.codeTitle')"
          show-icon
          type="success"
        />
        <Input
          v-model:value="phoneCode"
          autocomplete="one-time-code"
          data-testid="contract-signer-phone-code"
          :maxlength="6"
          :placeholder="$t('contracts.signers.phone.codePlaceholder')"
          @press-enter="verifyPhone"
        />
        <Button
          block
          class="verify-button"
          :disabled="!/^\d{6}$/u.test(phoneCode.trim())"
          :loading="phoneVerifying"
          type="primary"
          @click="verifyPhone"
        >
          {{ $t('contracts.signers.phone.verifyCode') }}
        </Button>
      </template>
    </VbenModalCompat>
  </component>
</template>

<style scoped>
.signer-page {
  display: grid;
  gap: 16px;
}

.page-toolbar {
  overflow: hidden;
  border: 1px solid color-mix(in srgb, hsl(var(--primary)) 24%, hsl(var(--border)));
}

.page-toolbar-content {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 24px;
}
.page-toolbar-content strong {
  display: block;
  margin-bottom: 6px;
  font-size: 18px;
}
.page-toolbar-content p {
  max-width: 720px;
  margin: 0;
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
}

.signer-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.signer-card {
  border-top: 3px solid hsl(var(--border));
}

.signer-card--ready {
  border-top-color: #22c55e;
}

.signer-heading,
.verification-block > div,
.challenge-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.signer-name {
  margin-bottom: 4px;
  font-size: 18px;
  font-weight: 700;
}

.masked-profile {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin: 18px 0;
}

.masked-profile div {
  min-width: 0;
  padding: 10px;
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
}

.masked-profile dt {
  margin-bottom: 4px;
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.masked-profile dd {
  overflow: hidden;
  margin: 0;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.verification-block {
  display: grid;
  gap: 10px;
  margin-bottom: 16px;
  padding: 12px;
  background: hsl(var(--muted) / 40%);
  border-radius: 10px;
}

.modal-alert,
.challenge-heading,
.challenge-prompt,
.challenge-image,
.challenge-answer {
  margin-bottom: 14px;
}

.challenge-prompt {
  padding: 14px;
  background: hsl(var(--muted));
  border-radius: 8px;
  font-size: 16px;
  font-weight: 600;
}

.challenge-image {
  display: block;
  max-width: 100%;
  margin-inline: auto;
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
}

.verify-button {
  margin-top: 12px;
}

@media (max-width: 900px) {
  .signer-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .page-toolbar-content {
    align-items: stretch;
    flex-direction: column;
  }

  .masked-profile {
    grid-template-columns: 1fr;
  }
}
</style>

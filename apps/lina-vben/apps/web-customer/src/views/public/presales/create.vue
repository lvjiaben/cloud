<script lang="ts" setup>
import type {
  PresalesDestinationKind,
  PresalesHumanChallenge,
  PresalesPresentationKind,
  PresalesVerificationCode,
} from '#/api/idc/presales';
import type {
  CustomerSupportCustomFieldDefinition,
  CustomerTicketCategory,
  CustomerTicketCustomFieldSubmission,
} from '#/api/idc/support';

import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { preferences } from '@vben/preferences';

import {
  Alert,
  Button,
  Card,
  Empty,
  Form,
  Input,
  Radio,
  Select,
  Spin,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  completePresalesHumanChallenge,
  createPresalesTicket,
  listPresalesCategories,
  listPresalesCustomFields,
  requestPresalesHumanChallenge,
  requestPresalesVerificationCode,
} from '#/api/idc/presales';
import { $t } from '#/locales';
import SupportCustomFields from '#/components/idc/SupportCustomFields.vue';

const router = useRouter();
const browserNonce = randomValue();
const operationNo = ref(randomValue());
const categories = ref<CustomerTicketCategory[]>([]);
const loadingCategories = ref(false);
const categoriesFailed = ref(false);
const customFields = ref<CustomerSupportCustomFieldDefinition[]>([]);
const customFieldsFailed = ref(false);
const loadingCustomFields = ref(true);
const customFieldForm = ref<{
  submissions: () => CustomerTicketCustomFieldSubmission[];
  validate: () => boolean;
}>();
const destinationKind = ref<PresalesDestinationKind>('email');
const destination = ref('');
const categoryNo = ref<string>();
const subject = ref('');
const body = ref('');
const presentationKind = ref<PresalesPresentationKind>('image');
const challenge = ref<PresalesHumanChallenge>();
const challengeAnswer = ref('');
const requestingChallenge = ref(false);
const sendingCode = ref(false);
const verification = ref<PresalesVerificationCode>();
const code = ref('');
const submitting = ref(false);
const challengeUnavailable = ref(false);
const codeResultUncertain = ref(false);
const submissionUncertain = ref(false);
const formError = ref('');
const now = ref(Date.now());
const activeBinding = ref<{
  destination: string;
  destinationKind: PresalesDestinationKind;
  locale: 'en-US' | 'zh-CN';
}>();
const timer = window.setInterval(() => {
  now.value = Date.now();
}, 1000);

const locale = computed<'en-US' | 'zh-CN'>(() =>
  preferences.app.locale === 'en-US' ? 'en-US' : 'zh-CN',
);
const locked = computed(() => Boolean(verification.value));
const detailsLocked = computed(() =>
  Boolean(challenge.value || requestingChallenge.value || verification.value),
);
const retrySeconds = computed(() => {
  const retryAfterAt = verification.value?.retryAfterAt || 0;
  return Math.max(0, Math.ceil((retryAfterAt - now.value) / 1000));
});
const canStartChallenge = computed(
  () =>
    Boolean(
      destination.value.trim() &&
      categoryNo.value &&
      subject.value.trim() &&
      body.value.trim(),
    ) &&
    !loadingCustomFields.value &&
    !customFieldsFailed.value &&
    !detailsLocked.value,
);
const challengeCanBeAnswered = computed(() =>
  Boolean(
    challenge.value &&
    (challenge.value.kind === 'image' ||
      challenge.value.kind === 'accessible') &&
    challengeAnswer.value.trim(),
  ),
);

function randomValue() {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID();
  const bytes = crypto.getRandomValues(new Uint8Array(24));
  return Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join(
    '',
  );
}

function formatTime(timestamp?: number) {
  return timestamp ? dayjs(timestamp).format('YYYY-MM-DD HH:mm') : '';
}

function categoryName(item: CustomerTicketCategory) {
  return locale.value === 'en-US' ? item.nameEnUS : item.nameZhCN;
}

function resetChallenge() {
  challenge.value = undefined;
  challengeAnswer.value = '';
  challengeUnavailable.value = false;
}

function validateDetails() {
  if (!destination.value.trim()) return $t('presales.destinationRequired');
  if (!categoryNo.value) return $t('presales.categoryRequired');
  if (!subject.value.trim()) return $t('presales.subjectRequired');
  if (!body.value.trim()) return $t('presales.messageRequired');
  if (customFieldsFailed.value) return $t('presales.customFieldsLoadFailed');
  if (!(customFieldForm.value?.validate() ?? false)) {
    return $t('presales.customFieldsInvalid');
  }
  return '';
}

async function loadCategories() {
  loadingCategories.value = true;
  categoriesFailed.value = false;
  try {
    const result = await listPresalesCategories({ limit: 100 });
    categories.value = result.items || [];
  } catch {
    categoriesFailed.value = true;
  } finally {
    loadingCategories.value = false;
  }
}

async function loadCustomFields() {
  loadingCustomFields.value = true;
  customFieldsFailed.value = false;
  try {
    const result = await listPresalesCustomFields();
    customFields.value = result.items || [];
  } catch {
    customFields.value = [];
    customFieldsFailed.value = true;
  } finally {
    loadingCustomFields.value = false;
  }
}

async function loadFormData() {
  await Promise.all([loadCategories(), loadCustomFields()]);
}

async function startChallenge(kind = presentationKind.value) {
  const error = validateDetails();
  formError.value = error;
  if (error) return;

  presentationKind.value = kind;
  requestingChallenge.value = true;
  codeResultUncertain.value = false;
  resetChallenge();
  try {
    const binding = {
      browserNonce,
      destination: destination.value.trim(),
      destinationKind: destinationKind.value,
      locale: locale.value,
      operation: 'create',
    } as const;
    activeBinding.value = binding;
    const result = await requestPresalesHumanChallenge({
      ...binding,
      presentationKind: kind,
    });
    challenge.value = result;
    challengeUnavailable.value =
      result.kind !== 'image' && result.kind !== 'accessible';
  } catch {
    challengeUnavailable.value = true;
  } finally {
    requestingChallenge.value = false;
  }
}

async function switchPresentation(kind: PresalesPresentationKind) {
  if (kind === presentationKind.value || locked.value) return;
  await startChallenge(kind);
}

async function verifyChallengeAndSendCode() {
  if (!challenge.value || !challengeCanBeAnswered.value || !activeBinding.value)
    return;
  sendingCode.value = true;
  codeResultUncertain.value = false;
  try {
    const binding = {
      ...activeBinding.value,
      browserNonce,
      operation: 'create' as const,
    };
    const proof = await completePresalesHumanChallenge({
      ...binding,
      answer: challengeAnswer.value.trim(),
      challengeNo: challenge.value.challengeNo,
    });
    const result = await requestPresalesVerificationCode({
      ...binding,
      idempotencyKey: randomValue(),
      proofToken: proof.proofToken,
    });
    verification.value = result;
    operationNo.value = randomValue();
    resetChallenge();
  } catch {
    codeResultUncertain.value = true;
    resetChallenge();
  } finally {
    sendingCode.value = false;
  }
}

async function submitTicket() {
  if (!verification.value || !code.value.trim() || !activeBinding.value) return;
  submitting.value = true;
  submissionUncertain.value = false;
  try {
    const result = await createPresalesTicket({
      body: body.value.trim(),
      browserNonce,
      categoryNo: categoryNo.value || '',
      code: code.value.trim(),
      destination: activeBinding.value.destination,
      destinationKind: activeBinding.value.destinationKind,
      locale: activeBinding.value.locale,
      operationNo: operationNo.value,
      customFields: customFieldForm.value?.submissions() || [],
      subject: subject.value.trim(),
      verificationNo: verification.value.verificationNo,
    });
    await router.replace({
      name: 'PublicPresalesAccess',
      params: { token: result.accessToken },
      query: { ticket: result.ticketNo },
      state: { linkDeliveryState: result.linkDeliveryState },
    });
  } catch {
    submissionUncertain.value = true;
  } finally {
    submitting.value = false;
  }
}

async function restartCodeVerification() {
  if (retrySeconds.value > 0) return;
  verification.value = undefined;
  code.value = '';
  submissionUncertain.value = false;
  operationNo.value = randomValue();
  activeBinding.value = undefined;
  await startChallenge();
}

onMounted(loadFormData);
onBeforeUnmount(() => window.clearInterval(timer));
</script>

<template>
  <Card class="presales-card" :bordered="false">
    <div class="mb-6 text-center">
      <h1 class="m-0 text-2xl font-semibold">
        {{ $t('presales.createTitle') }}
      </h1>
      <p class="text-muted-foreground mt-2 text-sm/6">
        {{ $t('presales.createHint') }}
      </p>
    </div>

    <Alert
      class="mb-5"
      :message="$t('presales.linkLifetimeHint')"
      show-icon
      type="info"
    />

    <Form layout="vertical" @submit.prevent>
      <div class="grid gap-x-4 md:grid-cols-2">
        <Form.Item :label="$t('presales.channel')" required>
          <Radio.Group
            v-model:value="destinationKind"
            data-testid="presales-destination-kind"
            :disabled="detailsLocked"
          >
            <Radio.Button value="email">
              {{ $t('presales.email') }}
            </Radio.Button>
            <Radio.Button value="sms">{{ $t('presales.sms') }}</Radio.Button>
          </Radio.Group>
        </Form.Item>
        <Form.Item :label="$t('presales.destination')" required>
          <Input
            v-model:value="destination"
            data-testid="presales-destination"
            :disabled="detailsLocked"
            :placeholder="$t('presales.destinationPlaceholder')"
          />
        </Form.Item>
      </div>

      <Form.Item :label="$t('presales.category')" required>
        <Spin :spinning="loadingCategories">
          <Alert
            v-if="categoriesFailed"
            class="mb-3"
            :message="$t('presales.categoriesLoadFailed')"
            show-icon
            type="error"
          >
            <template #action>
              <Button size="small" @click="loadCategories">
                {{ $t('presales.retry') }}
              </Button>
            </template>
          </Alert>
          <Select
            v-if="categories.length > 0"
            v-model:value="categoryNo"
            data-testid="presales-category"
            :disabled="detailsLocked"
            :options="
              categories.map((item) => ({
                label: categoryName(item),
                value: item.categoryNo,
              }))
            "
            :placeholder="$t('presales.categoryPlaceholder')"
          />
          <Empty
            v-else-if="!loadingCategories && !categoriesFailed"
            :description="$t('presales.categoriesEmpty')"
          />
        </Spin>
      </Form.Item>

      <Form.Item :label="$t('presales.subject')" required>
        <Input
          v-model:value="subject"
          data-testid="presales-subject"
          :disabled="detailsLocked"
          :maxlength="200"
        />
      </Form.Item>
      <Form.Item :label="$t('presales.body')" required>
        <Input.TextArea
          v-model:value="body"
          data-testid="presales-body"
          :auto-size="{ minRows: 5, maxRows: 12 }"
          :disabled="detailsLocked"
          :maxlength="4000"
          show-count
        />
      </Form.Item>

      <Alert
        v-if="customFieldsFailed"
        class="mb-4"
        :message="$t('presales.customFieldsLoadFailed')"
        show-icon
        type="error"
      >
        <template #action>
          <Button size="small" @click="loadFormData">
            {{ $t('presales.retry') }}
          </Button>
        </template>
      </Alert>
      <SupportCustomFields
        ref="customFieldForm"
        :definitions="customFields"
        :disabled="detailsLocked"
        test-id-prefix="presales-custom-field"
      />

      <Alert
        v-if="formError"
        class="mb-4"
        :message="formError"
        show-icon
        type="error"
      />

      <template v-if="!verification">
        <div
          v-if="challenge || challengeUnavailable"
          class="challenge-panel"
          data-testid="presales-challenge"
        >
          <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
            <strong>{{ $t('presales.humanCheck') }}</strong>
            <Radio.Group
              :value="presentationKind"
              button-style="solid"
              size="small"
              @change="switchPresentation($event.target.value)"
            >
              <Radio.Button value="image">
                {{ $t('presales.imageChallenge') }}
              </Radio.Button>
              <Radio.Button value="accessible">
                {{ $t('presales.accessibleChallenge') }}
              </Radio.Button>
            </Radio.Group>
          </div>

          <Alert
            v-if="challengeUnavailable"
            data-testid="presales-challenge-unavailable"
            :description="$t('presales.challengeUnavailableDescription')"
            :message="$t('presales.challengeUnavailable')"
            show-icon
            type="warning"
          />
          <template v-else-if="challenge">
            <img
              v-if="challenge.kind === 'image' && challenge.imageDataUrl"
              :alt="$t('presales.imageChallengeAlt')"
              class="challenge-image"
              data-testid="presales-challenge-image"
              :src="challenge.imageDataUrl"
            />
            <p
              v-else-if="challenge.kind === 'accessible'"
              class="bg-muted rounded-lg p-4 text-base font-medium"
              data-testid="presales-accessible-prompt"
            >
              {{ challenge.accessiblePrompt }}
            </p>
            <Input
              v-model:value="challengeAnswer"
              class="mt-3"
              data-testid="presales-challenge-answer"
              :placeholder="$t('presales.challengeAnswer')"
              @press-enter="verifyChallengeAndSendCode"
            />
            <Button
              class="mt-3"
              data-testid="presales-send-code"
              :disabled="!challengeCanBeAnswered"
              :loading="sendingCode"
              type="primary"
              @click="verifyChallengeAndSendCode"
            >
              {{ $t('presales.verifyAndSendCode') }}
            </Button>
          </template>
        </div>

        <Button
          v-else
          block
          data-testid="presales-start-verification"
          :disabled="!canStartChallenge"
          :loading="requestingChallenge"
          type="primary"
          @click="startChallenge()"
        >
          {{ $t('presales.startVerification') }}
        </Button>
      </template>

      <section
        v-else
        class="verification-panel"
        data-testid="presales-code-panel"
      >
        <Alert
          class="mb-4"
          :description="
            $t('presales.codeSentDescription', {
              destination: verification.maskedDestination,
              date: formatTime(verification.expiresAt),
            })
          "
          :message="$t('presales.codeSent')"
          show-icon
          :type="
            verification.deliveryState.includes('unknown')
              ? 'warning'
              : 'success'
          "
        />
        <Form.Item :label="$t('presales.code')" required>
          <Input
            v-model:value="code"
            autocomplete="one-time-code"
            data-testid="presales-code"
            inputmode="numeric"
            :maxlength="6"
            :placeholder="$t('presales.codePlaceholder')"
          />
        </Form.Item>
        <p
          class="text-muted-foreground mb-3 text-sm"
          data-testid="presales-retry-status"
        >
          {{
            retrySeconds > 0
              ? $t('presales.retryAfter', { seconds: retrySeconds })
              : $t('presales.retryReady')
          }}
        </p>
        <Button
          class="mb-3"
          data-testid="presales-resend-code"
          :disabled="retrySeconds > 0"
          @click="restartCodeVerification"
        >
          {{ $t('presales.restartHumanCheck') }}
        </Button>
        <Button
          block
          data-testid="presales-submit"
          :disabled="code.trim().length !== 6"
          :loading="submitting"
          type="primary"
          @click="submitTicket"
        >
          {{ $t('presales.create') }}
        </Button>
      </section>

      <Alert
        v-if="codeResultUncertain"
        class="mt-4"
        data-testid="presales-code-uncertain"
        :description="$t('presales.unknownResultDescription')"
        :message="$t('presales.unknownResult')"
        show-icon
        type="warning"
      />
      <Alert
        v-if="submissionUncertain"
        class="mt-4"
        data-testid="presales-submit-uncertain"
        :description="$t('presales.submissionUnknownDescription')"
        :message="$t('presales.submissionUnknown')"
        show-icon
        type="warning"
      />
    </Form>

    <div class="mt-6 text-center">
      <Button type="link" @click="router.push('/auth/login')">
        {{ $t('presales.customerLogin') }}
      </Button>
    </div>
  </Card>
</template>

<style scoped>
.presales-card {
  width: min(100%, 46rem);
  max-width: min(100%, 46rem);
  border: 1px solid var(--border);
  border-radius: 1rem;
  box-shadow: 0 18px 55px rgb(15 23 42 / 10%);
}

.challenge-panel,
.verification-panel {
  padding: 1rem;
  border: 1px solid var(--border);
  border-radius: 0.75rem;
}

.challenge-image {
  display: block;
  width: min(100%, 20rem);
  min-height: 5rem;
  object-fit: contain;
  background: white;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
}

@media (max-width: 640px) {
  .presales-card {
    border-radius: 0.75rem;
  }

  .presales-card :deep(.ant-card-body) {
    padding: 1.25rem;
  }
}
</style>

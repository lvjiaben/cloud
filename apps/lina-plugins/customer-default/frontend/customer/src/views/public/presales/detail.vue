<script lang="ts" setup>
import type {
  PresalesDestinationKind,
  PresalesHumanChallenge,
  PresalesPresentationKind,
  PresalesTicketDetail,
  PresalesVerificationCode,
} from '#/api/idc/presales';
import type {
  CustomerTicketAttachment,
  CustomerTicketCustomFieldValue,
} from '#/api/idc/support';
import type { UploadFile, UploadProps } from 'ant-design-vue';

import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { preferences } from '@vben/preferences';

import {
  Alert,
  Button,
  Card,
  Divider,
  Empty,
  Form,
  Input,
  message,
  Modal,
  Radio,
  Rate,
  Spin,
  Tag,
  Upload,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  closePresales,
  completePresalesHumanChallenge,
  downloadPresalesAttachment,
  ratePresales,
  reopenPresales,
  renewPresalesAccess,
  replyPresales,
  requestPresalesHumanChallenge,
  requestPresalesVerificationCode,
  uploadPresalesReplyAttachments,
  verifyPresalesAccess,
} from '#/api/idc/presales';
import { $t } from '#/locales';
import { downloadBlob } from '#/utils/download';

const route = useRoute();
const router = useRouter();
const token = computed(() => String(route.params.token || ''));
const ticket = ref<PresalesTicketDetail>();
const loading = ref(false);
const failed = ref(false);
const reply = ref('');
const replyFiles = ref<UploadFile[]>([]);
const replyUploadKey = ref(0);
const submittingReply = ref(false);
const actionLoading = ref(false);
const ratingOpen = ref(false);
const ratingStars = ref(0);
const ratingFeedback = ref('');
const downloadingAttachmentNo = ref('');
const UploadDragger = Upload.Dragger;
const maxAttachmentBytes = 20 * 1024 * 1024;
const maxTicketAttachmentBytes = 200 * 1024 * 1024;
const maxReplyAttachments = 20;
const browserNonce = randomValue();
const renewalOperationNo = ref(randomValue());
const renewalDestinationKind = ref<PresalesDestinationKind>('email');
const renewalDestination = ref('');
const renewalPresentationKind = ref<PresalesPresentationKind>('image');
const renewalChallenge = ref<PresalesHumanChallenge>();
const renewalChallengeAnswer = ref('');
const renewalVerification = ref<PresalesVerificationCode>();
const renewalCode = ref('');
const requestingRenewalChallenge = ref(false);
const sendingRenewalCode = ref(false);
const renewing = ref(false);
const renewalUnavailable = ref(false);
const renewalResultUncertain = ref(false);
const renewalBinding = ref<{
  destination: string;
  destinationKind: PresalesDestinationKind;
  locale: 'en-US' | 'zh-CN';
}>();
const now = ref(Date.now());
const timer = window.setInterval(() => {
  now.value = Date.now();
}, 1000);

const locale = computed<'en-US' | 'zh-CN'>(() =>
  preferences.app.locale === 'en-US' ? 'en-US' : 'zh-CN',
);
const safeTicketNo = computed(() => {
  const value =
    typeof route.query.ticket === 'string' ? route.query.ticket : '';
  return /^[\w-]{3,80}$/u.test(value) ? value : '';
});
const renewalRetrySeconds = computed(() =>
  Math.max(
    0,
    Math.ceil(
      ((renewalVerification.value?.retryAfterAt || 0) - now.value) / 1000,
    ),
  ),
);
const renewalDetailsLocked = computed(() =>
  Boolean(
    renewalChallenge.value ||
    requestingRenewalChallenge.value ||
    renewalVerification.value,
  ),
);
const deliveryUncertain = computed(() => {
  const state = String(
    window.history.state?.linkDeliveryState || '',
  ).toLowerCase();
  return (
    Boolean(state) &&
    !['accepted', 'delivered', 'queued', 'sent'].includes(state)
  );
});
const knownAttachmentBytes = computed(
  () =>
    ticket.value?.messages.reduce(
      (total, item) =>
        total +
        item.attachments.reduce(
          (messageTotal, attachment) =>
            messageTotal + attachment.sizeBytes,
          0,
        ),
      0,
    ) || 0,
);

type RequestFailure = {
  error?: string;
  errorCode?: string;
  response?: {
    data?: { error?: string; errorCode?: string };
    status?: number;
  };
};

function isAccessUnavailableError(error: unknown) {
  const failure = error as RequestFailure;
  const response = failure?.response;
  const stableCode =
    failure?.errorCode ||
    failure?.error ||
    response?.data?.errorCode ||
    response?.data?.error ||
    '';
  return (
    response?.status === 401 ||
    response?.status === 403 ||
    stableCode === 'IDC_SUPPORT_TICKET_NOT_ACCESSIBLE'
  );
}

const beforeReplyUpload: UploadProps['beforeUpload'] = (file, batch) => {
  if (file.size <= 0) {
    message.error($t('presales.attachmentEmpty'));
    return Upload.LIST_IGNORE;
  }
  if (file.size > maxAttachmentBytes) {
    message.error($t('presales.attachmentTooLarge'));
    return Upload.LIST_IGNORE;
  }
  const isFirstInBatch = file.uid === batch[0]?.uid;
  if (replyFiles.value.length + batch.length > maxReplyAttachments) {
    if (isFirstInBatch) {
      message.error($t('presales.attachmentCountExceeded'));
    }
    return Upload.LIST_IGNORE;
  }
  const selectedBytes = replyFiles.value.reduce(
    (total, item) => total + (item.originFileObj?.size || 0),
    0,
  );
  const batchBytes = batch.reduce((total, item) => total + item.size, 0);
  if (
    knownAttachmentBytes.value + selectedBytes + batchBytes >
    maxTicketAttachmentBytes
  ) {
    if (isFirstInBatch) {
      message.error($t('presales.attachmentTotalTooLarge'));
    }
    return Upload.LIST_IGNORE;
  }
  return false;
};

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

function formatFileSize(sizeBytes: number) {
  if (sizeBytes < 1024) return `${sizeBytes} B`;
  if (sizeBytes < 1024 * 1024) return `${(sizeBytes / 1024).toFixed(1)} KiB`;
  return `${(sizeBytes / 1024 / 1024).toFixed(1)} MiB`;
}

function categoryName() {
  if (!ticket.value?.category) return '';
  return preferences.app.locale === 'en-US'
    ? ticket.value.category.nameEnUS
    : ticket.value.category.nameZhCN;
}

function localizedName(item: { nameEnUS: string; nameZhCN: string }) {
  return preferences.app.locale === 'en-US'
    ? item.nameEnUS || item.nameZhCN
    : item.nameZhCN || item.nameEnUS;
}

function localizedDescription(item: {
  descriptionEnUS?: string;
  descriptionZhCN?: string;
}) {
  return preferences.app.locale === 'en-US'
    ? item.descriptionEnUS || item.descriptionZhCN || ''
    : item.descriptionZhCN || item.descriptionEnUS || '';
}

function customFieldValue(field: CustomerTicketCustomFieldValue) {
  if (field.selectedOptions?.length) {
    return field.selectedOptions.map(localizedName).join(', ');
  }
  if (field.fieldType === 'boolean') {
    return field.normalizedValue === 'true'
      ? $t('support.yes')
      : $t('support.no');
  }
  return field.normalizedValue || $t('support.notProvided');
}

function authorLabel(author: string) {
  if (author === 'customer' || author === 'visitor') return $t('support.you');
  if (author === 'system') return $t('support.system');
  return $t('support.supportTeam');
}

async function loadAccess() {
  loading.value = true;
  failed.value = false;
  try {
    ticket.value = await verifyPresalesAccess(token.value);
  } catch {
    ticket.value = undefined;
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

function resetRenewalChallenge() {
  renewalChallenge.value = undefined;
  renewalChallengeAnswer.value = '';
  renewalUnavailable.value = false;
}

async function startRenewalChallenge(
  kind: PresalesPresentationKind = renewalPresentationKind.value,
) {
  if (!safeTicketNo.value || !renewalDestination.value.trim()) return;
  renewalPresentationKind.value = kind;
  requestingRenewalChallenge.value = true;
  renewalResultUncertain.value = false;
  resetRenewalChallenge();
  try {
    const binding = {
      browserNonce,
      destination: renewalDestination.value.trim(),
      destinationKind: renewalDestinationKind.value,
      locale: locale.value,
      operation: 'renew',
      ticketNo: safeTicketNo.value,
    } as const;
    renewalBinding.value = binding;
    const result = await requestPresalesHumanChallenge({
      ...binding,
      presentationKind: kind,
    });
    renewalChallenge.value = result;
    renewalUnavailable.value =
      result.kind !== 'image' && result.kind !== 'accessible';
  } catch {
    renewalUnavailable.value = true;
  } finally {
    requestingRenewalChallenge.value = false;
  }
}

async function switchRenewalPresentation(kind: PresalesPresentationKind) {
  if (kind !== renewalPresentationKind.value) await startRenewalChallenge(kind);
}

async function sendRenewalCode() {
  if (
    !renewalChallenge.value ||
    !renewalChallengeAnswer.value.trim() ||
    !renewalBinding.value
  )
    return;
  sendingRenewalCode.value = true;
  renewalResultUncertain.value = false;
  try {
    const binding = {
      ...renewalBinding.value,
      browserNonce,
      operation: 'renew' as const,
    };
    const proof = await completePresalesHumanChallenge({
      ...binding,
      answer: renewalChallengeAnswer.value.trim(),
      challengeNo: renewalChallenge.value.challengeNo,
    });
    renewalVerification.value = await requestPresalesVerificationCode({
      ...binding,
      idempotencyKey: randomValue(),
      proofToken: proof.proofToken,
    });
    renewalOperationNo.value = randomValue();
    resetRenewalChallenge();
  } catch {
    renewalResultUncertain.value = true;
    resetRenewalChallenge();
  } finally {
    sendingRenewalCode.value = false;
  }
}

async function submitRenewal() {
  if (
    !renewalVerification.value ||
    renewalCode.value.trim().length !== 6 ||
    !renewalBinding.value
  )
    return;
  renewing.value = true;
  renewalResultUncertain.value = false;
  try {
    const result = await renewPresalesAccess({
      browserNonce,
      code: renewalCode.value.trim(),
      destination: renewalBinding.value.destination,
      destinationKind: renewalBinding.value.destinationKind,
      locale: renewalBinding.value.locale,
      operationNo: renewalOperationNo.value,
      ticketNo: safeTicketNo.value,
      verificationNo: renewalVerification.value.verificationNo,
    });
    await router.replace({
      name: 'PublicPresalesAccess',
      params: { token: result.accessToken },
      query: { ticket: result.ticketNo },
      state: { linkDeliveryState: result.linkDeliveryState },
    });
    await loadAccess();
  } catch {
    renewalResultUncertain.value = true;
  } finally {
    renewing.value = false;
  }
}

async function restartRenewalVerification() {
  if (renewalRetrySeconds.value > 0) return;
  renewalVerification.value = undefined;
  renewalCode.value = '';
  renewalResultUncertain.value = false;
  renewalOperationNo.value = randomValue();
  renewalBinding.value = undefined;
  await startRenewalChallenge();
}

async function sendReply() {
  if (!reply.value.trim() || !ticket.value) return;
  submittingReply.value = true;
  try {
    const files = replyFiles.value.flatMap((file) =>
      file.originFileObj ? [file.originFileObj as File] : [],
    );
    const uploadNos = await uploadPresalesReplyAttachments(token.value, files);
    ticket.value = await replyPresales(
      token.value,
      reply.value.trim(),
      ticket.value.version,
      uploadNos,
    );
    reply.value = '';
    replyFiles.value = [];
    replyUploadKey.value += 1;
  } catch (error) {
    if (isAccessUnavailableError(error)) {
      ticket.value = undefined;
      failed.value = true;
      message.warning($t('presales.accessUnavailable'));
    } else {
      message.error($t('presales.replyFailedDraftPreserved'));
      await loadAccess();
    }
  } finally {
    submittingReply.value = false;
  }
}

async function handleTicketActionFailure(error: unknown) {
  if (isAccessUnavailableError(error)) {
    ticket.value = undefined;
    failed.value = true;
    message.warning($t('presales.accessUnavailable'));
    return;
  }
  message.error($t('presales.actionFailed'));
  await loadAccess();
}

function confirmClose() {
  if (!ticket.value?.actions.canClose || actionLoading.value) return;
  Modal.confirm({
    cancelText: $t('common.cancel'),
    content: $t('presales.closeConfirm'),
    okText: $t('presales.close'),
    title: $t('presales.close'),
    async onOk() {
      if (!ticket.value) return;
      actionLoading.value = true;
      try {
        ticket.value = await closePresales(token.value, ticket.value.version);
        message.success($t('presales.closeSuccess'));
      } catch (error) {
        await handleTicketActionFailure(error);
      } finally {
        actionLoading.value = false;
      }
    },
  });
}

async function reopenTicket() {
  if (!ticket.value?.actions.canReopen || actionLoading.value) return;
  actionLoading.value = true;
  try {
    ticket.value = await reopenPresales(token.value, ticket.value.version);
    message.success($t('presales.reopenSuccess'));
  } catch (error) {
    await handleTicketActionFailure(error);
  } finally {
    actionLoading.value = false;
  }
}

function openRating() {
  ratingStars.value = 0;
  ratingFeedback.value = '';
  ratingOpen.value = true;
}

async function submitRating() {
  if (!ticket.value?.actions.canRate || actionLoading.value) return;
  if (ratingStars.value < 1 || ratingStars.value > 5) {
    message.warning($t('support.ratingRequired'));
    return;
  }
  actionLoading.value = true;
  try {
    ticket.value = await ratePresales(
      token.value,
      ticket.value.version,
      ticket.value.currentRound,
      ratingStars.value,
      ratingFeedback.value.trim(),
    );
    ratingOpen.value = false;
    message.success($t('presales.ratingSuccess'));
  } catch (error) {
    await handleTicketActionFailure(error);
  } finally {
    actionLoading.value = false;
  }
}

async function downloadAttachment(attachment: CustomerTicketAttachment) {
  downloadingAttachmentNo.value = attachment.attachmentNo;
  try {
    const data = await downloadPresalesAttachment(token.value, attachment);
    downloadBlob(data, attachment.fileName, attachment.mediaType);
  } catch {
    message.error($t('support.attachmentDownloadFailed'));
  } finally {
    downloadingAttachmentNo.value = '';
  }
}

onMounted(loadAccess);
onBeforeUnmount(() => window.clearInterval(timer));
</script>

<template>
  <Card class="presales-detail-card" :bordered="false">
    <Spin :spinning="loading">
      <section v-if="failed" data-testid="presales-access-unavailable">
        <Alert
          class="mb-4"
          :description="$t('presales.accessUnavailableDescription')"
          :message="$t('presales.accessUnavailable')"
          show-icon
          type="warning"
        />
        <Alert
          class="mb-5"
          :message="$t('presales.linkLifetimeHint')"
          show-icon
          type="info"
        />
        <section
          v-if="safeTicketNo"
          class="renewal-panel"
          data-testid="presales-renewal"
        >
          <h2 class="mt-0 text-base font-semibold">
            {{ $t('presales.requestNewLink') }}
          </h2>
          <p class="text-muted-foreground text-sm">
            {{ $t('presales.accessExpiredHint') }}
          </p>
          <Form layout="vertical" @submit.prevent>
            <Form.Item :label="$t('presales.channel')" required>
              <Radio.Group
                v-model:value="renewalDestinationKind"
                data-testid="presales-renewal-kind"
                :disabled="renewalDetailsLocked"
              >
                <Radio.Button value="email">
                  {{ $t('presales.email') }}
                </Radio.Button>
                <Radio.Button value="sms">
                  {{ $t('presales.sms') }}
                </Radio.Button>
              </Radio.Group>
            </Form.Item>
            <Form.Item :label="$t('presales.destination')" required>
              <Input
                v-model:value="renewalDestination"
                data-testid="presales-renewal-destination"
                :disabled="renewalDetailsLocked"
                :placeholder="$t('presales.destinationPlaceholder')"
              />
            </Form.Item>

            <div
              v-if="renewalChallenge || renewalUnavailable"
              class="challenge-panel"
              data-testid="presales-renewal-challenge"
            >
              <Radio.Group
                :value="renewalPresentationKind"
                class="mb-3"
                size="small"
                @change="switchRenewalPresentation($event.target.value)"
              >
                <Radio.Button value="image">
                  {{ $t('presales.imageChallenge') }}
                </Radio.Button>
                <Radio.Button value="accessible">
                  {{ $t('presales.accessibleChallenge') }}
                </Radio.Button>
              </Radio.Group>
              <Alert
                v-if="renewalUnavailable"
                data-testid="presales-renewal-challenge-unavailable"
                :message="$t('presales.challengeUnavailable')"
                show-icon
                type="warning"
              />
              <template v-else-if="renewalChallenge">
                <img
                  v-if="
                    renewalChallenge.kind === 'image' &&
                    renewalChallenge.imageDataUrl
                  "
                  :alt="$t('presales.imageChallengeAlt')"
                  class="challenge-image"
                  :src="renewalChallenge.imageDataUrl"
                />
                <p
                  v-else-if="renewalChallenge.kind === 'accessible'"
                  class="bg-muted rounded-lg p-4"
                  data-testid="presales-renewal-accessible-prompt"
                >
                  {{ renewalChallenge.accessiblePrompt }}
                </p>
                <Input
                  v-model:value="renewalChallengeAnswer"
                  class="mt-3"
                  data-testid="presales-renewal-challenge-answer"
                  :placeholder="$t('presales.challengeAnswer')"
                />
                <Button
                  class="mt-3"
                  data-testid="presales-renewal-send-code"
                  :disabled="!renewalChallengeAnswer.trim()"
                  :loading="sendingRenewalCode"
                  type="primary"
                  @click="sendRenewalCode"
                >
                  {{ $t('presales.verifyAndSendCode') }}
                </Button>
              </template>
            </div>

            <Button
              v-else-if="!renewalVerification"
              block
              data-testid="presales-renewal-start"
              :disabled="!renewalDestination.trim()"
              :loading="requestingRenewalChallenge"
              type="primary"
              @click="startRenewalChallenge()"
            >
              {{ $t('presales.startVerification') }}
            </Button>

            <template v-if="renewalVerification">
              <Alert
                class="mb-3"
                :message="
                  $t('presales.codeSentDescription', {
                    destination: renewalVerification.maskedDestination,
                    date: formatTime(renewalVerification.expiresAt),
                  })
                "
                show-icon
                type="success"
              />
              <Form.Item :label="$t('presales.code')" required>
                <Input
                  v-model:value="renewalCode"
                  autocomplete="one-time-code"
                  data-testid="presales-renewal-code"
                  inputmode="numeric"
                  :maxlength="6"
                />
              </Form.Item>
              <p class="text-muted-foreground text-sm">
                {{
                  renewalRetrySeconds > 0
                    ? $t('presales.retryAfter', {
                        seconds: renewalRetrySeconds,
                      })
                    : $t('presales.retryReady')
                }}
              </p>
              <Button
                class="mb-3"
                data-testid="presales-renewal-resend-code"
                :disabled="renewalRetrySeconds > 0"
                @click="restartRenewalVerification"
              >
                {{ $t('presales.restartHumanCheck') }}
              </Button>
              <Button
                block
                data-testid="presales-renewal-submit"
                :disabled="renewalCode.trim().length !== 6"
                :loading="renewing"
                type="primary"
                @click="submitRenewal"
              >
                {{ $t('presales.requestNewLink') }}
              </Button>
            </template>
            <Alert
              v-if="renewalResultUncertain"
              class="mt-3"
              data-testid="presales-renewal-uncertain"
              :description="$t('presales.unknownResultDescription')"
              :message="$t('presales.unknownResult')"
              show-icon
              type="warning"
            />
          </Form>
        </section>
        <Alert
          v-else
          class="mb-3"
          data-testid="presales-renewal-needs-safe-ticket"
          :description="$t('presales.reverificationNeedsSafeTicketDescription')"
          :message="$t('presales.reverificationNeedsSafeTicket')"
          show-icon
          type="info"
        />
        <Button
          class="mt-3"
          block
          type="primary"
          @click="router.push('/auth/presales')"
        >
          {{ $t('presales.backToCreate') }}
        </Button>
      </section>

      <Empty v-else-if="!ticket && !loading" />

      <section v-else-if="ticket" data-testid="presales-detail">
        <Alert
          v-if="deliveryUncertain"
          class="mb-4"
          data-testid="presales-link-delivery-uncertain"
          :description="$t('presales.linkDeliveryUncertainDescription')"
          :message="$t('presales.linkDeliveryUncertain')"
          show-icon
          type="warning"
        />
        <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 class="m-0 text-xl font-semibold">{{ ticket.subject }}</h1>
            <p class="text-muted-foreground mt-2 text-sm">
              {{ $t('presales.linkLifetimeHint') }}
            </p>
            <p
              v-if="ticket.accessExpiresAt > 0"
              class="mt-1 text-sm font-medium"
              data-testid="presales-access-expires-at"
            >
              {{
                $t('presales.accessUntil', {
                  date: formatTime(ticket.accessExpiresAt),
                })
              }}
            </p>
          </div>
          <Button @click="router.push('/auth/presales')">
            {{ $t('presales.backToCreate') }}
          </Button>
        </div>

        <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_17rem]">
          <Card :bordered="false" class="border-border border">
            <div class="timeline" data-testid="presales-timeline">
              <article
                v-for="item in ticket.messages"
                :key="item.messageNo"
                class="message-item"
              >
                <div class="flex items-center justify-between gap-3">
                  <strong>{{ authorLabel(item.author) }}</strong>
                  <time class="text-muted-foreground text-xs">
                    {{ formatTime(item.createdAt) }}
                  </time>
                </div>
                <p class="mt-3 whitespace-pre-wrap">{{ item.body }}</p>
                <div v-if="item.attachments.length > 0" class="mt-3 grid gap-1">
                  <Button
                    v-for="attachment in item.attachments"
                    :key="attachment.attachmentNo"
                    :aria-label="
                      $t('support.attachmentDownload', {
                        name: attachment.fileName,
                      })
                    "
                    class="h-auto w-fit p-0 text-left"
                    :loading="
                      downloadingAttachmentNo === attachment.attachmentNo
                    "
                    type="link"
                    @click="downloadAttachment(attachment)"
                  >
                    {{ attachment.fileName }} ·
                    {{ formatFileSize(attachment.sizeBytes) }}
                  </Button>
                </div>
              </article>
            </div>

            <template v-if="ticket.actions.canReply">
              <Divider />
              <Input.TextArea
                v-model:value="reply"
                data-testid="presales-reply"
                :auto-size="{ minRows: 4, maxRows: 12 }"
                :placeholder="$t('presales.replyPlaceholder')"
              />
              <div class="mt-3 flex flex-wrap items-start justify-between gap-3">
                <UploadDragger
                  :key="replyUploadKey"
                  v-model:file-list="replyFiles"
                  class="max-w-md"
                  data-testid="presales-reply-attachments"
                  :before-upload="beforeReplyUpload"
                  :disabled="submittingReply"
                  :max-count="maxReplyAttachments"
                  multiple
                >
                  <p class="ant-upload-text">
                    {{ $t('presales.addAttachments') }}
                  </p>
                  <p class="ant-upload-hint">
                    {{ $t('presales.attachmentHint') }}
                  </p>
                </UploadDragger>
                <Button
                  data-testid="presales-reply-submit"
                  :disabled="!reply.trim()"
                  :loading="submittingReply"
                  type="primary"
                  @click="sendReply"
                >
                  {{ $t('presales.reply') }}
                </Button>
              </div>
            </template>
          </Card>

          <aside class="grid content-start gap-4">
            <Card :bordered="false" class="border-border border">
              <dl class="ticket-facts">
                <dt>{{ $t('presales.ticketNo') }}</dt>
                <dd>{{ ticket.ticketNo }}</dd>
                <dt>{{ $t('presales.stage') }}</dt>
                <dd>
                  <Tag>{{ $t(`support.${ticket.stage}`) }}</Tag>
                </dd>
                <template v-if="categoryName()">
                  <dt>{{ $t('presales.category') }}</dt>
                  <dd>{{ categoryName() }}</dd>
                </template>
                <dt>{{ $t('presales.updatedAt') }}</dt>
                <dd>{{ formatTime(ticket.updatedAt) }}</dd>
              </dl>
            </Card>
            <Card
              v-if="ticket.customFields.length"
              :bordered="false"
              class="border-border border"
              data-testid="presales-custom-field-snapshots"
            >
              <strong>{{ $t('support.additionalInformation') }}</strong>
              <dl class="ticket-facts mt-3">
                <template v-for="field in ticket.customFields" :key="field.fieldNo">
                  <dt>
                    {{ localizedName(field) }}
                    <small v-if="localizedDescription(field)" class="block font-normal">
                      {{ localizedDescription(field) }}
                    </small>
                  </dt>
                  <dd>{{ customFieldValue(field) }}</dd>
                </template>
              </dl>
            </Card>
            <Card
              v-if="
                ticket.actions.canClose ||
                ticket.actions.canReopen ||
                (ticket.actions.canRate && !ticket.rating)
              "
              :bordered="false"
              class="border-border border"
              data-testid="presales-actions"
            >
              <div class="grid gap-2">
                <Button
                  v-if="ticket.actions.canClose"
                  data-testid="presales-close"
                  :loading="actionLoading"
                  type="primary"
                  @click="confirmClose"
                >
                  {{ $t('presales.close') }}
                </Button>
                <Button
                  v-if="ticket.actions.canReopen"
                  data-testid="presales-reopen"
                  :loading="actionLoading"
                  @click="reopenTicket"
                >
                  {{ $t('presales.reopen') }}
                </Button>
                <p
                  v-if="ticket.actions.canReopen && ticket.reopenDeadline"
                  class="text-muted-foreground text-xs"
                >
                  {{
                    $t('presales.reopenDeadline', {
                      date: formatTime(ticket.reopenDeadline),
                    })
                  }}
                </p>
                <Button
                  v-if="ticket.actions.canRate && !ticket.rating"
                  data-testid="presales-rate"
                  @click="openRating"
                >
                  {{ $t('presales.rate') }}
                </Button>
              </div>
              <div
                v-if="ratingOpen && ticket.actions.canRate && !ticket.rating"
                class="mt-4 grid gap-3"
                data-testid="presales-rating-form"
              >
                <Rate
                  v-model:value="ratingStars"
                  :aria-label="$t('presales.rate')"
                  data-testid="presales-rating-stars"
                />
                <Alert
                  v-if="ratingStars > 0 && ratingStars <= 2"
                  :message="$t('presales.ratingLowHint')"
                  show-icon
                  type="info"
                />
                <Input.TextArea
                  v-model:value="ratingFeedback"
                  data-testid="presales-rating-feedback"
                  :placeholder="$t('presales.ratingComment')"
                  :rows="3"
                />
                <div class="flex justify-end gap-2">
                  <Button @click="ratingOpen = false">
                    {{ $t('common.cancel') }}
                  </Button>
                  <Button
                    data-testid="presales-rating-submit"
                    :loading="actionLoading"
                    type="primary"
                    @click="submitRating"
                  >
                    {{ $t('support.submitRating') }}
                  </Button>
                </div>
              </div>
            </Card>
            <Card
              v-if="ticket.rating"
              :bordered="false"
              class="border-border border"
              data-testid="presales-rating-summary"
            >
              <strong>{{ $t('support.submittedRating') }}</strong>
              <p class="mt-2">
                {{ $t('support.ratingStars', { stars: ticket.rating.stars }) }}
              </p>
              <p v-if="ticket.rating.feedback" class="mt-1 whitespace-pre-wrap">
                {{ ticket.rating.feedback }}
              </p>
            </Card>
            <Alert
              v-if="
                !ticket.actions.canClose &&
                !ticket.actions.canReopen &&
                !ticket.actions.canRate
              "
              :description="$t('presales.actionsUnavailableDescription')"
              :message="$t('presales.actionsUnavailable')"
              show-icon
              type="info"
            />
          </aside>
        </div>
      </section>
    </Spin>
  </Card>
</template>

<style scoped>
.presales-detail-card {
  width: min(100%, 68rem);
  max-width: min(100%, 68rem);
  max-height: min(88vh, 60rem);
  overflow-y: auto;
  border: 1px solid var(--border);
  border-radius: 1rem;
  box-shadow: 0 18px 55px rgb(15 23 42 / 10%);
}

.timeline {
  display: grid;
  gap: 1rem;
}

.message-item {
  padding: 1rem;
  background: var(--muted);
  border-radius: 0.75rem;
}

.message-item p {
  margin-bottom: 0;
  line-height: 1.75;
}

.ticket-facts {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.75rem 1rem;
  margin: 0;
}

.ticket-facts dt {
  color: var(--muted-foreground);
}

.ticket-facts dd {
  min-width: 0;
  margin: 0;
  text-align: right;
  overflow-wrap: anywhere;
}

.renewal-panel,
.challenge-panel {
  padding: 1rem;
  margin-bottom: 1rem;
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
  .presales-detail-card {
    max-height: none;
    border-radius: 0.75rem;
  }

  .presales-detail-card :deep(.ant-card-body) {
    padding: 1.25rem;
  }
}
</style>

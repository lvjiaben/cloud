<script lang="ts" setup>
import type {
  CustomerTicketAttachment,
  CustomerTicketCustomFieldValue,
  CustomerTicketDetail,
} from '#/api/idc/support';
import type { UploadFile } from 'ant-design-vue';

import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page, useVbenModal } from '@vben/common-ui';
import { preferences } from '@vben/preferences';

import {
  Alert,
  Button,
  Card,
  Divider,
  Empty,
  Input,
  Rate,
  Space,
  Spin,
  Tag,
  Upload,
  message,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  closeCustomerTicket,
  downloadCustomerTicketAttachment,
  getCustomerTicket,
  rateCustomerTicket,
  reopenCustomerTicket,
  replyCustomerTicket,
} from '#/api/idc/support';
import { $t } from '#/locales';
import { downloadBlob } from '#/utils/download';

const route = useRoute();
const router = useRouter();
const ticketNo = computed(() => String(route.params.ticketNo || ''));
const ticket = ref<CustomerTicketDetail>();
const loading = ref(false);
const failed = ref(false);
const reply = ref('');
const replyFiles = ref<UploadFile[]>([]);
const submittingReply = ref(false);
const actionLoading = ref(false);
const downloadingAttachmentNo = ref('');
const ratingStars = ref(0);
const ratingFeedback = ref('');
const UploadDragger = Upload.Dragger;

const [CloseModal, closeModalApi] = useVbenModal({
  fullscreenButton: false,
  onConfirm: closeTicket,
});

const [RatingModal, ratingModalApi] = useVbenModal({
  fullscreenButton: false,
  onConfirm: submitRating,
});

async function loadTicket() {
  loading.value = true;
  failed.value = false;
  try {
    ticket.value = await getCustomerTicket(ticketNo.value);
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

async function sendReply() {
  if (!reply.value.trim() || !ticket.value) return;
  submittingReply.value = true;
  try {
    const files = replyFiles.value.flatMap((file) =>
      file.originFileObj ? [file.originFileObj as File] : [],
    );
    ticket.value = await replyCustomerTicket(
      ticketNo.value,
      reply.value.trim(),
      ticket.value.version,
      files,
    );
    reply.value = '';
    replyFiles.value = [];
  } finally {
    submittingReply.value = false;
  }
}

function confirmClose() {
  closeModalApi.setState({
    cancelText: $t('pages.common.cancel'),
    confirmText: $t('support.close'),
  });
  closeModalApi.open();
}

async function closeTicket() {
  if (!ticket.value) return;
  actionLoading.value = true;
  closeModalApi.setState({ loading: true });
  try {
    ticket.value = await closeCustomerTicket(
      ticketNo.value,
      ticket.value.version,
    );
    closeModalApi.close();
  } finally {
    actionLoading.value = false;
    closeModalApi.setState({ loading: false });
  }
}

async function reopen() {
  if (!ticket.value) return;
  actionLoading.value = true;
  try {
    ticket.value = await reopenCustomerTicket(
      ticketNo.value,
      ticket.value.version,
    );
    message.success($t('support.reopenSuccess'));
  } finally {
    actionLoading.value = false;
  }
}

function openRating() {
  ratingStars.value = 0;
  ratingFeedback.value = '';
  ratingModalApi.setState({
    cancelText: $t('pages.common.cancel'),
    confirmText: $t('support.submitRating'),
  });
  ratingModalApi.open();
}

async function submitRating() {
  if (!ticket.value?.actions.canRate || ticket.value.currentRound <= 0) return;
  if (ratingStars.value < 1 || ratingStars.value > 5) {
    message.warning($t('support.ratingRequired'));
    return;
  }
  ratingModalApi.setState({ loading: true });
  try {
    ticket.value = await rateCustomerTicket(
      ticketNo.value,
      ratingStars.value,
      ticket.value.currentRound,
      ticket.value.version,
      ratingFeedback.value.trim(),
    );
    ratingModalApi.close();
    message.success($t('support.ratingSuccess'));
  } finally {
    ratingModalApi.setState({ loading: false });
  }
}

async function downloadAttachment(attachment: CustomerTicketAttachment) {
  downloadingAttachmentNo.value = attachment.attachmentNo;
  try {
    const data = await downloadCustomerTicketAttachment(
      attachment.attachmentNo,
    );
    downloadBlob(data, attachment.fileName, attachment.mediaType);
  } catch {
    message.error($t('support.attachmentDownloadFailed'));
  } finally {
    downloadingAttachmentNo.value = '';
  }
}

function formatTime(timestamp?: number) {
  return timestamp ? dayjs(timestamp).format('YYYY-MM-DD HH:mm') : '';
}

function formatFileSize(sizeBytes: number) {
  if (sizeBytes < 1024) return `${sizeBytes} B`;
  if (sizeBytes < 1024 * 1024) return `${(sizeBytes / 1024).toFixed(1)} KiB`;
  return `${(sizeBytes / 1024 / 1024).toFixed(1)} MiB`;
}

function authorLabel(author: string) {
  if (author === 'customer' || author === 'visitor') return $t('support.you');
  if (author === 'system') {
    return $t('support.system');
  }
  return $t('support.supportTeam');
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
    return field.normalizedValue === 'true' ? $t('support.yes') : $t('support.no');
  }
  return field.normalizedValue || $t('support.notProvided');
}

function entitlementName(current = false) {
  const entitlement = ticket.value?.relatedEntitlement;
  if (!entitlement) return '';
  if (current && entitlement.currentAvailable) {
    return preferences.app.locale === 'en-US'
      ? entitlement.currentNameEnUS || entitlement.currentNameZhCN
      : entitlement.currentNameZhCN || entitlement.currentNameEnUS;
  }
  return preferences.app.locale === 'en-US'
    ? entitlement.snapshotNameEnUS || entitlement.snapshotNameZhCN
    : entitlement.snapshotNameZhCN || entitlement.snapshotNameEnUS;
}

function entitlementStatus(value?: string) {
  if (!value) return $t('support.entitlementStatusUnavailable');
  const key = `support.entitlementStatus.${value}`;
  const translated = $t(key);
  return translated === key
    ? $t('support.entitlementStatusUnavailable')
    : translated;
}

onMounted(loadTicket);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="ticket?.subject || $t('support.detailTitle')">
    <template #extra>
      <Button @click="router.push('/account/support')">{{
        $t('support.back')
      }}</Button>
    </template>
    <Alert
      v-if="failed"
      class="mb-4"
      :message="$t('support.loadFailed')"
      show-icon
      type="error"
    />
    <Spin :spinning="loading">
      <Empty v-if="!ticket && !loading" />
      <div
        v-else-if="ticket"
        class="ticket-details-layout"
      >
        <Card :bordered="false">
          <div class="timeline" data-testid="ticket-timeline">
            <article
              v-for="item in ticket.messages"
              :key="item.messageNo"
              class="message-item"
            >
              <div class="flex items-center justify-between gap-3">
                <strong>{{ authorLabel(item.author) }}</strong>
                <time class="text-xs text-muted-foreground">{{
                  formatTime(item.createdAt)
                }}</time>
              </div>
              <p class="mt-3 whitespace-pre-wrap">{{ item.body }}</p>
              <div v-if="item.attachments.length" class="mt-3 grid gap-1">
                <Button
                  v-for="attachment in item.attachments"
                  :key="attachment.attachmentNo"
                  :aria-label="
                    $t('support.attachmentDownload', {
                      name: attachment.fileName,
                    })
                  "
                  class="h-auto w-fit p-0 text-left"
                  :loading="downloadingAttachmentNo === attachment.attachmentNo"
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
              data-testid="ticket-reply"
              :auto-size="{ minRows: 4, maxRows: 12 }"
              :placeholder="$t('support.replyPlaceholder')"
            />
            <div class="mt-3 flex flex-wrap items-start justify-between gap-3">
              <UploadDragger
                v-model:file-list="replyFiles"
                class="max-w-md"
                :before-upload="() => false"
                multiple
              >
                <p class="ant-upload-text">{{ $t('pages.common.upload') }}</p>
                <p class="ant-upload-hint">
                  {{ $t('support.attachmentHint') }}
                </p>
              </UploadDragger>
              <Button
                data-testid="ticket-reply-submit"
                :disabled="!reply.trim()"
                :loading="submittingReply"
                type="primary"
                @click="sendReply"
                >{{ $t('support.reply') }}</Button
              >
            </div>
          </template>
        </Card>

        <aside class="grid content-start gap-4">
          <Card :bordered="false">
            <dl class="ticket-facts">
              <dt>{{ $t('support.ticketNo') }}</dt>
              <dd>{{ ticket.ticketNo }}</dd>
              <dt>{{ $t('support.stage') }}</dt>
              <dd>
                <Tag>{{ $t(`support.${ticket.stage}`) }}</Tag>
              </dd>
              <template v-if="categoryName()">
                <dt>{{ $t('support.category') }}</dt>
                <dd>{{ categoryName() }}</dd>
              </template>
              <dt>{{ $t('support.updatedAt') }}</dt>
              <dd>{{ formatTime(ticket.updatedAt) }}</dd>
            </dl>
          </Card>
          <Card
            v-if="ticket.relatedEntitlement"
            :bordered="false"
            data-testid="ticket-related-entitlement"
          >
            <strong>{{ $t('support.relatedEntitlement') }}</strong>
            <p class="mb-0 mt-2 text-sm">
              {{ $t('support.entitlementAtCreation') }}：{{ entitlementName() }}
            </p>
            <p class="mb-0 mt-2 text-sm text-muted-foreground">
              {{ entitlementStatus(ticket.relatedEntitlement.snapshotStatus) }}
            </p>
            <template v-if="ticket.relatedEntitlement.currentAvailable">
              <p class="mb-0 mt-3 text-sm">
                {{ $t('support.entitlementCurrent') }}：{{ entitlementName(true) }}
              </p>
              <p class="mb-0 mt-2 text-sm text-muted-foreground">
                {{ entitlementStatus(ticket.relatedEntitlement.currentStatus) }}
              </p>
            </template>
            <Alert
              v-else
              class="mt-3"
              :message="$t('support.entitlementCurrentUnavailable')"
              show-icon
              type="info"
            />
          </Card>
          <Card
            v-if="ticket.customFields.length"
            :bordered="false"
            data-testid="ticket-custom-field-snapshots"
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
          >
            <Space direction="vertical" class="w-full">
              <Button
                v-if="ticket.actions.canClose"
                block
                :loading="actionLoading"
                @click="confirmClose"
              >
                {{ $t('support.close') }}
              </Button>
              <Button
                v-if="ticket.actions.canReopen"
                block
                :loading="actionLoading"
                type="primary"
                @click="reopen"
              >
                {{ $t('support.reopen') }}
              </Button>
              <Button
                v-if="ticket.actions.canRate && !ticket.rating"
                block
                data-testid="ticket-rate"
                @click="openRating"
              >
                {{ $t('support.rate') }}
              </Button>
              <p
                v-if="ticket.actions.canReopen && ticket.reopenDeadline"
                class="m-0 text-xs text-muted-foreground"
              >
                {{
                  $t('support.reopenDeadline', {
                    date: formatTime(ticket.reopenDeadline),
                  })
                }}
              </p>
            </Space>
          </Card>
          <Card v-if="ticket.rating" :bordered="false">
            <strong>{{ $t('support.submittedRating') }}</strong>
            <p class="mb-0 mt-2">
              {{ $t('support.ratingStars', { stars: ticket.rating.stars }) }}
            </p>
            <p
              v-if="ticket.rating.feedback"
              class="mb-0 mt-2 whitespace-pre-wrap text-sm text-muted-foreground"
            >
              {{ ticket.rating.feedback }}
            </p>
          </Card>
        </aside>
      </div>
    </Spin>

    <CloseModal :title="$t('support.close')">
      <p>{{ $t('support.closeConfirm') }}</p>
    </CloseModal>
    <RatingModal :title="$t('support.rate')">
      <Space class="w-full" direction="vertical" size="middle">
        <Rate
          v-model:value="ratingStars"
          :aria-label="$t('support.rate')"
          data-testid="ticket-rating-stars"
        />
        <Alert
          v-if="ratingStars > 0 && ratingStars <= 2"
          :message="$t('support.ratingLowHint')"
          show-icon
          type="info"
        />
        <Input.TextArea
          v-model:value="ratingFeedback"
          data-testid="ticket-rating-feedback"
          :auto-size="{ minRows: 3, maxRows: 8 }"
          :placeholder="$t('support.ratingComment')"
        />
      </Space>
    </RatingModal>
  </Page>
</template>

<style scoped>
.ticket-details-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-items: start;
  gap: 16px;
}
.ticket-details-layout > * {
  min-width: 0;
}
@media (min-width: 1200px) {
  .ticket-details-layout {
    grid-template-columns: minmax(0, 1fr) 288px;
  }
}
.timeline {
  display: grid;
  gap: 1rem;
}
.message-item {
  padding: 1rem;
  background: hsl(var(--muted));
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
  color: hsl(var(--muted-foreground));
}
.ticket-facts dd {
  min-width: 0;
  margin: 0;
  text-align: right;
  overflow-wrap: anywhere;
}
</style>

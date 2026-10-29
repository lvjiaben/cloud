<script setup lang="ts">
import type { CustomerAnnouncement } from '#/api/idc/announcements';
import { computed, ref } from 'vue';
import { useVbenModal } from '@vben/common-ui';
import { IconifyIcon } from '@vben/icons';
import { Alert, Button, Empty, Pagination, Spin, Tag } from 'ant-design-vue';
import dayjs from 'dayjs';
import { getCustomerAnnouncement, listCustomerAnnouncements, readCustomerAnnouncement } from '#/api/idc/announcements';
import { $t } from '#/locales';
import { announcementHtml } from '#/utils/announcement-html';

const items = ref<CustomerAnnouncement[]>([]);
const total = ref(0);
const page = ref(1);
const size = ref(10);
const loading = ref(false);
const failed = ref(false);
const detail = ref<CustomerAnnouncement>();
const detailLoading = ref(false);
const detailFailed = ref(false);
const readFailed = ref(false);
const html = computed(() => announcementHtml(detail.value?.content || ''));
let listAttempt = 0;
let detailAttempt = 0;
const [ListModal, listModal] = useVbenModal({ centered: true, fullscreenButton: false, footer: false });
const [DetailModal, detailModal] = useVbenModal({ centered: true, fullscreenButton: false, footer: false, onOpenChange(open) { if (!open) detailAttempt += 1; } });
const time = (value: number) => value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—';
async function load() {
  const attempt = ++listAttempt;
  loading.value = true;
  failed.value = false;
  try {
    const result = await listCustomerAnnouncements(page.value, size.value);
    if (attempt !== listAttempt) return;
    items.value = result.list;
    total.value = result.total;
  } catch { if (attempt === listAttempt) failed.value = true; }
  finally { if (attempt === listAttempt) loading.value = false; }
}
function open() { page.value = 1; listModal.open(); void load(); }
function changePage(current: number, pageSize: number) { page.value = current; size.value = pageSize; void load(); }
async function showDetail(id: number) {
  const attempt = ++detailAttempt;
  detail.value = undefined;
  detailLoading.value = true;
  detailFailed.value = false;
  readFailed.value = false;
  detailModal.open();
  try {
    const item = await getCustomerAnnouncement(id);
    if (attempt !== detailAttempt) return;
    detail.value = item;
    detailLoading.value = false;
    if (!item.read) {
      try {
        await readCustomerAnnouncement(id);
        const row = items.value.find((entry) => entry.id === id);
        if (row) row.read = true;
      } catch { if (attempt === detailAttempt) readFailed.value = true; }
    }
  } catch { if (attempt === detailAttempt) detailFailed.value = true; }
  finally { if (attempt === detailAttempt) detailLoading.value = false; }
}
</script>

<template>
  <Button type="text" :aria-label="$t('announcements.title')" class="announcement-trigger" @click="open">
    <IconifyIcon icon="lucide:bell" class="size-5" />
  </Button>
  <ListModal :title="$t('announcements.title')" class="w-[min(94vw,720px)]">
    <Spin :spinning="loading">
      <div class="announcement-list" data-testid="announcement-list">
        <Alert v-if="failed" type="error" :message="$t('announcements.loadFailed')" show-icon>
          <template #action><Button size="small" @click="load">{{ $t('announcements.retry') }}</Button></template>
        </Alert>
        <template v-else>
          <Empty v-if="!loading && items.length === 0" :description="$t('announcements.empty')" />
          <button v-for="item in items" :key="item.id" type="button" class="announcement-row" @click="showDetail(item.id)">
            <span class="announcement-row-title">{{ item.title }}</span>
            <Tag :color="item.read ? undefined : 'blue'">{{ $t(item.read ? 'announcements.read' : 'announcements.unread') }}</Tag>
            <time>{{ time(item.createdAt) }}</time>
          </button>
        </template>
      </div>
    </Spin>
    <Pagination :current="page" :page-size="size" :total="total" :show-total="(count: number) => $t('announcements.total', { count })" :page-size-options="['10', '20', '50']" show-size-changer @change="changePage" />
  </ListModal>
  <DetailModal :title="detail?.title || $t('announcements.detail')" class="w-[min(94vw,800px)]">
    <Spin :spinning="detailLoading">
      <Alert v-if="detailFailed" type="error" :message="$t('announcements.loadFailed')" show-icon />
      <article v-if="detail" data-testid="announcement-detail">
        <p class="mb-4 text-muted-foreground">{{ time(detail.createdAt) }}</p>
        <div class="announcement-content" v-html="html"></div>
      </article>
      <Alert v-if="readFailed" class="mt-4" type="error" :message="$t('announcements.readFailed')" show-icon />
    </Spin>
  </DetailModal>
</template>

<style scoped>
.announcement-trigger { display: inline-flex; align-items: center; justify-content: center; }
.announcement-list { min-height: 80px; max-height: 54vh; overflow: auto; margin-bottom: 16px; }
.announcement-row { display: flex; width: 100%; align-items: center; gap: 12px; padding: 16px 0; border-bottom: 1px solid hsl(var(--border)); text-align: left; cursor: pointer; }
.announcement-row:hover { color: hsl(var(--primary)); }
.announcement-row-title { flex: 1; min-width: 0; overflow-wrap: anywhere; font-weight: 500; }
.announcement-row time { color: hsl(var(--muted-foreground)); font-size: 12px; }
.announcement-content { max-height: 60vh; overflow: auto; overflow-wrap: anywhere; line-height: 1.8; }
.announcement-content :deep(img) { display: block; max-width: 100%; height: auto; margin: 12px 0; }
.announcement-content :deep(p) { margin: 8px 0; }
.announcement-content :deep(ul),.announcement-content :deep(ol) { padding-left: 24px; }
@media (max-width: 600px) { .announcement-row { flex-wrap: wrap; gap: 8px; } .announcement-row time { width: 100%; } }
</style>

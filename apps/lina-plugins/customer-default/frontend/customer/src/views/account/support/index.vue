<script lang="ts" setup>
import type {
  CustomerTicketStage,
  CustomerTicketSummary,
} from '#/api/idc/support';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import { Page, useVbenDrawer } from '@vben/common-ui';
import { preferences } from '@vben/preferences';

import { Alert, Button, Card, Tag } from 'ant-design-vue';
import dayjs from 'dayjs';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { listCustomerTickets } from '#/api/idc/support';
import { $t } from '#/locales';
import SupportTicketCreate from './create.vue';

const router = useRouter();
const loading = ref(false);
const failed = ref(false);
const currentPage = ref(1);
const pageSize = ref(10);
const total = ref(0);
const appliedQuery = ref('');
let lastAttempt = { page: 1, size: 10, query: '' };

function ticketColumns(): VxeGridProps<CustomerTicketSummary>['columns'] {
  return [
    { field: 'ticketNo', minWidth: 180, title: $t('support.ticketNo') },
    {
      field: 'subject',
      minWidth: 240,
      title: $t('support.subject'),
      slots: { default: 'subject' },
    },
    {
      field: 'category',
      width: 160,
      title: $t('support.category'),
      formatter: ({ row }) => categoryName(row) || '—',
    },
    {
      field: 'stage',
      align: 'center',
      width: 120,
      title: $t('support.stage'),
      slots: { default: 'stage' },
    },
    {
      field: 'updatedAt',
      align: 'center',
      width: 170,
      title: $t('support.updatedAt'),
      formatter: ({ row }) => formatTime(row.updatedAt),
    },
    {
      field: 'action',
      align: 'center',
      fixed: 'right',
      width: 100,
      title: $t('support.action'),
      slots: { default: 'action' },
    },
  ];
}

function searchSchema() {
  return [
    {
      component: 'Input' as const,
      componentProps: {
        allowClear: true,
        'data-testid': 'ticket-search-query',
      },
      fieldName: 'query',
      label: $t('support.searchKeyword'),
    },
  ];
}

const [TicketGrid, ticketGridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: { labelWidth: 80 },
    handleReset: async () => {
      await ticketGridApi.formApi.resetForm();
      await loadTickets(1, pageSize.value, '');
    },
    handleSubmit: async (values) => {
      await loadTickets(1, pageSize.value, String(values.query || '').trim());
    },
    schema: searchSchema(),
    showCollapseButton: false,
    submitOnEnter: true,
    wrapperClass: 'grid-cols-1 md:grid-cols-2 xl:grid-cols-3',
  },
  gridEvents: {
    pageChange({ currentPage: page, pageSize: size }: { currentPage: number; pageSize: number }) {
      void loadTickets(size === pageSize.value ? page : 1, size);
    },
  },
  gridOptions: {
    columns: ticketColumns(),
    data: [],
    height: '100%',
    id: 'customer-support-tickets',
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    rowConfig: { keyField: 'ticketNo' },
    showOverflow: 'tooltip',
    toolbarConfig: { custom: true, refresh: false, search: true },
  },
});

const [CreateTicketDrawer, createTicketDrawerApi] = useVbenDrawer({
  appendToMain: true,
  class: 'w-[min(96vw,960px)]',
  closeOnClickModal: false,
  footer: false,
});

async function created(ticketNo: string) {
  createTicketDrawerApi.close();
  await loadTickets();
  await router.push(`/account/support/${ticketNo}`);
}

function stageColor(value: CustomerTicketStage) {
  return {
    closed: 'default',
    in_progress: 'processing',
    new: 'blue',
    resolved: 'success',
    waiting_customer: 'warning',
  }[value];
}

async function loadTickets(
  page = currentPage.value,
  size = pageSize.value,
  query = appliedQuery.value,
) {
  if (loading.value) {
    ticketGridApi.setGridOptions({ pagerConfig: { currentPage: currentPage.value, pageSize: pageSize.value } });
    return;
  }
  lastAttempt = { page, size, query };
  loading.value = true;
  ticketGridApi.setLoading(true);
  failed.value = false;
  try {
    const result = await listCustomerTickets({
      page,
      limit: size,
      query: query || undefined,
    });
    ticketGridApi.setGridOptions({
      data: result.items || [],
      pagerConfig: { currentPage: page, pageSize: size, total: result.total },
    });
    currentPage.value = page;
    pageSize.value = size;
    total.value = result.total;
    appliedQuery.value = query;
  } catch (error) {
    failed.value = true;
    const status = (error as { response?: { status?: number } })?.response?.status;
    if (status === 401 || status === 403) {
      total.value = 0;
      currentPage.value = 1;
      ticketGridApi.setGridOptions({ data: [] });
      createTicketDrawerApi.close();
    }
    ticketGridApi.setGridOptions({ pagerConfig: { currentPage: currentPage.value, pageSize: pageSize.value, total: total.value } });
  } finally {
    loading.value = false;
    ticketGridApi.setLoading(false);
  }
}

function retryLoad() {
  return loadTickets(
    lastAttempt.page,
    lastAttempt.size,
    lastAttempt.query,
  );
}

function formatTime(timestamp: number) {
  return dayjs(timestamp).format('YYYY-MM-DD HH:mm');
}

function categoryName(ticket: CustomerTicketSummary) {
  if (!ticket.category) return '';
  return preferences.app.locale === 'en-US'
    ? ticket.category.nameEnUS
    : ticket.category.nameZhCN;
}

function stageLabel(stage: CustomerTicketStage) {
  const key = `support.${stage}`;
  const label = $t(key);
  return label === key ? $t('support.statusUnavailable') : label;
}

watch(
  () => preferences.app.locale,
  () => {
    ticketGridApi.setGridOptions({ columns: ticketColumns() });
    ticketGridApi.formApi.updateSchema(searchSchema());
  },
);

onMounted(() => loadTickets());
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" auto-content-height :title="$t('support.menu')">
    <Card
      :bordered="false"
      class="support-list-card"
      data-testid="customer-support-page"
    >
      <Alert
        v-if="failed"
        class="mb-4"
        :message="$t('support.loadFailed')"
        show-icon
        type="error"
      >
        <template #action>
          <Button size="small" :disabled="loading" @click="retryLoad">{{
            $t('support.retry')
          }}</Button>
        </template>
      </Alert>

      <TicketGrid class="min-h-0 min-w-0 flex-1" data-testid="ticket-list">
        <template #toolbar-actions>
          <Button
            data-testid="new-ticket-action"
            type="primary"
            @click="createTicketDrawerApi.open()"
          >
            {{ $t('support.newTicket') }}
          </Button>
        </template>
        <template #toolbar-tools>
          <Button :loading="loading" @click="loadTickets()">{{
            $t('common.refresh')
          }}</Button>
        </template>
        <template #subject="{ row }">
          <Button
            class="max-w-full truncate !p-0"
            type="link"
            @click="router.push(`/account/support/${row.ticketNo}`)"
          >
            {{ row.subject }}
          </Button>
        </template>
        <template #stage="{ row }">
          <Tag :color="stageColor(row.stage)">{{ stageLabel(row.stage) }}</Tag>
        </template>
        <template #action="{ row }">
          <ghost-button
            :data-testid="`ticket-detail-${row.ticketNo}`"
            @click="router.push(`/account/support/${row.ticketNo}`)"
          >
            {{ $t('support.viewDetail') }}
          </ghost-button>
        </template>
        <template #empty>{{ $t('support.empty') }}</template>
      </TicketGrid>
    </Card>
    <CreateTicketDrawer :title="$t('support.createTitle')">
      <SupportTicketCreate
        embedded
        @cancel="createTicketDrawerApi.close()"
        @created="created"
      />
    </CreateTicketDrawer>
  </Page>
</template>

<style scoped>
.support-list-card {
  height: 100%;
  min-width: 0;
  overflow: hidden;
}

.support-list-card :deep(.ant-card-body) {
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  padding: 0;
}

</style>

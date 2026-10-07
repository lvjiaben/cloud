<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { CustomerProfile } from '#/api/idc/customer-account';
import type { FinancialOverview } from '#/api/idc/customer-finance';
import type { CustomerServer } from '#/api/idc/customer-servers';
import type { CustomerTicketSummary } from '#/api/idc/support';

import { computed, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';
import { IconifyIcon } from '@vben/icons';
import { preferences } from '@vben/preferences';

import { Alert, Button, Card, Skeleton, Tag } from 'ant-design-vue';

import { getCustomerProfile } from '#/api/idc/customer-account';
import { getFinancialOverview } from '#/api/idc/customer-finance';
import { listCustomerServers } from '#/api/idc/customer-servers';
import { listCustomerTickets } from '#/api/idc/support';
import { $t } from '#/locales';
import { useVbenVxeGrid } from '#/adapter/vxe-table';

const router = useRouter();
const loading = ref(true);
const failedSections = ref<string[]>([]);
const profile = ref<CustomerProfile>();
const finance = ref<FinancialOverview>();
const servers = ref<CustomerServer[]>([]);
const tickets = ref<CustomerTicketSummary[]>([]);
const hasMoreServers = ref(false);
const hasMoreTickets = ref(false);

const locale = computed(() =>
  preferences.app.locale === 'en-US' ? 'en-US' : 'zh-CN',
);
const balanceRows = computed(() => {
  if (!finance.value) return [];
  if (finance.value.balances.length) return finance.value.balances;
  return [
    {
      accountTypeCode: 'customer_balance',
      availableBalanceMinor: '0',
      balanceMinor: '0',
      currencyCode: 'CNY',
      heldBalanceMinor: '0',
      statusCode: 'active',
      updatedAt: 0,
      withdrawable: false,
    },
  ];
});
const openTicketCount = computed(
  () =>
    tickets.value.filter((item) => !['closed', 'resolved'].includes(item.stage))
      .length,
);
const activeServerCount = computed(
  () =>
    servers.value.filter(
      (item) =>
        item.entitlementStatus === 'active' || item.runtimeStatus === 'running',
    ).length,
);
const nearExpiryCount = computed(() => {
  const now = Date.now();
  const deadline = now + 7 * 24 * 60 * 60 * 1000;
  return servers.value.filter((item) => {
    const expiresAt = item.entitlementExpiresAt || 0;
    return expiresAt > now && expiresAt <= deadline;
  }).length;
});

const quickLinks = computed(() => [
  {
    description: $t('page.customer.quickAccess.productsDescription'),
    icon: 'lucide:boxes',
    path: '/account/products/catalog',
    title: $t('page.customer.quickAccess.products'),
  },
  {
    description: $t('page.customer.quickAccess.serversDescription'),
    icon: 'lucide:server',
    path: '/account/servers',
    title: $t('page.customer.quickAccess.servers'),
  },
  {
    description: $t('page.customer.quickAccess.ordersDescription'),
    icon: 'lucide:clipboard-list',
    path: '/account/orders',
    title: $t('page.customer.quickAccess.orders'),
  },
  {
    description: $t('page.customer.quickAccess.financeDescription'),
    icon: 'lucide:wallet-cards',
    path: '/account/billing',
    title: $t('page.customer.quickAccess.finance'),
  },
  {
    description: $t('page.customer.quickAccess.supportDescription'),
    icon: 'lucide:messages-square',
    path: '/account/support',
    title: $t('page.customer.quickAccess.support'),
  },
]);

function serverColumns(): VxeGridProps<CustomerServer>['columns'] {
  return [
    {
      field: 'serviceName',
      minWidth: 220,
      slots: { default: 'serviceName' },
      title: $t('page.customer.summary.serviceName'),
    },
    {
      field: 'serviceStatus',
      slots: { default: 'serviceStatus' },
      title: $t('page.customer.summary.serviceStatus'),
      width: 140,
    },
    {
      field: 'expiresAt',
      slots: { default: 'expiresAt' },
      title: $t('page.customer.summary.expiresAt'),
      width: 190,
    },
    {
      field: 'serviceNo',
      minWidth: 180,
      slots: { default: 'serviceNo' },
      title: $t('page.customer.summary.serviceNo'),
    },
    {
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      title: $t('page.customer.summary.action'),
      width: 100,
    },
  ];
}

const [ServerGrid, serverGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: serverColumns(),
    data: [],
    height: 300,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: 'serviceNo' },
    showOverflow: true,
  },
});

function formatAmount(value: string | undefined, currencyCode: string) {
  const minor = Number(value || 0);
  if (!Number.isFinite(minor)) return '—';
  return new Intl.NumberFormat(locale.value, {
    currency: currencyCode || 'CNY',
    style: 'currency',
  }).format(minor / 100);
}

function formatTime(value?: number) {
  return value ? new Date(value).toLocaleString(locale.value) : '—';
}

function serverName(item: CustomerServer) {
  return (
    item.customerAlias ||
    (locale.value === 'en-US' ? item.productNameEnUS : item.productNameZhCN) ||
    item.serviceNo
  );
}

function statusText(scope: 'server' | 'ticket', value: string) {
  const key =
    scope === 'server' ? `servers.status.${value}` : `support.${value}`;
  const result = $t(key);
  return result === key ? $t('page.customer.summary.unknown') : result;
}

function countText(value: number, hasMore: boolean) {
  return hasMore ? `${value}+` : String(value);
}

function navigate(path: string) {
  void router.push(path);
}

async function load() {
  loading.value = true;
  failedSections.value = [];
  const [profileResult, financeResult, serverResult, ticketResult] =
    await Promise.allSettled([
      getCustomerProfile(),
      getFinancialOverview(),
      listCustomerServers({ limit: 20 }),
      listCustomerTickets({ limit: 20 }),
    ]);

  if (profileResult.status === 'fulfilled') {
    profile.value = profileResult.value.item;
  } else failedSections.value.push('profile');

  if (financeResult.status === 'fulfilled') {
    finance.value = financeResult.value.overview;
  } else failedSections.value.push('finance');

  if (serverResult.status === 'fulfilled') {
    servers.value = serverResult.value.items || [];
    hasMoreServers.value = Boolean(serverResult.value.nextCursor);
  } else failedSections.value.push('servers');

  if (ticketResult.status === 'fulfilled') {
    tickets.value = ticketResult.value.items || [];
    hasMoreTickets.value = Boolean(ticketResult.value.nextCursor);
  } else failedSections.value.push('tickets');

  loading.value = false;
}

watch(
  servers,
  (items) => serverGridApi.setGridOptions({ data: items.slice(0, 5) }),
  { deep: true },
);
watch(
  () => preferences.app.locale,
  () => serverGridApi.setGridOptions({ columns: serverColumns() }),
);

onMounted(load);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content"
    :description="$t('page.customer.description')"
    :title="$t('page.customer.overview')"
  >
    <Skeleton v-if="loading" active :paragraph="{ rows: 14 }" />
    <div v-else class="customer-overview" data-testid="customer-overview">
      <Alert
        v-if="failedSections.length"
        :message="
          failedSections.length === 4
            ? $t('page.customer.summary.loadFailed')
            : $t('page.customer.summary.partialFailed')
        "
        show-icon
        type="warning"
      >
        <template #action>
          <Button size="small" @click="load">
            {{ $t('page.customer.summary.retry') }}
          </Button>
        </template>
      </Alert>

      <section class="account-strip">
        <div class="account-mark" aria-hidden="true">
          <IconifyIcon icon="lucide:user-round" />
        </div>
        <div class="account-copy">
          <span>{{ $t('page.customer.summary.welcome') }}</span>
          <strong>{{ profile?.displayName || '—' }}</strong>
          <small>
            {{ $t('page.customer.summary.customerCode') }}
            {{ profile?.customerCode || '—' }}
          </small>
        </div>
        <div class="account-tags">
          <Tag color="blue">
            {{
              profile?.type === 'enterprise'
                ? $t('page.customer.summary.enterprise')
                : $t('page.customer.summary.individual')
            }}
          </Tag>
          <Tag
            :color="
              profile?.verification === 'verified' ? 'success' : 'warning'
            "
          >
            {{
              profile?.verification === 'verified'
                ? $t('page.customer.summary.verified')
                : $t('page.customer.summary.unverified')
            }}
          </Tag>
        </div>
        <Button @click="navigate('/account/profile')">
          {{ $t('page.customer.summary.manageAccount') }}
        </Button>
      </section>

      <div class="overview-grid">
        <Card
          class="balance-panel"
          :title="$t('page.customer.summary.balance')"
        >
          <div v-if="balanceRows.length" class="balance-list">
            <div
              v-for="item in balanceRows"
              :key="`${item.accountTypeCode}-${item.currencyCode}`"
            >
              <span>{{ item.currencyCode }}</span>
              <strong>{{
                formatAmount(item.availableBalanceMinor, item.currencyCode)
              }}</strong>
              <small>
                {{ $t('page.customer.summary.frozen') }}
                {{ formatAmount(item.heldBalanceMinor, item.currencyCode) }}
              </small>
            </div>
          </div>
          <div v-else class="unavailable">
            {{ $t('page.customer.summary.unavailable') }}
          </div>
          <div class="panel-actions">
            <Button type="primary" @click="navigate('/account/billing')">
              {{ $t('page.customer.summary.recharge') }}
            </Button>
            <Button @click="navigate('/account/billing/transactions')">
              {{ $t('page.customer.summary.transactions') }}
            </Button>
          </div>
        </Card>

        <div class="metric-grid">
          <Button
            class="metric-card"
            @click="navigate('/account/billing/invoices')"
          >
            <span>{{ $t('page.customer.summary.openInvoices') }}</span>
            <strong>{{ finance?.openInvoiceCount ?? '—' }}</strong>
            <small>{{ $t('page.customer.summary.viewDetails') }}</small>
          </Button>
          <Button class="metric-card" @click="navigate('/account/servers')">
            <span>{{ $t('page.customer.summary.activeServers') }}</span>
            <strong>{{ countText(activeServerCount, hasMoreServers) }}</strong>
            <small>{{ $t('page.customer.summary.viewDetails') }}</small>
          </Button>
          <Button class="metric-card" @click="navigate('/account/servers')">
            <span>{{ $t('page.customer.summary.expiringSoon') }}</span>
            <strong>{{ countText(nearExpiryCount, hasMoreServers) }}</strong>
            <small>{{ $t('page.customer.summary.withinSevenDays') }}</small>
          </Button>
          <Button class="metric-card" @click="navigate('/account/support')">
            <span>{{ $t('page.customer.summary.openTickets') }}</span>
            <strong>{{ countText(openTicketCount, hasMoreTickets) }}</strong>
            <small>{{ $t('page.customer.summary.viewDetails') }}</small>
          </Button>
        </div>
      </div>

      <Card :title="$t('page.customer.summary.services')">
        <div v-if="servers.length" class="service-table-wrap">
          <ServerGrid>
            <template #serviceName="{ row }">
              <strong>{{ serverName(row) }}</strong>
            </template>
            <template #serviceStatus="{ row }">
              <Tag color="blue">{{
                statusText('server', row.runtimeStatus || row.entitlementStatus)
              }}</Tag>
            </template>
            <template #expiresAt="{ row }">
              {{ formatTime(row.entitlementExpiresAt) }}
            </template>
            <template #serviceNo="{ row }">
              <code>{{ row.serviceNo }}</code>
            </template>
            <template #action="{ row }">
              <Button
                type="link"
                @click="navigate(`/account/servers/${row.serviceNo}`)"
              >
                {{ $t('page.customer.summary.manage') }}
              </Button>
            </template>
          </ServerGrid>
        </div>
        <div v-else class="empty-state">
          {{ $t('page.customer.summary.noServices') }}
        </div>
      </Card>

      <Card :title="$t('page.customer.quickAccess.title')">
        <div class="quick-access-grid">
          <Button
            v-for="item in quickLinks"
            :key="item.path"
            class="quick-access-item"
            @click="navigate(item.path)"
          >
            <span class="quick-access-icon" aria-hidden="true">
              <IconifyIcon :icon="item.icon" />
            </span>
            <span class="quick-access-copy">
              <strong>{{ item.title }}</strong>
              <small>{{ item.description }}</small>
            </span>
            <IconifyIcon aria-hidden="true" icon="lucide:arrow-right" />
          </Button>
        </div>
      </Card>
    </div>
  </Page>
</template>

<style scoped>
.customer-overview {
  display: grid;
  gap: 16px;
}
.account-strip {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 16px;
  min-height: 88px;
  padding: 16px 20px;
  border: 1px solid hsl(var(--border));
  background: hsl(var(--card));
}
.account-mark {
  display: grid;
  width: 44px;
  height: 44px;
  place-items: center;
  background: hsl(var(--primary));
  color: white;
  font-size: 24px;
}
.account-copy {
  display: grid;
  gap: 3px;
}
.account-copy span,
.account-copy small {
  color: hsl(var(--muted-foreground));
}
.account-copy strong {
  font-size: 18px;
}
.account-tags {
  display: flex;
  gap: 8px;
}
.overview-grid {
  display: grid;
  grid-template-columns: minmax(300px, 0.8fr) minmax(520px, 2fr);
  gap: 16px;
}
.balance-list {
  display: grid;
  gap: 16px;
}
.balance-list > div {
  display: grid;
  gap: 4px;
}
.balance-list span,
.balance-list small {
  color: hsl(var(--muted-foreground));
}
.balance-list strong {
  color: hsl(var(--foreground));
  font-size: 30px;
  line-height: 1.2;
}
.panel-actions {
  display: flex;
  gap: 8px;
  margin-top: 20px;
}
.metric-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  border-top: 1px solid hsl(var(--border));
  border-left: 1px solid hsl(var(--border));
}
.metric-card.ant-btn {
  display: grid;
  height: auto;
  min-height: 104px;
  gap: 6px;
  padding: 18px;
  border-right: 1px solid hsl(var(--border));
  border-bottom: 1px solid hsl(var(--border));
  background: hsl(var(--card));
  color: inherit;
  text-align: left;
}
.metric-card.ant-btn:hover {
  background: hsl(var(--muted) / 70%);
}
.metric-grid span,
.metric-grid small {
  color: hsl(var(--muted-foreground));
}
.metric-grid strong {
  font-size: 28px;
  line-height: 1.2;
}
.metric-grid small {
  color: hsl(var(--primary));
}
.service-table-wrap {
  overflow-x: auto;
}
.service-table-wrap code {
  font-size: 12px;
}
.quick-access-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  border-top: 1px solid hsl(var(--border));
  border-left: 1px solid hsl(var(--border));
}
.quick-access-item {
  display: grid;
  height: auto;
  grid-template-columns: 34px minmax(0, 1fr) auto;
  align-items: center;
  min-height: 76px;
  gap: 10px;
  padding: 14px;
  border-right: 1px solid hsl(var(--border));
  border-bottom: 1px solid hsl(var(--border));
  background: hsl(var(--card));
  color: inherit;
  text-align: left;
}
.quick-access-item:hover {
  background: hsl(var(--muted) / 70%);
  color: hsl(var(--primary));
}
.quick-access-icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  background: hsl(var(--primary) / 10%);
  color: hsl(var(--primary));
  font-size: 17px;
}
.quick-access-copy {
  display: grid;
  min-width: 0;
  gap: 4px;
}
.quick-access-copy strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.quick-access-copy small,
.empty-state,
.unavailable {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  line-height: 1.5;
}
@media (max-width: 1100px) {
  .overview-grid {
    grid-template-columns: 1fr;
  }
  .quick-access-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 700px) {
  .account-strip {
    grid-template-columns: auto minmax(0, 1fr);
  }
  .account-tags,
  .account-strip > .ant-btn {
    grid-column: 1 / -1;
  }
  .account-strip > .ant-btn {
    width: 100%;
    margin: 0;
    justify-content: center;
  }
  .metric-grid,
  .quick-access-grid {
    grid-template-columns: 1fr;
  }
}
</style>

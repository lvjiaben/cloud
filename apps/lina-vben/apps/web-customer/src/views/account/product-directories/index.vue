<script setup lang="ts">
import type { VxeGridProps } from '#/adapter/vxe-table';
import type {
  CustomerProductDirectory,
  DirectoryService,
} from '#/api/idc/product-directories';

import { computed, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';
import { preferences } from '@vben/preferences';

import { Alert, Button, Pagination, Space, Tag } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  listCustomerProductDirectories,
  listDirectoryServices,
} from '#/api/idc/product-directories';
import { $t } from '#/locales';
import { refreshAccessibleState } from '#/router/access-refresh';

const router = useRouter();
const route = useRoute();
const directories = ref<CustomerProductDirectory[]>([]);
const selected = computed(() =>
  String(route.params.directoryNo || route.meta.productDirectoryNo || 'all'),
);
const directoryPage = ref(1);
const directoryTotal = ref(0);
const failed = ref(false);
const key = 'productDirectories.';
let directoryRevision = 0;
function name(row: CustomerProductDirectory) {
  return preferences.app.locale === 'zh-CN'
    ? row.nameZhCn || row.nameEnUs
    : row.nameEnUs || row.nameZhCn;
}
function productName(row: DirectoryService) {
  return preferences.app.locale === 'zh-CN'
    ? row.productNameZhCn || row.productNameEnUs
    : row.productNameEnUs || row.productNameZhCn;
}
function date(value: null | number, empty = 'noExpiry') {
  return value && value > 0
    ? new Date(value).toLocaleString(preferences.app.locale)
    : $t(key + empty);
}
function columns(): VxeGridProps<DirectoryService>['columns'] {
  return [
    {
      field: 'product',
      title: $t(`${key}product`),
      minWidth: 200,
      slots: { default: 'product' },
    },
    {
      field: 'statusCode',
      title: $t(`${key}status`),
      width: 140,
      slots: { default: 'status' },
    },
    {
      field: 'startedAt',
      title: $t(`${key}startedAt`),
      minWidth: 170,
      formatter: ({ cellValue }) => date(cellValue, 'notStarted'),
    },
    {
      field: 'expiresAt',
      title: $t(`${key}expiresAt`),
      minWidth: 170,
      formatter: ({ cellValue }) => date(cellValue),
    },
    {
      field: 'action',
      title: $t(`${key}actions`),
      width: 130,
      fixed: 'right',
      slots: { default: 'action' },
    },
  ];
}
const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: [],
    height: '100%',
    rowConfig: { keyField: 'entitlementNo' },
    pagerConfig: {
      enabled: true,
      pageSize: 20,
      pageSizes: [20, 50, 100],
      layouts: [
        'Total',
        'Sizes',
        'PrevJump',
        'PrevPage',
        'Number',
        'NextPage',
        'NextJump',
        'FullJump',
      ],
    },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async ({
          page,
        }: {
          page: { currentPage: number; pageSize: number };
        }) =>
          listDirectoryServices(
            selected.value,
            page.currentPage,
            page.pageSize,
          ),
      },
    },
    toolbarConfig: { refresh: true, zoom: true },
    id: 'customer-directory-services',
  },
});
async function refreshDirectories(page = directoryPage.value) {
  const revision = ++directoryRevision;
  try {
    const result = await listCustomerProductDirectories(page, 100);
    if (revision !== directoryRevision) return false;
    directories.value = result.items;
    directoryTotal.value = result.total;
    directoryPage.value = page;
    failed.value = false;
    return true;
  } catch (error) {
    if (revision === directoryRevision) failed.value = true;
    throw error;
  }
}
async function selectDirectory(directoryNo: string) {
  await router.push(
    directoryNo === 'all'
      ? '/account/product-directories'
      : `/account/product-directories/${encodeURIComponent(directoryNo)}`,
  );
}
async function changePage(page: number) {
  await refreshDirectories(page);
}
async function retryDirectories() {
  await refreshAccessibleState(router, {
    refreshUserInfo: false,
    skipRouteNavigation: true,
  });
  await refreshDirectories(
    Number(router.resolve(route.fullPath).meta.productDirectoryPage) || 1,
  );
}
watch(selected, async () => {
  await Promise.allSettled([
    refreshDirectories(
      Number(router.resolve(route.fullPath).meta.productDirectoryPage) || 1,
    ),
    gridApi.reload(),
  ]);
});
watch(
  () => preferences.app.locale,
  () => gridApi.setGridOptions({ columns: columns() }),
);
onMounted(async () => {
  gridApi.setGridOptions({ columns: columns() });
  await Promise.allSettled([
    refreshDirectories(
      Number(router.resolve(route.fullPath).meta.productDirectoryPage) || 1,
    ),
    gridApi.query(),
  ]);
});
</script>
<template>
  <Page auto-content-height>
    <div
      data-testid="customer-product-directories"
      class="flex h-full flex-col gap-3"
    >
      <Alert v-if="failed" type="error" :message="$t(`${key}loadFailed`)">
        <template #description>
          <Button @click="retryDirectories()">
            {{ $t(`${key}retry`) }}
          </Button>
        </template>
      </Alert>
      <Space wrap>
        <Button
          :type="selected === 'all' ? 'primary' : 'default'"
          data-testid="directory-all"
          @click="selectDirectory('all')"
        >
          {{ $t(`${key}all`) }}
        </Button>
        <Button
          v-for="row in directories"
          :key="row.directoryNo"
          :type="selected === row.directoryNo ? 'primary' : 'default'"
          @click="selectDirectory(row.directoryNo)"
        >
          {{ name(row) }} ({{ row.serviceCount }})
        </Button>
      </Space>
      <Pagination
        v-if="directoryTotal > 100"
        :current="directoryPage"
        :total="directoryTotal"
        :page-size="100"
        :show-size-changer="false"
        @change="changePage"
      />
      <Grid class="min-h-0 flex-1">
        <template #product="{ row }">{{ productName(row) }}</template>
        <template #status="{ row }">
          <Tag>{{ $t(`${key}states.${row.statusCode}`) }}</Tag>
        </template>
        <template #action="{ row }">
          <Button
            v-if="row.orderNo"
            type="link"
            @click="
              router.push({
                name: 'CustomerOrderDetail',
                params: { orderNo: row.orderNo },
              })
            "
          >
            {{ $t(`${key}viewOrder`) }}
          </Button>
        </template>
      </Grid>
    </div>
  </Page>
</template>

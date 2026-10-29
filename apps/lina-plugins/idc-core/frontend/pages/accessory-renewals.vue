<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/accessory-renewals",
  title: "Accessory Renewal Work Items",
};
</script>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { Page } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { useAccessStore, useUserStore } from "@vben/stores";
import { Alert, Button } from "ant-design-vue";
import { useVbenVxeGrid, type VxeTableGridOptions } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { useTenantStore } from "#/store/tenant";
import { formatTimestamp } from "#/utils/time";
import { useAccess } from "./plugin-access";
import {
  accessoryRenewalWorkItems,
  syncAccessoryRenewalNotifications,
  type AccessoryRenewalWorkItem,
} from "../slots/layout/header/actions/accessory-renewal-client";

const router = useRouter();
const accessStore = useAccessStore();
const userStore = useUserStore();
const tenantStore = useTenantStore();
const { hasAccessByCodes } = useAccess();
const allowed = computed(() =>
  Boolean(
    accessStore.accessToken &&
    userStore.userInfo?.userId &&
    !accessStore.loginExpired &&
    hasAccessByCodes(["idc:admin:resource:accessory-renewal:handle"]),
  ),
);
const canViewInvoice = computed(
  () =>
    allowed.value &&
    [
      "idc:admin:billing:overview",
      "idc:admin:billing:invoice:list",
      "idc:admin:billing:invoice:query",
    ].every((code) => hasAccessByCodes([code])),
);
const locale = computed(() =>
  preferences.app.locale === "en-US" ? "en-US" : "zh-CN",
);
const t = (key: string) => $t(`plugin.idc-core.renewal.${key}`);
const total = ref<number | null>(null);
const syncing = ref(false);
const syncFailed = ref(false);
const listFailed = ref(false);
let mounted = false;
let generation = 0;
let syncSequence = 0;
let listSequence = 0;
let currentPage = 1;
let pageSize = 20;
let timer: ReturnType<typeof setInterval> | undefined;

function columns(): VxeTableGridOptions<AccessoryRenewalWorkItem>["columns"] {
  return [
    { field: "subjectNo", title: t("subject"), minWidth: 190 },
    { field: "customerNo", title: t("customer"), width: 175 },
    { field: "invoiceNo", title: t("invoice"), width: 200 },
    {
      field: "reasonCode",
      title: t("status"),
      width: 175,
      slots: { default: "reason" },
    },
    {
      field: "createdAt",
      title: t("createdAt"),
      width: 175,
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
    },
    ...(canViewInvoice.value
      ? [
          {
            field: "action",
            title: t("action"),
            width: 120,
            fixed: "right" as const,
            slots: { default: "action" },
          },
        ]
      : []),
  ];
}

const [Grid, gridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange: ({
      currentPage: page,
      pageSize: size,
    }: {
      currentPage: number;
      pageSize: number;
    }) => void loadItems(page, size),
  },
  gridOptions: {
    columns: [],
    data: [],
    height: "100%",
    rowConfig: { keyField: "workItemNo" },
    pagerConfig: {
      enabled: true,
      currentPage: 1,
      pageSize: 20,
      total: 0,
      pageSizes: [20, 50],
      layouts: ["Total", "Sizes", "PrevPage", "Number", "NextPage"],
    },
    toolbarConfig: { enabled: false },
    emptyRender: { name: "Empty" },
  },
});

// Track the translation resources as well as the selected locale; resources
// can finish loading after the preference has already changed.
watch(columns, (value) => gridApi.setGridOptions({ columns: value }), {
  immediate: true,
});

function clearGrid() {
  currentPage = 1;
  pageSize = 20;
  gridApi.setGridOptions({
    data: [],
    loading: false,
    pagerConfig: { currentPage, pageSize, total: 0 },
  });
}

async function loadItems(page = currentPage, size = pageSize) {
  if (!mounted || !allowed.value) return;
  const scope = generation;
  const request = ++listSequence;
  currentPage = page;
  pageSize = size;
  listFailed.value = false;
  gridApi.setGridOptions({ loading: true });
  const current = () =>
    scope === generation && request === listSequence && allowed.value;
  try {
    const result = await accessoryRenewalWorkItems(page, size);
    if (!current()) return;
    // A completed item may remove the last page while the page is open.
    if (!result.items.length && page > 1 && result.total > 0) {
      return await loadItems(Math.ceil(result.total / size), size);
    }
    total.value = result.total;
    currentPage = result.total === 0 ? 1 : page;
    gridApi.setGridOptions({
      data: result.items,
      pagerConfig: { currentPage, pageSize: size, total: result.total },
    });
  } catch {
    if (!current()) return;
    listFailed.value = true;
    gridApi.setGridOptions({
      data: [],
      pagerConfig: { currentPage: page, pageSize: size, total: 0 },
    });
  } finally {
    if (current()) gridApi.setGridOptions({ loading: false });
  }
}

async function synchronize() {
  if (
    !mounted ||
    !allowed.value ||
    syncing.value ||
    document.visibilityState !== "visible" ||
    !navigator.onLine
  )
    return;
  const scope = generation;
  const request = ++syncSequence;
  syncing.value = true;
  const current = () =>
    scope === generation && request === syncSequence && allowed.value;
  try {
    const result = await syncAccessoryRenewalNotifications(locale.value);
    if (!current()) return;
    total.value = result.total;
    syncFailed.value = false;
  } catch {
    if (current()) syncFailed.value = true;
  } finally {
    if (current()) syncing.value = false;
  }
}

async function refresh() {
  const scope = generation;
  await synchronize();
  if (scope === generation) await loadItems();
}

function resetSession() {
  generation += 1;
  syncSequence += 1;
  listSequence += 1;
  total.value = null;
  syncing.value = false;
  syncFailed.value = false;
  listFailed.value = false;
  clearGrid();
  if (mounted && allowed.value) {
    gridApi.setGridOptions({ columns: columns() });
    void refresh();
  }
}

function openInvoice(row: AccessoryRenewalWorkItem) {
  if (!canViewInvoice.value) return;
  void router.push({
    path: "/idc/finance",
    query: { invoiceNo: row.invoiceNo },
  });
}

watch(
  () => [
    accessStore.accessToken,
    accessStore.loginExpired,
    accessStore.accessCodes.join("\u0000"),
    userStore.userInfo?.userId,
    tenantStore.currentTenant?.id,
    locale.value,
  ],
  resetSession,
  { flush: "sync" },
);

onMounted(() => {
  mounted = true;
  gridApi.setGridOptions({ columns: columns() });
  void refresh();
  timer = setInterval(() => {
    if (document.visibilityState === "visible" && navigator.onLine)
      void loadItems();
  }, 60_000);
  document.addEventListener("visibilitychange", synchronize);
  window.addEventListener("online", synchronize);
});
onBeforeUnmount(() => {
  mounted = false;
  generation += 1;
  clearInterval(timer);
  document.removeEventListener("visibilitychange", synchronize);
  window.removeEventListener("online", synchronize);
});
</script>

<template>
  <Page :auto-content-height="true" class="h-full">
    <template v-if="allowed">
      <div
        data-testid="accessory-renewals-content"
        class="renewal-content bg-card p-4"
      >
        <Alert type="info" show-icon :message="t('guidance')" />
        <Alert
          v-if="syncFailed"
          data-testid="accessory-renewals-sync-error"
          type="warning"
          show-icon
          :message="t('syncFailed')"
        />
        <Alert
          v-if="listFailed"
          data-testid="accessory-renewals-list-error"
          type="error"
          show-icon
          :message="t('listFailed')"
        />
        <div class="flex items-center justify-between gap-4">
          <h2 class="text-base font-medium">
            {{ t("title") }}
            <span data-testid="accessory-renewals-count"
              >({{ total ?? "—" }})</span
            >
          </h2>
          <Button
            data-testid="accessory-renewals-refresh"
            :loading="syncing"
            @click="refresh"
            >{{ t("refresh") }}</Button
          >
        </div>
        <div class="renewal-grid">
          <Grid>
            <template #reason="{ row }">
              {{
                row.reasonCode === "balance_payment_failed"
                  ? t("awaitingPayment")
                  : row.reasonCode === "deletion_invoice_pending"
                    ? t("cancellationPending")
                    : t("needsReview")
              }}
            </template>
            <template #action="{ row }">
              <ghost-button
                v-if="canViewInvoice"
                :data-testid="`accessory-renewal-invoice-${row.workItemNo}`"
                @click="openInvoice(row)"
                >{{ t("viewInvoice") }}</ghost-button
              >
            </template>
          </Grid>
        </div>
      </div>
    </template>
  </Page>
</template>

<style scoped>
.renewal-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
}
.renewal-grid {
  flex: 1;
  min-width: 0;
  min-height: 180px;
  height: 100%;
}
.renewal-grid :deep(.vxe-table--header-wrapper) {
  background: hsl(var(--muted));
}
.renewal-grid :deep(.vxe-table--body-wrapper),
.renewal-grid :deep(.vxe-grid--pager-wrapper) {
  background: hsl(var(--card));
}
.renewal-grid :deep(.vxe-pager),
.renewal-grid :deep(.vxe-pager--wrapper) {
  height: auto;
  min-height: 40px;
}
.renewal-grid :deep(.vxe-pager--wrapper) {
  flex-wrap: wrap;
  gap: 4px;
  padding-block: 4px;
}
.renewal-grid :deep(.vxe-pager--wrapper > *) {
  flex-shrink: 0;
}
@media (max-width: 640px) {
  .renewal-grid :deep(.vxe-pager--wrapper::before) {
    content: "";
    order: 1;
    flex-basis: 100%;
  }
  .renewal-grid
    :deep(
      .vxe-pager--wrapper > :not(.vxe-pager--sizes):not(.vxe-pager--total)
    ) {
    order: 2;
  }
  .renewal-grid :deep(.vxe-pager--total) {
    margin-right: auto;
  }
  .renewal-grid :deep(.vxe-pager--sizes) {
    margin-right: 0;
  }
}
</style>

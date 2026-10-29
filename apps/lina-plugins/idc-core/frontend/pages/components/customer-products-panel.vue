<script setup lang="ts">
import type { CustomerProduct } from "../customer-client";
import { nextTick, onBeforeUnmount, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { requestClient } from "#/api/request";
import { useVbenDrawer } from "@vben/common-ui";
import { $te } from "@vben/locales";
import ServerEditDrawer from "./customer-server-edit-drawer.vue";
import ProductManagementDrawer from "./customer-product-management-drawer.vue";
import { preferences } from "@vben/preferences";
import { Alert, Button, Tag, message } from "ant-design-vue";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import { useAccess } from "../plugin-access";
import { minorToMajorAmount } from "../catalog-money";
import { customerProducts } from "../customer-client";
import { productManagementKind, productManagementPermissions } from "../customer-product-management";

const props = defineProps<{ customerNo: string }>();
const router = useRouter();
const { hasAccessByCodes } = useAccess();
const failed = ref(false);
const ownersFailed = ref(false);
const owners = ref<
  {
    id: string;
    installed: number;
    enabled: number;
  }[]
>([]);
const text = (key: string) => $t(`plugin.idc-core.customer.workspace.${key}`);
const [EditDrawer, editDrawer] = useVbenDrawer({
  connectedComponent: ServerEditDrawer,
});
const [ManagementDrawer, managementDrawer] = useVbenDrawer({ connectedComponent: ProductManagementDrawer });
let sequence = 0;
let managementSequence = 0;
let page = 1;
let pageSize = 10;
const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: [
      {
        field: "nameZhCn",
        title: text("productName"),
        width: 230,
        slots: { default: "name" },
      },
      {
        field: "information",
        title: text("productInfo"),
        minWidth: 200,
        slots: { default: "information" },
      },
      {
        field: "amountMinor",
        title: text("amountAndPeriod"),
        width: 155,
        slots: { default: "amount" },
      },
      {
        field: "createdAt",
        title: text("productDates"),
        width: 230,
        slots: { default: "dates" },
      },
      {
        field: "status",
        title: text("status"),
        width: 115,
        fixed: "right",
        slots: { default: "status" },
      },
      {
        field: "action",
        title: text("actions"),
        width: 145,
        fixed: "right",
        slots: { default: "action" },
      },
    ],
    align: "left",
    autoResize: true,
    data: [],
    height: 450,
    rowConfig: { keyField: "number", height: 90 },
    showOverflow: false,
    toolbarConfig: { enabled: false },
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
  },
  gridEvents: {
    pageChange: ({
      currentPage,
      pageSize: size,
    }: {
      currentPage: number;
      pageSize: number;
    }) => {
      page = currentPage;
      pageSize = size;
      void load();
    },
  },
});
function editor(row: CustomerProduct) {
  if (row.deliveryKind === "pending") return;
  if (row.deliveryKind === "server") {
    const owner = owners.value.find(
      (item) =>
        item.id === "idc-resource" &&
        item.installed === 1 &&
        item.enabled === 1,
    );
    return owner &&
      hasAccessByCodes(["idc:admin:resource:server-accessory:view"]) &&
      hasAccessByCodes(["idc:admin:resource:server:edit"])
      ? {
          path: "/admin/idc/resource/servers",
          query: { resourceTab: "servers", serviceNo: row.number },
        }
      : undefined;
  }
  if (row.pluginId === "yunsu-ssl" && canManage(row)) return { path: "", query: {} };
  const owner = owners.value.find(
    (item) =>
      item.id === row.pluginId && item.installed === 1 && item.enabled === 1,
  );
  if (!owner) return;
  const route = router
    .getRoutes()
    .find(
      (item) =>
        item.meta.pluginSurfaceId === row.pluginId &&
        Array.isArray(item.meta.pluginSurfaceCapabilities) &&
        item.meta.pluginSurfaceCapabilities.includes(
          "service-entitlements.admin.v1",
        ),
    );
  if (!route) return;
  return {
    path: route.path,
    query: { pluginPath: `entitlements/${encodeURIComponent(row.number)}` },
  };
}
function canManage(row: CustomerProduct) {
  const kind = productManagementKind(row);
  if (!kind) return false;
  const requiredPlugins = kind === "legacy-server" ? ["idc-resource"] : kind === "native-server" ? ["idc-resource", row.pluginId] : [row.pluginId];
  return requiredPlugins.every(id => owners.value.some(item => item.id === id && item.installed === 1 && item.enabled === 1)) &&
    productManagementPermissions(kind).some(code => hasAccessByCodes([code]));
}
async function refreshEntryOwners() {
  const current = ++managementSequence;
  const directory = sequence;
  try {
    const plugins = await requestClient.get<{ list: typeof owners.value }>("/plugins/dynamic");
    if (current !== managementSequence || directory !== sequence) return false;
    owners.value = plugins.list;
    ownersFailed.value = false;
  } catch {
    if (current === managementSequence && directory === sequence) { owners.value = []; ownersFailed.value = true; }
    return false;
  }
  return true;
}
async function manage(row: CustomerProduct) {
  const kind = productManagementKind(row);
  if (!kind || !canManage(row) || !await refreshEntryOwners()) return;
  if (!canManage(row)) return;
  managementDrawer.setData({ serviceNo: row.number, name: productName(row), kind }).open();
}
async function edit(row: CustomerProduct) {
  if (row.pluginId === "yunsu-ssl") { await manage(row); return; }
  if (row.deliveryKind === "server" && !await refreshEntryOwners()) return;
  const target = editor(row);
  if (!target) return;
  if (row.deliveryKind === "server") {
    editDrawer
      .setData({ serviceNo: row.number, name: productName(row) })
      .open();
    return;
  }
  void router.push(target);
}
function productName(row: CustomerProduct) {
  return preferences.app.locale === "zh-CN"
    ? row.nameZhCn || row.nameEnUs
    : row.nameEnUs || row.nameZhCn;
}
function status(row: CustomerProduct) {
  const key = `plugin.idc-core.customer.workspace.productStatus.${row.status}`;
  return $te(key) ? $t(key) : text("processing");
}
function type(row: CustomerProduct) {
  return row.pluginId === "yunsu-ssl"
    ? text("ssl")
    : ["cloud_server", "bare_metal", "idc-resource"].includes(row.productType) ||
        row.deliveryKind === "server"
      ? text("server")
      : text("otherProduct");
}
function period(row: CustomerProduct) {
  return row.periodCount
    ? `${row.periodCount} ${$te(`plugin.idc-core.customer.workspace.periods.${row.periodUnit}`) ? text(`periods.${row.periodUnit}`) : text("period")}`
    : text("oneTime");
}
async function load() {
  const current = ++sequence;
  ++managementSequence;
  failed.value = false;
  ownersFailed.value = false;
  gridApi.setLoading(true);
  try {
    const [result, plugins] = await Promise.all([
      customerProducts(props.customerNo, page, pageSize),
      requestClient.get<{ list: typeof owners.value }>("/plugins/dynamic").catch(() => undefined),
    ]);
    if (current !== sequence) return;
    owners.value = plugins?.list ?? [];
    ownersFailed.value = !plugins;
    gridApi.setGridOptions({
      data: result.items,
      pagerConfig: {
        enabled: true,
        currentPage: page,
        pageSize,
        total: result.total,
      },
    });
    await nextTick();
    if (current === sequence) await gridApi.grid.recalculate(true);
  } catch {
    if (current === sequence) { failed.value = true; owners.value = []; }
  } finally {
    if (current === sequence) gridApi.setLoading(false);
  }
}
async function copy(value: string) {
  try {
    await navigator.clipboard.writeText(value);
    message.success(text("copied"));
  } catch {
    message.error(text("copyFailed"));
  }
}
watch(
  () => props.customerNo,
  () => {
    page = 1;
    pageSize = 10;
    gridApi.setGridOptions({ data: [], pagerConfig: { currentPage: 1, pageSize: 10, total: 0 } });
    void managementDrawer.close?.();
    void editDrawer.close?.();
    void load();
  },
  { immediate: true },
);
onBeforeUnmount(() => { ++sequence; ++managementSequence; });
</script>

<template>
  <section class="products-panel" data-testid="customer-products-panel">
    <Alert v-if="ownersFailed" type="warning" :message="text('managementUnavailable')"><template #action><Button @click="load">{{ text('retry') }}</Button></template></Alert>
    <Alert v-if="failed" type="error" :message="text('loadFailed')"
      ><template #action
        ><Button @click="load">{{ text("retry") }}</Button></template
      ></Alert
    >
    <Grid>
      <template #name="{ row }">
        <div class="product-cell">
          <strong class="product-name" :title="productName(row)">{{ productName(row) }}</strong>
          <span class="secondary">{{ type(row) }}</span>
          <button class="copy-number" :title="row.number" @click="copy(row.number)">
            {{ row.number }}
          </button>
        </div>
      </template>
      <template #information="{ row }">{{
        (preferences.app.locale === "zh-CN"
          ? row.information
          : row.informationEnUs || row.information) || "—"
      }}</template>
      <template #amount="{ row }">
        <div class="product-cell">
          <strong
            >{{ row.currencyCode }}
            {{ minorToMajorAmount(row.amountMinor, row.currencyCode) }}</strong
          >
          <span class="secondary">{{ period(row) }}</span>
        </div>
      </template>
      <template #dates="{ row }">
        <div class="product-cell product-dates">
          <span
            ><span class="secondary">{{ text("orderedAt") }}</span>
            {{ formatTimestamp(Number(row.createdAt)) }}</span
          >
          <span
            ><span class="secondary">{{ text("expiry") }}</span>
            {{
              row.expiresAt ? formatTimestamp(Number(row.expiresAt)) : "—"
            }}</span
          >
        </div>
      </template>
      <template #status="{ row }"
        ><Tag
          :color="
            ['active', 'paid', 'fulfilled'].includes(row.status)
              ? 'success'
              : ['cancelled', 'terminated', 'refunded'].includes(row.status)
                ? 'default'
                : 'warning'
          "
          >{{ status(row) }}</Tag
        ></template
      >
      <template #action="{ row }">
        <ghost-button v-if="canManage(row)" :data-testid="`customer-product-manage-${row.number}`" @click="manage(row)">{{ text("manage") }}</ghost-button>
        <ghost-button
          v-if="row.pluginId !== 'yunsu-ssl' && editor(row)"
          :data-testid="`customer-product-edit-${row.number}`"
          @click="edit(row)"
          >{{ text("edit") }}</ghost-button
        ><span v-if="!editor(row) && !canManage(row)">—</span></template
      >
    </Grid>
    <EditDrawer @changed="load" />
    <ManagementDrawer @changed="load" />
  </section>
</template>

<style scoped>
.products-panel {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.product-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  padding: 8px 0;
  line-height: 1.5;
}
.product-cell > * {
  max-width: 100%;
}
.product-name, .copy-number {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.product-cell strong {
  font-weight: 500;
  white-space: nowrap;
}
.secondary {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}
.product-dates {
  font-size: 12px;
  white-space: nowrap;
}
.copy-number {
  color: hsl(var(--primary));
  text-decoration: underline;
  cursor: pointer;
}
</style>

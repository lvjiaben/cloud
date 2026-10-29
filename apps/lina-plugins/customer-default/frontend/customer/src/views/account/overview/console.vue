<script setup lang="ts">
import type { CustomerProfile } from "#/api/idc/customer-account";
import type { FinancialOverview } from "#/api/idc/customer-finance";
import type { OrderSummary } from "#/api/idc/customer-commerce";
import type { DirectoryService } from "#/api/idc/product-directories";
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { Page } from "@vben/common-ui";
import { IconifyIcon } from "@vben/icons";
import { preferences } from "@vben/preferences";
import { useUserStore } from "@vben/stores";
import {
  Alert,
  Button,
  Card,
  Empty,
  Skeleton,
  Space,
  Tag,
} from "ant-design-vue";
import { getCustomerProfile } from "#/api/idc/customer-account";
import { getFinancialOverview } from "#/api/idc/customer-finance";
import { listOrders } from "#/api/idc/customer-commerce";
import { listDirectoryServices } from "#/api/idc/product-directories";
import { $t } from "#/locales";
import { managementPreparationLabelKey } from "#/views/account/commerce/customer-fulfillment-state";
import { serviceManagementPath } from "../product-directories/service-management";
import NativeServerList from "../native-servers/list.vue";
import { useNativeServerAccess } from "../native-servers/use-native-server-access";
import { consoleMoney } from "./console-money";

const router = useRouter();
const user = useUserStore();
const { available: nativeAvailable, ready: nativeReady } =
  useNativeServerAccess();
const profile = ref<CustomerProfile>();
const finance = ref<FinancialOverview>();
const orders = ref<OrderSummary[]>([]);
const financeLoading = ref(false),
  ordersLoading = ref(false);
const financeFailed = ref(false),
  ordersFailed = ref(false);
const services = ref<DirectoryService[]>([]);
const serviceTotal = ref<number>();
const servicesLoading = ref(false),
  servicesFailed = ref(false);
let generation = 0,
  serviceRead = 0,
  disposed = false;
const identity = computed(() =>
  String(user.userInfo?.userId || user.userInfo?.username || ""),
);
const locale = computed(() =>
  preferences.app.locale === "zh-CN" ? "zh-CN" : "en-US",
);
const tr = (key: string) => $t(`accountConsole.${key}`);
const welcome = computed(() =>
  $t("accountConsole.welcome", {
    name:
      profile.value?.displayName ||
      user.userInfo?.realName ||
      user.userInfo?.username ||
      "",
  }),
);
const status = (value: string) => {
  const key = `commerce.status.${value}`;
  const label = $t(key);
  return label === key ? $t("commerce.status.unknown") : label;
};
function money(value: string, currency: string) {
  return consoleMoney(value, currency, locale.value);
}
function date(value: number) {
  return new Date(value).toLocaleString(locale.value);
}
function serviceName(service: DirectoryService) {
  return locale.value === "zh-CN"
    ? service.productNameZhCn || service.productNameEnUs
    : service.productNameEnUs || service.productNameZhCn;
}
function serviceStatus(service: DirectoryService) {
  if (
    service.requiresManualProcessing &&
    service.statusCode === "pending_activation"
  )
    return $t("commerce.fulfillment.status.manual_processing");
  const allowed = [
    "pending_activation",
    "active",
    "suspended",
    "expired",
    "terminated",
    "cancelled",
  ];
  return allowed.includes(service.statusCode)
    ? $t(`productDirectories.states.${service.statusCode}`)
    : $t("commerce.status.unavailable");
}
async function loadServices(current = generation) {
  const request = ++serviceRead;
  servicesLoading.value = true;
  servicesFailed.value = false;
  try {
    const result = await listDirectoryServices("all", 1, 5);
    if (!disposed && generation === current && request === serviceRead) {
      services.value = result.items;
      serviceTotal.value = result.total;
    }
  } catch {
    if (!disposed && generation === current && request === serviceRead) {
      servicesFailed.value = true;
      serviceTotal.value = undefined;
    }
  } finally {
    if (!disposed && generation === current && request === serviceRead)
      servicesLoading.value = false;
  }
}

async function loadFinance(current = generation) {
  financeLoading.value = true;
  financeFailed.value = false;
  try {
    const result = await getFinancialOverview();
    if (!disposed && generation === current) finance.value = result.overview;
  } catch {
    if (!disposed && generation === current) financeFailed.value = true;
  } finally {
    if (!disposed && generation === current) financeLoading.value = false;
  }
}
async function loadOrders(current = generation) {
  ordersLoading.value = true;
  ordersFailed.value = false;
  try {
    const result = await listOrders({ limit: 5 });
    if (!disposed && generation === current) orders.value = result.orders;
  } catch {
    if (!disposed && generation === current) ordersFailed.value = true;
  } finally {
    if (!disposed && generation === current) ordersLoading.value = false;
  }
}
async function loadProfile(current: number) {
  try {
    const result = await getCustomerProfile();
    if (!disposed && generation === current) profile.value = result.item;
  } catch {
    /* Keep the already authenticated account name available. */
  }
}
watch(
  identity,
  () => {
    generation++;
    profile.value = undefined;
    finance.value = undefined;
    orders.value = [];
    services.value = [];
    serviceTotal.value = undefined;
    void loadProfile(generation);
    void loadFinance(generation);
    void loadOrders(generation);
    void loadServices(generation);
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  disposed = true;
  generation++;
});
</script>

<template>
  <Page
    class="customer-inner-page"
    header-class="customer-inner-heading"
    content-class="customer-inner-content"
    :title="tr('overview')"
    :description="welcome"
    data-testid="customer-console"
  >
    <div class="console-layout">
      <header class="console-actions">
        <p>{{ tr("intro") }}</p>
        <Space wrap
          ><Button @click="router.push('/account/orders')">{{
            tr("orders")
          }}</Button
          ><Button @click="router.push('/account/support')">{{
            tr("support")
          }}</Button
          ><Button
            type="primary"
            @click="router.push('/account/products/catalog')"
            ><IconifyIcon icon="lucide:plus" class="mr-1" />{{
              tr("purchase")
            }}</Button
          ></Space
        >
      </header>
      <section class="console-summary">
        <Card class="console-balance" :title="tr('balance')">
          <Skeleton
            v-if="financeLoading && !finance"
            active
            :paragraph="{ rows: 1 }"
            :title="false"
          />
          <Alert
            v-else-if="financeFailed"
            type="warning"
            :message="tr('balanceUnavailable')"
            ><template #action
              ><Button size="small" @click="loadFinance()">{{
                tr("retry")
              }}</Button></template
            ></Alert
          >
          <div v-else-if="finance?.balances.length" class="console-balances">
            <div
              v-for="balance in finance.balances"
              :key="`${balance.currencyCode}:${balance.accountTypeCode}`"
            >
              <strong>{{
                money(balance.availableBalanceMinor, balance.currencyCode)
              }}</strong
              ><span>{{ balance.currencyCode }}</span>
            </div>
          </div>
          <p v-else class="console-muted">{{ tr("noBalanceAccount") }}</p>
          <Button type="link" @click="router.push('/account/billing')"
            >{{ tr("manageBalance") }} →</Button
          >
        </Card>
        <button
          type="button"
          class="console-metric"
          data-testid="console-service-total"
          @click="router.push('/account/product-directories')"
        >
          <IconifyIcon icon="lucide:boxes" /><span>{{ tr("myServices") }}</span
          ><strong>{{ serviceTotal ?? "—" }}</strong
          ><small>{{ tr("allServices") }} →</small>
        </button>
        <button
          type="button"
          class="console-metric"
          @click="router.push('/account/billing/invoices')"
        >
          <IconifyIcon icon="lucide:receipt-text" /><span>{{
            tr("unpaidInvoices")
          }}</span
          ><strong>{{ finance?.openInvoiceCount ?? "—" }}</strong
          ><small>{{ tr("viewAndPay") }} →</small>
        </button>
      </section>
      <Card :title="tr('myServices')" data-testid="console-services">
        <template #extra
          ><Button
            type="link"
            @click="router.push('/account/product-directories')"
            >{{ tr("allServices") }} →</Button
          ></template
        >
        <Skeleton
          v-if="servicesLoading && !services.length"
          active
          :paragraph="{ rows: 2 }"
        />
        <Alert
          v-else-if="servicesFailed"
          type="warning"
          show-icon
          :message="tr('servicesUnavailable')"
        >
          <template #action
            ><Button
              size="small"
              :loading="servicesLoading"
              data-testid="console-services-retry"
              @click="loadServices()"
              >{{ tr("retry") }}</Button
            ></template
          >
        </Alert>
        <Empty v-else-if="!services.length" :description="tr('noServices')" />
        <ul v-else class="console-services">
          <li
            v-for="service in services"
            :key="service.entitlementNo"
            :data-testid="`console-service-${service.entitlementNo}`"
          >
            <div>
              <strong>{{ serviceName(service) }}</strong
              ><small>{{
                service.expiresAt
                  ? $t("productDirectories.expiresAt") +
                    " · " +
                    date(service.expiresAt)
                  : $t("productDirectories.noExpiry")
              }}</small>
              <small
                v-if="
                  managementPreparationLabelKey(service.managementPreparation)
                "
                data-testid="console-management-preparation"
                >{{
                  $t(
                    managementPreparationLabelKey(
                      service.managementPreparation,
                    ),
                  )
                }}</small
              >
            </div>
            <Tag
              :color="service.requiresManualProcessing ? 'warning' : undefined"
              >{{ serviceStatus(service) }}</Tag
            >
            <Button
              v-if="serviceManagementPath(service)"
              type="primary"
              :data-testid="`manage-service-${service.entitlementNo}`"
              @click="router.push(serviceManagementPath(service))"
              >{{ $t("nativeServers.manage") }}</Button
            >
            <Button
              v-if="service.orderNo"
              @click="
                router.push(
                  `/account/orders/${encodeURIComponent(service.orderNo)}`,
                )
              "
              >{{ tr("viewOrder") }}</Button
            >
          </li>
        </ul>
      </Card>
      <Skeleton v-if="!nativeReady" active :paragraph="{ rows: 3 }" />
      <NativeServerList compact v-if="nativeAvailable" :key="identity" />
      <Card :title="tr('recentOrders')" data-testid="console-recent-orders">
        <template #extra
          ><Button type="link" @click="router.push('/account/orders')"
            >{{ tr("allOrders") }} →</Button
          ></template
        >
        <Skeleton
          v-if="ordersLoading && !orders.length"
          active
          :paragraph="{ rows: 3 }"
        />
        <Alert
          v-else-if="ordersFailed"
          type="warning"
          show-icon
          :message="tr('ordersUnavailable')"
          ><template #action
            ><Button size="small" @click="loadOrders()">{{
              tr("retry")
            }}</Button></template
          ></Alert
        >
        <Empty v-else-if="!orders.length" :description="tr('noOrders')" />
        <ol v-else class="console-orders">
          <li v-for="order in orders" :key="order.orderNo">
            <div>
              <button
                type="button"
                class="console-order-link"
                @click="
                  router.push(
                    `/account/orders/${encodeURIComponent(order.orderNo)}`,
                  )
                "
              >
                {{ order.orderNo }}</button
              ><small>{{ date(order.createdAt) }}</small>
            </div>
            <Tag>{{ status(order.statusCode) }}</Tag
            ><strong>{{
              money(order.totalAmountMinor, order.currencyCode)
            }}</strong
            ><Button
              @click="
                router.push(
                  `/account/orders/${encodeURIComponent(order.orderNo)}`,
                )
              "
              >{{
                tr(order.canPreparePayment ? "viewAndPay" : "viewOrder")
              }}</Button
            >
          </li>
        </ol>
      </Card>
      <footer class="console-footer">
        <Button
          type="link"
          @click="router.push('/account/product-directories/all')"
          >{{ tr("allServices") }}</Button
        ><Button type="link" @click="router.push('/account/profile')">{{
          tr("accountSettings")
        }}</Button>
      </footer>
    </div>
  </Page>
</template>

<style scoped>
.console-layout {
  display: grid;
  gap: 22px;
}
.console-services {
  list-style: none;
  padding: 0;
  margin: 0;
}
.console-services li {
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 16px 0;
  border-bottom: 1px solid hsl(var(--border));
}
.console-services li:last-child {
  border-bottom: 0;
}
.console-services li > div {
  display: grid;
  flex: 1;
  min-width: 0;
  gap: 6px;
}
.console-services strong {
  overflow-wrap: anywhere;
  font-weight: 500;
}
.console-services small {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}
.console-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 18px;
}
.console-actions p {
  margin: 0;
  font-size: 14px;
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
}
.console-summary {
  display: grid;
  grid-template-columns: minmax(280px, 1.4fr) repeat(2, minmax(200px, 1fr));
  gap: 18px;
}
.console-balances {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
  margin-bottom: 12px;
}
.console-balances > div {
  min-width: 0;
  max-width: 100%;
  display: grid;
  gap: 4px;
}
.console-balances strong {
  font-size: 30px;
  font-weight: 600;
  letter-spacing: -0.5px;
  overflow-wrap: anywhere;
  min-width: 0;
  line-height: 1.4;
}
.console-balances span,
.console-muted {
  font-size: 12px;
  color: hsl(var(--muted-foreground));
}
.console-balance :deep(.ant-card-body) {
  padding-top: 8px;
}
.console-balance :deep(.ant-btn-link) {
  padding-left: 0;
}
.console-metric {
  display: grid;
  grid-template-columns: 28px 1fr;
  gap: 10px 12px;
  align-content: center;
  text-align: left;
  padding: 24px;
  border: 1px solid hsl(var(--border));
  background: hsl(var(--card));
  border-radius: 8px;
  cursor: pointer;
}
.console-metric > svg {
  color: hsl(var(--primary));
  font-size: 20px;
}
.console-metric span {
  font-size: 14px;
  color: hsl(var(--muted-foreground));
}
.console-metric strong {
  grid-column: 2;
  font-size: 32px;
  font-weight: 600;
  line-height: 1.2;
}
.console-metric small {
  grid-column: 2;
  font-size: 12px;
  color: hsl(var(--primary));
}
.console-metric:hover {
  border-color: hsl(var(--primary) / 45%);
}
.console-orders {
  list-style: none;
  margin: 0;
  padding: 0;
}
.console-orders li {
  display: grid;
  grid-template-columns: minmax(240px, 1fr) 130px 150px auto;
  align-items: center;
  gap: 18px;
  padding: 18px 0;
  border-bottom: 1px solid hsl(var(--border));
}
.console-orders li:last-child {
  border-bottom: 0;
}
.console-orders li > div {
  display: grid;
  gap: 6px;
  min-width: 0;
}
.console-order-link {
  text-align: left;
  color: hsl(var(--foreground));
  font-size: 13px;
  overflow-wrap: anywhere;
  cursor: pointer;
}
.console-order-link:hover {
  color: hsl(var(--primary));
}
.console-orders small {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}
.console-orders strong {
  font-size: 15px;
  font-weight: 500;
}
.console-orders :deep(.ant-tag) {
  justify-self: start;
}
.console-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
@media (max-width: 1080px) {
  .console-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .console-balance {
    grid-column: 1/-1;
  }
  .console-orders li {
    grid-template-columns: minmax(200px, 1fr) auto;
    gap: 10px;
  }
  .console-orders strong {
    grid-column: 1;
  }
}
@media (max-width: 700px) {
  .console-services li {
    flex-wrap: wrap;
    gap: 10px;
  }
  .console-services li > div {
    flex-basis: 100%;
  }
  .console-actions {
    flex-direction: column;
    align-items: flex-start;
  }
  .console-summary {
    grid-template-columns: 1fr;
  }
  .console-balance {
    grid-column: auto;
  }
  .console-metric {
    grid-template-columns: 28px minmax(0, 1fr) auto;
  }
  .console-metric strong {
    grid-column: 3;
    grid-row: 1/3;
    align-self: center;
  }
  .console-metric small {
    grid-column: 2;
  }
  .console-orders li {
    grid-template-columns: 1fr auto;
  }
  .console-orders li > div {
    grid-column: 1/-1;
  }
  .console-orders strong {
    grid-column: 1;
  }
  .console-footer {
    flex-wrap: wrap;
  }
}
</style>

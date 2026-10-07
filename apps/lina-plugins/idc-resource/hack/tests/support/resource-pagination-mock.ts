import type { Page, Route } from "@host-tests/support/playwright";
import { baseItem, installProviderHealthMock } from "./provider-health-mock";
import { baseTask, installProviderTaskMock } from "./provider-task-mock";
import { firstOperation, installProviderCapacityMock } from "./provider-capacity-mock";
import { installResourceOverviewMock } from "./resource-overview-mock";
import { installServerBatchRenewalMock } from "./server-batch-renewal-mock";

export type ResourceDirectoryKind = "capacity" | "health" | "tasks" | "renewals" | "overview";
export const resourceDirectories = {
  capacity: { route: "provider-capacity", root: "provider-capacity-operations-page", endpoint: "provider-capacity-observations", label: "分页容量" },
  health: { route: "health", root: "provider-health-page", endpoint: "channel-health", label: "分页渠道" },
  tasks: { route: "tasks", root: "provider-task-page", endpoint: "tasks", label: "分页任务" },
  renewals: { route: "server-renewals", root: "server-batch-renewal-page", endpoint: "server-renewals", label: "分页服务器" },
  overview: { route: "overview", root: "resource-overview-page", endpoint: "overview", label: "分页资源池" },
} as const;

/** One test-local complete collection; only requested rows are sent to the browser. */
export async function installResourcePaginationMock(page: Page, kind: ResourceDirectoryKind) {
  const install = { capacity: installProviderCapacityMock, health: installProviderHealthMock, tasks: installProviderTaskMock, renewals: installServerBatchRenewalMock, overview: installResourceOverviewMock }[kind];
  await install(page);
  const requests: URL[] = [];
  const state = { requests, failure: 0, empty: false };
  const directory = resourceDirectories[kind];
  const items: Record<string, unknown>[] = Array.from({ length: 121 }, (_, index) => {
    const number = index + 1;
    const name = `${directory.label}${String(number).padStart(3, "0")}`;
    if (kind === "health") return { ...baseItem, channelNo: `channel_page_${number}`, channelName: name };
    if (kind === "tasks") return { ...baseTask, taskNo: `task_page_${number}`, channelName: name };
    if (kind === "capacity") return { ...firstOperation, observationNo: `observation_page_${number}`, channelName: name };
    if (kind === "renewals") return { serverNo: `SVR-${number.toString(16).padStart(32, "0")}`, productNameZhCN: name, productNameEnUS: `Paged server ${number}` };
    return { channelName: "分页接口", channelNo: "channel_page", displayName: name, poolNo: `pool_page_${number}`, resourceKind: "virtual_server", routingPriority: number, routingWeight: 100, status: "enabled", updatedAt: 1786248000000 };
  });
  await page.route(new RegExp(`/x/idc-resource/api/v1/admin/resource/${directory.endpoint}(?:\\?.*)?$`), async (route: Route) => {
    const url = new URL(route.request().url());
    requests.push(url);
    if (state.failure) {
      await route.fulfill({ status: state.failure, json: { code: state.failure, message: "Denied or unavailable" } });
      return;
    }
    function slice(pageKey: string, sizeKey: string) {
      const current = Number(url.searchParams.get(pageKey) || 1);
      const size = Number(url.searchParams.get(sizeKey) || 10);
      return state.empty ? [] : items.slice((current - 1) * size, current * size);
    }
    const data = kind === "overview" ? { summary: {
      activeMappings: 121, enabledChannels: 1, enabledPools: 121, pendingTasks: 0, unknownTasks: 0, latestProviderCapacityAt: null,
      poolTotal: 121, financeTotal: 121, pools: slice("poolPage", "poolPageSize"),
      finance: slice("financePage", "financePageSize").map(item => ({ amount: "12.00", currencyCode: "CNY", channelName: item.displayName, channelNo: "channel_page", snapshotNo: item.poolNo, kind: "balance", externalResourceRef: "", expiresAt: null, observedAt: 1786248000000 })),
    } } : { items: slice("page", "limit"), total: 121, nextCursor: "", channels: [{ channelNo: "channel_mofang_prod", displayName: "魔方云生产渠道" }] };
    await route.fulfill({ json: { code: 0, message: "success", data } });
  });
  return state;
}

import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ResourcePaginationPage } from "../pages/ResourcePaginationPage";
import { installResourcePaginationMock } from "../support/resource-pagination-mock";
import { baseTask } from "../support/provider-task-mock";

test.describe("TC012 资源目录真实数字分页", () => {
  for (const [index, kind] of (["capacity", "health", "tasks", "renewals"] as const).entries()) {
    test(`TC012${String.fromCharCode(97 + index)}: ${kind} 页码/规模/失败/拒绝清理`, async ({ adminPage }) => {
      const state = await installResourcePaginationMock(adminPage, kind);
      const ui = new ResourcePaginationPage(adminPage, kind);
      await ui.goto();
      await expect(ui.rows()).toHaveCount(10);
      await expect(ui.pager()).toContainText("121");
      await ui.next();
      await expect(ui.active()).toHaveText("2");
      expect(state.requests.at(-1)?.searchParams.get("page")).toBe("2");
      for (const size of [20, 50, 100]) {
        await ui.size(size);
        await expect(ui.rows()).toHaveCount(size);
        await expect(ui.active()).toHaveText("1");
        expect(state.requests.at(-1)?.searchParams.get("limit")).toBe(String(size));
      }
      await ui.next();
      await expect(ui.rows()).toHaveCount(21);
      state.failure = 503;
      await ui.refresh();
      await expect(ui.rows()).toHaveCount(21);
      await expect(ui.active()).toHaveText("2");
      state.failure = 403;
      await ui.refresh();
      await expect(ui.rows()).toHaveCount(0);
      await expect(ui.pager()).toContainText("0");
      await expect(ui.root).not.toContainText("plugin.idc-resource");
      await ui.screenshot(`resource-pagination-${kind}-denied`);
      state.failure = 0;
      await ui.refresh();
      await expect(ui.rows()).toHaveCount(100);
      await expect(ui.active()).toHaveText("1");
      await ui.screenshot(`resource-pagination-${kind}-recovered`);
    });
  }
  test("TC012e: 总览两表独立翻页且空尾页保留权威总数", async ({ adminPage }) => {
    const state = await installResourcePaginationMock(adminPage, "overview");
    const ui = new ResourcePaginationPage(adminPage, "overview");
    await ui.goto();
    await expect(ui.rows(0)).toHaveCount(10);
    await expect(ui.rows(1)).toHaveCount(10);
    await ui.next(0);
    await expect(ui.active(0)).toHaveText("2");
    await expect(ui.active(1)).toHaveText("1");
    await ui.size(50, 1);
    await expect(ui.rows(1)).toHaveCount(50);
    expect(state.requests.at(-1)?.searchParams.get("poolPage")).toBe("2");
    expect(state.requests.at(-1)?.searchParams.get("financePageSize")).toBe("50");
    state.empty = true;
    await ui.next(1);
    await expect(ui.rows(1)).toHaveCount(0);
    await expect(ui.pager(1)).toContainText("121");
    await expect(ui.active(1)).toHaveText("2");
    await ui.screenshot("resource-overview-empty-page");
    state.failure = 403;
    await ui.refresh();
    await expect(ui.pager(0)).toContainText("0");
    await expect(ui.pager(1)).toContainText("0");
  });
  test("TC012f: 已关闭任务的迟到详情不能覆盖重新打开的任务", async ({ adminPage }) => {
    await installResourcePaginationMock(adminPage, "tasks");
    let releaseFirst!: () => void;
    const held = new Promise<void>(resolve => { releaseFirst = resolve; });
    let firstRequested = false;
    await adminPage.route(/\/x\/idc-resource\/api\/v1\/admin\/resource\/tasks\/task_page_[12]$/, async route => {
      const first = route.request().url().endsWith("task_page_1");
      if (first) { firstRequested = true; await held; }
      await route.fulfill({ json: { code: 0, data: { item: { task: { ...baseTask, taskNo: first ? "task_page_1" : "task_page_2", channelName: first ? "迟到的旧任务" : "当前第二条任务" }, evidence: [], residualResources: [], upstream: null } } } });
    });
    const ui = new ResourcePaginationPage(adminPage, "tasks");
    try {
      await ui.goto();
      await ui.root.getByTestId("provider-task-detail-task_page_1").click();
      await expect.poll(() => firstRequested).toBe(true);
      await adminPage.keyboard.press("Escape");
      await expect(adminPage.getByRole("dialog")).toBeHidden();
      await ui.root.getByTestId("provider-task-detail-task_page_2").click();
      await expect(adminPage.getByTestId("provider-task-detail")).toContainText("当前第二条任务");
      const response = adminPage.waitForResponse(url => url.url().endsWith("/tasks/task_page_1"));
      releaseFirst();
      await response;
      await expect(adminPage.getByTestId("provider-task-detail")).toContainText("当前第二条任务");
      await expect(adminPage.getByTestId("provider-task-detail")).not.toContainText("迟到的旧任务");
      await ui.screenshot("resource-task-detail-late-response");
    } finally { releaseFirst(); }
  });
});

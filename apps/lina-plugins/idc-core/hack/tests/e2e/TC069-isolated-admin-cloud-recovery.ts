// 管理员在真实隔离后台查看上游错误并修正原订单，第三方写入只到本地模拟器。
import { randomUUID } from "node:crypto";
import { test } from "@host-tests/fixtures/auth";
import { expect, request } from "@host-tests/support/playwright";
import {
  api,
  descriptor,
  fixture,
  get,
  saved,
  serversPath,
} from "../support/isolated-commerce";
import { createCustomer, paidOrder } from "../support/isolated-paid-order";
import { IsolatedFulfillmentPage } from "../pages/IsolatedFulfillmentPage";

test.describe("TC-69 实际后台处理开通失败", () => {
  test.skip(
    !descriptor,
    "Requires explicitly fenced local fake-commerce database",
  );
  test.setTimeout(240_000);
  test("TC-69a: 看见脱敏原始错误，选择正确节点镜像后原单只重开一次", async ({
    browser,
  }, testInfo) => {
    const f = (await fixture()) as Awaited<ReturnType<typeof fixture>> & {
      adminOrigin: string;
    };
    const origin = new URL(f.adminOrigin);
    expect(origin.protocol).toBe("http:");
    expect(origin.hostname).toBe("127.0.0.1");
    expect(["6030", "6031", "6032", "6680"]).not.toContain(origin.port);
    const admin = await api(f, f.adminAuth),
      key = "staff-recovery-" + randomUUID();
    const supplier = await request.newContext({
      baseURL: f.mockOrigin,
      ignoreHTTPSErrors: true,
    });
    let client: Awaited<ReturnType<typeof createCustomer>> | undefined;
    let restoreConnection: any;
    const channelsPath = '/x/idc-resource/api/v1/admin/resource/provider-configurations';
    const context = await browser.newContext();
    if (process.env.IDC_ISOLATED_ADMIN_ENTRY) {
      await context.route(f.adminOrigin + "/", route => route.fulfill({path: process.env.IDC_ISOLATED_ADMIN_ENTRY!, contentType: "text/html"}));
    }
    context.setDefaultTimeout(20_000);
    const allowed = new Set([f.adminOrigin, f.baseURL, f.mockOrigin]);
    await context.route("**/*", (r) =>
      allowed.has(new URL(r.request().url()).origin) ? r.fallback() : r.abort(),
    );
    try {
      client = await createCustomer(f, admin, key);
      expect(
        (
          await supplier.post("/fixture/mode", { data: { mode: "rejected" } })
        ).status(),
      ).toBe(200);
      const before = await saved(f, f.mockState);
      const { orderNo } = await paidOrder(f, client, key);
      const base = "/x/idc-core/api/v1/admin/product-fulfillment/tasks";
      let intentNo = "";
      await expect
        .poll(
          async () => {
            const rows = (await get(admin, base + "?orderNo=" + orderNo)).tasks;
            if (rows.length !== 1) return "";
            intentNo = rows[0].intentNo;
            return rows[0].statusCode;
          },
          { timeout: 90_000, intervals: [1000, 2000] },
        )
        .toBe("not_completed");
      const failed = (await get(admin, base + "/" + intentNo)).task;
      expect(failed.canRecoverPreparation).toBe(true);
      // A changed connection must produce a genuinely cold recovery projection
      // without changing the original purchase or relying on another test.
      const current = (await get(admin, channelsPath)).channels.find((row:any)=>row.channelNo===f.resourceChannelNo);
      expect(current.endpoint).toBe(f.mockOrigin + '/administrator');
      restoreConnection = current;
      const endpoint = current.endpoint + '/';
      const settings = {...current.nonSecretSettings};
      if (typeof settings.endpoint === 'string') settings.endpoint = endpoint;
      const updated = await admin.put(channelsPath+'/channels/'+current.channelNo,{data:{displayName:current.displayName,endpoint,nonSecretSettings:settings,status:'enabled',version:current.version}});
      expect((await updated.json()).code).toBe(0);
      const page = await context.newPage(),
        ui = new IsolatedFulfillmentPage(page, f.adminOrigin, orderNo);
      await ui.login();
      await ui.open();
      await expect(ui.detail).toContainText("隔离模拟：镜像未下载");
      await expect(ui.detail.locator("pre")).toContainText("httpStatus");
      await expect(ui.detail).not.toContainText("MUST_REDACT");
      await expect(ui.detail).not.toContainText("nightly-session-token");
      await expect(ui.detail).not.toContainText("nightly-api-password");
      await ui.screenshot("actual-admin-cloud-rejection-detail");
      await ui.recover();
      await expect(
        page.getByTestId("recovery-paid-specifications"),
      ).toContainText("2000");
      await expect(ui.action.locator('.ant-select-selection-item[title="隔离电信节点"]')).toBeVisible({timeout:30000});
      await ui.choose("上游节点", "隔离备用节点");
      await ui.choose("系统镜像", "Debian13-ready");
      await ui.passwordRules(12);
      await ui.screenshot("actual-admin-cloud-corrected-configuration");
      expect(
        (
          await supplier.post("/fixture/mode", { data: { mode: "success" } })
        ).status(),
      ).toBe(200);
      const submission = page.waitForResponse(
        (r) =>
          r.url() === f.adminOrigin + base + "/" + intentNo + "/recoveries" &&
          r.request().method() === "POST",
      );
      await ui.submit(
        "独立测试：修正节点和已下载镜像，保持原成交规格与金额",
        failed.recoveryReasons[0].labelZhCN,
      );
      const submitted = await submission;
      expect(submitted.request().postDataJSON().recoveryConfiguration).toMatchObject({ generationLength: 12, generationCharacters: "uppercase,digits" });
      expect((await submitted.json()).code).toBe(0);
      await expect(ui.action).toHaveCount(0);
      await expect
        .poll(
          async () => (await get(admin, base + "/" + intentNo)).task.statusCode,
          { timeout: 120_000, intervals: [1000, 2000] },
        )
        .toBe("completed");
      const completedTask = (await get(admin, base + "/" + intentNo)).task;
      const recoveryEvent = completedTask.events.filter((e:any) => e.eventCode === "preparation_recovered").at(-1);
      const completedEvent = completedTask.events.filter((e:any) => e.eventCode === "automation_completed").at(-1);
      const owner = (await get(admin, "/x/idc-resource/api/v1/admin/resource/tasks/" + completedTask.execution.resourceTaskNo)).item;
      // Instant fake upstream: exercise real scheduling and durable queues, not
      // mocked API results. Successful local owner progress must not wait 30s.
      expect(completedEvent.occurredAt - recoveryEvent.occurredAt).toBeLessThan(25_000);
      expect(completedEvent.occurredAt - owner.task.updatedAt).toBeGreaterThanOrEqual(0);
      expect(completedEvent.occurredAt - owner.task.updatedAt).toBeLessThan(7_000);
      await testInfo.attach("local-provisioning-timings", { contentType: "application/json", body: JSON.stringify({
        recoveryToCompletedMs: completedEvent.occurredAt - recoveryEvent.occurredAt,
        ownerSuccessToCompletedMs: completedEvent.occurredAt - owner.task.updatedAt,
        attemptCount: completedTask.execution.attemptCount,
      }) });
      const recoveryIndex = completedTask.events.indexOf(recoveryEvent);
      expect(completedTask.events.slice(recoveryIndex).filter((e:any) => e.eventCode === "automation_processing")).toHaveLength(1);
      const after = await saved(f, f.mockState);
      const forms = after.forms.slice(before.forms.length);
      expect(forms).toHaveLength(2);
      expect(forms[0]).toMatchObject({
        node: ["5"],
        os: ["97"],
        cpu: ["2"],
        memory: ["2000"],
      });
      expect(forms[1]).toMatchObject({
        node: ["6"],
        os: ["98"],
        cpu: ["2"],
        memory: ["2000"],
      });
      expect(after.violations.slice(before.violations.length)).toEqual([]);
      const { order } = await get(
        client,
        "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
      );
      expect(order.invoice).toMatchObject({
        statusCode: "paid",
        totalAmountMinor: "1100",
        paidAmountMinor: "1100",
        refundedAmountMinor: "0",
      });
      await expect
        .poll(
          async () =>
            (await get(client!, serversPath + "?page=1&pageSize=20")).total,
          { timeout: 30_000 },
        )
        .toBe(1);
      await ui.open();
      await expect(ui.detail).toContainText("已完成");
      await ui.screenshot("actual-admin-cloud-recovery-completed");
      const timeline = await ui.showTimeline();
      await expect(timeline.getByText("自动履约处理中", { exact: true })).toHaveCount(completedTask.events.filter((e:any) => e.eventCode === "automation_processing").length);
      await expect(timeline.getByText("自动履约已完成", { exact: true })).toBeVisible();
      await ui.screenshot("actual-admin-cloud-recovery-timeline");
    } finally {
      if (restoreConnection) {
        const latest = (await get(admin, channelsPath)).channels.find((row:any)=>row.channelNo===restoreConnection.channelNo);
        const restored = await admin.put(channelsPath+'/channels/'+latest.channelNo,{data:{displayName:restoreConnection.displayName,endpoint:restoreConnection.endpoint,nonSecretSettings:restoreConnection.nonSecretSettings,status:restoreConnection.status,version:latest.version}});
        expect((await restored.json()).code).toBe(0);
      }
      await supplier.post("/fixture/mode", { data: { mode: "success" } });
      await supplier.dispose();
      await client?.dispose();
      await admin.dispose();
      await context.close();
    }
  });
});

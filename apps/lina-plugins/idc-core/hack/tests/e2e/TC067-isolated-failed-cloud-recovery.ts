import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
// Exercise failed/unknown paid orders only in the explicit local fake-supplier
// environment. Every case creates its own customer, order and payment identity.
import { createHash, randomUUID } from "node:crypto";
import type { APIRequestContext } from "@host-tests/support/playwright";
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
import type { Fixture } from "../support/isolated-commerce";

import { createCustomer, paidOrder } from "../support/isolated-paid-order";

async function send(
  client: APIRequestContext,
  path: string,
  data: object,
  method = "POST",
) {
  const response = await client.fetch(path, { method, data, maxRedirects: 0 });
  expect(response.status()).toBe(200);
  return response.json();
}
async function write(
  client: APIRequestContext,
  path: string,
  data: object,
  method = "POST",
) {
  const result = await send(client, path, data, method);
  expect(result.code, result.message).toBe(0);
  return result.data;
}
async function mockMode(
  f: Fixture,
  mode: "success" | "rejected" | "uncertain",
) {
  const client = await request.newContext({
    baseURL: f.mockOrigin,
    ignoreHTTPSErrors: true,
  });
  try {
    const response = await client.post("/fixture/mode", {
      data: { mode },
      maxRedirects: 0,
    });
    expect(response.status()).toBe(200);
    expect(await response.json()).toEqual({ ok: true });
  } finally {
    await client.dispose();
  }
}
// The fixture captures this delay per newly-created task. Resetting the default
// cannot make an already running fake VM complete early.
async function mockTaskDelay(f: Fixture, seconds: number) {
  const client = await request.newContext({
    baseURL: f.mockOrigin,
    ignoreHTTPSErrors: true,
  });
  try {
    const response = await client.post("/fixture/task-delay", {
      data: { seconds },
      maxRedirects: 0,
    });
    expect(response.status()).toBe(200);
    expect(await response.json()).toEqual({ ok: true });
  } finally {
    await client.dispose();
  }
}
async function assertDedicatedChannels(f: Fixture, admin: APIRequestContext) {
  const resources = await get(
    admin,
    "/x/idc-resource/api/v1/admin/resource/provider-configurations",
  );
  expect(resources.channels.length).toBeGreaterThan(0);
  for (const channel of resources.channels) {
    expect(channel.endpoint).toBe(f.mockOrigin + "/administrator");
    // Existing fixture channels may retain the earlier compatible release;
    // the upgraded local directory connector is explicitly allowed as well.
    expect(["v0.3.9", "v0.3.14"]).toContain(channel.providerReleaseVersion);
  }
  const payments = await get(
    admin,
    "/x/idc-payment/api/v1/admin/payment-channels",
  );
  expect(payments.channels).toHaveLength(1);
  expect(payments.channels[0]).toMatchObject({
    channelNo: "EPAY_NIGHTLY",
    endpointOrigin: f.mockOrigin + "/submit.php",
  });
  const health = (
    await get(admin, "/x/idc-resource/api/v1/admin/resource/channel-health")
  ).items.find((row: any) => row.channelNo === f.resourceChannelNo);
  // Deliberate fake failures must not reuse the normal merchant threshold.
  // An already active protection deadline is never reset or shortened.
  await expect
    .poll(
      async () => {
        const current = (
          await get(
            admin,
            "/x/idc-resource/api/v1/admin/resource/channel-health",
          )
        ).items.find((row: any) => row.channelNo === f.resourceChannelNo);
        return (
          !current.circuitOpen || current.writeProtectedUntil <= Date.now()
        );
      },
      {
        timeout: 90_000,
        intervals: [1000],
        message:
          "Wait for the existing isolated protection deadline; never reset it",
      },
    )
    .toBe(true);
  expect(health.policy.minimumSamplesToProtect).toBe(100);
}
// Every secondary channel remains in the exact local fake origin already
// fenced by the descriptor; its credentials are fixed test values only.
async function createMockChannel(
  f: Fixture,
  admin: APIRequestContext,
  key: string,
) {
  const base = "/x/idc-resource/api/v1/admin/resource/provider-configurations";
  let channel = (
    await write(admin, base + "/channels", {
      displayName: "隔离备用上游" + key.slice(-8),
      endpoint: f.mockOrigin + "/administrator",
      providerPluginId: "idc-zjmfcloud",
      nonSecretSettings: {},
    })
  ).item;
  for (const [bindingName, secret] of [
    ["username", "nightly-api-user"],
    ["password", "nightly-api-password"],
  ]) {
    const path =
      "/x/idc-resource/api/v1/admin/resource/channels/" +
      channel.channelNo +
      "/secrets";
    const idempotencyKey = key + "-" + bindingName;
    const access = (
      await write(admin, path + "/access-contexts", {
        operation: "create",
        bindingName,
        secretDigest: createHash("sha256").update(secret!).digest("hex"),
        idempotencyKey,
      })
    ).item.token;
    await write(admin, path, {
      bindingName,
      secret: Buffer.from(secret!).toString("base64"),
      idempotencyKey,
      accessContext: access,
    });
  }
  expect(
    (
      await write(
        admin,
        base + "/channels/" + channel.channelNo + "/connection-verifications",
        {},
      )
    ).verified,
  ).toBe(true);
  const current = (await get(admin, base + "?channelKeyword=" + encodeURIComponent(channel.displayName))).channels.find(
    (row: any) => row.channelNo === channel.channelNo,
  );
  expect(current, "The newly created fixture channel must be found independently of directory pagination").toBeDefined();
  channel = (
    await write(
      admin,
      base + "/channels/" + channel.channelNo,
      {
        displayName: current.displayName,
        endpoint: current.endpoint,
        nonSecretSettings: current.nonSecretSettings,
        status: "enabled",
        version: current.version,
      },
      "PUT",
    )
  ).item;
  expect(channel.interfaceInstanceNo).toMatch(/^pfi_[0-9a-f]{32}$/);
  return channel;
}
async function fulfillment(
  admin: APIRequestContext,
  orderNo: string,
  status: string,
  timeout = 90_000,
) {
  let detail: any;
  await expect
    .poll(
      async () => {
        const list = await get(
          admin,
          "/x/idc-core/api/v1/admin/product-fulfillment/tasks?orderNo=" +
            orderNo,
        );
        if (!list.tasks.length) return "no_intent";
        expect(list.tasks).toHaveLength(1);
        detail = await get(
          admin,
          "/x/idc-core/api/v1/admin/product-fulfillment/tasks/" +
            list.tasks[0].intentNo,
        );
        return detail.task.statusCode;
      },
      { timeout, intervals: [1000, 2000] },
    )
    .toBe(status);
  return detail.task;
}
async function checkPaid(
  client: APIRequestContext,
  orderNo: string,
  expectedStatus: string,
) {
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
  expect(order.deliveries).toHaveLength(1);
  expect(order.deliveries[0].statusCode).toBe(expectedStatus);
  if (expectedStatus === "completed") {
    await expect
      .poll(
        async () => {
          const result = await get(
            client,
            "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
          );
          return result.order.serviceEntitlements.length;
        },
        { timeout: 20_000 },
      )
      .toBe(1);
  }
  return order;
}

// Directory inspection is a local projection and may initially be warming.
// Submit only the IDs actually confirmed by the same bounded read path as UI.
async function readyRecovery(
  admin: APIRequestContext,
  path: string,
  interfaceNo: string,
) {
  let editor: any;
  await expect
    .poll(
      async () => {
        editor = (
          await get(
            admin,
            path +
              "?includeRecoveryConfiguration=true&recoveryRegion=2&recoveryLine=6&recoveryOperatingSystem=98&recoveryInterfaceNo=" +
              encodeURIComponent(interfaceNo),
          )
        ).recoveryConfiguration;
        return editor.selection;
      },
      { timeout: 45000, intervals: [250, 1000, 2000] },
    )
    .toMatchObject({
      interfaceInstanceNo: interfaceNo,
      region: "2",
      line: "6",
      operatingSystem: "98",
    });
  return editor;
}

test.describe("TC-67 隔离订单明确失败与未知结果恢复", () => {
  test.skip(
    !descriptor,
    "Requires an explicit dedicated local fixture; never use live orders.",
  );
  test.setTimeout(180_000);
  test("TC-67a: 明确拒绝有脱敏正文，修正节点镜像后只生成一个后继且原价原规格不变", async () => {
    const f = await fixture(),
      admin = await api(f, f.adminAuth);
    let customer: APIRequestContext | undefined;
    try {
      await assertDedicatedChannels(f, admin);
      const key = "failed-" + randomUUID();
      customer = await createCustomer(f, admin, key);
      await mockMode(f, "rejected");
      const { orderNo } = await paidOrder(f, customer, key);
      const failed = await fulfillment(admin, orderNo, "not_completed");
      expect(failed).toMatchObject({
        canRecoverPreparation: true,
        canRequestRefund: true,
        canVerifyNonAcceptance: false,
        financiallySettled: true,
      });
      expect(failed.execution).toMatchObject({
        statusCode: "failed",
        failureCode: "upstream_request_rejected",
      });
      expect(JSON.parse(failed.execution.responseExcerpt)).toMatchObject({
        method: "POST",
        route: "/clouds",
        httpStatus: 200,
        response: {
          status: 400,
          msg: "隔离模拟：镜像未下载",
          request_echo: { _redactedFields: true },
        },
      });
      expect(failed.execution.responseExcerpt).not.toMatch(
        /MUST_REDACT|nightly-session-token|nightly-api-password/,
      );
      await checkPaid(customer, orderNo, "not_completed");
      expect(
        (await get(customer, serversPath + "?page=1&pageSize=20")).total,
      ).toBe(0);
      const path =
        "/x/idc-core/api/v1/admin/product-fulfillment/tasks/" + failed.intentNo;
      const originalInterface = (
        await get(admin, path + "?includeRecoveryConfiguration=true")
      ).recoveryConfiguration.original.interfaceInstanceNo;
      const editor = await readyRecovery(admin, path, originalInterface);
      expect(editor.specifications).toEqual(
        expect.arrayContaining([
          { key: "cpu", value: "2" },
          { key: "memory", value: "2000" },
          { key: "system_disk", value: "22" },
          { key: "data_disk", value: "22" },
        ]),
      );
      const command = {
        expectedVersion: failed.version,
        idempotencyKey: key + "-recovery",
        reasonCode: failed.recoveryReasons[0].code,
        note: "已核对隔离模拟上游，修正节点和已下载镜像，原成交规格金额保持不变",
        recoveryConfiguration: editor.selection,
      };
      const invalid = await send(admin, path + "/recoveries", {
        ...command,
        idempotencyKey: key + "-bad-image",
        recoveryConfiguration: { ...editor.selection, operatingSystem: "999" },
      });
      expect(invalid.code).not.toBe(0);
      const stale = await send(admin, path + "/recoveries", {
        ...command,
        idempotencyKey: key + "-stale",
        expectedVersion: failed.version + 100,
      });
      expect(stale.code).not.toBe(0);
      expect((await get(admin, path)).task.version).toBe(failed.version);
      await mockMode(f, "success");
      await write(admin, path + "/recoveries", command);
      await write(admin, path + "/recoveries", command);
      const completed = await fulfillment(admin, orderNo, "completed");
      expect(completed.recoveryConfigurations).toHaveLength(1);
      expect(completed.recoveryConfigurations[0].configuration).toMatchObject({
        line: "6",
        operatingSystem: "98",
      });
      expect(completed.execution.resourceTaskNo).not.toBe(
        failed.execution.resourceTaskNo,
      );
      await checkPaid(customer, orderNo, "completed");
      await expect
        .poll(
          async () =>
            (await get(customer!, serversPath + "?page=1&pageSize=20")).total,
          { timeout: 20_000 },
        )
        .toBe(1);
      const servers = await get(customer, serversPath + "?page=1&pageSize=20");
      expect(servers.total).toBe(1);
      const state = await saved(f, f.mockState);
      const digest = createHash("sha256")
        .update(
          "idc-resource/server/v1\0" +
            failed.intentNo +
            "\0" +
            failed.orderItemNo +
            "\0" +
            failed.unitSequence,
        )
        .digest("hex");
      const host = "srv-" + digest.slice(0, 16) + ".internal";
      expect(host).toBeTruthy();
      const forms = state.forms.filter(
        (row: any) => row.hostname?.[0] === host,
      );
      expect(forms).toHaveLength(2);
      expect(forms[0]).toMatchObject({
        node: ["5"],
        os: ["97"],
        cpu: ["2"],
        memory: ["2000"],
        system_disk_size: ["22"],
        "other_data_disk[0][size]": ["22"],
      });
      expect(forms[1]).toMatchObject({
        node: ["6"],
        os: ["98"],
        cpu: ["2"],
        memory: ["2000"],
        system_disk_size: ["22"],
        "other_data_disk[0][size]": ["22"],
      });
    } finally {
      await mockMode(f, "success");
      await customer?.dispose();
      await admin.dispose();
    }
  });
  test("TC-67b: 空响应保留未知正文和写入保护，核对确认后单独重开且不重复收费", async () => {
    const f = await fixture(),
      admin = await api(f, f.adminAuth);
    let customer: APIRequestContext | undefined;
    try {
      await assertDedicatedChannels(f, admin);
      const key = "unknown-" + randomUUID();
      customer = await createCustomer(f, admin, key);
      await mockMode(f, "uncertain");
      const { orderNo } = await paidOrder(f, customer, key);
      const unknown = await fulfillment(admin, orderNo, "manual_processing");
      expect(unknown).toMatchObject({
        canRecoverPreparation: false,
        canRequestRefund: false,
        canVerifyNonAcceptance: true,
        financiallySettled: true,
      });
      expect(unknown.execution.statusCode).toBe("result_unknown");
      expect(JSON.parse(unknown.execution.responseExcerpt)).toMatchObject({
        method: "POST",
        route: "/clouds",
        httpStatus: 200,
        response: {},
      });
      await checkPaid(customer, orderNo, "manual_processing");
      const path =
        "/x/idc-core/api/v1/admin/product-fulfillment/tasks/" +
        unknown.intentNo;
      const recovery = {
        expectedVersion: unknown.version,
        idempotencyKey: key + "-unsafe-recovery",
        reasonCode: "preparation_resolved",
        note: "隔离订单尚未确认，不得直接重放开通写入",
      };
      expect((await send(admin, path + "/recoveries", recovery)).code).not.toBe(
        0,
      );
      const fact = {
        expectedVersion: unknown.version,
        idempotencyKey: key + "-verification",
        note: "已检查本地模拟器状态，未创建实例或任务，保留原始未知响应历史",
        confirmedNoCreation: true,
      };
      expect(
        (
          await send(admin, path + "/non-acceptance-verifications", {
            ...fact,
            confirmedNoCreation: false,
          })
        ).code,
      ).not.toBe(0);
      expect(
        (
          await send(admin, path + "/non-acceptance-verifications", {
            ...fact,
            note: "短说明",
          })
        ).code,
      ).not.toBe(0);
      await write(admin, path + "/non-acceptance-verifications", fact);
      const verified = (
        await write(admin, path + "/non-acceptance-verifications", fact)
      ).task;
      expect(verified.nonAcceptanceVerifications).toHaveLength(1);
      expect(verified).toMatchObject({
        canRecoverPreparation: true,
        canRequestRefund: true,
        canVerifyNonAcceptance: false,
      });
      expect(
        (await get(customer, serversPath + "?page=1&pageSize=20")).total,
      ).toBe(0);
      await mockMode(f, "success");
      const command = {
        ...recovery,
        expectedVersion: verified.version,
        idempotencyKey: key + "-safe-recovery",
      };
      await write(admin, path + "/recoveries", command);
      await write(admin, path + "/recoveries", command);
      const completed = await fulfillment(admin, orderNo, "completed");
      expect(completed.nonAcceptanceVerifications).toHaveLength(1);
      await checkPaid(customer, orderNo, "completed");
      await expect
        .poll(
          async () =>
            (await get(customer!, serversPath + "?page=1&pageSize=20")).total,
          { timeout: 20_000 },
        )
        .toBe(1);
    } finally {
      await mockMode(f, "success");
      await customer?.dispose();
      await admin.dispose();
    }
  });
  test("TC-67c: 原上游明确失败后换上游开通，客户服务器仍归原付款订单", async () => {
    const f = await fixture(),
      admin = await api(f, f.adminAuth);
    let customer: APIRequestContext | undefined;
    try {
      await assertDedicatedChannels(f, admin);
      const key = "switch-" + randomUUID();
      const replacement = await createMockChannel(f, admin, key);
      customer = await createCustomer(f, admin, key);
      await mockMode(f, "rejected");
      const { orderNo } = await paidOrder(f, customer, key);
      const failed = await fulfillment(admin, orderNo, "not_completed");
      const path =
        "/x/idc-core/api/v1/admin/product-fulfillment/tasks/" + failed.intentNo;
      const editor = await readyRecovery(
        admin,
        path,
        replacement.interfaceInstanceNo,
      );
      expect(editor.selection.interfaceInstanceNo).toBe(
        replacement.interfaceInstanceNo,
      );
      expect(editor.original.interfaceInstanceNo).not.toBe(
        replacement.interfaceInstanceNo,
      );
      const command = {
        expectedVersion: failed.version,
        idempotencyKey: key + "-recovery",
        reasonCode: failed.recoveryReasons[0].code,
        note: "隔离上游明确拒绝，选择已验证的备用上游，仅修改对接，不改原成交规格与金额",
        recoveryConfiguration: editor.selection,
      };
      await mockMode(f, "success");
      await write(admin, path + "/recoveries", command);
      await write(admin, path + "/recoveries", command);
      const completed = await fulfillment(admin, orderNo, "completed");
      expect(completed.recoveryConfigurations).toHaveLength(1);
      expect(
        completed.recoveryConfigurations[0].configuration.interfaceInstanceNo,
      ).toBe(replacement.interfaceInstanceNo);
      await checkPaid(customer, orderNo, "completed");
      await expect
        .poll(
          async () =>
            (await get(customer!, serversPath + "?page=1&pageSize=20")).total,
          { timeout: 20_000 },
        )
        .toBe(1);
      // Successful provisioning and the local management handoff are separate
      // facts. A visible owned server must also have its reliable receipt.
      await expect
        .poll(
          async () => {
            const view = await get(
              customer!,
              "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
            );
            return view.order.serviceEntitlements[0]?.managementPreparation;
          },
          { timeout: 20000 },
        )
        .toBe("ready");
      const managed = (await get(admin, path)).task.managementPreparation;
      expect(managed.statusCode).toBe("ready");
      expect(managed.attemptCount).toBeGreaterThanOrEqual(1);
      const ownerView = await get(
        customer,
        "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
      );
      const entitlementNo =
        ownerView.order.serviceEntitlements[0].entitlementNo;
      const pluginRoot = fileURLToPath(new URL("../../../", import.meta.url));
      execFileSync(
        "go",
        [
          "run",
          "./hack/tools/isolated-management-fixture",
          orderNo,
          completed.intentNo,
          entitlementNo,
        ],
        {
          cwd: pluginRoot,
          timeout: 30000,
          env: {
            ...process.env,
            GOWORK: resolve(pluginRoot, "../../../temp/output/.build/go.work"),
          },
          stdio: "pipe",
        },
      );
      const stopped = (await get(admin, path)).task;
      expect(stopped.canRepairManagement).toBe(true);
      expect(stopped.managementPreparation.statusCode).toBe("needs_attention");
      expect(stopped.execution).toEqual(completed.execution);
      const repair = {
        expectedVersion: stopped.version,
        idempotencyKey: key + "-management-repair",
      };
      const repaired = await write(admin, path + "/management-repairs", repair);
      expect(repaired.task.version).toBe(completed.version);
      expect(repaired.task.statusCode).toBe("completed");
      await write(admin, path + "/management-repairs", repair);
      await expect
        .poll(
          async () =>
            (await get(admin, path)).task.managementPreparation.statusCode,
          { timeout: 20000 },
        )
        .toBe("ready");
      const ready = (await get(admin, path)).task;
      expect(ready.canRepairManagement).toBe(false);
      expect(ready.execution).toEqual(completed.execution);
      expect(ready.recoveryConfigurations).toEqual(
        completed.recoveryConfigurations,
      );
      expect(
        ready.events.filter(
          (event: any) => event.eventCode === "management_repair_queued",
        ),
      ).toHaveLength(1);
      await checkPaid(customer, orderNo, "completed");
      const list = await get(customer, serversPath + "?page=1&pageSize=20");
      const detail = await get(
        customer,
        serversPath + "/" + list.items[0].serverNo,
      );
      expect(detail).toMatchObject({
        orderNo,
        entitlementStatus: "active",
        runtimeState: "active",
        credentialAvailable: true,
      });
      const digest = createHash("sha256")
        .update(
          "idc-resource/server/v1\0" +
            failed.intentNo +
            "\0" +
            failed.orderItemNo +
            "\0" +
            failed.unitSequence,
        )
        .digest("hex");
      const forms = (await saved(f, f.mockState)).forms.filter(
        (row: any) =>
          row.hostname?.[0] === "srv-" + digest.slice(0, 16) + ".internal",
      );
      expect(forms).toHaveLength(2);
      expect(forms[1].client).not.toEqual(forms[0].client);
      expect(forms[1]).toMatchObject({
        node: ["6"],
        os: ["98"],
        cpu: ["2"],
        memory: ["2000"],
        system_disk_size: ["22"],
        "other_data_disk[0][size]": ["22"],
      });
    } finally {
      await mockMode(f, "success");
      await customer?.dispose();
      await admin.dispose();
    }
  });
  test("TC-67d: 重开任务持续正常运行超过一分钟，不提前转人工且只生成一台服务器", async () => {
    test.setTimeout(330_000);
    const f = await fixture(),
      admin = await api(f, f.adminAuth);
    let customer: APIRequestContext | undefined;
    try {
      await assertDedicatedChannels(f, admin);
      const key = "slow-" + randomUUID();
      customer = await createCustomer(f, admin, key);
      await mockMode(f, "rejected");
      const { orderNo } = await paidOrder(f, customer, key);
      const failed = await fulfillment(admin, orderNo, "not_completed");
      const path =
        "/x/idc-core/api/v1/admin/product-fulfillment/tasks/" + failed.intentNo;
      let editor: any;
      await expect.poll(async () => {
        editor = (await get(admin, path + "?includeRecoveryConfiguration=true")).recoveryConfiguration;
        return Boolean(editor?.selection?.region && editor.selection.line && editor.selection.operatingSystem);
      }, { timeout: 30_000 }).toBe(true);
      await mockTaskDelay(f, 70);
      await mockMode(f, "success");
      const startedAt = Date.now();
      const command = {
        expectedVersion: failed.version,
        idempotencyKey: key + "-recover",
        reasonCode: failed.recoveryReasons[0].code,
        note: "隔离上游已明确拒绝，修正后模拟正常开通耗时七十秒，不更改成交规格与金额",
        recoveryConfiguration: editor.selection,
      };
      await write(admin, path + "/recoveries", command);
      await write(admin, path + "/recoveries", command);
      // The resource owner retains external query backoff. Frequent local
      // progress reads must still allow this legitimate seventy-second task.
      const completed = await fulfillment(admin, orderNo, "completed", 200_000);
      expect(Date.now() - startedAt).toBeGreaterThanOrEqual(70_000);
      expect(completed.recoveryConfigurations).toHaveLength(1);
      expect(completed.execution.attemptCount).toBeGreaterThan(10);
      const ownerTask = (await get(admin, "/x/idc-resource/api/v1/admin/resource/tasks/" + completed.execution.resourceTaskNo)).item.task;
      const terminalEvent = completed.events.filter((e:any) => e.eventCode === "automation_completed").at(-1);
      expect(terminalEvent.occurredAt - ownerTask.updatedAt).toBeLessThan(7_000);
      await checkPaid(customer, orderNo, "completed");
      await expect
        .poll(
          async () =>
            (await get(customer!, serversPath + "?page=1&pageSize=20")).total,
          { timeout: 20_000 },
        )
        .toBe(1);
      const list = await get(customer, serversPath + "?page=1&pageSize=20");
      expect(
        await get(customer, serversPath + "/" + list.items[0].serverNo),
      ).toMatchObject({
        orderNo,
        entitlementStatus: "active",
        runtimeState: "active",
      });
      const digest = createHash("sha256")
        .update(
          "idc-resource/server/v1\0" +
            failed.intentNo +
            "\0" +
            failed.orderItemNo +
            "\0" +
            failed.unitSequence,
        )
        .digest("hex");
      const forms = (await saved(f, f.mockState)).forms.filter(
        (row: any) =>
          row.hostname?.[0] === "srv-" + digest.slice(0, 16) + ".internal",
      );
      expect(forms).toHaveLength(2);
      expect(forms[1]).toMatchObject({
        cpu: ["2"],
        memory: ["2000"],
        system_disk_size: ["22"],
        "other_data_disk[0][size]": ["22"],
      });
    } finally {
      await mockTaskDelay(f, 0);
      await mockMode(f, "success");
      await customer?.dispose();
      await admin.dispose();
    }
  });
  test("TC-67e: 六秒冷登录可验证连接并读取恢复配置，仍不发送新的开通", async () => {
    const f = await fixture(),
      admin = await api(f, f.adminAuth),
      key = "slow-login-" + randomUUID();
    let customer: APIRequestContext | undefined;
    const mock = await request.newContext({
      baseURL: f.mockOrigin,
      ignoreHTTPSErrors: true,
    });
    async function loginDelay(seconds: number) {
      const response = await mock.post("/fixture/login-delay", {
        data: { seconds },
        maxRedirects: 0,
      });
      expect(response.status()).toBe(200);
      expect(await response.json()).toEqual({ ok: true });
    }
    try {
      await assertDedicatedChannels(f, admin);
      customer = await createCustomer(f, admin, key);
      await mockMode(f, "rejected");
      const { orderNo } = await paidOrder(f, customer, key);
      const failed = await fulfillment(admin, orderNo, "not_completed");
      const before = await saved(f, f.mockState);
      await loginDelay(6);
      const replacement = await createMockChannel(f, admin, key);
      const recoveryPath =
        "/x/idc-core/api/v1/admin/product-fulfillment/tasks/" +
          failed.intentNo +
          "?includeRecoveryConfiguration=true&recoveryRegion=2&recoveryLine=6&recoveryOperatingSystem=98&recoveryInterfaceNo=" +
          replacement.interfaceInstanceNo;
      const started = Date.now();
      let detail = await get(admin, recoveryPath);
      // A new interface has no directory projection yet. The editor must
      // return promptly while the six-second cold login runs asynchronously.
      expect(Date.now() - started).toBeLessThan(3000);
      await expect.poll(async () => {
        detail = await get(admin, recoveryPath);
        return [detail.recoveryConfiguration.selection.line,
          detail.recoveryConfiguration.selection.operatingSystem];
      }, { timeout: 45000, intervals: [1000] }).toEqual(["6", "98"]);
      expect(detail.recoveryConfiguration.selection.interfaceInstanceNo).toBe(
        replacement.interfaceInstanceNo,
      );
      expect(detail.recoveryConfiguration.selection.line).toBe("6");
      expect(detail.recoveryConfiguration.selection.operatingSystem).toBe("98");
      expect(detail.task.statusCode).toBe("not_completed");
      const after = await saved(f, f.mockState);
      expect(after.provisions).toBe(before.provisions);
      expect(after.violations.slice(before.violations.length)).toEqual([]);
      await checkPaid(customer, orderNo, "not_completed");
    } finally {
      await loginDelay(0);
      await mockMode(f, "success");
      await customer?.dispose();
      await admin.dispose();
      await mock.dispose();
    }
  });
});

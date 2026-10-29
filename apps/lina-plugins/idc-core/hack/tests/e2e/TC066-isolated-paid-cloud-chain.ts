// Explicit local integration fixture only: real HTTP, payment owner and WASM
// connector, with a dedicated database and loopback supplier. Never live orders.
import { test } from "@host-tests/fixtures/auth";
import { expect } from "@host-tests/support/playwright";
import {
  api,
  descriptor,
  fixture,
  get,
  orderPath,
  saved,
  serversPath,
} from "../support/isolated-commerce";

test.describe("TC-66 隔离支付到云服务器真实链路", () => {
  test.skip(
    !descriptor,
    "Requires the explicit dedicated-database local fixture; never use a live system.",
  );
  test("TC-66a: 真实结清、一次开通、原规格、权益激活及密码交付条件一致", async () => {
    const f = await fixture(),
      customer = await api(f, f.customerAuth),
      admin = await api(f, f.adminAuth);
    try {
      const channels = await get(
        admin,
        "/x/idc-resource/api/v1/admin/resource/provider-configurations",
      );
      expect(channels.channels.length).toBeGreaterThan(0);
      expect(channels.channels[0].endpoint).toBe(
        f.mockOrigin + "/administrator",
      );
      expect(channels.channels[0].providerReleaseVersion).toBe("v0.3.9");
      const { order } = await get(customer, orderPath(f));
      expect(order.summary.statusCode).toBe("paid");
      expect(order.summary.totalAmountMinor).toBe("1100");
      expect(order.invoice).toMatchObject({
        statusCode: "paid",
        totalAmountMinor: "1100",
        paidAmountMinor: "1100",
        refundedAmountMinor: "0",
      });
      expect(order.deliveries).toHaveLength(1);
      expect(order.deliveries[0].statusCode).toBe("completed");
      expect(order.serviceEntitlements).toHaveLength(1);
      const entitlement = order.serviceEntitlements[0];
      expect(entitlement.statusCode).toBe("active");
      const list = await get(customer, serversPath + "?page=1&pageSize=20");
      expect(list.total).toBe(1);
      expect(list.items).toHaveLength(1);
      expect(list.items[0].serverNo).toBe(f.serverNo);
      const detail = await get(customer, serversPath + "/" + f.serverNo);
      expect(detail).toMatchObject({
        orderNo: f.orderNo,
        serverNo: f.serverNo,
        entitlementNo: entitlement.entitlementNo,
        entitlementStatus: "active",
        runtimeState: "active",
        credentialAvailable: true,
        addressesConfirmed: true,
        addresses: [f.expectedAddress],
      });
      expect(detail.startedAt).toBe(entitlement.startedAt);
      expect(detail.expiresAt).toBe(entitlement.expiresAt);
      expect(detail.startedAt).toBeGreaterThan(order.invoice.settledAt);
      expect(detail.expiresAt).toBeGreaterThan(detail.startedAt);
      expect(detail.startedAt - order.invoice.settledAt).toBeLessThan(75_000);
      expect(detail.availableActions).toEqual(
        expect.arrayContaining(["stop", "reboot"]),
      );
      for (const name of ["password", "secret", "initialPassword"]) {
        expect(detail).not.toHaveProperty(name);
      }
      const resolved = await get(
        customer,
        serversPath + "/resolve/" + entitlement.entitlementNo,
      );
      expect(resolved.serverNo).toBe(f.serverNo);
      const tasks = await get(
        admin,
        "/x/idc-core/api/v1/admin/product-fulfillment/tasks?orderNo=" +
          f.orderNo,
      );
      expect(tasks.total).toBe(1);
      expect(tasks.tasks[0].statusCode).toBe("completed");
      const { task } = await get(
        admin,
        "/x/idc-core/api/v1/admin/product-fulfillment/tasks/" +
          tasks.tasks[0].intentNo,
      );
      expect(task.financiallySettled).toBe(true);
      expect(task.execution.statusCode).toBe("succeeded");
      expect(JSON.parse(task.execution.responseExcerpt)).toMatchObject({
        method: "POST",
        route: "/clouds",
        httpStatus: 200,
        response: { data: { id: f.expectedCloudId, taskid: f.expectedTaskId } },
      });
      expect(task.execution.responseExcerpt).not.toMatch(
        /rootpass|nightly-api-password|nightly-session-token|nightly-epay-secret/,
      );
      const mock = await saved(f, f.mockState);
      expect(mock.violations).toEqual([]);
      const forms = mock.forms.filter(
        (row: any) => row.hostname?.[0] === f.expectedHostname,
      );
      expect(forms).toHaveLength(1);
      expect(forms[0]).toMatchObject({
        cpu: ["2"],
        memory: ["2000"],
        system_disk_size: ["22"],
        "other_data_disk[0][size]": ["22"],
        os: ["97"],
      });
      expect(forms[0]).not.toHaveProperty("rootpass");
    } finally {
      await customer.dispose();
      await admin.dispose();
    }
  });
  test("TC-66b: 错误通知不能改款，重复正确通知不重复付款或开通", async () => {
    const f = await fixture(),
      admin = await api(f, f.adminAuth),
      customer = await api(f, f.customerAuth);
    const callback = await saved(f, f.callback),
      checkout = await saved(f, f.checkout);
    try {
      const channels = await get(
        admin,
        "/x/idc-payment/api/v1/admin/payment-channels",
      );
      const channel = channels.channels.find(
        (row: any) => row.channelNo === f.channelNo,
      );
      expect(channel.endpointOrigin).toBe(f.mockOrigin + "/submit.php");
      expect(checkout.payment).toMatchObject({
        attemptNo: f.attemptNo,
        redirectMethod: "POST",
        redirectUrl: f.mockOrigin + "/submit.php",
        redirectFields: { money: "11.00" },
      });
      const path = "/x/idc-payment/api/v1/callbacks/payments/" + f.channelNo;
      expect(new URL(checkout.payment.redirectFields.notify_url).pathname).toBe(
        path,
      );
      const bad = new URLSearchParams({
        ...callback.values,
        sign: "0".repeat(32),
      });
      const invalid = await customer.get(path + "?" + bad, { maxRedirects: 0 });
      expect((await invalid.text()).trim()).not.toBe("success");
      const valid = new URLSearchParams(callback.values);
      const response = await customer.get(path + "?" + valid, {
        maxRedirects: 0,
      });
      expect(response.status()).toBe(200);
      expect((await response.text()).trim()).toBe("success");
      const { order } = await get(customer, orderPath(f));
      expect(order.invoice).toMatchObject({
        statusCode: "paid",
        totalAmountMinor: "1100",
        paidAmountMinor: "1100",
        refundedAmountMinor: "0",
      });
      expect(order.serviceEntitlements).toHaveLength(1);
      const list = await get(customer, serversPath + "?page=1&pageSize=20");
      expect(list.total).toBe(1);
      const mock = await saved(f, f.mockState);
      expect(
        mock.forms.filter(
          (row: any) => row.hostname?.[0] === f.expectedHostname,
        ),
      ).toHaveLength(1);
    } finally {
      await admin.dispose();
      await customer.dispose();
    }
  });
  test("TC-66c: 其他客户看不到该服务器，真实和不存在编号返回相同拒绝", async () => {
    const f = await fixture(),
      foreign = await api(f, f.foreignAuth);
    try {
      const responses = [];
      for (const no of [f.serverNo, "SVR-" + "0".repeat(32)]) {
        const r = await foreign.get(serversPath + "/" + no, {
          maxRedirects: 0,
        });
        const b = await r.json();
        expect(b.code).not.toBe(0);
        responses.push({
          status: r.status(),
          code: b.code,
          errorCode: b.errorCode,
          message: b.message,
        });
      }
      expect(responses[0]).toEqual(responses[1]);
      expect(responses[0]!.message).toBe("服务器不存在或当前不可访问");
      const list = await get(foreign, serversPath + "?page=1&pageSize=20");
      expect(list.items.some((row: any) => row.serverNo === f.serverNo)).toBe(
        false,
      );
    } finally {
      await foreign.dispose();
    }
  });
});

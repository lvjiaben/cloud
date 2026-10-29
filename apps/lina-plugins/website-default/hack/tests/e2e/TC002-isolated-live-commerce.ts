// Full browser-to-database flow with real local templates and frozen connector.
// Supplier/payment services are the fenced loopback fixture, never live funds.
import { createHash, randomUUID } from "node:crypto";
import { test, expect } from "@host-tests/fixtures/auth";
import { request } from "@host-tests/support/playwright";
import {
  api,
  descriptor,
  get,
  saved,
  serversPath,
  webFixture,
  customer,
  mockSuccess,
  elasticProduct,
} from "../support/isolated-web-commerce";
import { LiveCommercePage } from "../pages/LiveCommercePage";
import { NativeServerPage } from "../../../../customer-default/hack/tests/pages/NativeServerPage";

test.describe("TC-2 独立数据库真实官网交易", () => {
  test.skip(
    !descriptor,
    "Requires explicit local fake commerce fixture; never use main orders.",
  );
  test.setTimeout(420_000);
  for (const scenario of ["fixed", "elastic", "power"]) {
    const elastic = scenario === "elastic",
      power = scenario === "power";
    test(
      power
        ? "TC-2c: 客户经真实拥有方与冻结连接器关机开机重启"
        : elastic
          ? "TC-2b: 弹性CPU与内存实际计价并传递给上游开通"
          : "TC-2a: 游客选固定套餐，真实登录结清开通，客户可管理同一台服务器",
      async ({ browser }) => {
        const f = await webFixture(),
          admin = await api(f, f.adminAuth),
          key = "web-" + randomUUID(),
          password = randomUUID() + "aA1!";
        const account = await customer(f, admin, key, password);
        const slug = elastic
          ? await elasticProduct(admin, key)
          : "nightly-fixed-cloud";
        const amount = elastic ? "16.00" : "11.00",
          minor = elastic ? "1600" : "1100";
        await mockSuccess(f);
        const beforeMock = await saved(f, f.mockState);
        const context = await browser.newContext({ ignoreHTTPSErrors: true });
        const allowed = new Set([
          f.websiteOrigin,
          f.customerOrigin,
          f.rendererOrigin,
          f.baseURL,
          f.mockOrigin,
        ]);
        const unexpected: string[] = [];
        // This fence only permits real network traffic to the owned local stack;
        // it never fulfills or rewrites a core transaction response.
        await context.route("**/*", async (route) => {
          const url = new URL(route.request().url());
          if (!allowed.has(url.origin)) {
            unexpected.push(url.origin + url.pathname);
            await route.abort();
          } else await route.continue();
        });
        const page = await context.newPage(),
          view = new LiveCommercePage(page, f.websiteOrigin);
        let client: Awaited<ReturnType<typeof request.newContext>> | undefined;
        try {
          await view.open(slug);
          await expect(
            page.getByRole("heading", {
              name: elastic ? "V1隔离弹性云服务器" : "V1隔离固定云服务器",
              exact: true,
            }),
          ).toBeVisible();
          await view.selectOS();
          if (elastic) {
            await view.configurator
              .locator(".ant-form-item")
              .filter({ hasText: "处理器核心数" })
              .getByRole("spinbutton")
              .fill("4");
            await page
              .getByRole("spinbutton", { name: "内存（GiB）", exact: true })
              .fill("2");
            await page
              .getByRole("spinbutton", { name: "内存（GiB）", exact: true })
              .blur();
          }
          await view.quote();
          await expect(view.total).toContainText(amount);
          await view.screenshot("fixed-price");
          await view.add();
          await expect(
            view.cart.getByTestId("website-cart-configuration"),
          ).toContainText("Debian13-x86");
          await expect(
            view.cart.getByTestId("website-cart-preview"),
          ).toContainText("CNY " + amount);
          await view.screenshot("guest-cart");
          await view.signIn();
          const loginResponse = page
            .waitForResponse(
              (response) =>
                response.url() === f.customerOrigin + "/api/v1/auth/login" &&
                response.request().method() === "POST",
            )
            .then((response) => response.json());
          await view.login(account.email, account.password);
          const login = await loginResponse;
          expect(login.code).toBe(0);
          client = await request.newContext({
            baseURL: f.baseURL,
            extraHTTPHeaders: {
              Authorization: "Bearer " + login.data.accessToken,
            },
          });
          await page.waitForURL(f.websiteOrigin + "/cart", { timeout: 30_000 });
          await expect(
            view.cart.getByTestId("website-cart-configuration"),
          ).toContainText("Debian13-x86");
          await view.review();
          await expect(view.finalQuote).toContainText("CNY " + amount);
          await view.submitOrder();
          await page.waitForURL(
            (url) =>
              url.origin === f.customerOrigin &&
              /^\/account\/billing\/invoices\/INV_[a-f0-9]{32}$/.test(
                url.pathname,
              ),
          );
          const invoiceNo = new URL(page.url()).pathname.split("/").at(-1)!;
          const orders = await get(
            client,
            "/x/idc-core/api/v1/account/commerce/orders?limit=20",
          );
          expect(orders.orders).toHaveLength(1);
          const orderNo = orders.orders[0].orderNo;
          expect(
            (
              await get(
                client,
                "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
              )
            ).order.invoice.invoiceNo,
          ).toBe(invoiceNo);
          await expect(view.payment).toBeVisible();
          await expect(
            page.getByTestId("invoice-payment-submit"),
          ).toBeEnabled();
          await view.screenshot("invoice-unpaid");
          const submitRequest = page.waitForRequest(
            (req) =>
              req.url() === f.mockOrigin + "/submit.php" &&
              req.method() === "POST",
          );
          await view.pay();
          const submitted = new URLSearchParams(
            (await submitRequest).postData() || "",
          );
          expect(submitted.get("pid")).toBe("1108");
          expect(submitted.get("money")).toBe(amount);
          const attemptNo = submitted.get("out_trade_no")!;
          expect(attemptNo).toMatch(/^PAT_[a-f0-9]{32}$/);
          await page.waitForURL(
            (url) =>
              url.origin === f.mockOrigin && url.pathname === "/submit.php",
          );
          expect(
            (
              await get(
                client,
                "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
              )
            ).order.invoice.paidAmountMinor,
          ).toBe("0");
          expect(
            (await get(client, serversPath + "?page=1&pageSize=20")).total,
          ).toBe(0);
          const values: Record<string, string> = {
            pid: "1108",
            trade_no: key,
            out_trade_no: attemptNo,
            type: "alipay",
            name: "Isolated browser cloud",
            money: amount,
            trade_status: "TRADE_SUCCESS",
            sign_type: "MD5",
          };
          const canonical = Object.keys(values)
            .filter((k) => k !== "sign_type")
            .sort()
            .map((k) => k + "=" + values[k])
            .join("&");
          values.sign = createHash("md5")
            .update(canonical + "nightly-epay-secret")
            .digest("hex");
          const callback = await client.get(
            "/x/idc-payment/api/v1/callbacks/payments/EPAY_NIGHTLY?" +
              new URLSearchParams(values),
          );
          expect((await callback.text()).trim()).toBe("success");
          await expect
            .poll(
              async () =>
                (await get(client!, serversPath + "?page=1&pageSize=20")).total,
              { timeout: 120_000, intervals: [1000, 2000] },
            )
            .toBe(1);
          const list = await get(client, serversPath + "?page=1&pageSize=20"),
            server = list.items[0];
          const detail = await get(client, serversPath + "/" + server.serverNo);
          expect(detail).toMatchObject({
            orderNo,
            entitlementStatus: "active",
            runtimeState: "active",
            credentialAvailable: true,
          });
          const paid = (
            await get(
              client,
              "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
            )
          ).order;
          expect(paid.invoice).toMatchObject({
            statusCode: "paid",
            paidAmountMinor: minor,
            totalAmountMinor: minor,
            refundedAmountMinor: "0",
          });
          const native = new NativeServerPage(
            page,
            f.customerOrigin,
            server.serverNo,
          );
          await native.open();
          await expect(
            page.getByText("192.0.2.", { exact: false }).first(),
          ).toBeVisible();
          await expect(
            page.locator("svg.iconify--lucide").first(),
          ).toBeVisible();
          await native.screenshot(
            elastic ? "live-paid-elastic" : "live-paid-fixed",
          );
          const mock = await saved(f, f.mockState);
          expect(mock.violations.slice(beforeMock.violations.length)).toEqual(
            [],
          );
          const owned = (
            await get(
              admin,
              "/x/idc-core/api/v1/admin/product-fulfillment/tasks?orderNo=" +
                orderNo,
            )
          ).tasks;
          expect(owned).toHaveLength(1);
          const task = owned[0],
            digest = createHash("sha256")
              .update(
                "idc-resource/server/v1\0" +
                  task.intentNo +
                  "\0" +
                  task.orderItemNo +
                  "\0" +
                  task.unitSequence,
              )
              .digest("hex");
          const forms = mock.forms.filter(
            (row: any) =>
              row.hostname?.[0] === "srv-" + digest.slice(0, 16) + ".internal",
          );
          expect(forms).toHaveLength(1);
          expect(forms[0]).toMatchObject({
            cpu: [elastic ? "4" : "2"],
            memory: [elastic ? "2048" : "2000"],
            system_disk_size: ["22"],
            "other_data_disk[0][size]": ["22"],
            os: ["97"],
          });
          if (power) {
            const executionDetail = await get(admin,
              "/x/idc-core/api/v1/admin/product-fulfillment/tasks/" + task.intentNo);
            // Provider identity comes from its accepted receipt, not an IP
            // octet; addresses remain valid when the fixture exceeds 255 VMs.
            const cloud = JSON.parse(executionDetail.task.execution.responseExcerpt).response.data.id;
            expect(Number.isSafeInteger(cloud)).toBe(true);
            for (const [action, label, expectedState, route] of [
              ["stop", "关机", "stopped", "off"],
              ["start", "开机", "active", "on"],
              ["reboot", "重启", "active", "reboot"],
            ]) {
              const before = await saved(f, f.mockState);
              const submitted = page.waitForResponse(
                (response) =>
                  response.url() ===
                    f.customerOrigin +
                      serversPath +
                      "/" +
                      server.serverNo +
                      "/operations" && response.request().method() === "POST",
              );
              await page
                .getByRole("button", { name: label, exact: true })
                .click();
              if (action !== "start") {
                await expect(
                  page.getByRole("button", { name: "确认执行", exact: true }),
                ).toBeDisabled();
                await native.acceptImpact();
                await native.screenshot("live-" + action + "-confirmation");
                await native.confirm();
              }
              const response = await submitted,
                body = await response.json();
              expect(body.code).toBe(0);
              const operationNo = body.data.operation.operationNo;
              await expect
                .poll(
                  async () => {
                    const result = await get(
                      client!,
                      serversPath + "/" + server.serverNo,
                    );
                    return [
                      result.operations.find(
                        (o: any) => o.operationNo === operationNo,
                      )?.status,
                      result.runtimeState,
                    ];
                  },
                  { timeout: 120_000, intervals: [1000, 2000] },
                )
                .toEqual(["succeeded", expectedState]);
              await page
                .getByRole("button", { name: "刷新状态", exact: true })
                .click();
              await expect(
                page
                  .getByTestId("native-server-operations")
                  .locator("li")
                  .filter({ hasText: operationNo }),
              ).toContainText("已完成");
              await native.screenshot("live-" + action + "-completed");
              const after = await saved(f, f.mockState);
              expect(
                after.calls
                  .slice(before.calls.length)
                  .filter(
                    (c: any) =>
                      c.method === "POST" &&
                      c.path ===
                        "/administrator/v1/clouds/" + cloud + "/" + route,
                  ),
              ).toHaveLength(1);
              expect(after.violations.slice(before.violations.length)).toEqual(
                [],
              );
            }
            const after = await get(
              client,
              serversPath + "/" + server.serverNo,
            );
            expect(after.operations).toHaveLength(3);
            expect(after.credentialAvailable).toBe(true);
            await expect(
              page.getByRole("button", { name: "查看初始密码", exact: true }),
            ).toBeVisible();
            expect(
              after.operations.every((o: any) => o.status === "succeeded"),
            ).toBe(true);
            expect(
              (await get(client, serversPath + "?page=1&pageSize=20")).total,
            ).toBe(1);
          }
          expect(unexpected).toEqual([]);
        } finally {
          await client?.dispose();
          await context.close();
          await admin.dispose();
        }
      },
    );
  }
});

// Real current-customer HTTP acceptance. Dedicated users use public host APIs;
// plugin-owned fixtures and logs are isolated and removed in the finally block.
import { randomBytes } from "node:crypto";
import { mkdir } from "node:fs/promises";
import { expect, test } from "@host-tests/fixtures/auth";
import { config, workspacePath } from "@host-tests/fixtures/config";
import { execPgSQLStatements } from "@host-tests/support/postgres";

test.describe("TC012 customer open platform real", () => {
  test("TC-12a: 本人授权、重置不扩权、旧密钥失效、调用全记录和停用历史", async ({
    adminPage,
    browser,
  }) => {
    test.setTimeout(120_000);
    const stamp = randomBytes(12).toString("hex");
    const api = "/x/idc-channel/api/v1";
    const customer = `CPA_${stamp}`,
      other = `CPB_${stamp}`;
    const password = randomBytes(16).toString("hex");
    const users: number[] = [];
    let appId = "",
      auth = "";
    try {
      const req = adminPage.waitForRequest((r) =>
        r.url().includes("/admin/channel/accounts"),
      );
      await adminPage.goto(workspacePath("/idc/channel"));
      auth = (await (await req).allHeaders()).authorization!;
      expect(auth).toBeTruthy();
      const staff = { Authorization: auth };
      // Remove only orphan fixtures from earlier interrupted executions of this exact test.
      const directory = await (
        await adminPage.request.get(`${config.apiBaseURL}user?pageSize=100`, {
          headers: staff,
        })
      ).json();
      for (const row of directory.data.list.filter(
        (row: { username: string }) =>
          /^platform_[01]_[a-f0-9]{24}$/.test(row.username),
      )) {
        const removed = await (
          await adminPage.request.delete(`${config.apiBaseURL}user/${row.id}`, {
            headers: staff,
          })
        ).json();
        expect(removed.code).toBe(0);
      }
      for (const [index, number] of [customer, other].entries()) {
        const created = await (
          await adminPage.request.post(`${config.apiBaseURL}user`, {
            headers: staff,
            data: {
              username: `platform_${index}_${stamp}`,
              nickname: "Platform acceptance",
              password,
              status: 1,
            },
          })
        ).json();
        expect(
          created.code,
          JSON.stringify({
            message: created.message,
            errorCode: created.errorCode,
          }),
        ).toBe(0);
        const id = created.data.id;
        expect(id).toBeGreaterThan(0);
        users.push(id);
        execPgSQLStatements([
          `INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name,search_name) VALUES('${number}','individual','active','Platform acceptance','Platform acceptance');`,
          `INSERT INTO plugin_idc_core_customer_identity_binding(binding_no,sys_user_id,customer_no,source_code) VALUES('B${index}_${stamp}',${id},'${number}','migration');`,
          `INSERT INTO plugin_idc_core_principal_audience(sys_user_id,audience_code,status_code) VALUES(${id},'customer','active');`,
        ]);
      }
      const login = async (index: number) => {
        const result = await (
          await adminPage.request.post(`${config.apiBaseURL}auth/login`, {
            data: {
              username: `platform_${index}_${stamp}`,
              password,
              clientType: "web",
            },
          })
        ).json();
        expect(result.code).toBe(0);
        expect(result.data.accessToken).toBeTruthy();
        return { Authorization: `Bearer ${result.data.accessToken}` };
      };
      const own = await login(0),
        otherHeaders = await login(1);
      const read = async (path: string, headers: Record<string, string>) =>
        await (
          await adminPage.request.get(`${api}${path}`, { headers })
        ).json();
      const unbound = await read("/account/open-platform", own);
      expect(
        unbound.code,
        JSON.stringify({
          code: unbound.code,
          message: unbound.message,
          errorCode: unbound.errorCode,
        }),
      ).toBe(0);
      expect(unbound.data.authorized).toBe(false);
      const grant = await (
        await adminPage.request.post(`${api}/admin/channel/tokens`, {
          headers: staff,
          data: {
            customerNo: customer,
            displayName: "Platform acceptance",
            scopes: ["capability.read", "catalog.read"],
            sourceCidrs: ["127.0.0.0/8"],
            expiresAt: Date.now() + 3600000,
          },
        })
      ).json();
      expect(grant.code).toBe(0);
      appId = grant.data.appId;
      const overview = await read("/account/open-platform", own);
      expect(overview.code).toBe(0);
      expect(overview.data.appId).toBe(appId);
      expect(overview.data.canReset).toBe(true);
      expect(JSON.stringify(overview.data)).not.toContain(grant.data.token);
      expect(
        (await read("/account/open-platform", otherHeaders)).data.authorized,
      ).toBe(false);
      expect(
        (
          await read(
            "/account/open-platform/calls?appId=" + appId,
            otherHeaders,
          )
        ).code,
      ).not.toBe(0);
      const basic = (secret: string) => ({
        Authorization: `Basic ${Buffer.from(`${appId}:${secret}`).toString("base64")}`,
      });
      expect(
        (await read("/channel/capabilities", basic(grant.data.token))).code,
      ).toBe(0);
      expect(
        (await read("/channel/finance", basic(grant.data.token))).code,
      ).not.toBe(0);
      const resetResponse = await adminPage.request.post(
        `${api}/account/open-platform/secrets`,
        {
          headers: own,
          data: {
            tokenId: overview.data.tokenId,
            scopes: ["*"],
            expiresAt: Date.now() + 999999999,
          },
        },
      );
      expect(resetResponse.headers()["cache-control"]).toContain("no-store");
      const reset = await resetResponse.json();
      expect(reset.code).toBe(0);
      expect(reset.data.appId).toBe(appId);
      expect(reset.data.secret).not.toBe(grant.data.token);
      const after = await read("/account/open-platform", own);
      expect(after.data.scopes).toEqual(overview.data.scopes);
      expect(after.data.sourceCidrs).toEqual(overview.data.sourceCidrs);
      expect(after.data.expiresAt).toBe(overview.data.expiresAt);
      expect(
        (await read("/channel/capabilities", basic(grant.data.token))).code,
      ).not.toBe(0);
      expect(
        (await read("/channel/capabilities", basic(reset.data.secret))).code,
      ).toBe(0);
      expect(
        (
          await (
            await adminPage.request.post(
              `${api}/account/open-platform/secrets`,
              { headers: own, data: { tokenId: overview.data.tokenId } },
            )
          ).json()
        ).code,
      ).not.toBe(0);
      for (let i = 0; i < 8; i++)
        expect(
          (await read("/channel/capabilities", basic(reset.data.secret))).code,
        ).toBe(0);
      const calls = await read(
        "/account/open-platform/calls?page=2&pageSize=10",
        own,
      );
      expect(calls.code).toBe(0);
      expect(calls.data.total).toBe(12);
      expect(calls.data.list).toHaveLength(2);
      const first = await read("/account/open-platform/calls", own);
      expect(
        first.data.list.some((row: { succeeded: boolean }) => !row.succeeded),
      ).toBe(true);
      expect(JSON.stringify(first.data)).not.toContain(reset.data.secret);
      expect(JSON.stringify(first.data)).not.toContain("Authorization");
      // Real customer browser uses the same dedicated user and backend, without API mocking.
      const context = await browser.newContext({
        baseURL: process.env.E2E_CUSTOMER_BASE_URL || "http://127.0.0.1:6032",
      });
      try {
        const page = await context.newPage();
        await page.goto("/auth/login");
        await page.locator("input").nth(0).fill(`platform_0_${stamp}`);
        await page.locator("input").nth(1).fill(password);
        await page.getByRole("button", { name: "login", exact: true }).click();
        await page.waitForURL(/\/account/);
        await expect(
          page.getByRole("menuitem", { name: "开放平台", exact: true }),
        ).toBeVisible();
        await page
          .getByRole("menuitem", { name: "开放平台", exact: true })
          .click();
        await expect(page.getByTestId("platform-app-id")).toHaveText(appId);
        await page.getByRole("tab", { name: "调用记录", exact: true }).click();
        await expect(page.locator(".vxe-body--row")).toHaveCount(10);
        await expect(page.getByTestId("open-platform-page")).toContainText(
          "12",
        );
        await mkdir("../../temp/20261012/open-platform", { recursive: true });
        await expect(
          page.getByText("无法访问该客户资料", { exact: true }).first(),
        ).not.toBeVisible({ timeout: 15000 });
        await page.screenshot({
          path: `../../temp/20261012/open-platform/${Date.now()}-real-customer-platform.png`,
          fullPage: false,
        });
      } finally {
        await context.close();
      }
      const paused = await (
        await adminPage.request.post(
          `${api}/admin/channel/accounts/${appId}/status`,
          { headers: staff, data: { action: "pause" } },
        )
      ).json();
      expect(paused.code).toBe(0);
      const disabled = await read("/account/open-platform", own);
      expect(disabled.data.authorized).toBe(true);
      expect(disabled.data.canReset).toBe(false);
      expect(
        (
          await (
            await adminPage.request.post(
              `${api}/account/open-platform/secrets`,
              { headers: own, data: { tokenId: after.data.tokenId } },
            )
          ).json()
        ).code,
      ).not.toBe(0);
      expect((await read("/account/open-platform/calls", own)).data.total).toBe(
        12,
      );
    } finally {
      if (appId)
        execPgSQLStatements([
          `DELETE FROM plugin_idc_channel_api_call WHERE channel_id='${appId}';`,
          `DELETE FROM plugin_idc_channel_token_rate_window WHERE token_id IN(SELECT id FROM plugin_idc_channel_token WHERE channel_id='${appId}');`,
          `DELETE FROM plugin_idc_channel_token WHERE channel_id='${appId}';`,
          `DELETE FROM plugin_idc_channel_account WHERE channel_id='${appId}';`,
        ]);
      execPgSQLStatements([
        ...(users.length
          ? [
              `DELETE FROM plugin_idc_core_principal_audience WHERE sys_user_id IN(${users.join(",")});`,
            ]
          : []),
        `DELETE FROM plugin_idc_core_customer_identity_binding WHERE customer_no IN('${customer}','${other}');`,
        `DELETE FROM plugin_idc_core_customer_subject WHERE customer_no IN('${customer}','${other}');`,
      ]);
      for (const id of users) {
        const deleted = await (
          await adminPage.request.delete(`${config.apiBaseURL}user/${id}`, {
            headers: { Authorization: auth },
          })
        ).json();
        expect(deleted.code).toBe(0);
      }
    }
  });
});

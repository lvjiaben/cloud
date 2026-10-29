import { test, expect } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-8 认证信息保护", () => {
  test("TC-8a 未单独同意不能提交，401 不重放敏感信息", async ({
    consolePage: page,
  }) => {
    let submissions = 0;
    await page.route("**/account/compliance/personal-verification", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: { policyVersion: "policy-test-v1", item: null },
        },
      }),
    );
    await page.route(
      "**/account/compliance/personal-verifications/current",
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: { policyVersion: "policy-test-v1", item: null },
          },
        }),
    );
    await page.route(
      "**/account/compliance/personal-verification-consents",
      (r) => {
        expect(r.request().postDataJSON()).toMatchObject({
          policyVersion: "policy-test-v1",
          locale: "zh-CN",
        });
        return r.fulfill({
          json: { code: 0, data: { consentNo: "CONSENT-test" } },
        });
      },
    );
    await page.route("**/account/compliance/personal-verifications", (r) => {
      if (r.request().method() === "GET")
        return r.fulfill({
          json: {
            code: 0,
            data: { policyVersion: "policy-test-v1", item: null },
          },
        });
      submissions++;
      expect(r.request().postDataJSON()).toMatchObject({
        consentNo: "CONSENT-test",
        certName: "测试姓名",
      });
      return r.fulfill({ status: 401, json: { code: 401 } });
    });
    const view = new CustomerPage(page);
    await view.open("/account/verification");
    await page.getByLabel("本人姓名").fill("测试姓名");
    await page.getByLabel("身份证号码").fill("110101199001010000");
    await expect(view.button("同意并开始认证")).toBeDisabled();
    await page
      .getByText("我已阅读并单独同意上述处理，确认提交本人信息", {
        exact: true,
      })
      .click();
    await view.screenshot("identity-consent");
    await view.button("同意并开始认证").click();
    await expect(page).toHaveURL(/auth\/login/);
    expect(submissions).toBe(1);
    expect(
      await page.evaluate(() =>
        JSON.stringify({ ...sessionStorage, ...localStorage }),
      ),
    ).not.toContain("110101199001010000");
  });
});

// This case locks the customer-facing personal verification journey. Browser
// return and reload never count as verification; only the authoritative query
// may advance the state.
import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { config } from "../../../../../../hack/tests/fixtures/config";

import {
  installPersonalVerificationAccountMock,
  personalVerificationAccountBase,
} from "../support/personal-verification-account-mock";

const validIdentity = {
  certName: "测试用户",
  certNo: "11010519491231002X",
  certType: "IDENTITY_CARD",
  consentNo: "consent_20260812_0001",
};

async function openAccountOrigin(page: Parameters<typeof installPersonalVerificationAccountMock>[0]) {
  await page.goto(config.publicBaseURL, { waitUntil: "domcontentloaded" });
}

test.describe("TC-2 个人实名认证", () => {
  test("TC-2a: 发起只提交实名资料，不允许浏览器指定客户、租户或回跳地址", async ({
    page,
  }) => {
    const state = await installPersonalVerificationAccountMock(page);
    await openAccountOrigin(page);

    const started = await page.evaluate(
      async ({ base, input }) =>
        fetch(`${base}/personal-verifications`, {
          body: JSON.stringify(input),
          headers: { "Content-Type": "application/json" },
          method: "POST",
        }).then((response) => response.json()),
      { base: personalVerificationAccountBase, input: validIdentity },
    );

    expect(started.data.item.statusCode).toBe("verifying");
    expect(started.data.item.continuationUrl).toContain("antcloudauth.aliyuncs.com");
    expect(state.startRequests).toEqual([validIdentity]);
    expect(state.startRequests[0]).not.toHaveProperty("customerNo");
    expect(state.startRequests[0]).not.toHaveProperty("tenantId");
    expect(state.startRequests[0]).not.toHaveProperty("returnUrl");
  });

  test("TC-2b: 只有服务端主动查询到供应商通过，实名状态才变为已通过", async ({
    page,
  }) => {
    const state = await installPersonalVerificationAccountMock(page);
    await openAccountOrigin(page);

    const result = await page.evaluate(
      async ({ base, input }) => {
        const started = await fetch(`${base}/personal-verifications`, {
          body: JSON.stringify(input),
          headers: { "Content-Type": "application/json" },
          method: "POST",
        }).then((response) => response.json());
        const queried = await fetch(
          `${base}/personal-verifications/${started.data.item.attemptNo}/authoritative-state`,
          { method: "PUT" },
        ).then((response) => response.json());
        return { queried, started };
      },
      { base: personalVerificationAccountBase, input: validIdentity },
    );

    expect(result.started.data.item.statusCode).toBe("verifying");
    expect(result.queried.data.item.statusCode).toBe("verified");
    expect(result.queried.data.item.reasonCode).toBe("provider_passed");
    expect(state.refreshRequests).toEqual([
      { attemptNo: "pva_20260812_0001", body: null },
    ]);
  });

  test("TC-2c: 中途退出后恢复原尝试，再次查询而不是重复发起", async ({
    page,
  }) => {
    const state = await installPersonalVerificationAccountMock(page);
    await openAccountOrigin(page);

    await page.evaluate(
      async ({ base, input }) => {
        await fetch(`${base}/personal-verifications`, {
          body: JSON.stringify(input),
          headers: { "Content-Type": "application/json" },
          method: "POST",
        });
      },
      { base: personalVerificationAccountBase, input: validIdentity },
    );
    await page.reload({ waitUntil: "domcontentloaded" });
    const recovered = await page.evaluate(async (base) => {
      const current = await fetch(`${base}/personal-verification`).then((response) =>
        response.json(),
      );
      const queried = await fetch(
        `${base}/personal-verifications/${current.data.item.attemptNo}/authoritative-state`,
        { method: "PUT" },
      ).then((response) => response.json());
      return { current, queried };
    }, personalVerificationAccountBase);

    expect(recovered.current.data.item.statusCode).toBe("verifying");
    expect(recovered.queried.data.item.statusCode).toBe("verified");
    expect(state.startRequests).toHaveLength(1);
    expect(state.refreshRequests).toHaveLength(1);
  });

  test("TC-2d: 未满年龄或不支持的证件在本地拒绝，不创建认证尝试", async ({
    page,
  }) => {
    const state = await installPersonalVerificationAccountMock(page);
    await openAccountOrigin(page);

    const statuses = await page.evaluate(async ({ base, input }) => {
      const request = async (input: Record<string, unknown>) =>
        fetch(`${base}/personal-verifications`, {
          body: JSON.stringify(input),
          headers: { "Content-Type": "application/json" },
          method: "POST",
        }).then((response) => response.status);
      return Promise.all([
        request({
          ...input,
          certNo: "110105201801010010",
        }),
        request({
          ...input,
          certNo: "P12345678",
          certType: "PASSPORT",
        }),
      ]);
    }, { base: personalVerificationAccountBase, input: validIdentity });
    const current = await page.evaluate(async (base) =>
      fetch(`${base}/personal-verification`).then((response) => response.json()),
    personalVerificationAccountBase);

    expect(statuses).toEqual([400, 400]);
    expect(state.unsupportedRequests).toHaveLength(2);
    expect(state.refreshRequests).toHaveLength(0);
    expect(current.data.item.statusCode).toBe("unverified");
  });
});

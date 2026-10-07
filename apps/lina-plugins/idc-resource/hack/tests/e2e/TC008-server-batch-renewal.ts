import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ServerBatchRenewalPage } from "../pages/ServerBatchRenewalPage";
import { installServerBatchRenewalMock } from "../support/server-batch-renewal-mock";

test.describe("TC-8 服务器批量续费", () => {
  test("TC-8a: 只能从权益列表勾选并用统一周期逐项预览", async ({
    adminPage,
  }) => {
    const state = await installServerBatchRenewalMock(adminPage);
    const renewals = new ServerBatchRenewalPage(adminPage);
    await renewals.goto();
    await renewals.select("SVR-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA");
    await renewals.select("SVR-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB");
    await renewals.root.getByRole("spinbutton").fill("6");
    await renewals.preview();
    const dialog = adminPage.getByRole("dialog", { name: "确认服务器续费" });
    await expect(dialog).toContainText(/[¥￥]1,299\.00/);
    await expect(dialog).toContainText("暂时无法获取续费报价");
    expect(state.previews[0]).toMatchObject({
      renewalCycles: 6,
      serverNos: [
        "SVR-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
        "SVR-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
      ],
    });
    expect(JSON.stringify(state.previews[0])).not.toMatch(
      /entitlementNo|customerNo/,
    );
    await expect(renewals.root.getByRole("textbox")).toHaveCount(0);
  });

  test("TC-8b: 二次确认后逐项显示成功和失败", async ({ adminPage }) => {
    const state = await installServerBatchRenewalMock(adminPage);
    const renewals = new ServerBatchRenewalPage(adminPage);
    await renewals.goto();
    await renewals.select("SVR-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA");
    await renewals.select("SVR-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB");
    await renewals.preview();
    const dialog = adminPage.getByRole("dialog", { name: "确认服务器续费" });
    await dialog.getByRole("checkbox").check();
    await dialog.getByRole("button", { name: /确\s*认|确\s*定/ }).click();
    const result = adminPage.getByRole("dialog", { name: "服务器续费结果" });
    await expect(result).toContainText("已提交续费");
    await expect(result).toContainText("本项续费提交失败");
    expect(state.confirmations).toHaveLength(1);
    expect(state.confirmations[0]).toMatchObject({
      confirmationDigest: "a".repeat(64),
      renewalCycles: 1,
    });
  });
});

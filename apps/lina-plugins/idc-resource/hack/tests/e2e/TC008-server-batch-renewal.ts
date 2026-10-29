import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ServerBatchRenewalPage } from "../pages/ServerBatchRenewalPage";
import { installServerBatchRenewalMock } from "../support/server-batch-renewal-mock";
import {mkdir} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {resolve} from 'node:path';

test.describe("TC-8 服务器批量续费", () => {
  for(const [suffix,currency,expected] of [['c','CNY','9,007,199,254,740,993.01'],['d','JPY','900,719,925,474,099,301'],['e','KWD','900,719,925,474,099.301']] as const){
    test(`TC-8${suffix}: 续费预览显示精确${currency}金额且不确认续费`,async({adminPage})=>{
      const state=await installServerBatchRenewalMock(adminPage,{currencyCode:currency,amountMinor:'900719925474099301'});
      const renewals=new ServerBatchRenewalPage(adminPage);await renewals.goto();
      await renewals.select('SVR-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA');await renewals.preview();
      const dialog=adminPage.getByRole('dialog',{name:'确认服务器续费'});
      await expect(dialog).toContainText(expected);
      const amountCell=dialog.getByRole('cell').filter({hasText:expected}).first().locator('.vxe-cell');
      await expect.poll(()=>amountCell.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
      expect(state.confirmations).toHaveLength(0);
      const now=new Date().toISOString();const folder=fileURLToPath(new URL(`../../../../../../temp/${now.slice(0,10).replaceAll('-','')}/`,import.meta.url));await mkdir(folder,{recursive:true});
      await adminPage.screenshot({path:resolve(folder,`${now.slice(11,23).replace(/[:.]/g,'')}-exact-renewal-amount-${currency}.png`),animations:'disabled'});
    });
  }
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
    // The official pager has a page-number textbox. Server selection itself
    // must still come from the list rather than a typed server identifier.
    await expect(renewals.root.locator(".ant-input")).toHaveCount(0);
    await expect(renewals.root.locator(".vxe-pager").getByRole("textbox")).toHaveCount(1);
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

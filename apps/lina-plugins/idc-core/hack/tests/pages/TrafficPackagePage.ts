import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import type { Page } from "@host-tests/support/playwright";

export class TrafficPackagePage {
  constructor(readonly page: Page) {}
  get root() { return this.page.getByTestId("traffic-package-management"); }
  get editor() { return this.page.getByRole("dialog").filter({ has: this.page.getByRole("textbox", { name: "中文名称" }) }); }
  get rows() { return this.root.locator(".vxe-table--main-wrapper .vxe-body--row"); }
  get pager() { return this.root.locator(".vxe-pager"); }
  async open() {
    await this.page.goto("/admin/system/configuration-center", { waitUntil: "domcontentloaded" });
    await this.page.locator("article").filter({ hasText: "流量套餐" }).getByRole("button", { name: "进入配置" }).click();
    await this.root.waitFor();
  }
  async create() { await this.root.getByRole("button", { name: "新增套餐", exact: true }).click(); await this.editor.getByRole("textbox", { name: "中文名称" }).waitFor(); }
  async edit(name: string) { await this.rows.filter({ has: this.page.getByText(name, { exact: true }) }).getByRole("button", { name: "编辑套餐" }).click(); await this.editor.getByRole("textbox", { name: "中文名称" }).waitFor(); }
  async setTotal(quantity: number) {
    await this.editor.getByText("限量", { exact: true }).click();
    await this.editor.getByRole("spinbutton", { name: "总可售份数" }).fill(String(quantity));
  }
  async fillNew(name: string) {
    await this.editor.getByRole("textbox", { name: "中文名称" }).fill(name);
    await this.editor.getByRole("textbox", { name: "英文名称" }).fill("Test 100 GB package");
    await this.editor.getByRole("textbox", { name: "最终售价（含税）" }).fill("12.50");
    await this.setTotal(100);
    await this.editor.getByRole("combobox", { name: "适用服务器商品" }).click();
    await this.page.locator(".ant-select-dropdown:visible").getByText("回归云服务器", { exact: true }).click();
    await this.editor.getByRole("textbox", { name: "中文名称" }).click();
    await this.editor.getByRole("switch", { name: "上架销售" }).click();
  }
  async confirm() { await this.editor.getByRole("button", { name: /^确\s*认$/u }).click(); }
  async cancel() { await this.editor.getByRole("button", { name: /^取\s*消$/u }).click(); }
  async search(keyword: string) {
    await this.root.getByRole("textbox", { name: "套餐名称" }).fill(keyword);
    await this.root.getByRole("button", { name: /^搜\s*索$/u }).click();
  }
  async screenshot(label: string) {
    const stamp = new Date().toISOString();
    const directory = resolve("../../temp", stamp.slice(0, 10).replaceAll("-", ""));
    await mkdir(directory, { recursive: true });
    // Finish only actual animations; never replace data/permission readiness.
    await this.page.evaluate(async () => { await Promise.all(document.getAnimations().filter(a => a.effect?.getTiming().iterations !== Infinity).map(a => a.finished.catch(() => undefined))); });
    const path = resolve(directory, `${stamp.slice(11, 23).replace(/[:.]/gu, "")}-traffic-${label}.png`);
    await this.page.screenshot({ path });
    return path;
  }
}

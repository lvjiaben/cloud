import { mkdir } from "node:fs/promises";
import path from "node:path";

import type { Page } from "@host-tests/support/playwright";

import { test } from "../../../../../../hack/tests/fixtures/auth";

import { MigrationMenuPage } from "../pages/MigrationMenuPage";

async function captureEvidence(page: Page, name: string) {
  const now = new Date();
  const stamp = now
    .toLocaleString("sv-SE", { hour12: false, timeZone: "Asia/Shanghai" })
    .replaceAll("-", "")
    .replaceAll(":", "")
    .replace(" ", "-");
  const day = now
    .toLocaleDateString("sv-SE", { timeZone: "Asia/Shanghai" })
    .replaceAll("-", "");
  const dir = path.resolve(process.cwd(), "..", "..", "temp", day);
  await mkdir(dir, { recursive: true });
  await page.screenshot({
    fullPage: false,
    path: path.join(dir, `${stamp}-${name}.png`),
  });
}

test.describe("TC-2 迁移插件菜单双语", () => {
  test("TC-2a/2b: 中文和英文界面显示对应迁移菜单", async ({ adminPage }) => {
    const menu = new MigrationMenuPage(adminPage);

    await menu.expectChineseMenu();
    await captureEvidence(adminPage, "migration-menu-zh");

    await menu.switchToEnglish();
    await menu.expectEnglishMenu();
    await captureEvidence(adminPage, "migration-menu-en");
  });
});

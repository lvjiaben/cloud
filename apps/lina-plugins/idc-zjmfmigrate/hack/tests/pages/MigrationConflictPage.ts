import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import type { Page } from "@host-tests/support/playwright";
import { MigrationStandalonePage } from "./MigrationStandalonePage";

export class MigrationConflictPage extends MigrationStandalonePage {
  constructor(page: Page) { super(page, '[data-surface-path="/plugins/idc-zjmfmigrate/migration-checks"] [data-testid="plugin-sandbox-frame"]'); }
  async goto() {
    await this.page.goto("/admin/system/configuration-center",{waitUntil:"domcontentloaded"});
    await this.page.getByRole("tab",{name:"数据迁移",exact:true}).click();
    await this.page.locator("article.configuration-item").filter({has:this.page.getByText("迁移检查",{exact:true})}).getByRole("button").click();
    await this.createBatch();
  }
  async screenshot(name:string) {
    const now=new Date();
    const directory=resolve(process.cwd(),"../../temp",now.toISOString().slice(0,10).replaceAll("-",""));
    await mkdir(directory,{recursive:true});
    await this.page.screenshot({fullPage:false,path:resolve(directory,`${now.toISOString().slice(11,19).replaceAll(":","")}-${name}.png`)});
  }
}

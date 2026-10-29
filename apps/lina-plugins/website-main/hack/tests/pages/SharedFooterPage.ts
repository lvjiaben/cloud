import type {Page} from '@playwright/test';
export class SharedFooterPage{
 constructor(readonly page:Page){}
 async open(path:string){const response=await this.page.goto(path);await this.page.waitForLoadState('networkidle');return response;}
 async footer(){await this.page.locator('.sky-footer-container').scrollIntoViewIfNeeded();}
 async hoverHero(){await this.page.locator('.pagebanner-button a').first().hover();}
 screenshot(path:string){return this.page.screenshot({path});}
}

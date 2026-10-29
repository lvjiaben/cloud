import type {Page} from '@playwright/test';

/** Shared interactions for the seven explicit solution product slots. */
export class SolutionProductsPage {
 constructor(readonly page:Page){}
 get slot(){return this.page.locator('.solution-products');}
 get names(){return this.slot.locator('h3');}
 async open(path:string){await this.page.goto(path);await this.slot.waitFor();}
 async show(){await this.slot.scrollIntoViewIfNeeded();}
 async retry(){await this.slot.getByRole('button',{name:'重新加载'}).click();}
 async english(){await this.page.evaluate(()=>scrollTo(0,0));const button=this.page.locator('[data-public-locale-switch]');if(!await button.isVisible())await this.page.locator('.lina-menu-toggle').click();await button.click();}
 async mobile(){await this.page.setViewportSize({width:390,height:844});}
 async screenshot(path:string){await this.show();await this.page.screenshot({path});}
}

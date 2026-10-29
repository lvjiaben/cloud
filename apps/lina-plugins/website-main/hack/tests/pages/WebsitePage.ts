import {Page} from '@playwright/test';
import {resolve} from 'node:path';
export class WebsitePage {
 constructor(readonly page:Page){}
 async open(path:string){await this.page.goto(path);}
 async capture(name:string,fullPage=true){const path=resolve(import.meta.dirname,'../../../../../../temp',new Date().toISOString().slice(0,10).replaceAll('-',''),Date.now()+'-'+name.replace(/[<>:"/\\|?*\x00-\x1f]/g,'-')+'.png');await this.page.locator('.bg-overlay-content').filter({visible:true}).waitFor({state:'hidden'});await this.page.locator('.ant-spin-spinning').waitFor({state:'hidden'});await this.page.screenshot({path,fullPage,animations:'disabled'});return path;}
 async setViewport(width:number,height:number){await this.page.setViewportSize({width,height});}
 async layout(){return this.page.evaluate(()=>({width:innerWidth,scrollWidth:document.documentElement.scrollWidth,status:document.querySelector('[data-status]')?.getBoundingClientRect().toJSON()}));}
 async chooseTerm(){await this.page.locator('[data-quote] button').first().click();}
 async selectTerm(index:number){await this.page.locator('[data-quote] button').nth(index).click();}
 async submit(){await this.page.locator('[data-add]').click();}
 async searchProducts(value:string){await this.page.locator('#search-input').fill(value);await this.page.locator('#search-input').press('Enter');}
 async searchNavigation(value:string){await this.page.locator('.horizontal-menu-navbar-search .search-input').click();await this.page.getByRole('textbox',{name:'搜索商品',exact:true}).fill(value);await this.page.locator('.lina-nav-search button').click();}
 async toggleMenu(){await this.page.locator('.lina-menu-toggle').click();}
 async download(){await this.page.getByRole('button',{name:/^下载 /}).click();}
 async retryBrand(){await this.page.locator('[data-brand-retry]').click();}
 async searchContent(value:string){await this.page.locator('[data-content-search] input').fill(value);await this.page.locator('[data-content-search] button').click();}
 async setPurchaseCount(name:'quantity'|'cycles',value:string){await this.page.locator('[name='+name+']').fill(value);}
 async refreshConfiguration(){await this.page.locator('[data-refresh]').click();}
 async openCustomerCart(){await this.page.locator('[data-customer="/account/cart"]').first().click();}
 async saveConfiguration(){await this.page.getByRole('button',{name:'保存配置并前往用户中心'}).click();}
 // Dispatch while the form is inert to exercise the handler's own busy guard.
 async repeatConfigurationSave(){await this.page.getByRole('button',{name:'保存配置并前往用户中心'}).dispatchEvent('click');}
}

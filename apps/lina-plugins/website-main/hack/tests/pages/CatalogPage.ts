import {WebsitePage} from './SourceWebsitePage';
export class CatalogPage extends WebsitePage {
 async toggleGroup(name:string,key?:string){const button=this.page.getByRole('button',{name:name+'子分类',exact:true});if(key)await button.press(key);else await button.click();}
 async buy(slug:string){await this.product(slug).getByRole('link',{name:'立即购买',exact:true}).click();}
 menu(){return this.page.locator('.sky-cart-menu-item h2 a,.sky-cart-menu-item h2 .lina-catalog-group-heading');}
 product(slug:string){return this.page.locator('[data-product="'+slug+'"]');}
 async revealPrice(slug:string){await this.product(slug).locator('.price').scrollIntoViewIfNeeded();}
 category(no:string){return this.page.locator('[data-catalog-category="'+no+'"]');}
 async selectCategory(no:string){await this.category(no).click();}
 async retryCatalog(){await this.page.getByRole('button',{name:'重试',exact:true}).click();}
 purchaseNotice(){return this.page.locator('[data-catalog-purchase-notice]');}
 usageNotice(){return this.page.locator('[data-catalog-usage-notice]');}
 async reloadCatalog(){await this.page.reload();}
 async captureCatalog(name:string){const path=await this.capture(name,false);console.log('TC066 screenshot: '+path);}
}

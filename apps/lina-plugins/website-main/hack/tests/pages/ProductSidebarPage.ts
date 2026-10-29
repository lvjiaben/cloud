import {expect,type Locator,type Page} from '@playwright/test';

export class ProductSidebarPage{
 constructor(readonly page:Page){}
 async openConfiguration(slug:string){await this.page.goto('/configure.html?product='+encodeURIComponent(slug));}
 group(categoryNo:string):Locator{return this.page.locator('#server-group-'+categoryNo);}
 async expandCategory(name:string){await this.page.getByRole('button',{name:name+'子分类'}).click();}
 async expectServer(categoryNo:string,slug:string,name:string){const link=this.group(categoryNo).locator('[data-catalog-product="'+slug+'"]');await expect(link).toContainText(name);}
}

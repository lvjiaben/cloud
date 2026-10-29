import {Page} from '@playwright/test';
import {resolve} from 'node:path';
export class WebsitePage {
 constructor(readonly page:Page){}
 async open(path:string){return await this.page.goto(path);}
 async capture(name:string,fullPage=true){const path=resolve(import.meta.dirname,'../../../../../../temp',new Date().toISOString().slice(0,10).replaceAll('-',''),Date.now()+'-'+name+'.png');await this.page.locator('.bg-overlay-content').filter({visible:true}).waitFor({state:'hidden'});await this.page.locator('.ant-spin-spinning').waitFor({state:'hidden'});if(fullPage)await this.page.evaluate(()=>window.scrollTo(0,0));await this.page.screenshot({path,fullPage,animations:'disabled'});return path;}
 async setViewport(width:number,height:number){await this.page.setViewportSize({width,height});}
 async availableConfigurationPath(){return await this.page.locator('[data-products] a.buy-now-button[href^="/configure.html?product="]').first().getAttribute('href');}
 async inspectFullPage(){await this.page.evaluate(async()=>{for(let y=0;y<document.documentElement.scrollHeight;y+=innerHeight){window.scrollTo({top:y,behavior:'instant'});await new Promise<void>(resolve=>requestAnimationFrame(()=>resolve()));}window.scrollTo({top:0,left:0,behavior:'instant'});});}
 async layout(){return this.page.evaluate(()=>({width:innerWidth,scrollWidth:document.documentElement.scrollWidth,status:document.querySelector('[data-status]')?.getBoundingClientRect().toJSON()}));}
 async moreContent(){await this.page.locator('[data-more]').click();}
 async scrollArticleTable(){return await this.page.locator('[data-content-body] table').last().evaluate(el=>{el.scrollLeft=el.scrollWidth;return el.scrollLeft;});}
 async constrainArticle(width:number){await this.page.locator('[data-content-body]').evaluate((el,width)=>{el.style.width=width+'px';el.style.maxWidth=width+'px';},width);}
 async captureContent(path:string,y=0){await this.page.evaluate(y=>scrollTo({top:y,behavior:'instant'}),y);await this.page.screenshot({path,fullPage:false,animations:'disabled'});}
 async chooseTerm(){await this.page.locator('[data-quote] button').first().click();}
 async selectTerm(index:number){await this.page.locator('[data-quote] button').nth(index).click();}
 async submit(){await this.page.locator('[data-add]').click();}
 async repeatCartSubmit(){await this.page.locator('#addCartForm').dispatchEvent('submit');}
 async searchProducts(value:string){await this.page.locator('#search-input').fill(value);await this.page.locator('#search-input').press('Enter');}
 async searchNavigation(value:string){await this.page.locator('.horizontal-menu-navbar-search .search-input').click();await this.page.getByRole('textbox',{name:'搜索商品',exact:true}).fill(value);await this.page.locator('.lina-nav-search button').click();}
 async category(name:string){await this.page.locator('[data-categories]').getByRole('link',{name,exact:true}).click();}
 async toggleMenu(){await this.page.locator('.lina-menu-toggle').click();}
 async download(){await this.page.getByRole('button',{name:/^下载 /}).click();}
 async openArticleHeading(name:string){await this.page.locator('[data-content-directory]').getByRole('link',{name,exact:true}).click();}
 async articleTool(action:'copy'|'top'|'back'){await this.page.locator('[data-content-tool='+action+']').click();}
 async retryCategories(){await this.page.getByRole('button',{name:'重新加载商品分类',exact:true}).click();}
 async retryBrand(){await this.page.locator('[data-brand-retry]').click();}
 async retry(){await this.page.locator('[data-retry]').click();}
 async searchContent(value:string){await this.page.locator('[data-content-search] input').fill(value);await this.page.locator('[data-content-search] button').click();}
 async setPurchaseCount(name:'quantity'|'cycles',value:string){await this.page.locator('[name='+name+']').fill(value);}
 async refreshConfiguration(){await this.page.locator('.allocation-mian').evaluate(el=>{el.scrollTop=0;});const retry=this.page.locator('[data-refresh]');if(await retry.isVisible())await retry.click();else await this.page.locator('[data-models] a[aria-current=page]').click();}
 async openCustomerCart(){await this.page.locator('[data-customer="/account/cart"]').first().click();}
 async openCustomerConfiguration(){await this.page.locator('[data-customer-configure]').click();}
 async saveConfiguration(){await this.page.getByRole('button',{name:'保存配置并前往用户中心'}).click();}
 // Dispatch while the form is inert to exercise the handler's own busy guard.
 async repeatConfigurationSave(){await this.page.getByRole('button',{name:'保存配置并前往用户中心'}).dispatchEvent('click');}
}


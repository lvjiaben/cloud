import {WebsitePage} from './SourceWebsitePage';
export class ConfigurationControlsPage extends WebsitePage {
 field(token:string){return this.page.locator('#field-'+token);}
 options(label:string){return this.page.getByRole('group',{name:label+'选项',exact:true});}
 slider(label:string){return this.page.getByRole('slider',{name:label+'滑块',exact:true});}
 async option(label:string,name:string,key?:string){const button=this.options(label).getByRole('button',{name,exact:true});if(key){await button.focus();await button.press(key);}else await button.click();}
 async value(token:string,value:string){await this.field(token).fill(value);}
 async slide(label:string,key:string){await this.slider(label).focus();await this.slider(label).press(key);}
 async adjust(label:string,direction:'增加'|'减少'){await this.page.getByRole('button',{name:label+direction,exact:true}).click();}
 async term(index:number,key='Enter'){const button=this.page.locator('[data-quote] button').nth(index);await button.focus();await button.press(key);}
 models(){return this.page.locator('[data-page=configure] .lina-configure-model-list');}
 modelsToggle(){return this.page.locator('[data-page=configure] .lina-configure-model-toggle');}
 selectedPrice(){return this.page.locator('[data-total-amount]');}
 async retryModels(){await this.page.locator('.shopping-cart-side').getByRole('button',{name:'重新加载',exact:true}).click();}
 async captureControls(name:string,area:'fields'|'quote'='fields'){const feedback=this.page.locator('[data-status]');await this.page.locator(area==='fields'?'[data-fields]':await feedback.isVisible()?'[data-status]':'[data-quote]').scrollIntoViewIfNeeded();const path=await this.capture(name,false);console.log('TC037 screenshot: '+path);return path;}
}

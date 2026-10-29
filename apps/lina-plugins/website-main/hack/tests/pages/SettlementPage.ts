import {WebsitePage} from './SourceWebsitePage';
export class SettlementPage extends WebsitePage {
 async selectConfiguration(value:string){await this.page.locator('#field-spec').selectOption(value);}
 async scrollTo(fraction:number){await this.page.evaluate(f=>window.scrollTo(0,(document.documentElement.scrollHeight-innerHeight)*f),fraction);}
 async toggleDetails(){await this.page.locator('[data-breakdown-toggle]').click();}
 async geometry(){return this.page.locator('.allocation-footer').evaluate(el=>{const r=el.getBoundingClientRect(),side=document.querySelector('.shopping-cart-side .sky-cart-menu')!.getBoundingClientRect();return {bottom:r.bottom,height:r.height,left:r.left,right:r.right,sidebarRight:side.right,viewport:innerHeight,width:document.documentElement.clientWidth,position:getComputedStyle(el).position};});}
}

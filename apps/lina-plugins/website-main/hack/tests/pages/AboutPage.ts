import type {Page} from '@playwright/test';
import {collectAboutSource} from '../support/about-copy.mjs';
export class AboutPage{
 constructor(readonly page:Page){}
 async open(){const response=await this.page.goto('/about.html');await this.page.locator('[data-about] main').waitFor();await this.page.waitForLoadState('networkidle');return response;}
 copy(){return this.page.evaluate(collectAboutSource);}
 async section(id:string){await this.page.locator('#'+id).scrollIntoViewIfNeeded();}
 async anchor(index:number){await this.page.locator('.et-hero-tab a').nth(index).click();}
 async expand(index:number,mobile=false){const card=this.page.locator('.programme-content-item').nth(index);if(mobile)await card.locator('.programme-content-item-title').click();else await card.hover();await card.evaluate(async e=>{await Promise.all(e.getAnimations().map(a=>a.finished.catch(()=>{})));});}
 async screenshot(path:string){await this.page.screenshot({path,animations:'disabled'});}
 async injectLongCopy(){await this.page.locator('[data-about-introduction]').evaluate(e=>{e.textContent=Array.from({length:8},(_,i)=>'仅浏览器排版测试'+i+'：用于检查多段文字自然撑开，以及长英文 AutomaticLayoutCheckWithoutClippingOrHorizontalOverflow。').join('\n');for(let i=0;i<4;i++){const paragraph=e.cloneNode(true) as HTMLElement;paragraph.removeAttribute('data-about-introduction');e.parentElement!.append(paragraph);}});}
 retry(){return this.page.locator('[data-brand-retry]').click();}
 async initializeAgain(){return this.page.evaluate(async()=>{const {initializeAbout}=await import('/lina/about.js');const first=initializeAbout(document),root=document.querySelector('[data-about] main');return first===initializeAbout(document)&&root===document.querySelector('[data-about] main');});}
 async menu(){await this.page.waitForFunction(()=>[...document.querySelectorAll('.horizontal-menu-navbar-menu>li>a')].some(e=>e.textContent?.includes('了解我们'))||document.querySelector('[data-brand-retry]'),{},{timeout:40000});const parent=this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'了解我们'});if(!await parent.count())await this.retry();const toggle=this.page.locator('.lina-menu-toggle');if(await toggle.isVisible())await toggle.click();await parent.focus();await parent.press('Enter');}
 async enter(){await this.page.locator('.lina-open a').filter({has:this.page.getByText('公司简介',{exact:true})}).click();await this.page.locator('[data-about] main').waitFor();}
}

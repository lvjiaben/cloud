import type {Page} from '@playwright/test';
import {expect} from '@playwright/test';
export class WebmasterPage {
 constructor(readonly page:Page){}
 async open(){const response=await this.page.goto('/webmaster.html');await expect(this.page.locator('[data-brand-name]').first()).not.toHaveText('',{timeout:15000});return response;}
 async contact(){await this.page.locator('.wm-application-header a').click();}
 async checkCardHover(){const cards=this.page.locator('.wm-card');await expect(cards.nth(2)).toHaveClass(/is-active/);await cards.first().hover();await expect(cards.first()).toHaveClass(/is-active/);await expect(cards.nth(2)).not.toHaveClass(/is-active/);await this.page.locator('.wm-hero h1').hover();await expect(cards.nth(2)).toHaveClass(/is-active/);}
 async revealAll(){for(const section of await this.page.locator('.wm-section').all()){await section.scrollIntoViewIfNeeded();await expect(section).toHaveCSS('opacity','1');}await this.page.evaluate(()=>window.scrollTo(0,0));}
 async openNavigation(){await this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'生态合作'}).hover();}
 async switchLanguage(){await this.page.locator('[data-public-locale-switch]').click();}
 async screenshot(path:string){await this.revealAll();await this.page.screenshot({path,fullPage:true,animations:'disabled'});}
 async overflow(){return this.page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth);}
 async images(){return this.page.locator('[data-webmaster] img').evaluateAll(images=>images.every(image=>(image as HTMLImageElement).complete&&(image as HTMLImageElement).naturalWidth>0));}
}

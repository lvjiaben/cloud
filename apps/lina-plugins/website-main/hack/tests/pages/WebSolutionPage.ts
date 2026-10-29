import type {Page} from '@playwright/test';
export class WebSolutionPage {
 constructor(readonly page:Page){}
 async open(hash=''){await this.page.goto('/plan/web.html'+hash);await this.page.locator('[data-web-solution] .et-main').waitFor();await this.page.waitForLoadState('networkidle');}
 nav(id:string){return this.page.locator('.et-hero-tab a[href="#web-'+id+'"]');}
 async section(id:string){await this.nav(id).click();await this.page.waitForFunction(id=>Math.abs(document.getElementById('web-'+id)!.getBoundingClientRect().top-document.querySelector('[data-web-solution] .et-container')!.getBoundingClientRect().height)<3,id);}
 async language(){const button=this.page.locator('[data-public-locale-switch]');if(!await button.isVisible())await this.page.locator('.lina-menu-toggle').click();await button.click();}
 async hover(){await this.page.locator('.advantage-Content-list-item').first().hover();}
 async keyboardSection(id:string){await this.nav(id).focus();await this.nav(id).press('Enter');}
 async retry(){await this.page.locator('.web-retry').click();}
 async enterFromNavigation(){await this.page.goto('/');await this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'解决方案'}).hover();await this.page.locator('.lina-open .Submenu-item').filter({hasText:'网站解决方案'}).click();await this.page.waitForLoadState('networkidle');}
 async screenshot(path:string){await this.page.waitForFunction(()=>[...document.querySelectorAll('.web-reveal')].filter(e=>{const r=e.getBoundingClientRect();return r.top<innerHeight&&r.bottom>0;}).every(e=>getComputedStyle(e).opacity==='1'));await this.page.screenshot({path,fullPage:false});}
}

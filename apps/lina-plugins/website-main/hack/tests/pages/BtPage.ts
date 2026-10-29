import type {Page} from '@playwright/test';
import {collectBtCopy} from '../support/bt-copy.mjs';
export class BtPage{
 constructor(readonly page:Page){}
 async open(){const response=await this.page.goto('/bt.html');await this.ready();return response;}
 async ready(){await this.page.locator('[data-bt] main').waitFor();await this.page.waitForLoadState('networkidle');}
 copy(){return this.page.evaluate(collectBtCopy);}
 async section(id:string){await this.page.locator('.et-hero-tab a[href="#'+id+'"]').click();await this.page.waitForFunction(id=>{const el=document.getElementById(id)!,bar=document.querySelector('[data-bt] .et-container')!;return Math.abs(el.getBoundingClientRect().top-bar.getBoundingClientRect().height)<4||(scrollY+innerHeight>=document.documentElement.scrollHeight-2&&el.getBoundingClientRect().top<innerHeight);},id);}
 async menu(){const toggle=this.page.locator('.lina-menu-toggle');if(await toggle.isVisible())await toggle.click();const link=this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'支持与服务'});await link.focus();await link.press('Enter');await this.page.locator('.lina-open .lina-nav-panel').evaluate(async e=>{await Promise.all(e.getAnimations().map(a=>a.finished.catch(()=>{})));});}
 async enter(){await this.page.locator('.lina-open a').filter({has:this.page.getByText('宝塔面板',{exact:true})}).click();await this.ready();}
 screenshot(path:string){return this.page.screenshot({path});}
}

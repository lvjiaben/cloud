import {WebsitePage} from './SourceWebsitePage';

export class HeaderNavigationPage extends WebsitePage {
 group(name:string){return this.page.locator('.horizontal-menu-navbar-menu').getByRole('button',{name,exact:true});}
 link(name:string){return this.page.locator('.horizontal-menu-navbar-menu').getByRole('link',{name,exact:true});}
 async toggle(name:string,key?:string){if(key)await this.group(name).press(key);else await this.group(name).click();}
 async follow(name:string){await this.link(name).click();}
 async hitTarget(name:string){return this.link(name).evaluate(a=>{const r=a.getBoundingClientRect(),hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return {hit:a===hit||a.contains(hit),x:r.x,y:r.y,width:r.width,height:r.height,viewportWidth:innerWidth,viewportHeight:innerHeight};});}
}

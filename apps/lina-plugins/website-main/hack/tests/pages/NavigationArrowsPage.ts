import {type Page} from '@playwright/test';
export class NavigationArrowsPage{
 constructor(readonly page:Page){}
 arrows(){return this.page.locator('.horizontal-menu-navbar-menu>li>a.nav-arrow');}
 menu(name:string){return this.arrows().filter({hasText:name});}
 async open(path:string){
  await this.page.goto(path);await this.arrows().first().waitFor({state:'attached'});
  const toggle=this.page.locator('.lina-menu-toggle');if(await toggle.isVisible())await toggle.click();
 }
 async styles(){return this.arrows().evaluateAll(nodes=>nodes.map(e=>{const s=getComputedStyle(e,'::after');return{content:s.content,width:s.width,height:s.height,right:s.borderRightWidth,bottom:s.borderBottomWidth,transform:s.transform};}));}
 async toggle(name:string){await this.menu(name).focus();await this.menu(name).press('Enter');}
 async rotation(name:string){return this.menu(name).evaluate(e=>getComputedStyle(e,'::after').transform);}
 async close(){await this.page.keyboard.press('Escape');}
}

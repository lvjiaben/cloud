import {NavigationArrowsPage} from './NavigationArrowsPage';
export class NavigationTransparencyPage extends NavigationArrowsPage{
 async scroll(top:number){await this.page.evaluate(y=>window.scrollTo(0,y),top);}
 async open(path:string){
  await this.page.goto(path,{waitUntil:'domcontentloaded'});
  await this.arrows().first().waitFor({state:'attached'});
  await this.page.waitForFunction(()=>[...document.querySelectorAll<HTMLLinkElement>('link[rel="stylesheet"]')].every(link=>link.sheet));
  const toggle=this.page.locator('.lina-menu-toggle');if(await toggle.isVisible())await toggle.click();
 }
 async close(){await super.close();await this.page.locator('.lina-nav-panel[data-menu="生态合作"]').waitFor({state:'hidden'});}
}

import {Page} from '@playwright/test';

export class LocalePage {
 constructor(readonly page:Page){}
 async open(path:string){await this.page.goto(path);await this.page.bringToFront();await this.page.locator('[data-public-locale-switch]').waitFor({state:'attached'});}
 async toggle(){await this.page.locator('[data-public-locale-switch]').click();}
 async reload(){await this.page.reload();}
 async useMobileMenu(){await this.page.setViewportSize({width:390,height:844});await this.page.locator('.lina-menu-toggle').click();}
 async closeMobileMenu(){await this.page.locator('.lina-menu-toggle').click();}
 async overflow(){return this.page.locator('body').evaluate(el=>el.scrollWidth);}
 async termsLinksFit(){return this.page.locator('.terms-directory li').evaluateAll(items=>items.every(item=>{
  const edge=item.getBoundingClientRect().right;
  return [...item.querySelector('a')!.getClientRects()].every(rect=>rect.right<=edge+1);
 }));}
 async capture(path:string){await this.page.screenshot({path,animations:'disabled'});}
 async untranslated(selector:string){return this.page.locator(selector).evaluateAll(roots=>{
  const result:string[]=[];
  for(const root of roots){const walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);let node:Node|null;
   while((node=walker.nextNode())){const parent=node.parentElement;if(!parent||parent.closest('script,style,code,pre,textarea,[data-content-body],[data-about-company],.solution-products'))continue;
    const text=node.textContent?.trim()||'';if(/[\u3400-\u9fff]/.test(text))result.push(text);
   }
  }
  return [...new Set(result)];
 });}
}

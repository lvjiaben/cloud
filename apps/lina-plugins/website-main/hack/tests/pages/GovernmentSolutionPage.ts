import type {Page} from '@playwright/test';
export class GovernmentSolutionPage {
 constructor(readonly page:Page){}
 open(){return this.page.goto('/plan/government.html');}
 architecture(){return this.page.locator('.et-hero-tab a').nth(2).click();}
 reload(){return this.page.reload();}
 home(){return this.page.goto('/');}
 async enterIndustrial(){await this.page.locator('.lina-open .Submenu-title').filter({hasText:'工业解决方案'}).click();}
 async ready(){await this.page.locator('[data-government] main').waitFor();await this.page.waitForLoadState('networkidle');}
 async copy(){
  return this.page.evaluate(()=>{
   const names=['hero','anchors','advantages','problems','architecture','products','partners','more'];
   const root=document.querySelector('[data-government]')!;
   const roots=[root.querySelector('.globalBanner')!,root.querySelector('.et-container')!,...root.querySelectorAll('main.et-main>.et-slide')];
   const records:object[]=[];
   roots.forEach((element,index)=>{
    function visit(element:Element,path:string){
     [...element.childNodes].forEach((node,position)=>{
      if(node.nodeType===3&&node.textContent!.trim())records.push({section:names[index],order:records.length,parent:path,tag:element.tagName.toLowerCase(),headingLevel:/^H[1-6]$/.test(element.tagName)?Number(element.tagName[1]):null,text:node.textContent,interaction:index===1?'anchor-navigation':'visible'});
      if(node instanceof Element&&!['IMG','SCRIPT','STYLE'].includes(node.tagName))visit(node,path+'/'+node.tagName.toLowerCase()+(node.className?'.'+node.className.split(/\s+/).join('.'):'')+'['+position+']');
     });
    }
    visit(element,names[index]);
   });
   return records;
  });
 }
 async section(id:string){
  const anchor=this.page.locator('.et-hero-tab a[href="#government-'+id+'"]');
  await anchor.scrollIntoViewIfNeeded();await anchor.click();
  await this.page.waitForFunction(id=>{
   const section=document.getElementById('government-'+id)!,top=section.getBoundingClientRect().top;
   const height=document.querySelector('[data-government] .et-container')!.getBoundingClientRect().height;
   const atBottom=scrollY+innerHeight>=document.documentElement.scrollHeight-2;
   return (Math.abs(top-height)<4||atBottom&&top>=height-4&&top<innerHeight)&&location.hash==='#government-'+id;
  },id,{timeout:10000});
 }
 async menu(name:string){
  const link=this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:name});
  if(await this.page.locator('.lina-menu-toggle').isVisible())await this.page.locator('.lina-menu-toggle').click();
  await link.focus();await link.press('Enter');
  await this.page.locator('.lina-open .lina-nav-panel').evaluate(async panel=>{await Promise.all(panel.getAnimations().map(animation=>animation.finished.catch(()=>{})));});
 }
 async menuTab(name:string){await this.page.locator('.lina-open .lina-nav-category').filter({hasText:name}).click();}
 async escapeMenu(){await this.page.keyboard.press('Escape');const toggle=this.page.locator('.lina-menu-toggle');if(await toggle.isVisible()&&await toggle.getAttribute('aria-expanded')==='true')await toggle.click();}
 async retryBrand(){await this.page.locator('[data-brand-retry]').click();}
 async screenshot(path:string){await this.page.screenshot({path});}
}

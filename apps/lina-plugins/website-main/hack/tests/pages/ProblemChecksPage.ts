import type {Page} from '@playwright/test';

export class ProblemChecksPage {
 constructor(readonly page:Page){}
 async open(path:string){
  const response=await this.page.goto(path);
  await this.page.locator('.problem-left-ul li').first().waitFor();
  await this.page.waitForLoadState('networkidle');
  return response;
 }
 async checks(){
  return this.page.locator('.problem-left-ul li').evaluateAll(items=>items.map(item=>{
   const style=getComputedStyle(item,'::before');
   return {text:item.textContent?.trim(),content:style.content,image:style.backgroundImage,width:style.width,height:style.height,left:style.left,position:style.position,size:style.backgroundSize};
  }));
 }
 async screenshot(path:string){
  await this.page.locator('.problem-left-ul').scrollIntoViewIfNeeded();
  await this.page.locator('.problem-Content-left').screenshot({path,animations:'disabled'});
 }
}

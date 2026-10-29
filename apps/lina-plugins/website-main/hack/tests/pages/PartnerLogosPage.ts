import type {Page} from '@playwright/test';

export class PartnerLogosPage {
 constructor(readonly page:Page){}
 async open(path:string){
  const response=await this.page.goto(path);
  await this.page.locator('.partner-Content').waitFor();
  await this.page.waitForLoadState('networkidle');
  await this.page.locator('.partner-Content').scrollIntoViewIfNeeded();
  return response;
 }
 async logos(){
  return this.page.locator('.partner-Content-row img').evaluateAll(async images=>Promise.all(images.map(async image=>{
   const img=image as HTMLImageElement;await img.decode();const rect=img.getBoundingClientRect();
   return {src:img.src,name:new URL(img.src).pathname.split('/').pop(),loaded:img.naturalWidth>0&&img.naturalHeight>0,visible:rect.width>0&&rect.height>0&&rect.left>=0&&rect.right<=innerWidth+1};
  })));
 }
 async screenshot(path:string){await this.page.locator('.partner-Content').screenshot({path,animations:'disabled'});}
}

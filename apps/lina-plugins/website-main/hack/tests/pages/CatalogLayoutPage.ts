import {CatalogPage} from './CatalogPage';
export class CatalogLayoutPage extends CatalogPage{
 async geometry(){return this.page.evaluate(()=>{
  const main=document.querySelector('.shopping-cart-page')!,footer=document.querySelector('.sky-footer-container')!;
  const m=main.getBoundingClientRect(),f=footer.getBoundingClientRect();
  return {mainTop:m.top,mainBottom:m.bottom,footerTop:f.top,footerHeight:f.height,footerDisplay:getComputedStyle(footer).display,viewport:innerHeight,width:innerWidth,scrollWidth:document.documentElement.scrollWidth,background:getComputedStyle(document.querySelector('.headerBg')!).backgroundColor};
 });}
 async bottom(){await this.page.evaluate(()=>window.scrollTo(0,document.documentElement.scrollHeight));}
 async top(){await this.page.evaluate(()=>window.scrollTo(0,0));}
 async scrollProducts(){return this.page.locator('.shopping-cart-product').evaluate(e=>{e.scrollTop=e.scrollHeight;return {top:e.scrollTop,height:e.clientHeight,total:e.scrollHeight,page:scrollY};});}
 async screenshot(path:string){await this.page.screenshot({path});}
}

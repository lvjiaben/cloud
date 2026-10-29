import {NavigationPage} from './NavigationPage';

export class ProductMenuLayoutPage extends NavigationPage {
 async openProducts(url:string,width:number){
  await this.page.setViewportSize({width,height:1000});
  await this.open(url);
  if(await this.page.locator('.lina-menu-toggle').isVisible())await this.toggleMenu();
  await this.clickTitle('产品与服务');
 }
 products(){return this.panel().locator('.product_services .Submenu-middle-content .Submenu-item');}
 async panelGeometry(){
  return this.panel().evaluate(element=>{
   const box=element.getBoundingClientRect();
   return {height:box.height,bottom:box.bottom,scrollHeight:element.scrollHeight,clientHeight:element.clientHeight,
    contentBottom:element.querySelector('.lina-nav-all')!.getBoundingClientRect().bottom};
  });
 }
 async geometry(){
  return this.panel().locator('.product_services .Submenu-middle-content').evaluate(element=>{
   const box=element.getBoundingClientRect();
   const parent=element.parentElement!,style=getComputedStyle(parent);
   return {width:box.width,available:parent.clientWidth-parseFloat(style.paddingLeft)-parseFloat(style.paddingRight),
    items:Array.from(element.querySelectorAll('.Submenu-item')).map(item=>{
     const rect=item.getBoundingClientRect();return {x:rect.x,y:rect.y,right:rect.right,width:rect.width};
    }),viewport:innerWidth,scroll:document.documentElement.scrollWidth};
  });
 }
 async capture(path:string){await this.page.screenshot({path,fullPage:false});return path;}
}

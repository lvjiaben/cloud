import {HomePage} from './HomePage';
export class HomeDetailPage extends HomePage {
 async openProductEntry(kind:string){if(kind==='hero')await this.page.locator('.swiper-text:visible .Button').click();else if(kind==='shortcut')await this.clickShortcut(1);else if(kind==='catalog')await this.page.locator('[data-products] a[href]').first().click();else{await this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'产品与服务'}).hover();await this.page.locator('.lina-open .lina-nav-product-list a[href],.lina-open .Submenu-middle-right a[href*="product="]').first().click();}}
 async buyProductCard(){await this.page.locator('[data-products] .buy-now-button').click();}
 async leaveShortcut(){await this.page.mouse.move(0,0);}
 async constrainSolutionStrip(width:number|null){await this.page.locator('.main-option').evaluate((n:HTMLElement,width)=>{if(width===null){n.style.removeProperty('width');n.style.removeProperty('max-width');}else{n.style.width=width+'px';n.style.maxWidth='none';}},width);}
 async dragGlobe(){const box=await this.page.locator('#network_canvas').boundingBox();if(!box)throw new Error('Missing globe');const x=Math.min(box.x+box.width/2,1200),y=box.y+100;await this.page.mouse.move(x,y);await this.page.mouse.down();await this.page.mouse.move(x+45,y+20,{steps:8});await this.page.mouse.up();}
 async loseGlobeContext(){await this.page.locator('#network_canvas').evaluate((canvas:HTMLCanvasElement)=>{const gl=canvas.getContext('webgl2')||canvas.getContext('webgl');const extension=gl?.getExtension('WEBGL_lose_context');if(!extension)throw new Error('Context loss extension unavailable');extension.loseContext();});}
 async restoreCachedPage(){await this.page.evaluate(()=>{dispatchEvent(new PageTransitionEvent('pagehide',{persisted:true}));dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true}));});}
 async consultation(){await this.page.locator('[data-home-product-intro] .systemproduct-right-bottom a').click();}
 async firstArticle(){await this.page.locator('[data-content-list] a').first().click();}
 async suggestProduct(value:string){await this.page.locator('#search-input').fill(value);}
 async clearProductSearch(){await this.page.getByRole('button',{name:'清空商品搜索'}).click();}
 async catalogSuggestion(name:string){await this.page.locator('.lina-catalog-search-results').getByRole('link',{name,exact:true}).click();}
 async renderFooter(navigation:unknown[]|undefined){await this.page.evaluate(async navigation=>{const {renderBrandFooter}=await import('/lina/footer.js');const safe=(value:string)=>{try{const u=new URL(value,location.origin);return ['http:','https:'].includes(u.protocol)?u:null;}catch{return null;}};renderBrandFooter(document,{footerNavigation:navigation,templateConfig:{icpnumber:'模拟编号'}},safe);},navigation);}
 async hoverNews(index:number){await this.page.locator("[data-home-news] > a").nth(index).hover();}
 async hoverShortcut(index:number){await this.page.locator('[data-home-blocks] > .bannerbelow-list').nth(index).hover();}
 async clickShortcut(index:number){await this.page.locator('[data-home-blocks] > .bannerbelow-list').nth(index).click();}
 async solutionByKeyboard(index:number){await this.page.locator('.main-option-name').first().focus();for(let i=0;i<index;i++)await this.page.keyboard.press('Tab');await this.page.keyboard.press('Enter');}
 async captureSection(selector:string,path:string){await this.focusSection(selector);await this.page.screenshot({path,animations:'disabled'});}
 async focusSection(selector:string){await this.page.locator(selector).scrollIntoViewIfNeeded();}
 async hoverPartners(){await this.page.locator('.partners-item-container').first().hover();}
 async leavePartners(){await this.page.mouse.move(0,0);}
 async motion(value:'reduce'|'no-preference'){await this.page.emulateMedia({reducedMotion:value});}
 async tracks(){return this.page.locator('.partners-item-container:visible').evaluateAll(rows=>rows.map(row=>{
  const track=row.querySelector('.partners-item-row')!,first=track.firstElementChild!,repeat=track.children[1];
  return {viewport:row.clientWidth,width:first.getBoundingClientRect().width,copies:first.querySelectorAll('[data-partner-repeat]').length,
   names:[...first.querySelectorAll('.partners-item:not([data-partner-repeat])')].map(e=>e.querySelector('img')?.getAttribute('alt')||e.textContent?.trim()),
   duplicateHidden:repeat.getAttribute('aria-hidden')==='true'&&(repeat as HTMLElement).inert,
   transform:getComputedStyle(first).transform,animation:getComputedStyle(first).animationName};
 }));}
 async geometry(){return this.page.evaluate(()=>{const hero=document.querySelector('.Rotation-layoutcontainer')!,shortcut=document.querySelector('.bannerbelow-layoutcontainer')!;return {heroBottom:hero.getBoundingClientRect().bottom,shortcutTop:shortcut.getBoundingClientRect().top,margin:getComputedStyle(shortcut).marginTop};});}
}

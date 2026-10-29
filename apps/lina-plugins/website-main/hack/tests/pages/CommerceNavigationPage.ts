import {WebsitePage} from './SourceWebsitePage';
export class CommerceNavigationPage extends WebsitePage {
 server(slug:string){return this.page.locator('[data-catalog-product="'+slug+'"]');}
 async chooseServer(slug:string){await this.server(slug).click();}
 async moreServers(categoryNo:string){await this.page.locator('#server-group-'+categoryNo+' .lina-server-menu-more').click();}
 async toggleFees(){await this.page.locator('[data-breakdown-toggle]').click();}
 async revealTerms(){await this.page.locator('[data-quote]').scrollIntoViewIfNeeded();}
 async revealModels(){await this.page.locator('[data-models]').scrollIntoViewIfNeeded();}
 async chooseAlternativeModel(){await this.page.locator('.lina-configure-model-list a[href]:not([aria-current=page])').first().click();}
 async retryModels(){await this.page.locator('.lina-configure-model-more').click();}
 async termsLayout(){return this.page.locator('[data-quote]').evaluate(el=>({width:el.getBoundingClientRect().width,scrollWidth:el.scrollWidth,buttons:[...el.children].map(button=>({height:button.getBoundingClientRect().height,width:button.getBoundingClientRect().width}))}));}
 group(name:string){return this.page.getByRole('button',{name:name+'子分类',exact:true});}
 async toggle(name:string){await this.group(name).click();}
 async buy(){await this.page.locator('[data-products] .buy-now-button[href]').first().click();}
 async backToCatalog(){await this.page.locator('.nav-top-page .return').click();}
 async browserBack(){await this.page.goBack();}
 async browserForward(){await this.page.goForward();}
 async reload(){await this.page.reload();}
 async observeMotion(){await this.page.addInitScript(()=>{
  const original=Element.prototype.animate;(window as any).categoryMotions=[];
  Element.prototype.animate=function(...args:Parameters<Element['animate']>){
   const animation=original.apply(this,args);
   if(this.classList.contains('sky-cart-submenu'))(window as any).categoryMotions.push(animation.effect?.getTiming().duration);
   return animation;
  };
 });}
 async motionDurations(){return this.page.evaluate(()=>(window as any).categoryMotions);}
 async arrowDuration(name:string){return this.group(name).evaluate(e=>getComputedStyle(e,'::after').transitionDuration);}
 async reduceMotion(){await this.page.emulateMedia({reducedMotion:'reduce'});}
}

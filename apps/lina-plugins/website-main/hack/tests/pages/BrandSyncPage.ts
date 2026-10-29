import {NavigationPage} from './NavigationPage';
export class BrandSyncPage extends NavigationPage{
 async refreshBrand(){await this.page.evaluate(()=>dispatchEvent(new Event('focus')));}
 async visibility(hidden:boolean){await this.page.evaluate(hidden=>{Object.defineProperty(document,'hidden',{configurable:true,value:hidden});document.dispatchEvent(new Event('visibilitychange'));},hidden);}
 async lifecycleRestore(){await this.page.evaluate(()=>{dispatchEvent(new PageTransitionEvent('pagehide',{persisted:true}));dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true}));});}
 async markNavigation(){await this.page.locator('.horizontal-menu-navbar-menu > li').first().evaluate(n=>n.setAttribute('data-sync-marker','kept'));}
 async exitNavigation(){await this.page.locator('.horizontal-menu-navbar-search .search-input').click();await this.leaveNavigation();}
}

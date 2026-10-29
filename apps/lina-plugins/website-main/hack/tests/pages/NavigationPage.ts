import {HomeDetailPage} from './HomeDetailPage';
export class NavigationPage extends HomeDetailPage{
 async hoverTitle(name:string){await this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:name}).hover();}
 panel(){return this.page.locator('.lina-open .lina-nav-panel');}
 async selectNavigationCategory(name:string){await this.panel().getByRole('button',{name,exact:true}).hover();}
 async clickTitle(name:string){await this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:name}).click();}
 async retryNavigation(){await this.panel().getByRole('button',{name:'重试',exact:true}).click();}
 async openSearch(){await this.page.locator('.horizontal-menu-navbar-search .search-input').click();}
 async searchIn(target:'all'|'news'|'docs',value:string){await this.openSearch();await this.page.locator('[data-search-target='+target+']').click();await this.page.locator('.nav-search-input').fill(value);await this.page.locator('.nav-search-input').press('Enter');}
 async hoverGroup(index:number){await this.page.locator('.horizontal-menu-navbar-menu > li > a').nth(index).hover();}
 async enterPanel(){await this.page.locator('.lina-open .Submenu-middle-right').hover({position:{x:12,y:12}});}
 async closePanel(){await this.page.locator('.lina-open .lina-nav-close').click();}
 async leaveNavigation(){await this.page.mouse.move(0,850);}
 async focusGroup(index:number){await this.page.locator('.horizontal-menu-navbar-menu > li > [aria-expanded]').nth(index).focus();}
 async escape(){await this.page.keyboard.press('Escape');}
 async market(){await this.page.getByRole('link',{name:'应用市场',exact:true}).click();}
}

import {WebsitePage} from './SourceWebsitePage';
export class HomePage extends WebsitePage {
 async top(){await this.page.evaluate(()=>window.scrollTo({top:0,left:0,behavior:'instant'}));}
 async nextHero(){await this.page.getByRole('button',{name:'下一张',exact:true}).click();}
 async previousHero(){await this.page.getByRole('button',{name:'上一张',exact:true}).click();}
 async hero(index:number){await this.page.locator('.swipervertical-title').nth(index).click();}
 async heroByKeyboard(index:number){await this.page.locator('.swipervertical-title').nth(index).focus();await this.page.keyboard.press('Enter');}
 async solution(index:number){await this.page.locator('.main-option-name').nth(index).click();}
 async category(index:number){await this.page.locator('[data-categories] a').nth(index).click();}
}

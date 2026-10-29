import type {Page} from '@playwright/test';
export class SupportBarPage {
 constructor(readonly page:Page){}
 get bar(){return this.page.locator('[data-lina-support-bar]');}
 trigger(name:string){return this.bar.getByRole('button',{name,exact:true});}
 async open(path='/'){await this.page.goto(path);}
 async hover(name:string){await this.trigger(name).hover();}
 async leave(){await this.page.mouse.move(20,100);}
 async keyboard(name:string,key:string){await this.trigger(name).focus();await this.page.keyboard.press(key);}
 async mobile(){await this.page.setViewportSize({width:390,height:844});}
 async click(name:string){await this.trigger(name).click();}
 async bottom(){await this.page.evaluate(()=>window.scrollTo(0,document.body.scrollHeight));}
 async top(){await this.bar.getByRole('button',{name:'返回顶部'}).click();}
 async capture(path:string){await this.page.screenshot({path});}
}

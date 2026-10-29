import {CommerceNavigationPage} from './CommerceNavigationPage';
export class ServerSpecificationsPage extends CommerceNavigationPage {
 specification(key:string){return this.page.locator('[data-server-specification="'+key+'"]');}
 titles(){return this.page.locator('[data-fields] .label-title span');}
 async returnFocus(){await this.page.evaluate(()=>window.dispatchEvent(new Event('focus')));}
 async chooseSpecification(key:string,index:number){await this.specification(key).locator('.lina-option-buttons button').nth(index).click();}
 async captureSpecifications(name:string,key='region'){await this.specification(key).scrollIntoViewIfNeeded();return this.capture(name,false);}
}

import {Page} from '@playwright/test';

/** Existing customer center; no replacement routes or responses. */
export class CustomerCheckoutPage {
 constructor(readonly page:Page){}
 async login(username:string,password:string){
  await this.page.getByPlaceholder('请输入用户名').fill(username);
  await this.page.getByPlaceholder('请输入密码').fill(password);
  await this.page.getByRole('button',{name:'login',exact:true}).click();
 }
 async reprice(){await this.page.getByRole('button',{name:/^(重新核价|再次核价)$/}).click();}
 async quoteConfiguration(){await this.page.getByRole('button',{name:'校验并查看价格',exact:true}).click();}
 async clearLoginPassword(){const field=this.page.getByPlaceholder('请输入密码');if(await field.isVisible())await field.fill('');}
 async checkout(){await this.page.getByRole('button',{name:'确认并提交订单',exact:true}).click();}
 async openOrder(orderNo:string){await this.page.getByRole('button',{name:orderNo,exact:true}).click();}
}

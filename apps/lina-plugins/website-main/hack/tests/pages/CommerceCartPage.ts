import {Page} from '@playwright/test';
export class CommerceCartPage{
 constructor(readonly page:Page){}
 async open(){await this.page.goto('/cart.html');}
 get workspace(){return this.page.locator('#commerce-page');}
 get status(){return this.workspace.locator('.cart-status');}
 get total(){return this.workspace.locator('.cart-total>strong');}
 item(no:string){return this.workspace.locator(`[data-item="${no}"]`);}
 async quantity(no:string,value:string){const item=this.item(no);await item.locator('input[type=number]').fill(value);await item.getByRole('button',{name:'更新',exact:true}).click();}
 async next(){await this.workspace.getByRole('button',{name:'去结算',exact:true}).click();}
 async submit(){await this.workspace.getByRole('button',{name:'确认订单',exact:true}).click();}
 async english(){await this.page.getByRole('button',{name:'Switch to English',exact:true}).click();}
 async coupon(code:string){await this.workspace.getByRole('textbox',{name:'优惠码',exact:true}).fill(code);await this.workspace.getByRole('button',{name:'应用',exact:true}).click();}
 async payment(name:string){await this.workspace.getByRole('radio',{name,exact:true}).check();}
 async openPayment(){const [popup]=await Promise.all([this.page.waitForEvent('popup'),this.workspace.getByText('打开支付页面',{exact:true}).click()]);return popup;}
 async checkPayment(){await this.workspace.getByRole('button',{name:'查询付款状态',exact:true}).click();}
 async continuePayment(){await this.workspace.getByRole('button',{name:'继续付款',exact:true}).click();}
 async showSteps(){await this.workspace.locator('.cart-steps').scrollIntoViewIfNeeded();}
 async showCheckout(){await this.workspace.getByRole('button',{name:'Proceed to checkout',exact:true}).scrollIntoViewIfNeeded();}
 async screenshot(path:string){await this.page.screenshot({path,fullPage:false,animations:'disabled'});}
}

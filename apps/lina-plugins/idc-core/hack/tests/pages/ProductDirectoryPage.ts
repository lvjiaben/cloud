import {mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import type {Page} from '@host-tests/support/playwright';
import {LoginPage} from '@host-tests/pages/LoginPage';
export class ProductDirectoryPage {
 constructor(readonly page:Page){}
 get merchant(){return this.page.getByTestId('merchant-product-directories');}
 get editor(){return this.page.getByRole('dialog').filter({has:this.page.getByTestId('product-directory-editor')});}
 get customer(){return this.page.getByTestId('customer-product-directories');}
 async openMerchant(){await this.page.goto('/idc/catalog/product-directories');await this.merchant.waitFor();}
 async create(){await this.merchant.getByRole('button',{name:'新增目录',exact:true}).click();await this.editor.waitFor();}
 async fill(name:string){await this.editor.getByRole('textbox',{name:'中文名称'}).fill(name);await this.editor.getByRole('textbox',{name:'英文名称'}).fill('Cloud servers');await this.editor.getByRole('combobox',{name:'关联商品'}).click();await this.page.locator('.ant-select-dropdown:visible').getByText('回归云服务器',{exact:true}).click();await this.editor.getByRole('textbox',{name:'中文名称'}).click();}
 async searchProducts(){await this.editor.getByRole('combobox',{name:'关联商品'}).fill('不存在的商品');}
 async save(){await this.editor.getByRole('button',{name:/^确\s*认$/u}).click();}
 async loginCustomer(){await this.page.goto('http://127.0.0.1:6032/auth/login');await new LoginPage(this.page).login('directory-customer','test-only-password');await this.customer.waitFor();}
 async selectMenu(name:string){await this.page.getByRole('menuitem',{name,exact:true}).first().click();}
 async switchLanguage(label:'English'|'简体中文'){await new LoginPage(this.page).switchLanguage(label);}
 async selectTab(name:string){await this.customer.getByRole('button',{name,exact:true}).click();}
 async screenshot(label:string){const stamp=new Date().toISOString();const dir=resolve('../../temp',stamp.slice(0,10).replaceAll('-',''));await mkdir(dir,{recursive:true});const path=resolve(dir,`${stamp.slice(11,23).replace(/[:.]/gu,'')}-directory-${label}.png`);await this.page.locator('#__app-loading__').waitFor({state:'hidden'});await this.page.evaluate(async()=>{await Promise.all(document.getAnimations().filter(animation=>animation.effect?.getTiming().iterations!==Infinity).map(animation=>animation.finished.catch(()=>undefined)));});await this.page.screenshot({path});return path;}
}

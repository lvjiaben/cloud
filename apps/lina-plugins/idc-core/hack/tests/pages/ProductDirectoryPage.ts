import {mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import {expect, type Page} from '@host-tests/support/playwright';
import {LoginPage} from '@host-tests/pages/LoginPage';
import {workspacePath} from '@host-tests/fixtures/config';
export class ProductDirectoryPage {
 constructor(readonly page:Page){}
 get merchant(){return this.page.getByTestId('merchant-product-directories');}
 get editor(){return this.page.getByRole('dialog').filter({has:this.page.getByTestId('product-directory-editor')});}
 get customer(){return this.page.getByTestId('customer-product-directories');}
 async openMerchant(){await this.page.goto(workspacePath('/idc/member-navigation'));await this.merchant.waitFor();}
 async create(){await this.merchant.getByRole('button',{name:'新增菜单',exact:true}).click();await this.editor.waitFor();await expect(this.editor.getByRole('button',{name:/^确\s*认$/u})).toBeEnabled();}
 async fill(name:string){await this.editor.getByRole('textbox',{name:'中文名称'}).fill(name);await this.editor.getByRole('textbox',{name:'英文名称'}).fill('Cloud servers');}
 async edit(name:string){await this.merchant.locator('tr').filter({hasText:name}).getByRole('button',{name:'编辑菜单',exact:true}).click();await this.editor.waitFor();await expect(this.editor.getByRole('button',{name:/^确\s*认$/u})).toBeEnabled();}
 async save(){await this.editor.getByRole('button',{name:/^确\s*认$/u}).click();}
 async loginCustomer(){await this.page.goto('http://127.0.0.1:6032/auth/login');await new LoginPage(this.page).login('directory-customer','test-only-password');await this.customer.waitFor();}
 async selectMenu(name:string){await this.page.getByRole('menuitem',{name,exact:true}).first().click();}
 async switchLanguage(label:'English'|'简体中文'){await new LoginPage(this.page).switchLanguage(label);}
 async selectTab(name:string){await this.customer.getByRole('button',{name,exact:true}).click();}
 async screenshot(label:string){const stamp=new Date().toISOString();const dir=resolve('../../temp',stamp.slice(0,10).replaceAll('-',''));await mkdir(dir,{recursive:true});const path=resolve(dir,`${stamp.slice(11,23).replace(/[:.]/gu,'')}-directory-${label}.png`);await this.page.locator('#__app-loading__').waitFor({state:'hidden'});await this.page.locator('.bg-overlay-content:not(.invisible)').waitFor({state:'hidden'});await this.page.screenshot({path,animations:'disabled'});return path;}
}

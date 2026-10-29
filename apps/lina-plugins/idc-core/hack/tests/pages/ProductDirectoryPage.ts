import {fileURLToPath} from 'node:url';
import {mkdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import {expect, type Page} from '@host-tests/support/playwright';
import {LoginPage} from '@host-tests/pages/LoginPage';
import {waitForRouteReady} from '@host-tests/support/ui';
import {workspacePath} from '@host-tests/fixtures/config';
export class ProductDirectoryPage {
 constructor(readonly page:Page){}
 get merchant(){return this.page.getByTestId('merchant-product-directories');}
 get editor(){return this.page.getByRole('dialog').filter({has:this.page.getByTestId('product-directory-editor')});}
 get customer(){return this.page.getByTestId('customer-product-directories');}
 async openMerchant(){await this.page.reload({waitUntil:'domcontentloaded'});await waitForRouteReady(this.page,15000);const url=new URL(this.page.url());await this.page.goto(url.hash.startsWith('#/')?`${url.origin}${workspacePath('/')}#/idc/member-navigation`:workspacePath('/idc/member-navigation'));await this.merchant.waitFor();}
 async create(){await this.merchant.getByRole('button',{name:'新增菜单',exact:true}).click();await this.editor.waitFor();await expect(this.editor.getByRole('button',{name:/^确\s*认$/u})).toBeEnabled();}
 async fill(name:string){await this.editor.getByRole('textbox',{name:'菜单名称'}).fill(name);await this.editor.getByRole('textbox',{name:'英文名称'}).fill('Cloud servers');}
 async edit(name:string){const rowid=await this.merchant.locator('.vxe-table--main-wrapper tr').filter({hasText:name}).first().getAttribute('rowid');await this.merchant.locator(`tr[rowid="${rowid}"]`).getByRole('button',{name:'编辑菜单',exact:true}).filter({visible:true}).click();await this.editor.waitFor();await expect(this.editor.getByRole('button',{name:/^确\s*认$/u})).toBeEnabled();}
 sortInput(no:string){return this.merchant.locator(`input[data-testid="directory-sort-${no}"], [data-testid="directory-sort-${no}"] input`);}
 async enterSort(no:string,value:string){await this.sortInput(no).fill(value);}
 async commitSort(no:string){await this.sortInput(no).press('Tab');}
 get iconPopover(){return this.page.getByRole('dialog').filter({has:this.page.getByPlaceholder('搜索图标...')});}
 async openIcons(){await this.editor.getByTestId('directory-icon-picker').click();await expect(this.iconPopover).toBeVisible();}
 async searchIcon(name:string){await this.iconPopover.getByPlaceholder('搜索图标...').fill(name);}
 async selectFilteredIcon(){const icons=this.iconPopover.locator('.grid button');await expect(icons).toHaveCount(1);await icons.click();await expect(this.iconPopover).toBeHidden();}
 async save(){await this.editor.getByRole('button',{name:/^确\s*认$/u}).click();}
 async loginCustomer(){await this.page.goto('http://127.0.0.1:6032/auth/login');await new LoginPage(this.page).login('directory-customer','test-only-password');await this.customer.waitFor();}
 async selectMenu(name:string){await this.page.getByRole('menuitem',{name,exact:true}).first().click();}
 async switchLanguage(label:'English'|'简体中文'){await new LoginPage(this.page).switchLanguage(label);}
 async selectTab(name:string){await this.customer.getByRole('button',{name,exact:true}).click();}
 async screenshot(label:string){const stamp=new Date().toISOString();const dir=resolve(fileURLToPath(new URL('../../../../../../temp/',import.meta.url)),stamp.slice(0,10).replaceAll('-',''));await mkdir(dir,{recursive:true});const path=resolve(dir,`${stamp.slice(11,23).replace(/[:.]/gu,'')}-directory-${label}.png`);await this.page.locator('#__app-loading__').waitFor({state:'hidden'});await this.page.locator('.bg-overlay-content:not(.invisible)').waitFor({state:'hidden'});await this.page.screenshot({path,animations:'disabled'});return path;}
}

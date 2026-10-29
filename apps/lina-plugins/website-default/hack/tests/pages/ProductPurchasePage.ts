import type { Page } from '@playwright/test';
import { expect } from '@host-tests/fixtures/auth';
import { resolve } from 'node:path';
import { mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
const repositoryRoot=fileURLToPath(new URL('../../../../../../',import.meta.url));
export class ProductPurchasePage {
 constructor(readonly page:Page,readonly origin:string){}
 open(slug='11',english=false){return this.page.goto(`${this.origin}${english?'/en-US':''}/products/${slug}`);}
 async assertDetail(){await expect(this.page.getByTestId('public-catalog-detail')).toContainText('配置购买测试服务器');await expect(this.page.getByTestId('public-product-configurator')).toContainText('选择配置');}
 selectSpecification(label:string){return this.page.locator('.ant-radio-button-wrapper').filter({hasText:label}).click();}
 fillHost(value:string){return this.page.getByRole('textbox',{name:'主机名称',exact:true}).fill(value);}
 async chooseOS(label:string){const control=this.page.getByRole('combobox',{name:/操作系统/});await control.focus();await control.press('ArrowDown');await this.page.locator('.ant-select-dropdown:visible').getByText(label,{exact:true}).click();}
 async chooseFamily(family:string){await this.page.getByTestId('purchase-os-family-'+family).click();}
 async chooseVersion(label:string,english=false){const control=this.page.locator('.os-version .ant-select-selector');await control.click();await this.page.locator('.ant-select-dropdown:visible').getByText(label,{exact:true}).click();}
 async chooseRegion(label:string){await this.page.getByTestId('purchase-region-selector').getByRole('button',{name:label,exact:true}).click();}
 async changeQuantity(label:string,value:string){await this.page.getByRole('spinbutton',{name:label,exact:true}).fill(value);}
 async increaseQuantity(label:string,english=false){await this.page.getByRole('button',{name:(english?'Increase ':'增加 ')+label,exact:true}).click();}
 async fillServerLogin(hostname:string,password:string,english=false){const section=this.page.getByTestId('purchase-server-login');await section.getByRole('textbox',{name:english?'Hostname':'主机名',exact:true}).fill(hostname);await section.getByLabel(english?'Server password':'主机密码',{exact:true}).fill(password);}
 async checkPrice(english=false){await this.page.getByRole('button',{name:english?'Check exact price':'查看准确价格',exact:true}).click();}
 async openCheckout(){return this.page.goto(this.origin+'/checkout');}
 async completeConfiguration(){await this.page.getByRole('button',{name:'补全配置',exact:true}).click();}
 async saveConfiguration(){await this.page.getByRole('button',{name:'保存配置并继续',exact:true}).click();}
 async fillHostname(value:string){await this.page.getByTestId('purchase-server-login').getByRole('textbox',{name:'主机名',exact:true}).fill(value);}
 async addToCart(){await this.page.getByTestId('public-add-to-cart').click();}
 async screenshot(step:string){await this.page.evaluate(()=>window.scrollTo(0,0));const now=new Date();const dir=resolve(repositoryRoot,'temp',now.toISOString().slice(0,10).replaceAll('-',''));await mkdir(dir,{recursive:true});return this.page.screenshot({path:resolve(dir,now.toISOString().slice(11,19).replaceAll(':','')+'-public-purchase-'+step+'.png'),fullPage:true,animations:'disabled'});}
}

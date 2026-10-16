import { expect, type Page } from '@playwright/test';
import path from 'node:path';
export class CustomerTemplatePage {
 constructor(readonly page:Page){}
 get root(){return this.page.locator('[data-plugin-frontend-root="idc-portal"]');}
 get dialog(){return this.page.getByRole('dialog',{name:'开发文档',exact:true});}
 async open(){await this.page.goto('/admin/system/customer-center-configuration');}
 async assertConfiguration(){await expect(this.root.getByRole('heading',{name:'用户中心配置',exact:true})).toBeVisible();await expect(this.root).not.toContainText('plugin.idc-portal.');}
 async openGuide(){await this.root.getByRole('button',{name:/开.*发.*文.*档/}).click();await expect(this.dialog).toContainText('/api/v1/auth/login');}
 async searchOrders(){await this.dialog.getByPlaceholder('搜索接口，如 login、orders、wallet、support').fill('/account/commerce/orders');await expect(this.dialog.locator('summary').first()).toBeVisible();}
 async screenshot(name:string){await this.page.screenshot({path:path.resolve('../../temp/20261016/'+name+'.png'),fullPage:true});}
 async assertCustomerLogin(){await this.page.goto('http://127.0.0.1:6032/auth/login');await expect(this.page.locator('html')).toHaveAttribute('data-customer-template','customer-default');await expect(this.page.locator('input[type="password"]').first()).toBeVisible();}
 async openBusiness(pathname:string,testId:string){await this.page.goto('http://127.0.0.1:6032'+pathname);await expect(this.page.getByTestId(testId)).toBeVisible();await this.page.waitForLoadState('networkidle');await expect(this.page.locator('.ant-skeleton-active')).toHaveCount(0);await this.page.reload();await expect(this.page.getByTestId(testId)).toBeVisible();await this.page.waitForLoadState('networkidle');await expect(this.page.locator('.ant-skeleton-active')).toHaveCount(0);}
}

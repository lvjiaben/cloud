import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { SSLLiveDiscoveryPage } from '../pages/SSLLiveDiscoveryPage';

test.describe('TC-3 SSL真实客户插件发现',()=>{
  test('TC-3a: 真实协助身份可发现SSL菜单，匿名无页面且无效或结束身份被拒绝',async({authenticatedPage:admin})=>{
    const code=process.env.E2E_SSL_CUSTOMER_ID;
    test.skip(!code,'需要显式指定本机已授权的演示客户编号；本测试不创建客户或证书。');
    const live=new SSLLiveDiscoveryPage(admin);
    let customer:Awaited<ReturnType<typeof live.openAssistedCustomer>>|undefined;
    try {
      customer=await live.openAssistedCustomer(code!);
      const errors:string[]=[];
      customer.on('pageerror',error=>errors.push(error.message));
      const assets:Array<{path:string;status:number}>=[];
      customer.on('response',response=>{ const path=new URL(response.url()).pathname;if(path.startsWith('/x-assets/'))assets.push({path,status:response.status()}); });
      expect(await live.discovery(customer,'anonymous')).toEqual({status:200,surfaces:[]});
      expect(await live.discovery(customer,'invalid')).toEqual({status:401,surfaces:[]});
      const authenticated=await live.discovery(customer,'current');
      expect(authenticated.status).toBe(200);
      expect(authenticated.surfaces).toContainEqual(expect.objectContaining({pluginId:'yunsu-ssl',entryUrl:expect.stringMatching(/^\/x-assets\/yunsu-ssl\/v\d+\.\d+\.\d+\/customer\/certificates\/index\.html$/u)}));
      const frame=await live.openCertificates(customer);
      await expect(frame.locator('[role="alert"]')).toHaveCount(0);
      await expect(frame.getByTestId('ssl-error')).toBeHidden();
      await expect(frame.locator('[data-testid="ssl-empty"]:visible, [data-testid="ssl-content"]:visible')).toHaveCount(1);
      await expect(customer.locator('iframe[data-testid="plugin-sandbox-frame"]')).not.toHaveAttribute('sandbox',/allow-same-origin/u);
      expect(assets).toContainEqual({path:'/x-assets/vben-adapter/v0.1.0/isolated/runtime.js',status:200});
      expect(errors).toEqual([]);
      const directory=resolve(process.cwd(),'../../temp',new Date().toISOString().slice(0,10).replaceAll('-',''));
      await mkdir(directory,{recursive:true});
      await customer.screenshot({path:resolve(directory,`${new Date().toISOString().slice(11,19).replaceAll(':','')}-ssl-real-customer-discovery.png`)});
      await live.endAssistance();
      expect(await live.discovery(customer,'current')).toEqual({status:401,surfaces:[]});
    } finally {
      await live.endAssistance();
      await customer?.close();
    }
  });
});

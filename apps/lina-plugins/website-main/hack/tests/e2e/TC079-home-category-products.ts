import {test,expect} from '../support/source-site';
import {previewResponse} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680'});
test.describe('TC-79 首页按分类展示服务器',()=>{
 for(const failure of [false,true])test(failure?'TC-79b: 分类请求失败后可重试':'TC-79a: 仅展示真实分类及所属商品',async({page})=>{
  let fail=failure;
  await page.route('**/x/**',async route=>{const u=new URL(route.request().url());
   if(u.pathname.endsWith('/catalog/products')){
    if(fail&&u.searchParams.has('categoryNo'))return route.fulfill({status:503,json:{code:1}});
    const selected=u.searchParams.get('categoryNo')||'a';
    return route.fulfill({json:{code:0,data:{categories:[{categoryNo:'a',nameZhCN:'云服务器'},{categoryNo:'b',nameZhCN:'独立服务器'}],products:[{categoryNo:selected,nameZhCN:selected==='a'?'云服务器甲':'独立服务器乙',publicSlug:selected,availability:'available'}],nextCursor:''}}});
   }
   const r=previewResponse(u,route.request().method());return route.fulfill({status:r.status,json:{code:0,data:r.data}});
  });
  await page.goto('/index.html');const section=page.locator('.systemproduct-layoutcontainer');
  if(failure){await expect(section.locator('[data-status]')).toHaveText('商品加载失败，请刷新重试。');fail=false;await section.getByRole('button',{name:'重试',exact:true}).click();}
  await expect(section.locator('.systemproduct-name')).toHaveText(['云服务器甲']);
  await expect(section.locator('[data-categories] a')).toHaveText(['云服务器','独立服务器']);
  await expect(section.locator('.systemproduct-left-title')).toHaveText('云服务器');await expect(section.getByRole('link',{name:'定制咨询',exact:true})).toHaveAttribute('href','/contact.html');
  await expect(section.getByText('全部商品',{exact:true})).toHaveCount(0);
  await section.getByRole('link',{name:'独立服务器',exact:true}).click();
  await expect(section.locator('.systemproduct-name')).toHaveText(['独立服务器乙']);
  await expect(section.locator('[aria-current=true]')).toHaveText('独立服务器');await expect(section.locator('.systemproduct-left-title')).toHaveText('独立服务器');
  await expect(section.locator('.systemproduct-left-desc')).toBeVisible();
  await expect(section.locator('.systemproduct-left-desc')).toHaveText('各项配置均衡，支持按套餐配置进行升级或单独升级硬盘、带宽等附加配置。');
  await section.getByRole('link',{name:'云服务器',exact:true}).click();
  await expect(section.locator('.systemproduct-left-title')).toHaveText('云服务器');
  await expect(section.locator('.systemproduct-left-desc')).toHaveText('各项配置均衡，支持按套餐配置进行升级或单独升级硬盘、带宽等附加配置。');
 });
});

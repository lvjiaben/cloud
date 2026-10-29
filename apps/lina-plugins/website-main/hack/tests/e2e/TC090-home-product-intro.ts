import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';

const categoryUrl='http://127.0.0.1:6680/?categoryNo=ctg_0843ae4ede9a24327b46e12f828c3dff';
const title='提供企业一站式产品与服务';
const description='携手三大公有云，提供快速、稳定、安全的全方位需求服务，帮助企业获得高效的动能';

test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:900}});
test.describe('TC-90 首页商品区介绍',()=>{
 test('分类参数不改变介绍文案，移除查看全部且保留商品区结构',async({page},info)=>{
  const html=await readFile(resolve(import.meta.dirname,'../../../frontend/public/index.html'),'utf8');
  const css=await readFile(resolve(import.meta.dirname,'../../../frontend/public/lina/site.css'),'utf8');
  if(process.env.WEBSITE_INTRO_VERIFY_LIVE!=='1'){
   await page.route(categoryUrl,route=>route.fulfill({status:200,contentType:'text/html; charset=utf-8',body:html}));
   await page.route('http://127.0.0.1:6680/lina/site.css',route=>route.fulfill({status:200,contentType:'text/css; charset=utf-8',body:css}));
  }
  await page.goto(categoryUrl,{waitUntil:'domcontentloaded'});

  const section=page.locator('.systemproduct-layoutcontainer');
  const intro=section.locator('.home-product-intro');
  await expect(intro.getByRole('heading',{level:1})).toHaveText(title);
  await expect(intro.locator('.home-product-intro-description')).toHaveText(description);
  await expect(intro.getByRole('link',{name:'查看全部'})).toHaveCount(0);
  await expect(section.locator('.systemproduct-categories[data-categories]')).toHaveCount(1);
  await expect(section.locator('.systemproduct-right-product[data-products]')).toHaveCount(1);
  await expect(intro.locator('.sk-title')).toHaveCSS('font-size','36px');
  await expect(intro.locator('.home-product-intro-description')).toHaveCSS('font-size','18px');
  await expect(intro).toHaveCSS('text-align','center');
  await intro.screenshot({path:info.outputPath('home-product-intro.png')});
 });
});

import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {ActivationIndicatorPage} from '../pages/ActivationIndicatorPage';

test.use({baseURL:'http://localhost:6680',websiteGateway:!process.env.WEBSITE_HOME_CANDIDATE,headless:true});
test.describe('TC-145 导航分类禁止用零散缓存冒充完整结果',()=>{
 for(const mobile of [false,true])test(`TC-145${mobile?'b':'a'}: 单商品缓存不抢先显示，整组出现后不被缓存事件降为一项`,async({page},info)=>{
  await controlledBusiness(page);await page.setViewportSize({width:mobile?390:1440,height:900});
  const names=['弹性云','云服务器演示','计算型服务器演示','高防服务器演示'];
  const categories=names.map((name,i)=>({categoryNo:'complete-'+i,nameZhCN:name}));
  const products=Array.from({length:8},(_,i)=>({categoryNo:'complete-0',publicSlug:'complete-product-'+i,nameZhCN:'弹性云完整商品'+i,availability:'available'}));
  await page.addInitScript(({categories,products})=>localStorage.setItem('lina.public-catalog.v1',JSON.stringify({savedAt:Date.now(),categories,products:products.slice(0,1)})),{categories,products});
  let release!:()=>void;const gate=new Promise<void>(resolve=>release=resolve);let slow=false,pending=0;
  await page.route('**'+prefix+'/catalog/products?*',async route=>{
   pending++;
   try{
   const category=new URL(route.request().url()).searchParams.get('categoryNo');
   if(category==='complete-0')await gate;
   if(slow)await new Promise(resolve=>setTimeout(resolve,1000));
   const rows=category==='complete-0'?products:category?[]:products.slice(0,1);
   await route.fulfill({json:{code:0,data:{categories,products:rows,nextCursor:''}}});
   }finally{pending--;}
  });
  const site=new ActivationIndicatorPage(page);await site.open('/');if(mobile)await site.toggleMenu();await site.clickTitle('产品与服务');
  try{
   await expect(site.panel()).toContainText('正在加载商品…');
   await expect(site.panel().locator('.Submenu-item')).toHaveCount(0);
   await expect(site.category(names[0])).toHaveAttribute('aria-pressed','true');
   await site.panel().evaluate(async el=>{await Promise.all(el.getAnimations().map(a=>a.finished.catch(()=>{})));});
   await site.captureIndicator(info.outputPath('complete-category-loading.png'));
  }finally{release();}
  await expect(site.panel().locator('.Submenu-title')).toHaveText(products.map(p=>p.nameZhCN));
  await page.evaluate(()=>document.dispatchEvent(new CustomEvent('lina:catalog-cache-updated')));
  await expect(site.panel().locator('.Submenu-title')).toHaveText(products.map(p=>p.nameZhCN));
  await site.click(site.category(names[1]));await expect(site.panel()).toContainText('暂无可展示商品。');
  slow=true;await site.click(site.category(names[0]));
  await expect(site.panel().locator('.Submenu-title')).toHaveText(products.map(p=>p.nameZhCN),{timeout:500});
  await site.captureIndicator(info.outputPath('complete-category-loaded.png'));
  await expect.poll(()=>pending).toBe(0);
 });
});

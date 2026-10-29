import {test,expect} from '@playwright/test';
import {ConfigurationControlsPage} from '../pages/ConfigurationControlsPage';

const slugs=['ecs-zhenjiang-balanced-draft-20261020','ecs-shanghai-compute-draft-20261020','ecs-guangzhou-memory-draft-20261020','ecs-ningbo-protection-draft-20261020','ecs-zhenjiang-configurable-simulation','ecs-shanghai-configurable-simulation','ecs-guangzhou-configurable-simulation','ecs-ningbo-configurable-simulation'];
test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:1000}});

test.describe('TC-81 所有已发布模拟服务器共用系统卡片',()=>{
 for(const slug of slugs)test('TC-81 '+slug,async({page})=>{
  const site=new ConfigurationControlsPage(page);
  await site.open('/configure.html?product='+slug);
  // Exercise the same ordinary URL as visitors, including its module dependencies.
  for(const path of ['lina/configuration-controls.js','lina/configuration-controls.css']){
   const response=await page.request.get(new URL(path,page.url()).href);
   expect(response.ok(),path+' must be served by the running website').toBe(true);
  }
  const cards=page.locator('.os-card');
  await expect(cards).toHaveCount(6);
  await expect(page.locator('[data-quote] button')).toHaveCount(4);
  await expect.poll(()=>cards.locator('img').evaluateAll(images=>images.every(img=>(img as HTMLImageElement).complete&&(img as HTMLImageElement).naturalWidth>0))).toBe(true);
  const ubuntu=cards.filter({has:page.getByText('Ubuntu',{exact:true})});
  await ubuntu.hover();
  await expect.poll(()=>ubuntu.locator('img').evaluate(el=>getComputedStyle(el).filter)).toContain('grayscale(0)');
  expect(await ubuntu.locator('img').evaluate(el=>getComputedStyle(el).transition)).toContain('0.3s');
  const before=await page.locator('[data-fields]>.sky-config-row').last().boundingBox();
  await ubuntu.locator('summary').click();
  await expect(ubuntu.locator('details')).toHaveAttribute('open','');
  await expect(ubuntu.locator('.version-list')).toBeVisible();
  const after=await page.locator('[data-fields]>.sky-config-row').last().boundingBox();
  expect(Math.abs(before!.y-after!.y)).toBeLessThan(1);
  const quoted=page.waitForResponse(response=>response.url().includes('/purchase-configuration/quote')&&response.request().method()==='POST');
  await ubuntu.getByRole('button',{name:'Ubuntu 22.04',exact:true}).click();
  expect((await quoted).ok()).toBe(true);
  await expect(ubuntu).toHaveClass(/active/);
  await expect(ubuntu.locator('.dropdown-btn')).toHaveText('Ubuntu 22.04');
  await expect(page.locator('[data-quote] button')).toHaveCount(4);
  const terms=page.locator('[data-quote] button');
  const monthly=terms.filter({hasText:/^(?:1月|月)[:：]/});
  const amount=Number((await monthly.innerText()).split('CNY ')[1]);
  const fixedMonthly=[69,199,499,399][slugs.indexOf(slug)];
  if(fixedMonthly!==undefined)expect(amount).toBe(fixedMonthly);
  for(const months of [1,3,6,12]){
   const termLabels:Record<number,string>={1:'1月|月',3:'3月|季',6:'6月|半年',12:'1年|年'};
   await terms.filter({hasText:new RegExp('^(?:'+termLabels[months]+')[:：]')}).click();
   await expect(site.selectedPrice()).toHaveText('¥'+(amount*months).toFixed(2));
   await expect(page.locator('[data-add]')).toBeEnabled();
  }
  await site.captureControls('tc081-'+slug);
 });
});

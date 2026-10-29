import {test,expect} from '@playwright/test';
import {LiveBillingPeriodsPage} from '../pages/LiveBillingPeriodsPage';
const slugs=['ecs-zhenjiang-balanced-draft-20261020','ecs-shanghai-compute-draft-20261020','ecs-guangzhou-memory-draft-20261020','ecs-ningbo-protection-draft-20261020','ecs-zhenjiang-configurable-simulation','ecs-shanghai-configurable-simulation','ecs-guangzhou-configurable-simulation','ecs-ningbo-configurable-simulation'];
test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:1000}});
test.describe('TC-80 已发布八款服务器多周期',()=>{
 for(const slug of slugs)test('TC-80 '+slug,async({page})=>{
  const site=new LiveBillingPeriodsPage(page);await site.open('/configure.html?product='+slug);
  await expect(site.terms).toHaveCount(4);
  const month=Number((await site.terms.filter({hasText:'1月：'}).innerText()).split('CNY ')[1]);
  for(const months of [1,3,6,12]){await site.choosePeriod(months);await expect(site.total).toHaveText('CNY '+(month*months).toFixed(2));await expect(site.terms.filter({hasText:months===12?'1年：':months+'月：'})).toHaveAttribute('aria-pressed','true');}
  if(slug.includes('configurable')){await site.changeCpu();await expect(site.terms).toHaveCount(4);const changed=Number((await site.terms.filter({hasText:'1月：'}).innerText()).split('CNY ')[1]);expect(changed).not.toBe(month);for(const months of [1,3,6,12]){await site.choosePeriod(months);await expect(site.total).toHaveText('CNY '+(changed*months).toFixed(2));}}
  await expect(page.locator('[data-add]')).toBeEnabled();
  await site.terms.first().scrollIntoViewIfNeeded();await site.capture('tc080-'+slug,false);
 });
});

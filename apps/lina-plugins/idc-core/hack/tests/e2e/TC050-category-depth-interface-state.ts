// Verify category depth and actual channel state without supplier operations.
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import { writeFileSync } from 'node:fs';
import { test, expect } from '../support/catalog-auth';
import { pluginApiPath } from '@host-tests/fixtures/config';
import { CatalogManagementPage } from '../pages/CatalogManagementPage';
import { AutomationConfigurationPage } from '../pages/AutomationConfigurationPage';

test('TC-50a: 分类最多二级，接口默认不展示已删除记录', async ({adminPage}) => {
 test.setTimeout(90000);
 adminPage.setDefaultTimeout(15000);
 const token = await adminPage.evaluate(() => {
  const key=Object.keys(localStorage).find(k=>k.endsWith('core-access'))!;
  return JSON.parse(localStorage.getItem(key)!).accessToken;
 });
 await adminPage.context().setExtraHTTPHeaders({Authorization:'Bearer '+token});
 const api=adminPage.request, base=pluginApiPath('idc-core','admin/catalog/categories');
 const categories:any[]=[];
 const create=async(parentCategoryNo='')=>{
  const name='层级回归-'+randomUUID().slice(0,8);
  return (await api.post(base,{data:{nameZhCn:name,nameEnUs:name,parentCategoryNo,sortOrder:100}})).json();
 };
 try {
  const root=await create();expect(root.code,JSON.stringify(root)).toBe(0);categories.push(root.data.category);
  const child=await create(root.data.category.categoryNo);expect(child.code,JSON.stringify(child)).toBe(0);categories.push(child.data.category);
  const third=await create(child.data.category.categoryNo);expect(third.errorCode ?? third.data?.errorCode ?? third.code).toBeTruthy();expect(third.code).not.toBe(0);expect(third.message).toContain('二级');
  const other=await create();expect(other.code).toBe(0);categories.push(other.data.category);
  const moved=await(await api.put(base+'/'+root.data.category.categoryNo,{data:{nameZhCn:root.data.category.nameZhCn,nameEnUs:root.data.category.nameEnUs,parentCategoryNo:other.data.category.categoryNo,sortOrder:100,version:1}})).json();expect(moved.code).not.toBe(0);expect(moved.message).toContain('二级');
  const catalog=new CatalogManagementPage(adminPage);await catalog.goto();
  const options=await catalog.openCategoryParentChoices();
  await expect(options).toContainText(root.data.category.nameZhCn);await expect(options).not.toContainText(child.data.category.nameZhCn);
  await adminPage.screenshot({path:path.resolve('../../temp/20261016/102000-category-two-level.png')});
  await adminPage.reload();
  const center=new AutomationConfigurationPage(adminPage);await center.goto();await center.openAutomation();
  const response=await(await api.get(pluginApiPath('idc-resource','admin/resource/provider-configurations'),{params:{availableChannelsOnly:true}})).json();
  expect(response.code).toBe(0);expect(response.data.channels.every((c:any)=>c.status!=='deleted')).toBe(true);
  writeFileSync(path.resolve('../../temp/20261016/102000-channel-state.json'),JSON.stringify(response.data.channels.map((c:any)=>({name:c.displayName,status:c.status,secretFields:c.secretFields,schema:c.interfaceConfiguration})),null,2));
  await expect(center.deletedChannelsToggle).toContainText('显示已删除');
  await center.waitForChannelCount(response.data.channelTotal);
  await adminPage.screenshot({path:path.resolve('../../temp/20261016/114500-interface-active-only.png')});
  const allChannels=await center.toggleDeletedChannels();
  expect(allChannels.code).toBe(0);
  expect(allChannels.data.channelTotal).toBeGreaterThanOrEqual(response.data.channelTotal);
  await center.waitForChannelCount(allChannels.data.channelTotal);
  await expect(center.deletedChannelsToggle).toContainText('隐藏已删除');
  await adminPage.screenshot({path:path.resolve('../../temp/20261016/102100-interface-status.png')});
  const activeChannels=await center.toggleDeletedChannels();
  expect(activeChannels.code).toBe(0);
  expect(activeChannels.data.channels.every((c:any)=>c.status!=='deleted')).toBe(true);
  await center.waitForChannelCount(activeChannels.data.channelTotal);
  await expect(center.deletedChannelsToggle).toContainText('显示已删除');
 } finally {
  for(const c of categories.reverse()){
   const removed=await(await api.delete(base+'/'+c.categoryNo,{params:{version:1}})).json();expect(removed.code,JSON.stringify(removed)).toBe(0);
  }
 }
});

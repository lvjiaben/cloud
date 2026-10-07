import assert from 'node:assert/strict';
import { mkdir, readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const repository = resolve(root, '../../../..');
const require = createRequire(resolve(repository, 'apps/lina-vben/package.json'));
const { chromium } = require('playwright');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
const errors = [];
let failNextRuntime = false;
let runtimeRequests = 0;
page.on('pageerror', (error) => errors.push(error.message));
const host = '<!doctype html><iframe title="Isolated Vben" sandbox="allow-scripts" src="http://plugin.test/frame" style="width:100%;height:700px;border:0"></iframe>';
const frame = `<!doctype html><html lang="fr" dir="ltr" data-vxe-ui-theme="dark" style="--host-marker: 7"><head><meta charset="UTF-8"><link rel="stylesheet" href="/runtime.css"></head><body data-host="unchanged"><div id="app"></div><script type="module">
import { mountIsolatedVbenTable } from '/x-assets/vben-adapter/v0.1.0/isolated/index.js';
window.mountTable = mountIsolatedVbenTable;
window.displayLocale = 'zh-CN';
window.inputRenders = 0;
window.table = mountIsolatedVbenTable(document.querySelector('#app'), { height:420, rowKey:'domain', columns:[{field:'domain',title:'证书域名',minWidth:180},{field:'state',title:'状态',width:100,cell:row => window.displayLocale === 'en-US' ? 'Issued' : row.state},{field:'input',title:'备注',width:180,cell:() => { window.inputRenders++;const input=document.createElement('input');input.setAttribute('data-testid','focus-input');return input; }}],rows:[{domain:'example.test',state:'已签发'}],locale:'zh-CN' });
await window.table.ready;
</script></body></html>`;
await page.route('http://plugin.test/**', async (route) => {
  const path = new URL(route.request().url()).pathname;
  if (path.endsWith('/isolated/runtime.js')) {
    runtimeRequests++;
    if (failNextRuntime) { failNextRuntime=false; await route.abort('failed'); return; }
  }
  const files = {
    '/runtime.css': ['text/css', resolve(root, 'dist/isolated/vben-plugin-adapter.css')],
    '/vue.js': ['application/javascript', resolve(root, 'node_modules/vue/dist/vue.esm-browser.prod.js')],
  };
  const runtimeFile = path.split('/isolated/')[1];
  const file = files[path] ?? (runtimeFile && /^[\w.-]+\.js$/.test(runtimeFile) ? ['application/javascript', resolve(root, 'dist/isolated', runtimeFile)] : undefined);
  await route.fulfill({
    headers: { 'Access-Control-Allow-Origin': '*' },
    contentType: file?.[0] || 'text/html',
    body: file ? await readFile(file[1]) : path === '/frame' ? frame : host,
  });
});
try {
  await page.goto('http://plugin.test/host');
  const iframe = page.frameLocator('iframe');
  await iframe.locator('.vxe-table--body').getByText('example.test').waitFor({ state: 'visible', timeout: 20000 });
  assert.equal(await page.locator('iframe').getAttribute('sandbox'), 'allow-scripts');
  assert.equal(await iframe.locator('.vxe-grid').count(), 1);
  assert.deepEqual(await iframe.locator('html').evaluate((element) => Object.fromEntries(Array.from(element.attributes, (attribute) => [attribute.name, attribute.value]))), {lang:'fr',dir:'ltr','data-vxe-ui-theme':'dark',style:'--host-marker: 7'});
  assert.equal(await iframe.locator('.vxe-header--column').getByText('证书域名').count(), 1);
  assert.notEqual(await iframe.locator('.vxe-table--header-wrapper').evaluate((element) => getComputedStyle(element).backgroundColor), 'rgb(255, 255, 255)');
  assert.equal(await iframe.locator('.vxe-table--header').evaluate((element) => getComputedStyle(element).backgroundColor),await iframe.locator('.vxe-table--header-wrapper').evaluate((element) => getComputedStyle(element).backgroundColor));
  assert.deepEqual(errors, []);
  assert.equal(await iframe.locator('.vxe-pager').count(),0);
  await iframe.locator('html').evaluate(() => {
    window.pageChanges=[];
    window.committedPagination={currentPage:1,pageSize:10,total:93,onChange:page=>window.pageChanges.push(page)};
    window.table.update({pagination:window.committedPagination});
  });
  await iframe.locator('.vxe-pager').waitFor();
  assert.equal(await iframe.locator('.vxe-pager--total').innerText(),'共 93 条记录');
  await iframe.locator('.vxe-pager--next-btn').click();
  assert.deepEqual(await iframe.locator('html').evaluate(()=>window.pageChanges),[{currentPage:2,pageSize:10}]);
  await iframe.locator('html').evaluate(()=>window.table.update({pagination:window.committedPagination}));
  await iframe.locator('.vxe-pager--num-btn.is--active').getByText('1',{exact:true}).waitFor();
  await iframe.locator('.vxe-pager--end-btn').click();
  assert.deepEqual(await iframe.locator('html').evaluate(()=>window.pageChanges.at(-1)),{currentPage:10,pageSize:10});
  await iframe.locator('html').evaluate(()=>window.table.update({pagination:{...window.committedPagination,currentPage:10}}));
  await iframe.locator('.vxe-pager--num-btn.is--active').getByText('10',{exact:true}).waitFor();
  await iframe.locator('.vxe-pager--prev-btn').click();
  assert.deepEqual(await iframe.locator('html').evaluate(()=>window.pageChanges.at(-1)),{currentPage:9,pageSize:10});
  await iframe.locator('.vxe-pager--home-btn').click();
  assert.deepEqual(await iframe.locator('html').evaluate(()=>window.pageChanges.at(-1)),{currentPage:1,pageSize:10});
  await iframe.locator('html').evaluate(()=>window.table.update({pagination:window.committedPagination}));
  await iframe.locator('.vxe-pager--sizes .vxe-input').click();
  for (const size of [10,20,50,100]) await iframe.locator('.vxe-select-option').getByText(`${size}条/页`,{exact:true}).waitFor();
  await iframe.locator('.vxe-select-option').getByText('20条/页',{exact:true}).click();
  assert.deepEqual(await iframe.locator('html').evaluate(()=>window.pageChanges.at(-1)),{currentPage:1,pageSize:20});
  await iframe.locator('html').evaluate(()=>window.table.update({pagination:undefined}));
  await iframe.locator('.vxe-pager').waitFor({state:'detached'});
  const directory = resolve(repository, 'temp/20260904');
  await mkdir(directory, { recursive: true });
  const image = resolve(directory, `${new Date().toISOString().slice(11, 19).replaceAll(':', '')}-vben-opaque-spike.png`);
  await page.screenshot({ path: image });
  const input = iframe.getByTestId('focus-input');
  await input.fill('keep my draft');
  const initialRenders = await input.evaluate(() => window.inputRenders);
  await iframe.locator('html').evaluate(() => window.table.update({loading:true}));
  assert.equal(await input.evaluate(element => document.activeElement === element),true);
  assert.equal(await input.inputValue(),'keep my draft');
  assert.equal(await input.evaluate(() => window.inputRenders),initialRenders);
  await iframe.locator('html').evaluate(() => { window.displayLocale='en-US';window.table.update({locale:'en-US',loading:false}); });
  await iframe.getByText('Issued',{exact:true}).waitFor({state:'visible'});
  const cell = iframe.locator('.vxe-body--column').first();
  const normalColor = await cell.evaluate(element => getComputedStyle(element).backgroundColor);
  await cell.hover();
  assert.notEqual(await cell.evaluate(element => getComputedStyle(element).backgroundColor),normalColor);
  await iframe.locator('html').evaluate(element => {
    element.style.setProperty('--card','222 47% 11%');
    element.style.setProperty('--foreground','0 0% 98%');
    element.style.setProperty('--accent','215 28% 17%');
    element.style.setProperty('--muted','215 28% 17%');
  });
  assert.notEqual(await iframe.locator('.vxe-table--header-wrapper').evaluate(element => getComputedStyle(element).backgroundColor),'rgb(255, 255, 255)');
  assert.equal(await iframe.locator('.vben-isolated-table').evaluate(element => getComputedStyle(element).color),'rgb(250, 250, 250)');
  // The same JS is safe inside an existing host DOM when its stylesheet is not
  // imported. No root attributes or pre-existing input focus may be changed.
  await page.route('http://plugin.test/embedded', route => route.fulfill({contentType:'text/html',body:frame.replace('<link rel="stylesheet" href="/runtime.css">','')}));
  await page.goto('http://plugin.test/embedded');
  await page.locator('.vxe-grid').waitFor();
  assert.deepEqual(await page.locator('html').evaluate((element) => Object.fromEntries(Array.from(element.attributes, (attribute) => [attribute.name, attribute.value]))), {lang:'fr',dir:'ltr','data-vxe-ui-theme':'dark',style:'--host-marker: 7'});
  assert.equal(await page.locator('head style,head link').count(),0);
  await page.evaluate(() => window.table.update({locale:'en-US'}));
  assert.equal(await page.locator('html').getAttribute('lang'),'fr');
  failNextRuntime=true;
  const initialRuntimeRequests=runtimeRequests;
  await page.route('http://plugin.test/retry',route=>route.fulfill({contentType:'text/html',body:frame.replace('<div id="app"></div>','<div id="app"></div><div id="other"></div>').replace('await window.table.ready;', `window.other=window.mountTable(document.querySelector('#other'),{rowKey:'id',columns:[{field:'id',title:'Other'}],rows:[{id:'other'}]}); const results=await Promise.allSettled([window.table.ready,window.other.ready]);window.failed=results.every(result=>result.status==='rejected');`)}));
  await page.goto('http://plugin.test/retry');
  await page.waitForFunction(()=>window.failed===true);
  const retry=await page.evaluate(async()=>{
    window.table.destroy();
    window.other.destroy();
    window.table=window.mountTable(document.querySelector('#app'),{height:420,rowKey:'id',columns:[{field:'id',title:'Retry'}],rows:[{id:'retry-ok'}]});
    window.other=window.mountTable(document.querySelector('#other'),{rowKey:'id',columns:[{field:'id',title:'Other'}],rows:[{id:'must-not-mount'}]});
    window.other.destroy();
    window.table.update({rows:[{id:'latest-before-ready'}]});
    try {await Promise.all([window.table.ready,window.other.ready]);return true;} catch {return false;}
  });
  console.log(JSON.stringify({retry,runtimeRequests:runtimeRequests-initialRuntimeRequests}));
  assert.equal(retry,true,'remount after runtime network failure must recover');
  assert.equal(runtimeRequests-initialRuntimeRequests,2,'concurrent mounts must share each attempt');
  await page.getByText('latest-before-ready',{exact:true}).waitFor();
  assert.equal(await page.locator('#other .vxe-grid').count(),0);
  assert.deepEqual(errors,[]);
  console.log(JSON.stringify({ passed: true, actualVbenGrids: 1, sandbox: 'allow-scripts', embeddedHostUnchanged:true, errors, screenshot: image }));
} catch (error) {
  console.error(JSON.stringify({ errors, snapshot: await page.content(), frames: await Promise.all(page.frames().map(async (frame) => ({ url:frame.url(), content:(await frame.content()).slice(-6000) }))) }));
  throw error;
} finally {
  await browser.close();
}

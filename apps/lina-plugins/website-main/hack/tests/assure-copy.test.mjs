import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {parseGameSource} from './support/game-copy.mjs';
import {collectAssureCopy,canonicalAssureCopy} from './support/assure-copy.mjs';
import {assureContent,assureHTML,assureShell} from '../../frontend/public/lina/assure-content.js';
import {composeNavigation} from '../../frontend/public/lina/navigation-content.js';

test('服务保障附件独立原文与40项标签层级、有效空格一致，电话独立绑定',async()=>{
 const reference=JSON.parse(await readFile(new URL('./support/assure-copy-reference.json',import.meta.url),'utf8'));
 const source=await readFile(new URL('./support/assure-source.html',import.meta.url),'utf8');
 assert.deepEqual(collectAssureCopy(parseGameSource(source)),reference.copy);
 assert.deepEqual(assureContent,reference);
 assert.equal(reference.copy.length,40);
 assert.deepEqual(canonicalAssureCopy(collectAssureCopy(parseGameSource(assureHTML))),canonicalAssureCopy(reference.copy));
 const d=parseGameSource(assureHTML);
 assert.equal(d.querySelector('.refundNotice p').tagName,'P');
 assert.equal(d.querySelectorAll('.reporting-text-title').length,2);
 assert.equal(d.querySelectorAll('.refundNotice-content p').length,9);
});

test('身份隔离、本地图片、工单按本方语义映射、缺失业务沿本地404',async()=>{
 const d=parseGameSource(assureHTML+assureShell.headerHTML+assureShell.footerHTML);
 assert.doesNotMatch(d.toString(),/xiaobaiyun|400-8876675|1531335070|work.weixin.qq.com|江苏白猿/);
 assert.equal(d.querySelector('[data-customer="/account/support"]').textContent,'我的工单');
 assert.equal(d.querySelector('[data-customer="/account/support/new"]').textContent,'提交工单');
 assert.equal(d.querySelectorAll('a[href^="/404.html?feature=assure-"]').length,2);
 const assets=JSON.parse(await readFile(new URL('../../docs/assure-images-source-20261025.json',import.meta.url),'utf8'));
 for(const asset of assets)assert.ok((await readFile(new URL('../../frontend/public/lina/assure-assets/'+asset.name,import.meta.url))).length>0);
});

test('服务保障导航覆盖后台旧404，保持公共说明',()=>{
 const nav=composeNavigation([{label:'支持与服务',children:[{label:'服务保障',href:'/404.html'}]}]);
 const item=nav.find(i=>i.label==='支持与服务').children.find(i=>i.label==='服务保障');
 assert.equal(item.href,'/Assure.html');
 assert.equal(item.description,'了解服务保障范围及相关说明。');
});

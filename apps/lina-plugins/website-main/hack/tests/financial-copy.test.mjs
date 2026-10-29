import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {financialContent,financialHTML,financialShell} from '../../frontend/public/lina/financial-content.js';
import {collectGameCopy,parseGameSource} from './support/game-copy.mjs';
import {composeNavigation} from '../../frontend/public/lina/navigation-content.js';

test('金融页54项附件原文、标签、父子关系和标题等级保持一致',async()=>{
 const reference=JSON.parse(await readFile(new URL('./support/financial-copy-reference.json',import.meta.url),'utf8'));
 assert.equal(reference.copy.length,54);
 assert.deepEqual(financialContent,reference);
 assert.deepEqual(collectGameCopy(parseGameSource(financialHTML)),reference.copy);
 const d=parseGameSource(financialHTML);
 assert.equal(d.querySelector('.problem-left-title').tagName,'H3');
 assert.equal(d.querySelector('.problem-left-desc').tagName,'H5');
 assert.equal(d.querySelectorAll('.problem-left-ul li').length,6);
 assert.equal(d.querySelectorAll('.problem-item').length,4);
 assert.ok(reference.copy.some(i=>i.text.includes(' Intel Haswell CPU、DDR4 ')));
 assert.ok(reference.copy.some(i=>i.text.includes(' I/O ')));
});

test('金融页展示图片本地化、无竞品身份或业务链接，商品不补造',async()=>{
 const d=parseGameSource(financialHTML+financialShell.headerHTML+financialShell.footerHTML);
 assert.equal(d.querySelectorAll('script,.financial-image-slot').length,0);
 assert.equal(d.querySelectorAll('.partner-Content-logo-item img').length,12);
 const reference=JSON.parse(await readFile(new URL('./support/financial-image-reference.json',import.meta.url),'utf8'));
 assert.deepEqual([...parseGameSource(financialHTML).querySelectorAll('img')].map(i=>i.getAttribute('src')),reference.images);
 for(const path of reference.assets)assert.ok((await readFile(new URL('../../frontend/public'+path,import.meta.url))).length>0);
 assert.equal(d.querySelector('.correlation-Content-row').children.length,0);
 for(const card of d.querySelectorAll('.sky-footer-top > *'))assert.ok(card.querySelector('img[src^="/lina/financial-assets/"]:not([hidden])'));
 assert.doesNotMatch(d.toString(),/xiaobaiyun|江苏白猿|小白云|kfcf5866a4206ffbab2|400-8876675|1531335070|苏ICP备/i);
});

test('金融导航覆盖旧404及后台旧目标，不改变其他导航契约',()=>{
 const nav=composeNavigation([{label:'解决方案',children:[{label:'行业解决方案',children:[{label:'金融解决方案',href:'/404.html'}]}]}]);
 const financial=nav.find(i=>i.label==='解决方案').children[0].children.find(i=>i.label==='金融解决方案');
 assert.equal(financial.href,'/plan/financial.html');
 assert.equal(financial.description,'了解金融业务的应用场景。');
});

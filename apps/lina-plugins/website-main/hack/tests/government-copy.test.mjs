import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {governmentHTML,governmentContent,governmentShell} from '../../frontend/public/lina/government-content.js';
import {collectGameCopy,parseGameSource} from './support/game-copy.mjs';
import {composeNavigation} from '../../frontend/public/lina/navigation-content.js';
const reference=JSON.parse(await readFile(new URL('./support/government-copy-reference.json',import.meta.url),'utf8'));
const normalize=rows=>rows.map(row=>({...row,text:row.text.replace(/\r\n?/g,'\n')}));
test('在线快照独立基准：51项原文、有效空格、顺序、父子路径、标签等级及状态一致',async()=>{
 assert.equal(reference.copy.length,51);
 assert.deepEqual(normalize(collectGameCopy(parseGameSource(governmentHTML))),normalize(reference.copy));
 const {source:referenceSource,...publicReference}=reference;
 assert.ok(referenceSource);
 assert.deepEqual(governmentContent,publicReference);
 assert.equal(Object.hasOwn(governmentContent,'source'),false);
 assert.ok(reference.copy.find(x=>x.text.endsWith('入云环境。 ')));
 const source=await readFile(new URL('./support/government-source.html',import.meta.url),'utf8');
 assert.deepEqual(normalize(collectGameCopy(parseGameSource(source))),normalize(reference.copy));
});

test('工业导航即使后台仍给旧404也进入用户指定政企页',()=>{
 const items=composeNavigation([{label:'解决方案',children:[{label:'通用解决方案',children:[{label:'工业解决方案',href:'/404.html'}]}]}]);
 const industrial=items.find(i=>i.label==='解决方案').children.find(i=>i.label==='通用解决方案').children.find(i=>i.label==='工业解决方案');
 assert.equal(industrial.href,'/plan/government.html');
 assert.equal(industrial.description,'了解工业数字化的应用场景。');
});
test('导航沿用附件的分组、隐藏面板和文案，后台只提供目的地',()=>{
 const nav=composeNavigation([{label:'解决方案',description:'不应覆盖原文',children:[]}],[],governmentContent.navigation);
 const copy=items=>items.map(({label,description,headingTag,promotionCopy,children})=>({label,description,headingTag,promotionCopy,children:children&&copy(children)}));
 assert.deepEqual(copy(nav),copy(governmentContent.navigation));
});
test('身份隔离、展示图本地化及三个咨询位置',()=>{
 const d=parseGameSource(governmentHTML+governmentShell.headerHTML+governmentShell.footerHTML);
 // The shared footer has four neutral service illustrations; the government source
 // itself stays free of identity assets until its local display images render.
 assert.equal(d.querySelectorAll('img[src]').length,4);
 assert.equal(d.querySelectorAll('script').length,0);
 assert.equal(d.querySelectorAll('.partner-Content-logo-item .government-image-slot').length,12);
 assert.equal(d.querySelectorAll('[data-government-contact]').length,3);
 assert.equal(d.querySelectorAll('.correlation-Content-row>*').length,0);
 assert.doesNotMatch(d.toString(),/xiaobaiyun|江苏白猿|小白云|400-8876675|1531335070|苏ICP备|work.weixin.qq.com/i);
});

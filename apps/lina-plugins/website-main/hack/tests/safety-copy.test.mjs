import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {safetyHTML,safetyContent,safetyShell,safetyShellCopy} from '../../frontend/public/lina/safety-content.js';
import {collectGameCopy,parseGameSource} from './support/game-copy.mjs';
import {composeNavigation} from '../../frontend/public/lina/navigation-content.js';
const reference=JSON.parse(await readFile(new URL('./support/safety-copy-reference.json',import.meta.url),'utf8'));
const shellReference=JSON.parse(await readFile(new URL('./support/safety-shell-reference.json',import.meta.url),'utf8'));

test('公共壳148条来源记录与页脚静态标签保持附件原文',()=>{
 assert.deepEqual(safetyShellCopy,shellReference);
 const footer=parseGameSource(safetyShell.footerHTML);
 const actual=[...footer.querySelectorAll('.sky-footer-title,.sky-footer-nav-heading,.sky-footer-nav-item a')].map(e=>({tag:e.tagName.toLowerCase(),text:e.textContent}));
 const expected=shellReference.filter(x=>x.section.startsWith('footer-')).map(({tag,text})=>({tag,text}));
 assert.deepEqual(actual,expected);
});
test('附件独立基准：52项原文、有效空格、顺序、父子路径、标签等级及状态一致',()=>{
 assert.equal(reference.copy.length,52);
 assert.deepEqual(collectGameCopy(parseGameSource(safetyHTML)),reference.copy);
 assert.deepEqual(safetyContent,reference);
 assert.ok(reference.copy.find(x=>x.text.endsWith('风险。 ')));
});
test('导航沿用附件的分组、隐藏面板和文案，后台只提供目的地',()=>{
 const nav=composeNavigation([{label:'解决方案',description:'不应覆盖原文',children:[]}],[],safetyContent.navigation);
 const copy=items=>items.map(({label,description,headingTag,promotionCopy,children})=>({label,description,headingTag,promotionCopy,children:children&&copy(children)}));
 assert.deepEqual(copy(nav),copy(safetyContent.navigation));
});
test('身份隔离、展示图本地化及三个咨询位置',()=>{
 const d=parseGameSource(safetyHTML+safetyShell.headerHTML+safetyShell.footerHTML);
 // The shared footer has four neutral service illustrations; the safety source
 // itself stays free of identity assets until its local display images render.
 assert.equal(d.querySelectorAll('img[src]').length,4);
 assert.equal(d.querySelectorAll('script').length,0);
 assert.equal(d.querySelectorAll('.partner-Content-logo-item .safety-image-slot').length,12);
 assert.equal(d.querySelectorAll('[data-safety-contact]').length,3);
 assert.equal(d.querySelectorAll('.correlation-Content-row>*').length,0);
 assert.doesNotMatch(d.toString(),/xiaobaiyun|江苏白猿|小白云|400-8876675|1531335070|苏ICP备|work.weixin.qq.com/i);
});

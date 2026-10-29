import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {parseHTML} from 'linkedom';
import {goingseaHTML,goingseaShell} from '../../frontend/public/lina/goingsea-content.js';
import {collectGameCopy} from './support/game-copy.mjs';

const source=await readFile(new URL('./support/goingsea-source.html',import.meta.url),'utf8');
const expected=JSON.parse(await readFile(new URL('./support/goingsea-copy-reference.json',import.meta.url),'utf8'));
const raw=parseHTML(source).document,actual=parseHTML(goingseaHTML).document;
test('出海页53项原文、顺序、父子路径和标题标签与独立原网页一致',()=>{
 assert.equal(createHash('sha256').update(source).digest('hex'),expected.sha256);
 assert.equal(expected.copy.length,53);
 assert.deepEqual(collectGameCopy(actual),expected.copy);
 const shape=node=>node.nodeType===3?node.textContent:node.nodeType===1?{tag:node.tagName,class:node.className,children:[...node.childNodes].map(shape)}:null;
 for(const selector of ['.globalBanner','.et-container','main.et-main'])assert.deepEqual(shape(actual.querySelector(selector)),shape(raw.querySelector(selector)));
 assert.equal(actual.querySelector('.architecture-advantage h1').textContent,'架构优势');
 assert.equal(actual.querySelector('.correlation-Content-row').textContent.trim(),'');
});
test('图片逐张映射原站，身份和客服资料不混入静态页面',async()=>{
 const images=JSON.parse(await readFile(new URL('./support/goingsea-image-reference.json',import.meta.url),'utf8'));
 const document=parseHTML(goingseaHTML+goingseaShell.footerHTML+goingseaShell.headerHTML).document;
 assert.deepEqual([...document.querySelectorAll('img[src]')].map(i=>i.getAttribute('src')),images.images.map(i=>i.local));
 assert.equal(document.querySelectorAll('.partner-Content-logo-item img').length,12);
 assert.equal(document.querySelectorAll('script').length,0);
 for(const a of document.querySelectorAll('a[href]'))assert.match(a.getAttribute('href'),/^(?:\/(?!\/)|#)/);
 assert.doesNotMatch(goingseaHTML+goingseaShell.footerHTML+goingseaShell.headerHTML,/小白云|江苏白猿|work\.weixin|local66a31854be284|local66a33c183298f|local672f527cd1a26|\/game-assets\//);
});

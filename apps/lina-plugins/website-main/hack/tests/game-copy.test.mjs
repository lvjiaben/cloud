import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {gameHTML,gameContent,gameShell} from '../../frontend/public/lina/game-content.js';
import {collectGameCopy,parseGameSource} from './support/game-copy.mjs';
import {composeNavigation} from '../../frontend/public/lina/navigation-content.js';

test('游戏正文53项原始文本、所属栏目、顺序、父子路径及标题级别一致',async()=>{
 const reference=JSON.parse(await readFile(new URL('./support/game-copy-reference.json',import.meta.url),'utf8'));
 assert.equal(reference.copy.length,53);
 assert.deepEqual(collectGameCopy(parseGameSource(gameHTML)),reference.copy);
 assert.deepEqual(gameContent,reference);
 assert.ok(reference.copy.find(item=>item.text.includes('结合 ，')));
 assert.ok(reference.copy.find(item=>item.text.includes('服务服务涵盖')));
 assert.ok(reference.copy.find(item=>item.text.endsWith('基石。 ')));
});

test('游戏页菜单原文不被后台标签或默认概述覆盖，其他页面保持原规则',()=>{
 const overrides=[{label:'解决方案',description:'不应展示的后台描述',children:[{label:'额外栏目',href:'/404.html'}]}];
 const actual=composeNavigation(overrides,[],gameContent.navigation);
 const copy=items=>items.map(({label,description,headingTag,promotionCopy,children})=>({label,description,headingTag,promotionCopy,children:children&&copy(children)}));
 assert.deepEqual(copy(actual),copy(gameContent.navigation));
 const defaults=composeNavigation();
 assert.equal(defaults.find(item=>item.label==='游戏解决方案'),undefined);
 assert.equal(defaults.find(item=>item.label==='解决方案').description,'探索不同行业与通用业务的应用场景。');
});

test('展示图片使用本地资源，十二张伙伴图齐全，不带对方Logo和联系资料',()=>{
 const d=parseGameSource(gameHTML+gameShell.headerHTML+gameShell.footerHTML);
 assert.equal(d.querySelectorAll('img[src]').length,25);
 for(const image of d.querySelectorAll('img[src]'))assert.match(image.getAttribute('src'),/^\/lina\/game-assets\//);
 assert.equal(d.querySelectorAll('script').length,0);
 assert.equal(d.querySelectorAll('.game-partner-image-slot').length,12);
 assert.equal(d.querySelectorAll('.game-partner-image-slot img').length,12);
 assert.doesNotMatch(d.toString(),/local66a31854be284|local66a33c183298f|local672f527cd1a26/);
 assert.doesNotMatch(d.toString(),/江苏白猿|小白云|400-8876675|1531335070|苏ICP备/);
});

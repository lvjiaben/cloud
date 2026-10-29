import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {parseHTML} from 'linkedom';
import {partnerHTML} from '../../frontend/public/lina/partner-content.js';
import {copy,reference,shape,sourceDocument} from './support/partner-reference.mjs';

const local=parseHTML(partnerHTML).document;
const normalize=doc=>{
 for(const e of doc.querySelectorAll('[href]')){const v=e.getAttribute('href');if(v?.includes('work.weixin.qq.com'))e.setAttribute('href','/contact.html');}
 for(const e of doc.querySelectorAll('[src]'))e.setAttribute('src',e.getAttribute('src').replace('/themes/web/www/static/picture/','/lina/partner-assets/'));
 doc.querySelector('.globalBanner').setAttribute('style','background-image:url(/lina/partner-assets/remoteL17131030094606.jpeg)');
 const header=doc.querySelector('.grade-table-header-cell:last-child');header.lastChild.textContent=header.lastChild.textContent.replace('砖石代理','钻石代理');
 return doc;
};
const selectors=['.globalBanner','.et-container','main.et-main'];
test('合作伙伴页面文字、顺序、父子结构和标题级别与独立原站快照一致（仅权益表头按用户要求改字）',()=>{
 assert.equal(reference.source,'https://www.xiaobaiyun.cn/cooperation/partner.html');
 assert.equal(reference.sourceCopy.length,copy(sourceDocument).length);
 assert.deepEqual(copy(normalize(local)),reference.sourceCopy);
 for(const selector of selectors)assert.deepEqual(shape(normalize(local).querySelector(selector)),shape(normalize(sourceDocument).querySelector(selector)));
 assert.equal(local.querySelector('.grade-table-header-cell:last-child').textContent.trim(),'钻石代理');
 assert.equal(local.querySelector('.partner_table .partner_row:last-child .partner_cell:first-child').textContent.trim(),'砖石代理');
 assert.equal(local.querySelectorAll('.introduce-section').length,5);
 assert.deepEqual([...local.querySelectorAll('.problem-content-item h3')].map(e=>e.tagName),['H3','H3','H3']);
});
test('伙伴权益八项支持状态及申请要求表格逐格匹配独立快照',()=>{
 const sourceRows=[...sourceDocument.querySelectorAll('.grade-table tr')].slice(1);
 const rows=[...local.querySelectorAll('.grade-table tr')].slice(1);
 assert.equal(rows.length,sourceRows.length);
 rows.forEach((row,i)=>assert.deepEqual([...row.querySelectorAll('.grade-status')].map(e=>e.classList.contains('grade-status-yes')?'y':'n'),[...sourceRows[i].querySelectorAll('.grade-status')].map(e=>e.classList.contains('grade-status-yes')?'y':'n')));
 assert.deepEqual([...local.querySelectorAll('.partner_table .partner_row')].map(r=>[...r.children].map(c=>c.textContent)),[...sourceDocument.querySelectorAll('.partner_table .partner_row')].map(r=>[...r.children].map(c=>c.textContent)));
});
test('本地展示资源齐备，CTA 使用本方联系入口',async()=>{
 const paths=['remoteL17131030094606.jpeg','remoteL17130183791832.png','remoteL17069792867331.png','remoteL17075794763815.png','remoteL17075795114472.png','correct.svg','error.svg'];
 for(const name of paths)assert.ok((await readFile(new URL('../../frontend/public/lina/partner-assets/'+name,import.meta.url))).length>0,name);
 assert.equal(local.querySelectorAll('a[href="/contact.html"]').length,3);
 assert.doesNotMatch(partnerHTML,/xiaobaiyun\.cn|work\.weixin\.qq\.com|local66a31854be284|local66a33c183298f|local672f527cd1a26/);
});

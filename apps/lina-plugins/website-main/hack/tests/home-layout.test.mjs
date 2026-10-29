import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile,readdir} from 'node:fs/promises';
import {parseHTML} from 'linkedom';

test('public homepage preserves product descendants alongside top-level marketing sections',async()=>{
 const {document}=parseHTML(await readFile(new URL('../../frontend/public/index.html',import.meta.url),'utf8'));
 assert.ok(document.querySelector('.systemproduct-layoutcontainer .systemproduct-container .systemproducts .systemproduct-right-product[data-products]'));
 assert.equal(document.querySelectorAll('[data-products]').length,1);
 assert.equal(document.querySelector('#product-card').content.querySelector('.systemproduct-tagline span').textContent,'NEW');
 assert.ok(document.querySelector('.systemproduct-layoutcontainer [data-status]'));
 const shortcuts=[...document.querySelectorAll('[data-home-blocks] > .bannerbelow-list')];
 assert.deepEqual(shortcuts.map(card=>card.querySelector('.bannerbelow-title').textContent.trim()),['最新活动','产品特惠、轻松上云','推广赚佣 轻松赚钱','弹性计算 管理简单']);
 assert.deepEqual(shortcuts.map(card=>card.querySelector('.bannerbelow-description').textContent.trim()),['产品促销、助您降本增效','4核2G西信高防云','每笔推荐均可赚钱（现金返佣）','云控制台、弹性配置，灵活管理']);
 for(const card of shortcuts){assert.equal(card.hasAttribute('href'),false);assert.equal(card.getAttribute('aria-disabled'),'true');assert.equal(card.getAttribute('title'),'尚未关联本方业务入口');}
 assert.equal(document.querySelector('.bannerbelow-layoutcontainer [data-home-blocks]')?.children.length,4);
 assert.equal(document.querySelector('.container_bg15')?.hidden,false);
 assert.equal(document.querySelector('.container_bg27')?.hidden,false);
 assert.ok(document.querySelector('.home-pagination-container .home-pagination'));
 assert.equal(document.querySelector('.home-pagination-container')?.hidden,false);
 assert.ok(document.querySelector('.container_bg15')?.children.length>0);
 assert.ok(document.querySelector('.container_bg30')?.children.length>0);
 assert.equal(document.querySelectorAll('section.container_bg1 .chooseUs-container .chooseUs-itme').length,4);
 assert.equal(document.querySelector('section.container_bg1:has(.chooseUs-container)')?.hidden,false);
 assert.ok(document.querySelector('section.container_bg1:has(.chooseUs-container)')?.nextElementSibling.matches('.container_bg27'));
 assert.ok(document.querySelector('.container_bg6 [data-home-news]'));
 assert.equal(document.querySelector('.systemproduct-layoutcontainer .lina-home-block-empty'),null);
 assert.equal(document.querySelectorAll('script:not([src="lina/site.js"])').length,0);
 assert.doesNotMatch(document.body.textContent,/\u5c0f\u767d\u4e91|白猿|As9929/i);
});

test('all public pages omit empty brand logo slots',async()=>{
 const root=new URL('../../frontend/public/',import.meta.url);
 const pages=(await readdir(root)).filter(name=>name.endsWith('.html'));
 assert.ok(pages.length>=12,'include the original public pages and any newly added pages');
 assert.ok(pages.includes('404.html'));
 for(const name of pages){const {document}=parseHTML(await readFile(new URL(name,root),'utf8'));assert.equal(document.querySelector('[data-footer-logo]'),null,name);}
 const activity=parseHTML(await readFile(new URL('newestactivity.html',root),'utf8')).document;
 assert.equal(activity.body.dataset.page,'activity');
 assert.deepEqual([...activity.querySelectorAll('[data-activity-filter]')].map(node=>node.textContent.trim()),['全部活动','正在进行','活动预告','已结束']);
 assert.equal(activity.querySelector('[data-activity-empty]').textContent.trim(),'更多优惠活动，敬请期待！');
 const home=parseHTML(await readFile(new URL('index.html',root),'utf8')).document;
 assert.ok(home.querySelector('.sky-footer-container [data-brand-footer]'));
 assert.equal(home.querySelector('.sky-footer-container .sky-footer-left-logo'),null);
});

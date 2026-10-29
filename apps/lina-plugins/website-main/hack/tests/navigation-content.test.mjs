import assert from 'node:assert/strict';
import test from 'node:test';
import {composeNavigation} from '../../frontend/public/lina/navigation-content.js';

const names=['产品与服务','解决方案','生态合作','应用市场','支持与服务','了解我们'];
test('terms entry overrides stale local and source-site destinations',()=>{
 for(const href of ['/404.html','https://www.xiaobaiyun.cn/terms.html']){
  const menus=composeNavigation([{label:'支持与服务',children:[{label:'服务条款',href,customer:true,external:true}]}]);
  const entry=menus.find(m=>m.label==='支持与服务').children.find(c=>c.label==='服务条款');
  assert.equal(entry.href,'/terms.html');assert.equal(entry.customer,false);assert.equal(entry.external,false);
  assert.equal(entry.description,'查看服务范围、使用规则与相关条款。');
 }
});
test('license lookup uses the explicitly configured ministry URL despite legacy targets',()=>{
 for(const href of ['/404.html','https://beian.miit.gov.cn/','https://www.xiaobaiyun.cn/']){
  const menus=composeNavigation([{label:'了解我们',children:[{label:'增值许可证查询',href,customer:true}]}]);
  const item=menus.find(m=>m.label==='了解我们').children.find(c=>c.label==='增值许可证查询');
  assert.equal(item.href,'https://dxyw.miit.gov.cn/dxxzsp/xkz/xkzgl/resource/qiyesearch.jsp?num=B1-xxx&type=xuke');
  assert.equal(item.external,true);assert.equal(item.customer,false);
 }
});
test('goingsea navigation uses the local exact-case path even with a published legacy target',()=>{
 const menus=composeNavigation([{label:'解决方案',children:[{label:'行业解决方案',children:[{label:'出海解决方案',href:'https://www.xiaobaiyun.cn/plan/Goingtosea.html',customer:true}]}]}]);
 const entry=menus.find(item=>item.label==='解决方案').children[0].children.find(item=>item.label==='出海解决方案');
 assert.equal(entry.href,'/plan/Goingtosea.html');assert.equal(entry.customer,false);assert.equal(entry.external,false);
});
test('website solution resolves locally even when published navigation has a legacy target',()=>{
 const menus=composeNavigation([{label:'解决方案',children:[{label:'行业解决方案',children:[{label:'网站解决方案',href:'/404.html',customer:true}]}]}]);
 const entry=menus.find(item=>item.label==='解决方案').children[0].children.find(item=>item.label==='网站解决方案');
 assert.equal(entry.href,'/plan/web.html');assert.equal(entry.customer,false);assert.equal(entry.external,false);
 assert.equal(entry.description,'了解网站部署与运行的应用场景。');
});
test('existing e-commerce solution retains its copy and resolves to the local nested page',()=>{
 const menus=composeNavigation([{label:'解决方案',children:[{label:'行业解决方案',children:[{label:'电商解决方案',href:'/404.html',customer:true}]}]}]);
 const entries=menus.find(item=>item.label==='解决方案').children[0].children.filter(item=>item.label==='电商解决方案');
 assert.equal(entries.length,1);assert.equal(entries[0].href,'/plan/electricity.html');assert.equal(entries[0].customer,false);
 assert.equal(entries[0].description,'了解线上商城与数字化经营的应用场景。');
});
test('existing sponsorship entry resolves to the new local page even with a published legacy target',()=>{
 const menus=composeNavigation([{label:'生态合作',group:true,children:[{label:'百站赞助公益计划',href:'/404.html',customer:true}]}]);
 const entries=menus.find(item=>item.label==='生态合作').children.filter(item=>item.label==='百站赞助公益计划');
 assert.equal(entries.length,1);assert.equal(entries[0].href,'/webmaster.html');assert.equal(entries[0].customer,false);
});
test('six reference menu shells retain local pages, descriptions, groups, and fallback targets',()=>{
 const menus=composeNavigation([{label:'首页',href:'/'},{label:'产品与服务',href:'/products.html'},{label:'支持与服务',group:true,children:[{label:'联系我们',href:'/contact.html'},{label:'本方新栏目',href:'/help.html'}]},{label:'外部菜单',href:'https://www.xiaobaiyun.cn/path'}],['http://localhost:6032']);
 assert.deepEqual(menus.slice(1,7).map(item=>item.label),names);
 assert.equal(menus[1].description,undefined);
 assert.equal(menus[2].children[0].children.length,7);
 assert.equal(menus[4].href,'/404.html');
 assert.equal(menus[5].children.find(item=>item.label==='联系我们').href,'/contact.html');
 assert.equal(menus[5].children.some(item=>item.label==='本方新栏目'),true);
 assert.equal(menus.at(-1).href,'/404.html');
 assert.equal(menus[1].children,undefined);
});

test('customer-center targets are preserved only for allowed local origins',()=>{
 const menus=composeNavigation([{label:'生态合作',group:true,children:[{label:'推广返利',href:'/account/referrals',customer:true},{label:'合作伙伴',href:'https://www.xiaobaiyun.cn/partner',customer:true},{label:'安全的客户中心',href:'https://localhost:6032/account',customer:true}]}],['http://localhost:6032','https://localhost:6032']);
 const children=menus.find(item=>item.label==='生态合作').children;
 assert.equal(children.find(item=>item.label==='推广返利').customer,false);
 assert.equal(children.find(item=>item.label==='推广返利').href,'/aff.html');
 assert.equal(children.find(item=>item.label==='合作伙伴').href,'/cooperation/partner.html');
 assert.equal(children.find(item=>item.label==='合作伙伴').customer,false);
 assert.equal(children.find(item=>item.label==='安全的客户中心').customer,true);
 const product=composeNavigation([{label:'产品与服务',href:'/products.html'}])[0];
 assert.equal(product.href,'/products.html');
});

test('partner entry resolves locally with defaults and stale published targets',()=>{
 for(const href of [undefined,'/404.html','https://www.xiaobaiyun.cn/cooperation/partner.html']){
  const menus=composeNavigation(href?[{label:'生态合作',children:[{label:'合作伙伴',href,customer:true,external:true}]}]:[]);
  const partner=menus.find(item=>item.label==='生态合作').children.find(item=>item.label==='合作伙伴');
  assert.equal(partner.href,'/cooperation/partner.html');
  assert.equal(partner.description,'共同发展，实现互利共赢的合作伙伴关系');
  assert.equal(partner.customer,false);assert.equal(partner.external,false);
 }
});

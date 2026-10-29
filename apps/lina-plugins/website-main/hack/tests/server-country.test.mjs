import {test} from 'node:test';
import assert from 'node:assert/strict';
import {parseHTML} from 'linkedom';
import {serverCountry,prependServerFlag} from '../../frontend/public/lina/server-country.js';

test('country flags follow explicit regions, unknown and ambiguous names remain names only',()=>{
 for(const name of ['江苏镇江','上海超级计算型','广州超大内存型','宁波超级高防型','陕西西安','Jiangsu Zhenjiang'])assert.equal(serverCountry(name)?.code,'cn');
 assert.equal(serverCountry('日本直连')?.code,'jp');assert.equal(serverCountry('美国9929高防区')?.code,'us');
 for(const name of ['演示云服务器 标准型01','日本美国双区',''])assert.equal(serverCountry(name),null);
 const {document}=parseHTML('<h5>江苏镇江</h5>'),title=document.querySelector('h5');
 prependServerFlag(title,{nameZhCN:'江苏镇江'});prependServerFlag(title,{nameZhCN:'江苏镇江'});
 assert.equal(title.querySelectorAll('img').length,1);assert.equal(title.querySelector('img').alt,'中国国旗');assert.equal(title.textContent,'江苏镇江');
 prependServerFlag(title,'未知');assert.equal(title.querySelectorAll('img').length,0);
});

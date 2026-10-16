import { readFileSync, writeFileSync, readdirSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve, basename } from 'node:path';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..');
for(const locale of ['zh-CN','en-US']){
 const source=resolve(root,'frontend/customer/src/locales/langs',locale);
 const messages={plugin:{'customer-default':locale==='zh-CN'?{name:'默认用户中心',description:'独立用户中心模板，支持服务、订单、财务和工单。'}:{name:'Default customer center',description:'Independent customer template with services, orders, finance and support.'}}};
 for(const file of readdirSync(source).filter(n=>n.endsWith('.json')))messages[basename(file,'.json')]=JSON.parse(readFileSync(resolve(source,file),'utf8'));
 Object.assign(messages,locale==='zh-CN'?{cancel:'取消',confirm:'确认',collapse:'收起',expand:'展开'}:{cancel:'Cancel',confirm:'Confirm',collapse:'Collapse',expand:'Expand'});
 const target=resolve(root,'manifest/i18n',locale);mkdirSync(target,{recursive:true});
 writeFileSync(resolve(target,'customer.json'),JSON.stringify(messages,null,2)+String.fromCharCode(10));
}

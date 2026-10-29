import {test} from 'node:test';
import assert from 'node:assert/strict';
import {serverSpecificationRows,serverSpecificationOptions,defaultUsageNotice} from '../../frontend/public/lina/server-specifications.js';
import {applyConfigurationDefaults} from '../../frontend/public/lina/configuration-defaults.js';

test('numeric and categorical choices preserve merchant order, token, disabled state and backend selection',()=>{
 for(const labels of [['100 GiB','30 GiB','50 GiB'],['32 核','4 核','16 核'],['1 Gbps','500 Mbps','20 Mbps'],['Windows Server 2022','Windows Server 2019']]){
  const options=labels.map((labelZhCN,index)=>({labelZhCN,optionToken:'token-'+index,disabled:index===2})),field={fieldToken:'specification',type:'single_select',visible:true,options},configuration={fields:[field],values:[]};
  applyConfigurationDefaults(configuration);const before=structuredClone(configuration);assert.equal(serverSpecificationOptions({field}),options);assert.deepEqual(configuration,before);assert.deepEqual(configuration.values[0].optionTokens,['token-0']);assert.equal(options[2]?.disabled,labels.length>2?true:undefined);
 }
});

test('all products share reference order and expose only backend-configured fields',()=>{
 const region={fieldToken:'real-region-token',labelZhCN:'模拟地区',visible:true,type:'single_select'};
 const disk={fieldToken:'real-disk-token',labelZhCN:'数据盘（GiB，0表示不要）',visible:true,type:'integer'};
 const systemDisk={fieldToken:'real-system-disk-token',labelZhCN:'系统盘（GiB）',visible:true,type:'single_select'};
 const extra={fieldToken:'extra-token',labelZhCN:'单机防御',visible:true,type:'single_select'};
 const hidden={fieldToken:'hidden-token',labelZhCN:'CPU',visible:false,type:'single_select'};
 const rows=serverSpecificationRows([disk,extra,hidden,systemDisk,region]);
 assert.deepEqual(rows.map(row=>row.title),['区域','系统盘','数据盘','单机防御']);
 assert.equal(rows[0].field,region);
 assert.equal(rows[1].field,systemDisk);
 assert.equal(rows[2].field,disk);
 assert.equal(rows[3].field,extra);
 assert.equal(rows.some(row=>row.field===hidden),false);
 assert.ok(defaultUsageNotice.startsWith('温馨提示：'));
});

test('backend credentials stay one real field without invented hostname or password rows',()=>{
 const generated={fieldToken:'credentials-token',labelZhCN:'主机名和密码',visible:true,type:'single_select',options:[{labelZhCN:'人工交付时安全生成，不在订单保存明文'}]};
 const disk={fieldToken:'system-disk',labelZhCN:'系统盘（GiB）',visible:true,type:'single_select'};
 const rows=serverSpecificationRows([generated,disk]);
 assert.equal(rows.find(row=>row.key==='systemDisk').field,disk);
 assert.equal(rows.some(row=>['hostname','password'].includes(row.key)),false);
 assert.equal(rows.find(row=>row.field===generated).title,'主机名和密码');
 assert.ok(rows.every(row=>row.field));
 const input={fieldToken:'host',labelZhCN:'主机名',visible:true,type:'single_line'};
 const inputRows=serverSpecificationRows([input]);
 assert.equal(inputRows.find(row=>row.key==='hostname').field,input);
 assert.equal(serverSpecificationRows([{...generated,options:[]}])[0].field.fieldToken,'credentials-token');
 assert.equal(serverSpecificationRows([{...input,visible:false}]).length,0);
});

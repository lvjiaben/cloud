import {test} from 'node:test';
import assert from 'node:assert/strict';
import {applyConfigurationDefaults} from '../../frontend/public/lina/configuration-defaults.js';
test('default first choices and numeric minima without inventing text or replacing existing values',()=>{
 const fields=[{fieldToken:'os',type:'single_select',options:[{optionToken:'blocked',disabled:true},{optionToken:'linux'}]},{fieldToken:'multi',type:'multi_select',options:[{optionToken:'a'}]},{fieldToken:'address',type:'address',options:[{optionToken:'province',children:[{optionToken:'city'}]}]},{fieldToken:'count',type:'integer',minimumInteger:'0'},{fieldToken:'text',type:'text'},{fieldToken:'file',type:'file'},{fieldToken:'kept',type:'single_select',options:[{optionToken:'first'},{optionToken:'second'}]},{fieldToken:'disabled',type:'single_select',disabled:true,options:[{optionToken:'no'}]}].map(f=>({...f,visible:true}));
 const state={fields,values:[{fieldToken:'kept',type:'single_select',optionTokens:['second']}]};
 assert.equal(applyConfigurationDefaults(state),true);
 assert.deepEqual(state.values.find(v=>v.fieldToken==='os').optionTokens,['linux']);assert.deepEqual(state.values.find(v=>v.fieldToken==='address').addressOptionTokens,['province','city']);assert.equal(state.values.find(v=>v.fieldToken==='count').integerValue,'0');assert.deepEqual(state.values.find(v=>v.fieldToken==='kept').optionTokens,['second']);
 assert.equal(state.values.some(v=>['text','file','disabled'].includes(v.fieldToken)),false);assert.equal(applyConfigurationDefaults(state),false);
});

test('first sellable OS follows merchant order and never overrides an existing backend selection',()=>{
 const field={fieldToken:'os',type:'single_select',visible:true,options:[{optionToken:'alma-9',labelZhCN:'AlmaLinux 9.2'},{optionToken:'centos-9',labelZhCN:'CentOS 9 Stream'}]};
 const fresh={fields:[field],values:[]};assert.equal(applyConfigurationDefaults(fresh),true);assert.deepEqual(fresh.values[0].optionTokens,['alma-9']);
 const selected={fields:[field],values:[{fieldToken:'os',type:'single_select',optionTokens:['centos-9']}]};assert.equal(applyConfigurationDefaults(selected),false);assert.deepEqual(selected.values[0].optionTokens,['centos-9']);
});

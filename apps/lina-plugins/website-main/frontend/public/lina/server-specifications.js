import {localized} from './locale.js';
// Presentation aliases only. Values, candidates, constraints and pricing remain backend-owned.
const specifications=[
 ['region','区域',['区域','地区','模拟地区','Region','Simulated region']],
 ['os','操作系统',['操作系统','Operating system']],
 ['cpu','CPU',['CPU','CPU核心数','处理器核心数','CPU cores']],
 ['memory','内存',['内存','内存（GiB）','内存（GB）','Memory','Memory (GiB)']],
 ['systemDisk','系统盘',['系统盘','系统盘（GiB）','System disk (GiB)']],
 ['network','网络类型',['网络类型','Network type']],
 ['bandwidth','带宽',['带宽','带宽（Mbps）','上行带宽','上行带宽（Mbps）','Outbound bandwidth (Mbps)']],
 ['inbound','流入带宽',['流入带宽','流入带宽（Mbps）','Inbound bandwidth (Mbps)']],
 ['ip','IP数量',['IP数量','IPv4数量','IPv4 数量','IPv4 addresses']],
 ['disk','数据盘',['数据盘','数据盘（GiB）','数据盘（GiB，0表示不要）','Data disk (GiB)']],
 ['port','远程端口',['远程端口','Remote port']],
 ['hostname','主机名',['主机名','Hostname']],
 ['password','主机密码',['主机密码','密码','Host password','Password']],
];
const normalize=value=>String(value||'').replace(/\s+/g,'').replace(/（/g,'(').replace(/）/g,')').toLowerCase();
export const defaultUsageNotice='温馨提示：服务器禁止涉及色情、赌博、政治、诈骗等违法违规业务，请遵守所在地以及中国大陆法律法规，合法使用云产品资源。';

export const serverSpecificationOptions=({field})=>field.options||[];

export function serverSpecificationRows(fields=[]){
 const visible=fields.filter(field=>field.visible),used=new Set();
 const rows=specifications.map(([key,title,aliases])=>{
  const field=visible.find(candidate=>!used.has(candidate)&&((key==='os'&&candidate.presentation==='operating_system')||(key==='region'&&candidate.presentation==='region')||aliases.some(alias=>[candidate.labelZhCN,candidate.labelEnUS].some(name=>normalize(name)===normalize(alias)))));
  if(field){used.add(field);return {key,title,field};}
  return null;
 });
 // Keep familiar layout aliases, but never use them as a field whitelist.
 return rows.filter(Boolean).concat(visible.filter(field=>!used.has(field)).map(field=>({
  key:field.fieldToken,title:localized(field,'label'),field,
 })));
}

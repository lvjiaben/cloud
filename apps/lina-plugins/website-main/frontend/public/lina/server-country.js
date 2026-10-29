// Presentation metadata comes only from explicit country/region names, never product slugs.
const countries=[
 ['jp','日本',/日本|东京|大阪|\b(?:japan|tokyo|osaka)\b/i],
 ['us','美国',/美国|洛杉矶|西雅图|硅谷|纽约|\b(?:united states|usa|los angeles|seattle|silicon valley|new york)\b/i],
 ['cn','中国',/中国|江苏|镇江|上海|广东|广州|深圳|浙江|宁波|杭州|北京|天津|重庆|四川|成都|陕西|西安|山东|青岛|湖北|武汉|湖南|长沙|河南|郑州|河北|福建|厦门|贵州|贵阳|安徽|合肥|江西|南昌|广西|南宁|海南|海口|山西|太原|辽宁|沈阳|吉林|长春|黑龙江|哈尔滨|云南|昆明|甘肃|兰州|青海|西宁|宁夏|银川|新疆|乌鲁木齐|西藏|拉萨|内蒙古|呼和浩特|\b(?:china|jiangsu|zhenjiang|shanghai|guangdong|guangzhou|shenzhen|zhejiang|ningbo|hangzhou|beijing|tianjin|chongqing|sichuan|chengdu|shaanxi|xi.?an)\b/i],
];
export function serverCountry(value){
 const text=typeof value==='string'?value:[value?.nameZhCN,value?.nameEnUS,value?.labelZhCN,value?.labelEnUS].filter(Boolean).join(' ');
 const matches=countries.filter(([, ,pattern])=>pattern.test(text));
 return matches.length===1?{code:matches[0][0],name:matches[0][1]}:null;
}
export function prependServerFlag(element,value){
 element.querySelector(':scope > .lina-server-flag')?.remove();
 const country=serverCountry(value);if(!country)return;
 const img=element.ownerDocument.createElement('img');
 img.className='lina-server-flag';img.src='/lina/flags/'+country.code+'.svg';img.alt=country.name+'国旗';img.width=30;img.height=20;img.decoding='async';
 element.prepend(img);
}

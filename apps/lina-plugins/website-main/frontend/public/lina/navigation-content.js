// Presentation copy belongs to this website; destinations come from our published tree.
import {translate} from './locale.js';

// Canonical names identify menus even when different entries share an English translation.
const canonicalLabels=new WeakMap();
const displayLabels=new WeakMap();
export const navigationLabel=item=>canonicalLabels.get(item)||(typeof item.label==='string'?item.label:item.label?.zhCN||item.label?.enUS||'');
// Preserve the exact Chinese source for the observer; duplicate English labels are not reversible.
export const navigationDisplayLabel=item=>displayLabels.get(item)??item.label;
const text=navigationLabel;
const entry=(label,description,href='/404.html',details={})=>{
 const item={...details,href,get label(){return translate(label);}};
 canonicalLabels.set(item,label);
 if(description!==undefined)Object.defineProperty(item,'description',{enumerable:true,get:()=>translate(description)});
 return item;
};
const solutions=[
 entry('电商解决方案','了解线上商城与数字化经营的应用场景。','/plan/electricity.html'),
 entry('网站解决方案','了解网站部署与运行的应用场景。','/plan/web.html'),
 entry('游戏解决方案','了解游戏业务部署的应用场景。','/plan/game.html'),
 entry('安全解决方案','了解数据保护与业务安全的应用场景。','/plan/safety.html'),
 entry('出海解决方案','了解海外业务部署的应用场景。','/plan/Goingtosea.html'),
 entry('金融解决方案','了解金融业务的应用场景。','/plan/financial.html'),
 entry('政企解决方案','了解政企数字化的应用场景。','/plan/government.html'),
];
const templates=[
 entry('产品与服务',undefined,'/products.html'),
 entry('解决方案','探索不同行业与通用业务的应用场景。',undefined,{group:true,children:[
  entry('行业解决方案','按行业查找应用场景。',undefined,{group:true,children:solutions}),
  entry('通用解决方案','了解通用业务的部署场景。',undefined,{group:true,children:[entry('工业解决方案','了解工业数字化的应用场景。','/plan/government.html')]}),
 ]}),
 entry('生态合作','了解合作方式，探索共同发展的机会。',undefined,{group:true,promotion:true,children:[
  entry('百站赞助公益计划','了解公益赞助计划及相关信息。','/webmaster.html',{get badge(){return translate('赞助');}}),
  entry('推广返利','查看推广计划、参与方式与适用规则。','/aff.html'),
  entry('合作伙伴','共同发展，实现互利共赢的合作伙伴关系','/cooperation/partner.html'),
 ]}),
 entry('应用市场',undefined,'/404.html'),
 entry('支持与服务','查找使用指南、下载资源与联系渠道。',undefined,{group:true,children:[
  entry('联系我们','欢迎与我们沟通，查找本方联系方式。','/contact.html'),
  entry('服务保障','了解服务保障范围及相关说明。','/Assure.html'),
  entry('帮助文档','查阅使用指南和常见问题解答。','/help.html'),
  entry('服务条款','查看服务范围、使用规则与相关条款。','/terms.html'),
  entry('下载中心','查找已发布的文件与下载资源。','/downloads.html'),
  entry('宝塔面板','查找面板安装及使用的相关资料。','/bt.html'),
 ]}),
 entry('了解我们','了解公司信息，查看新闻动态与公开资料。',undefined,{group:true,children:[
  entry('公司简介','了解公司的介绍、使命和价值观。','/about.html'),
  entry('新闻动态','浏览本方发布的新闻与最新资讯。','/news.html'),
  entry('荣誉资质','查看已公开的资质与荣誉资料。','/certificate.html'),
  entry('增值许可证查询','查找公开的增值业务许可证资料。','https://dxyw.miit.gov.cn/dxxzsp/xkz/xkzgl/resource/qiyesearch.jsp?num=B1-xxx&type=xuke',{external:true}),
 ]}),
];
// No source-site or unrelated external targets can enter the presentation layer.
function destination(item,origins){
 const href=item.href||'';
 if(href.startsWith('/')&&!href.startsWith('//'))return {...item};
 try{const url=new URL(href);if(origins.includes(url.origin)&&!url.username&&!url.password&&['http:','https:'].includes(url.protocol))return {...item};}catch{}
 return {...item,href:'/404.html',customer:false,external:false};
}
export function composeNavigation(published=[],origins=[],copyTemplates){
 const activeTemplates=copyTemplates||templates;
 const merge=(template,source)=>{
  const supplied=(source||[]).find(item=>text(item)===text(template)||(text(template)==='荣誉资质'&&text(item)==='资质荣誉'));
  const result={...template,...(supplied?destination(supplied,origins):{}),description:template.description};
  canonicalLabels.set(result,text(template));
  displayLabels.set(result,copyTemplates?template.label:supplied?supplied.label:text(template));
  if(copyTemplates)result.label=template.label;
  if(text(template)==='产品与服务'){result.href='/products.html';result.customer=false;}
  if(text(template)==='服务条款'){result.href='/terms.html';result.customer=false;result.external=false;}
  if(text(template)==='宝塔面板'){result.href='/bt.html';result.customer=false;result.external=false;}
  if(text(template)==='服务保障'){result.href='/Assure.html';result.customer=false;result.external=false;}
  if(text(template)==='公司简介'){result.href='/about.html';result.customer=false;result.external=false;}
  if(['荣誉资质','资质荣誉'].includes(text(template))){result.label=template.label;result.href='/certificate.html';result.customer=false;result.external=false;}
  if(text(template)==='推广返利'){result.href='/aff.html';result.customer=false;result.external=false;}
  if(text(template)==='百站赞助公益计划'){result.href='/webmaster.html';result.customer=false;result.external=false;}
  if(text(template)==='合作伙伴'){result.href='/cooperation/partner.html';result.customer=false;result.external=false;}
  if(text(template)==='增值许可证查询'){result.href='https://dxyw.miit.gov.cn/dxxzsp/xkz/xkzgl/resource/qiyesearch.jsp?num=B1-xxx&type=xuke';result.customer=false;result.external=true;}
  if(text(template)==='电商解决方案'){result.href='/plan/electricity.html';result.customer=false;result.external=false;}
  if(text(template)==='网站解决方案'){result.href='/plan/web.html';result.customer=false;result.external=false;}
  if(text(template)==='游戏解决方案'){result.href='/plan/game.html';result.customer=false;result.external=false;}
  if(text(template)==='金融解决方案'){result.href='/plan/financial.html';result.customer=false;result.external=false;}
 if(text(template)==='安全解决方案'){result.href='/plan/safety.html';result.customer=false;result.external=false;}
 if(text(template)==='政企解决方案'){result.href='/plan/government.html';result.customer=false;result.external=false;}
 if(text(template)==='工业解决方案'){result.href='/plan/government.html';result.customer=false;result.external=false;}
  if(text(template)==='出海解决方案'){result.href='/plan/Goingtosea.html';result.customer=false;result.external=false;}
  if(template.children){
   // A configured leaf cannot replace one of the six expandable menus.
   result.group=true;result.href='';
   const children=supplied?.children||[];
   result.children=template.children.map(child=>merge(child,children));
   if(!copyTemplates)for(const child of children)if(!template.children.some(known=>text(known)===text(child)||(text(known)==='荣誉资质'&&text(child)==='资质荣誉')))result.children.push(clean(child));
  }
  return result;
 };
 const clean=item=>({...destination(item,origins),...(item.children?{children:item.children.map(clean)}:{})});
 const result=activeTemplates.map(template=>merge(template,published));
 if(copyTemplates)return result;
 // Keep our home link and administrator-defined extensions without copying business data.
 const extras=published.filter(item=>!templates.some(template=>text(template)===text(item))).map(clean);
 return [...extras.filter(item=>item.href==='/'),...result,...extras.filter(item=>item.href!=='/')];
}

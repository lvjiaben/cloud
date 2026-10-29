// Isolated, explicitly labelled synthetic presentation. Never seed a business DB.
export const previewBrand={siteName:{zhCN:'模拟预览 · 官网模板'},primaryColor:'#0052d9',
 heroTitle:{zhCN:'模拟预览：官网首页'},heroDescription:{zhCN:'沿用本地原页面结构。以下内容仅供桌面布局和交互验收，不是真实商品或服务承诺。'},heroImage:'/static/picture/remoteL16905328288110.svg',
 navigation:[['首页','/index.html'],['产品与服务','/products.html'],['新闻公告','/news.html'],['帮助文档','/help.html'],['资源下载','/downloads.html'],['联系我们','/contact.html']].map(([label,href])=>({label:{zhCN:label},href})),
 footer:{zhCN:'模拟预览，全部示例均不代表正式业务。公司、联系、备案和资质资料未提供。'},templateConfig:{}};
const config=previewBrand.templateConfig;
config.hero1href='/products.html';
config.productintrotitle='产品介绍（模拟）';config.productintrodescription='此处保留原商品左侧介绍卡；所有商品由目录接口生成，不使用原站套餐。';
for(let i=2;i<=4;i++){config['hero'+i+'title']='模拟轮播 '+i+'：原页面交互';config['hero'+i+'description']='可用左右箭头、标题、圆点和键盘切换；这些资料未写入后台。';config['hero'+i+'image']='/static/picture/remoteL16905328288110.svg';config['hero'+i+'href']='/products.html';}
for(const [prefix,count,name]of [['shortcut',4,'模拟快捷入口'],['solution',3,'模拟方案'],['benefit',4,'模拟能力说明'],['region',3,'模拟地域说明'],['partner',6,'模拟伙伴'],['promise',4,'模拟页脚说明']])for(let i=1;i<=count;i++){
 config[prefix+i+'title']=name+' '+i;config[prefix+i+'description']='用于检查原模板布局，不对应已发布业务或任何服务保证。';config[prefix+i+'href']='/products.html';
}
config.solutionheading='方案区域（模拟内容）';config.solutiondescription='保留原侧卡、页签和内容区域；实际经营资料尚未配置。';
config.benefitheading='服务能力区域（模拟内容）';config.benefitdescription='保留原四卡结构，仅检查排版，不代表本方服务承诺。';
config.regionheading='地域区域（模拟内容）';config.regiondescription='仅展示配置文本，不代表商品所在机房；地图坐标和业务地域接口未接入。';config.partnerheading='合作伙伴区域（模拟名称）';
// Exercise the actual bounded owner configuration format used in the package.
for(const [key,prefix,count,start,columns]of [
 ['heroslides','hero',3,2,['title','description','image','href']],['shortcuts','shortcut',4,1,['title','description','href']],
 ['solutions','solution',3,1,['title','description','image','href']],['benefits','benefit',4,1,['title','description','image']],
 ['regions','region',3,1,['title','description']],['partners','partner',6,1,['title','image']],['promises','promise',4,1,['title','href']]
]){config[key]=Array.from({length:count},(_,i)=>columns.map(column=>config[prefix+(start+i)+column]||'-').join(' | ')).join('\n');for(let i=start;i<start+count;i++)for(const column of columns)delete config[prefix+i+column];}
export const previewCategories=Array.from({length:4},(_,i)=>({categoryNo:'preview-category-'+i,nameZhCN:'模拟分类 '+(i+1)}));
export const previewProducts=Array.from({length:12},(_,i)=>({publicSlug:'preview-product-'+i,nameZhCN:'模拟商品 '+String(i+1).padStart(2,'0'),summaryZhCN:'仅用于展示原产品卡片布局。配置、售价和开通均不代表实际业务。',categoryNo:previewCategories[i%4].categoryNo,availability:'available',currencies:['CNY']}));
export function previewResponse(url,method){
 if(!['GET','HEAD'].includes(method))return {status:403,data:null,message:'模拟预览禁止业务写入，未提交任何购买。'};
 if(url.pathname==='/x/idc-portal/api/v1/public/brand')return {status:200,data:{brand:previewBrand}};
 if(url.pathname==='/x/idc-core/api/v1/public/catalog/products'){
  const rows=previewProducts.filter(p=>(!url.searchParams.get('categoryNo')||p.categoryNo===url.searchParams.get('categoryNo'))&&(!url.searchParams.get('namePrefix')||p.nameZhCN.startsWith(url.searchParams.get('namePrefix'))));
  return {status:200,data:{products:rows,categories:previewCategories,nextCursor:'',availableCurrencies:['CNY']}};
 }
 if(url.pathname==='/x/idc-core/api/v1/public/content/items'){
  const kind=url.searchParams.get('type')||'news';return {status:200,data:{items:Array.from({length:6},(_,i)=>({contentNo:'preview-'+kind+'-'+i,title:'模拟'+(kind==='announcement'?'公告':'文章')+' '+(i+1)+'：首页原模板展示',summary:'仅供原页面排版和悬停交互检查，不是正式发布内容。',publishedAt:1792200000000-i*86400000})),nextCursor:''}};
 }
 return {status:404,data:null,message:'该业务未接入模拟预览；真实验收请使用6680。'};
}

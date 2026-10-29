import {electricityHTML} from './electricity-content.js';
import {localized,localizeAssetImage} from './locale.js';

export function initializeElectricity(document){
 const root=document.querySelector('[data-electricity]');if(!root)return;
 root.innerHTML=electricityHTML;
 localizeAssetImage(root.querySelector('.architecture-item-img img'),'/lina/electricity-assets/architecture.svg','/lina/electricity-assets/architecture.en.svg');
 const view=document.defaultView,bar=root.querySelector('.et-container'),links=[...bar.querySelectorAll('.et-hero-tab a')];
 const sections=[...root.querySelectorAll('.et-slide')],slider=bar.querySelector('.et-hero-tab-slider');
 const marker=document.createElement('div');marker.className='electricity-nav-marker';bar.before(marker);
 let frame;
 function update(){
  frame=undefined;const stuck=marker.getBoundingClientRect().top<=0;
  bar.classList.toggle('is-stuck',stuck);document.body.classList.toggle('electricity-reading',stuck);
  const offset=bar.getBoundingClientRect().height+12;
  let index=0;for(let i=0;i<sections.length;i++)if(sections[i].getBoundingClientRect().top<=offset)index=i;
  links.forEach((a,i)=>{a.parentElement.classList.toggle('active',i===index);if(i===index)a.setAttribute('aria-current','location');else a.removeAttribute('aria-current');});
  const current=links[index],track=bar.querySelector('.et-hero-tabs-container');
  slider.style.width=current.getBoundingClientRect().width+'px';
  slider.style.left=(current.getBoundingClientRect().left-track.getBoundingClientRect().left)+'px';
 }
 const schedule=()=>{frame??=view.requestAnimationFrame(update);};
 links.forEach((a,index)=>a.addEventListener('click',event=>{
  event.preventDefault();const top=view.scrollY+sections[index].getBoundingClientRect().top-bar.getBoundingClientRect().height;
  view.history.replaceState(null,'',a.getAttribute('href'));
  view.scrollTo({top,behavior:view.matchMedia('(prefers-reduced-motion: reduce)').matches?'instant':'smooth'});
 }));
 view.addEventListener('scroll',schedule,{passive:true});view.addEventListener('resize',schedule);
 document.addEventListener('lina:locale-changed',schedule);
 const resize=new view.ResizeObserver(schedule);resize.observe(bar);
 view.addEventListener('pagehide',()=>{view.cancelAnimationFrame(frame);resize.disconnect();view.removeEventListener('scroll',schedule);view.removeEventListener('resize',schedule);},{once:true});
 view.addEventListener('pageshow',()=>{resize.observe(bar);view.addEventListener('scroll',schedule,{passive:true});view.addEventListener('resize',schedule);schedule();});
 // Reuse the website's explicit copy registry; no third-party translation or business API.
 for(const pair of electricityCopy)localized({zhCN:pair[0],enUS:pair[1]});
 if(view.location.hash){const index=sections.findIndex(s=>'#'+s.id===view.location.hash);if(index>=0)view.requestAnimationFrame(()=>view.scrollTo({top:view.scrollY+sections[index].getBoundingClientRect().top-bar.getBoundingClientRect().height,behavior:'instant'}));}
 schedule();
}

const electricityCopy=[
 ['电商云解决方案','E-commerce cloud solution'],['我们的优势','Our advantages'],['面临的问题','Challenges'],['解决方案架构','Solution architecture'],['相关产品','Related products'],['更多方案','More solutions'],['页面分区','Page sections'],
 ['高效、安全、可扩展的电商云解决方案，帮助您实现全面电商转型，提升销售业绩，满足用户购物需求。','Efficient, secure and scalable cloud solutions help you transform your e-commerce business, improve sales and meet customer shopping needs.'],
 ['方案定制','Customize a solution'],['创新技术、高品质产品、卓越服务','Innovative technology, quality products and exceptional service'],['高效运维','Efficient operations'],['全景备份','Comprehensive backup'],['弹性扩展','Elastic scaling'],
 ['365天7*24小时高效运维服务，提供专业环境搭建、系统搭建、主动监控、实时服务的贴心服务。','24/7 operations throughout the year, with professional environment and system setup, proactive monitoring and real-time support.'],
 ['服务器均采用 Intel Haswell CPU、DDR4 内存，拥有更好的内存计算能力；默认开通 I/O 优化实例，搭配 SSD 固态云盘获得更好的存储性能。','Servers use Intel Haswell CPUs and DDR4 memory for improved computing performance. I/O-optimized instances and SSD cloud disks provide better storage performance.'],
 ['在面对大促、活动、以及大型市场活动时期流量激增问题，针对电商云用户个性的临时资源扩展服务，秒级扩展IT资源，轻松应对业务激增，帮助企业节约成本。','Scale IT resources in seconds during promotions, events and major marketing campaigns, handling traffic surges with temporary resources tailored to e-commerce needs while reducing costs.'],
 ['提升网店性能，解决流量高峰，确保稳定运营，提供良好的用户体验。','Improve store performance, handle peak traffic, maintain stable operations and deliver a better user experience.'],['服务器性能','Server performance'],['流量管理','Traffic management'],['稳定运营','Stable operations'],['提高用户体验','Improve user experience'],['1V1 咨询','1-to-1 consultation'],
 ['应对业务浪涌风险高','High risk from traffic surges'],['用户体验差','Poor user experience'],['商业决策缺乏数据支撑','Business decisions lack data'],['安全性难以保证','Security challenges'],
 ['促销、秒杀、爆款等电商业务场景，瞬间访问量达到平常的几十至数百倍，导致服务器负载高，系统响应慢；瞬间的访问流量暴增，导致带宽占满、数据库瘫痪等，以致整个系统服务不可用。','Promotions, flash sales and popular products can drive traffic to tens or hundreds of times normal levels, overloading servers, slowing responses, saturating bandwidth and overwhelming databases until services become unavailable.'],
 ['电商业务涉及大量的静态数据，如产品图片，产品视频等，这些数据按照传统的方式放在服务器中，加载速度慢、耗时费钱，不同网络用户访问电商网站出现网页打开慢、网络延时等问题。','E-commerce relies on static data such as product images and videos. Traditional server storage can be slow and expensive, causing slow page loads and network delays for visitors on different networks.'],
 ['由于缺乏大数据平台及分析工具，无法对已有用户、商品、交易数据进行有效分析，导致电商网站存在推广投入高，用户二次下单率低等问题。','Without data platforms and analytics tools, user, product and transaction data cannot be analyzed effectively, resulting in higher marketing costs and lower repeat purchase rates.'],
 ['电商在站外引流、注册登录、浏览比较、获取优惠、下单、支付、交付及评价等环节存在撞库爆破、薅羊毛、黄牛倒卖、网页篡改、DDoS攻击、账号泄露、木马植入等一系列风险。','Customer acquisition, registration, browsing, promotions, ordering, payment, delivery and reviews face risks including credential attacks, promotion abuse, scalping, page tampering, DDoS attacks, account leaks and malware.'],
 ['提供高效解决方案，满足不同需求。','Efficient solutions for different needs.'],['方案架构','Architecture'],['架构优势','Architecture advantages'],
 ['服务电商云解决方案具备高弹性可扩展性、帮助电商企业快速实现平台搭建、节约成本、应对高业务并发、强化安全防护能力等目标，助力电商用户快速实现互联网金融创新。','The e-commerce cloud solution provides elastic scaling to help businesses launch platforms quickly, reduce costs, handle concurrent traffic and strengthen security, supporting rapid innovation in internet finance.'],
 ['安全可靠的运行环境','A secure and reliable environment'],['可承载高并发性','High concurrency support'],['更高的I/O读写性能存储','Higher I/O read and write performance'],['强大运维管理平台','Powerful operations management'],['服务咨询 →','Service consultation →'],
 ['根据您的业务为您推荐相关产品','Related products for your business'],['他们正使用我们的产品和服务','They use our products and services'],['暂无已发布的合作伙伴。','No partners have been published.'],
 ['网站云解决方案','Website cloud solution'],['游戏云解决方案','Gaming cloud solution'],['移动云解决方案','Mobile cloud solution'],['安全解决方案','Security solution'],
];

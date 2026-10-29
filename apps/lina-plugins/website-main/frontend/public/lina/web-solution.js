import {webSolutionHTML} from './web-solution-content.js';
import {localized} from './locale.js';

export function initializeWebSolution(document){
 const root=document.querySelector('[data-web-solution]');if(!root)return null;
 for(const [zhCN,enUS] of copy)localized({zhCN,enUS});
 root.innerHTML=webSolutionHTML;
 const view=document.defaultView,bar=root.querySelector('.et-container'),links=[...bar.querySelectorAll('.et-hero-tab a')],sections=[...root.querySelectorAll('.et-slide')],slider=bar.querySelector('.et-hero-tab-slider');
 const marker=document.createElement('div');marker.className='web-nav-marker';bar.before(marker);
 const reduced=view.matchMedia('(prefers-reduced-motion: reduce)');let frame,brand;
 function schedule(){frame??=view.requestAnimationFrame(update);}
 function update(){
  frame=undefined;const stuck=marker.getBoundingClientRect().top<=0;bar.classList.toggle('is-stuck',stuck);document.body.classList.toggle('web-reading',stuck);
  const offset=bar.getBoundingClientRect().height+14;let index=0;
  sections.forEach((s,i)=>{if(s.getBoundingClientRect().top<=offset)index=i;});
  links.forEach((a,i)=>{a.parentElement.classList.toggle('active',i===index);if(i===index)a.setAttribute('aria-current','location');else a.removeAttribute('aria-current');});
  const track=bar.querySelector('ul'),current=links[index];slider.style.width=current.getBoundingClientRect().width+'px';slider.style.left=current.offsetLeft+'px';
  if(track.scrollWidth>track.clientWidth){const x=current.offsetLeft-(track.clientWidth-current.offsetWidth)/2;if(Math.abs(track.scrollLeft-x)>10)track.scrollTo({left:Math.max(0,x),behavior:reduced.matches?'instant':'smooth'});}
 }
 function navigate(hash,smooth=false){
  const section=sections.find(s=>'#'+s.id===hash);if(!section)return;
  view.scrollTo({top:view.scrollY+section.getBoundingClientRect().top-bar.getBoundingClientRect().height,behavior:smooth&&!reduced.matches?'smooth':'instant'});schedule();
 }
 links.forEach(a=>a.addEventListener('click',event=>{event.preventDefault();const hash=a.getAttribute('href');if(view.location.hash!==hash)view.history.pushState(null,'',hash);navigate(hash,true);}));
 const restore=()=>navigate(view.location.hash);view.addEventListener('popstate',restore);view.addEventListener('hashchange',restore);
 view.addEventListener('scroll',schedule,{passive:true});view.addEventListener('resize',schedule);
 const resize=new view.ResizeObserver(schedule);resize.observe(bar);
 const reveals=[...root.querySelectorAll('.web-reveal')];
 const observer=new view.IntersectionObserver(entries=>entries.forEach(e=>{if(e.isIntersecting){e.target.classList.add('is-visible');observer.unobserve(e.target);}}),{threshold:0.08});
 function motion(){reveals.forEach(e=>{if(reduced.matches){e.classList.add('is-visible');observer.unobserve(e);}else if(!e.classList.contains('is-visible'))observer.observe(e);});}
 reduced.addEventListener('change',motion);motion();
 const t=zh=>localized({zhCN:zh,enUS:copy.find(p=>p[0]===zh)?.[1]||zh});
 function title(){if(brand)document.title=t('网站解决方案')+' - '+localized(brand.siteName);}
 document.addEventListener('lina:locale-changed',()=>{title();schedule();});
 view.addEventListener('pageshow',()=>{resize.observe(bar);motion();restore();schedule();});
 view.addEventListener('pagehide',()=>{view.cancelAnimationFrame(frame);frame=undefined;resize.disconnect();observer.disconnect();});
 // Preserve history restoration and hash alignment after local fonts have loaded.
 view.requestAnimationFrame(restore);document.fonts?.ready.then(()=>{restore();schedule();});
 schedule();
 return {renderBrand(value){brand=value;title();}};
}

const copy=[
 ['网站解决方案','Website solution'],['网站建设服务说明尚待确认。','Website construction service details are awaiting confirmation.'],
 ['我们的优势','Our advantages'],['面临的问题','Challenges'],['解决方案架构','Solution architecture'],['相关产品','Related products'],['合作伙伴','Partners'],['更多方案','More solutions'],['页面分区','Page sections'],
 ['创新技术、高品质产品、卓越服务','Innovative technology, quality products and exceptional service'],['弹性扩展，节约成本','Elastic scaling, lower costs'],['全景备份','Comprehensive backup'],['自带防御','Built-in protection'],['待确认','Awaiting confirmation'],
 ['以下为待核对的服务主题，尚未确认为本方可提供的服务。','The following service topics are under review and are not confirmed as services we provide.'],['服务能力说明尚待确认，暂不作为可用服务承诺。','Service capabilities await confirmation and are not a commitment of availability.'],
 ['用户常面临的问题包括安全威胁、性能瓶颈、用户体验差这直接影响了网站的可靠性、安全性和用户满意度。','Common challenges include security threats, performance bottlenecks and poor user experience, which directly affect website reliability, security and user satisfaction.'],
 ['安全威胁','Security threats'],['性能优化','Performance optimization'],['用户体验','User experience'],['数据保护','Data protection'],['合规性','Compliance'],['1V1 咨询','1-to-1 consultation'],['方案定制','Customize a solution'],
 ['配置选型','Configuration selection'],['DDOS攻击','DDoS attacks'],['数据备份','Data backup'],['IO读写','I/O reads and writes'],
 ['如果用户已拥有网站程序，那么就需根据实际业务情况选择合适的云服务器，并在服务器上部署程序运行所需要的环境。如果用户初次使用云服务，那么如何选择CPU，硬盘，内存，带宽的大小就成为用户首先遇到的问题。','If you already have a website application, you need to select a suitable cloud server for your business and deploy the environment required to run it. For first-time cloud users, choosing CPU, disk, memory and bandwidth capacity is the first challenge.'],
 ['如果用户顺利部署了网站在云服务器上，如何保障网站安全稳定的运行，是所有网站管理者都关心的问题。但是如果自行搭建安全体系往往会让成本成倍的增长，而且很复杂。无疑给用户带来更大的成本以及时间压力。','After deploying a website on a cloud server, every administrator is concerned about keeping it secure and stable. Building a security system independently is often complex and can multiply costs, adding financial and time pressure.'],
 ['随着网站的发展，积累的数据越来越多，比如静态文件（代码、图片等）和数据库数据。如果需要用户实时自行备份，那所花费的时间以及精力无疑也会成倍增长。所以解决数据备份又是企业以及开发者面临的问题之一！','As a website grows, it accumulates more data, including static files such as code and images, and database records. Performing real-time backups independently can multiply the time and effort required, making data backup another challenge for businesses and developers.'],
 ['大部分网站都属于交互型网站，速度快，能让多处理器的性能发挥出来。当前技术条件下，I/O是系统性能提高的瓶颈，I/O问题没有解决好， 处理器数量增加不一定带来性能的提升，新增资源有可能被I/O全部消耗掉。','Most websites are interactive. Fast execution helps make use of multiple processors. Under current technical conditions, I/O is a bottleneck to improving system performance. Without solving I/O issues, adding processors may not improve performance, and I/O can consume all the additional resources.'],
 ['提供高效解决方案，满足不同需求。','Efficient solutions for different needs.'],['方案架构','Architecture'],['架构优势','Architecture advantages'],['架构能力说明尚待确认。','Architecture capabilities are awaiting confirmation.'],['服务咨询 →','Service consultation →'],
 ['概念示意，实际部署与服务能力以双方确认的方案为准。','Concept only. Actual deployment and service capabilities depend on the mutually agreed plan.'],['网站部署概念示意图，不代表已提供的服务能力','Conceptual website deployment diagram; not a statement of available services'],
 ['根据您的业务为您推荐相关产品','Related products for your business'],['展示后台已发布商品，适用性请咨询确认。','Published catalog products are shown here. Please confirm their suitability with us.'],['他们正使用我们的产品和服务','They use our products and services'],['暂无已发布的合作伙伴。','No partners have been published.'],
 ['电商云解决方案','E-commerce cloud solution'],['网站云解决方案','Website cloud solution'],['游戏云解决方案','Gaming cloud solution'],['移动云解决方案','Mobile cloud solution'],['安全解决方案','Security solution'],
 ['正在加载商品…','Loading products…'],['商品加载失败，请重试。','Products could not be loaded. Please try again.'],['暂无已发布商品。','No products have been published.'],['重新加载','Retry'],['默认配置月费','Default configuration per month'],['选择配置','Choose configuration'],['查看商品','View product'],
 ['基于优质基础资源，提供网站建设的一站式服务，涵盖PC站、手机站、H5站、公众号等多种类型，通过私密独立系统满足各行业客户网站建设需求，助力政企客户信息化转型。','A one-stop website construction service based on quality infrastructure for PC sites, mobile sites, H5 sites and public accounts, helping government and enterprise customers advance their digital transformation through private independent systems.'],
 ['当网站用户在面对云服务器配置选型的问题时，针对网站用户个性的配置选择需求、同时可临时资源扩展的诉求，支持产品随时随地秒级扩展IT资源，轻松解决配置选型问题，高效便捷，节约成本。','When website users need to choose cloud server configurations or temporarily expand resources, products support second-level IT resource expansion anytime and anywhere, making configuration selection efficient and cost-effective.'],
 ['针对网站用户面临的备份问题，服务推出高效便捷的实时全景备份/全景恢复功能。用户可以自定义备份事件，通过控制台一键备份，使数据安全在得到最大的保证的同时，降低用户备份成本。','For website backup needs, the service provides efficient real-time full backup and recovery. Users can customize backup events and back up with one click in the console, improving data safety while reducing costs.'],
 ['随着互联网的发展，DDOS、CC攻击已成为非常普遍的攻击模式。网站安全稳定运行成为了每个用户的愿望！网站解决方案，针对这一问题，各个已开放的节点自带默认防御峰值。真正从根本上解决网络安全问题，保护用户网站免受攻击威胁！','As the internet has developed, DDoS and CC attacks have become common. The website solution provides default protection peaks at each available node to protect websites from attack threats.'],
 ['为企业以及开发者用户实现灵活弹性自动化的基础IT设施建设， 把传统的IDC数据中心改造成了一个高度简化、标准化、自动化和弹性灵活的云数据中心。使得企业以及开发者的IT支撑系统从“成本中心”而转型成为推动企业核心业务不断发展的引擎。','For enterprises and developers, build flexible, elastic and automated infrastructure and transform traditional IDC data centers into simplified, standardized, automated and elastic cloud data centers that drive core business growth.'],
 ['资源灵活调整、业务分离、可动态扩展','Flexible resource adjustment, business separation and dynamic scaling'],['高可用、故障转移、高并发性','High availability, failover and high concurrency'],['访问数据加密','Encrypted data access'],['动态负载均衡','Dynamic load balancing'],
];

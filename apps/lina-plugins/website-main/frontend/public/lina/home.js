import {localized,localizedDate} from './locale.js';
import {readPublicContentList} from './public-content-client.js';
// Fixed homepage presentation; news and products retain their own public APIs.
import {renderTemplateHome} from "./home-template.js";
export function renderHomeBrand(document){renderTemplateHome(document);}

export async function loadHomeNews(document,request,safeURL){
 const status=document.querySelector('[data-home-news-status]'),retry=document.querySelector('[data-home-news-retry]');
 const slot=document.querySelector('[data-home-news]');
 retry.hidden=true;retry.disabled=true;slot.replaceChildren();
 status.textContent='正在加载新闻与公告…';
 try{
  // Two bounded collection calls, no per-item detail request waterfall.
  const results=await Promise.allSettled(['news','announcement'].map(type=>readPublicContentList(request,{type,limit:6})));
  const items=results.flatMap((result,index)=>result.status==='fulfilled'?(result.value.items||[]).map(item=>({...item,kind:index===0?'新闻':'公告'})):[])
   .sort((a,b)=>(b.publishedAt||0)-(a.publishedAt||0)).slice(0,6);
  for(const [index,item]of items.entries()){
   const href=safeURL('/content.html?id='+encodeURIComponent(item.contentNo));
   const card=document.querySelector('#home-news-card').content.firstElementChild.cloneNode(true);
   card.href=href.href;card.querySelector('.news-article-item-title span').textContent=localized(item.category,'name')||item.kind;
   card.querySelector('.news-article-item-title p').textContent=item.title;
   card.querySelector('.news-article-item-desc span').textContent=item.summary||'';
   const date=new Date(item.publishedAt);card.querySelector('.news-article-item-date').textContent=item.publishedAt&&!Number.isNaN(date.getTime())?localizedDate(item.publishedAt):'';
   if(index===0)card.classList.add('active');
   const activate=()=>{for(const sibling of slot.children)sibling.classList.toggle('active',sibling===card);};
   let hoverTimer;card.addEventListener('mouseenter',()=>{hoverTimer=setTimeout(activate,388);});card.addEventListener('mouseleave',()=>clearTimeout(hoverTimer));card.addEventListener('focus',activate);
   slot.append(card);
  }
  const failed=results.some(result=>result.status==='rejected');
  status.textContent=failed?(items.length?'部分内容加载失败，可重试。':'新闻与公告加载失败，请重试。'):items.length?'':'暂无已发布的新闻与公告。';
  retry.hidden=!failed;
 }catch{status.textContent='新闻与公告加载失败，请重试。';retry.hidden=false;}
 finally{retry.disabled=false;}
}

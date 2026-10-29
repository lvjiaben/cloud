// Directly project the local article page; no source text, business links or scripts.
export function contentTemplate(document){
 const main=document.querySelector('.et-main.articleContent')?.cloneNode(true);
 if(!main?.querySelector('.news-txt-content'))throw new Error('Original article structure is missing');
 const clear=element=>{for(const child of [...element.childNodes]){if(child.nodeType===1){for(const attr of [...child.attributes])if(attr.name!=='class')child.removeAttribute(attr.name);clear(child);}else child.remove();}};
 for(const attr of [...main.attributes])if(attr.name!=='class')main.removeAttribute(attr.name);
 main.querySelectorAll('script,style,iframe,object,svg').forEach(el=>el.remove());clear(main);
 // Restore the reviewed article sidebar width only; never copy source inline styles.
 main.querySelector('.main-row').style.setProperty('--news_left_w','328px');
 main.querySelector('.breadcrumb').innerHTML='<a href="/news.html" data-content-parent>内容列表</a><span aria-hidden="true">&gt;</span><span data-content-category hidden></span><span data-content-breadcrumb>内容详情</span>';
 const title=main.querySelector('.news-txt-title h1');title.setAttribute('data-page-title','');
 const info=main.querySelector('.news-txt-title-info');info.hidden=true;
 info.innerHTML='<span class="news-txt-title-info-time" data-content-time hidden></span><span data-content-category hidden></span>';
 const body=main.querySelector('.news-txt-content');body.replaceChildren();body.setAttribute('data-content-body','');
 main.querySelector('.news-txtpaging')?.remove();
 body.insertAdjacentHTML('afterend','<ul data-content-list hidden></ul><p data-status role="status">正在加载内容…</p><button type="button" data-more hidden>加载更多</button>');
 const groups=main.querySelectorAll('.news_righ_item');
 groups[0].querySelector('.article-module-title').textContent='更多栏目';
 groups[0].querySelector('.create-article').innerHTML='<a href="/news.html"><img src="static/picture/sidebar-newspaper.svg" alt="" width="38" height="38"><span>新闻动态</span></a><a href="/announcements.html"><img src="static/picture/sidebar-megaphone.svg" alt="" width="38" height="38"><span>公告</span></a><a href="/help.html"><img src="static/picture/sidebar-book-open.svg" alt="" width="38" height="38"><span>帮助文档</span></a><a href="/downloads.html"><img src="static/picture/sidebar-download.svg" alt="" width="38" height="38"><span>资源下载</span></a>';
 groups[1].querySelector('.article-module-title').textContent='目录结构';
 const directory=groups[1].querySelector('.directoryStructure');directory.replaceChildren();directory.setAttribute('data-content-directory','');
 directory.innerHTML='<span class="nothing">全文</span>';
 const tools=main.querySelector('.social-icons'),original=[...tools.children];tools.replaceChildren();
 for(const [index,action,label,file] of [[2,'copy','分享链接','remoteL17092453231715.svg'],[3,'back','返回列表','remoteL17092453245142.svg'],[4,'top','返回顶部','remoteL17092453245429.svg']]){
  const link=original[index];link.classList.remove('BackToTop');link.setAttribute('data-content-tool',action);link.setAttribute('aria-label',label);link.title=label;
  if(action==='back')link.href='/news.html';else{link.setAttribute('role','button');link.tabIndex=0;}
  const img=link.querySelector('img');img.src='static/picture/'+file;img.alt='';tools.append(link);
 }
 return main;
}

import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {parseHTML} from 'linkedom';
const root=new URL('../../frontend/public/lina/',import.meta.url);
const copy=await readFile(new URL('locale-copy.js',root),'utf8'),pages=await readFile(new URL('locale-pages-copy.js',root),'utf8');
const url=source=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64');
const copyURL=url(copy.replace("'./locale-pages-copy.js'",JSON.stringify(url(pages))));
const source=(await readFile(new URL('locale.js',root),'utf8')).replace("'./locale-copy.js'",JSON.stringify(copyURL));
const {initializeLocale}=await import(url(source));
test('content placeholders translate and restore while real article text and markup remain intact',async()=>{
 const dom=parseHTML('<html><body><div data-locale-actions></div><p>知识库标题</p><p>知识库摘要知识库摘要知识库摘要知识库摘要知识库摘要</p><article data-content-body><p>新闻正文新闻正文新闻正文新闻正文新闻正文新闻正文</p><p>真实文章内容不应被替换</p><pre>新闻正文新闻正文新闻正文新闻正文新闻正文新闻正文</pre></article></body></html>');
 try {
  const document=dom.window.document,article=document.querySelector('article');
  const original=article.innerHTML,locale=initializeLocale(document);locale.setLocale('en-US');
  assert.equal(document.querySelector('body>p').textContent,'Knowledge base title');
  assert.equal(document.querySelectorAll('body>p')[1].textContent,'Knowledge base summary');
  assert.equal(article.querySelector('p').textContent,'News content');
  for(const [zh,en] of [['公告正文公告正文公告正文公告正文公告正文公告正文公告正文','Announcement content'],['知识库正文知识库正文知识库正文知识库正文知识库正文知识库正文知识库正文','Knowledge base content']]){
   const paragraph=document.createElement('p');paragraph.textContent=zh;article.append(paragraph);
   locale.render();assert.equal(paragraph.textContent,en);
   locale.setLocale('zh-CN');assert.equal(paragraph.textContent,zh);
   locale.setLocale('en-US');assert.equal(paragraph.textContent,en);paragraph.remove();
  }
  assert.equal(article.querySelectorAll('p')[1].textContent,'真实文章内容不应被替换');
  assert.equal(article.querySelector('pre').textContent,'新闻正文新闻正文新闻正文新闻正文新闻正文新闻正文');
  const late=document.createElement('p');late.textContent='新闻正文新闻正文新闻正文新闻正文新闻正文新闻正文';article.append(late);
  await new Promise(resolve=>dom.window.setTimeout(resolve,10));assert.equal(late.textContent,'News content');late.remove();
  locale.setLocale('zh-CN');assert.equal(article.innerHTML,original);
 }finally{}
});

import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewResponse} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-53 内容栏目与安全返回上下文',()=>{
 test('TC-53a: 四类列表本方栏目、搜索返回保留及恶意地址拒绝',async({page},info)=>{
  let type='news';await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/content/items')){type=u.searchParams.get('type')||'news';r.data={items:[{contentNo:'context-test',type,title:'模拟本方文章',category:{nameZhCN:'本方栏目'},summary:'本方公开摘要'}],nextCursor:''};}if(u.pathname.endsWith('/content/items/context-test')){r.status=200;r.data={item:{contentNo:'context-test',type,title:'模拟本方文章',body:'<p>本方正文</p>'}};}await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new HomeDetailPage(page);
  for(const path of ['news','announcements','help','downloads']){await site.open('/'+path+'.html?categoryNo=local-category&token=secret-not-returned');await site.searchContent('审计筛选');await expect(page.locator('[data-content-list] .lina-content-category')).toHaveText('本方栏目');await site.firstArticle();await expect(page.locator('[data-content-body]')).toHaveText('本方正文');const back=page.locator('[data-content-tool=back]');await expect(back).toHaveAttribute('href',new RegExp('^/'+path+'[.]html[?]query='));await expect(back).not.toHaveAttribute('href',/token=/);await site.articleTool('back');await expect(page.locator('[data-content-search] input')).toHaveValue('审计筛选');expect(new URL(page.url()).searchParams.get('categoryNo')).toBe('local-category');await site.captureSection('[data-content-list]',info.outputPath(path+'-restored.png'));}
  type='news';for(const returnTo of ['https://example.invalid/news.html?query=bad','//example.invalid/news.html','/help.html?query=wrong','/news.html?token=secret','/news.html?query=ok&token=secret']){await site.open('/content.html?'+new URLSearchParams({id:'context-test',returnTo}));await expect(page.locator('[data-content-body]')).toHaveText('本方正文');await expect(page.locator('[data-content-tool=back]')).toHaveAttribute('href',returnTo.includes('query=ok')?'/news.html?query=ok':'/news.html');}
 });
});

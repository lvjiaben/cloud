import{test,expect}from'../support/source-site';import{controlledBusiness}from'../support/controlled-business';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000}});
test.describe('TC-158 官网跨页登录连续性',()=>{
 test('TC-158a: 一次已登录桥接保存HttpOnly会话，重新进入购物车无需第三方页面',async({page},info)=>{
  await controlledBusiness(page);let frames=0,saves=0,cartReads=0;const writes:string[]=[];
  await page.route('**/__website/session',route=>{const r=route.request();if(r.method()==='PUT'){saves++;expect(r.headers().authorization).toBe('Bearer customer-session');return route.fulfill({headers:{'set-cookie':'continuity_session=ready; Path=/__website/session; HttpOnly; SameSite=Lax'},json:{checked:true}});}const ready=(r.headers().cookie||'').includes('continuity_session=ready');return route.fulfill({json:{checked:true,accessToken:ready?'customer-session':''}});});
  await page.route('http://localhost:6032/auth/website-session**',route=>{frames++;if(frames>1)return route.abort();const nonce=new URL(route.request().url()).searchParams.get('nonce');return route.fulfill({contentType:'text/html',body:'<script>parent.postMessage('+JSON.stringify({type:'lina-website-session',nonce,accessToken:'customer-session'})+',"http://localhost:6680")</script>'});});
  await page.route('**/x/idc-core/api/v1/account/**',route=>{const r=route.request();if(r.method()!=='GET')writes.push(new URL(r.url()).pathname);if(r.url().endsWith('/commerce/cart')){cartReads++;expect(r.headers().authorization).toBe('Bearer customer-session');return route.fulfill({json:{code:0,data:{cart:{version:1,currencyCode:'CNY',items:[]}}}});}return route.fulfill({status:404,json:{code:404}});});
  for(let i=0;i<3;i++){await page.goto('/cart.html');await expect(page.locator('.cart-empty')).toContainText('购物车还是空的');await expect(page.locator('.cart-status')).not.toContainText('登录后即可');}
  expect(frames).toBe(1);expect(saves).toBe(1);expect(cartReads).toBe(3);expect(writes).toEqual([]);expect(await page.evaluate(()=>JSON.stringify({...sessionStorage,...localStorage}))).not.toContain('customer-session');await page.screenshot({path:info.outputPath(Date.now()+'-signed-cart-after-navigation.png'),animations:'disabled'});
 });
});

import {test,expect} from '../support/source-site';
import {ActivationIndicatorPage} from '../pages/ActivationIndicatorPage';
test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,viewport:{width:1440,height:900}});
test.describe('TC-141 导航激活指示线伸展',()=>{
 test('TC-141f: 手机点击一级导航显示指示线，关闭后收回',async({page},info)=>{
  const site=new ActivationIndicatorPage(page);await site.setViewport(390,844);await site.open('/');await site.toggleMenu();await site.clickTitle('解决方案');await expect(site.title('解决方案')).toHaveAttribute('aria-expanded','true');await expect.poll(async()=>(await site.measureTitle('解决方案')).scale).toBe(1);expect((await site.measureTitle('解决方案')).arrow).toBe('1px');await site.captureIndicator(info.outputPath('top-navigation-mobile.png'));await site.escape();await site.focus(page.locator('.lina-menu-toggle'));await site.leaveNavigation();await expect.poll(async()=>(await site.measureTitle('解决方案')).scale).toBe(0);
 });
 test('TC-141e: 同一条顶部指示线在七个菜单之间平移，兼容键盘和箭头',async({page},info)=>{
  const site=new ActivationIndicatorPage(page);await site.open('/');
  expect((await site.measureTitle('解决方案')).scale).toBe(0);
  await site.hoverTitle('产品与服务');await expect.poll(async()=>(await site.measureTitle('产品与服务')).scale).toBe(1);
  for(const name of ['解决方案','生态合作','应用市场','支持与服务','了解我们','产品与服务']){
   const previous=await site.lineState(),target=(await site.titleBox(name))!;
   await site.hoverTitle(name);const middle=await site.sampleMove();expect(middle).not.toBeNull();expect(middle!).toBeGreaterThan(Math.min(previous.left,target.x));expect(middle!).toBeLessThan(Math.max(previous.left,target.x));
   await expect.poll(async()=>Math.round((await site.lineState()).left)).toBe(Math.round(target.x));await expect.poll(async()=>Math.round((await site.lineState()).width)).toBe(Math.round(target.width));
  }
  await site.keyboardFocusTitle('解决方案');await expect(site.title('解决方案')).toBeFocused();await expect.poll(async()=>(await site.measureTitle('解决方案')).scale).toBe(1);await site.captureIndicator(info.outputPath('top-navigation-focus.png'));
  const line=await site.measureTitle('解决方案');expect(line.height).toBe('2px');expect(line.color).toBe('rgb(0, 82, 217)');expect(line.arrow).toBe('1px');
  await site.reduce();await site.hoverTitle('了解我们');expect(await site.sampleMove()).toBeNull();expect((await site.measureTitle('了解我们')).scale).toBe(1);
  await expect(page.locator('.headerBg')).toHaveCSS('background-color','rgba(0, 0, 0, 0)');
 });
 test('TC-141d: 首页已选中项再次悬停必须重新伸展，移出仍保持选中',async({page},info)=>{
  const site=new ActivationIndicatorPage(page);await site.open('/');await site.clickTitle('解决方案');
  const selected=site.category('行业解决方案');await expect(selected).toHaveAttribute('aria-pressed','true');
  await site.enterPanel();await expect.poll(async()=>Math.round((await site.measure(selected)).width)).toBe(Math.round((await site.measure(selected)).full));
  for(let pass=0;pass<2;pass++){
   await site.hover(selected);const sample=await site.sampleReplay(selected);expect(sample).not.toBeNull();expect(sample!.middle).toBeGreaterThan(0);expect(sample!.middle).toBeLessThan(1);expect(sample!.duration).toBe(300);
   await expect.poll(async()=>Math.round((await site.measure(selected)).width)).toBe(Math.round((await site.measure(selected)).full));
   await site.enterPanel();expect((await site.measure(selected)).width).toBeCloseTo((await site.measure(selected)).full,0);await expect(selected).toHaveAttribute('aria-pressed','true');
  }
  await site.hover(selected);await site.captureIndicator(info.outputPath('home-selected-replay.png'));
  await site.reduce();expect(await site.sampleReplay(selected)).toBeNull();
 });

 test.beforeEach(async({page})=>{await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand:{navigation:[{label:'产品与服务',href:'/products.html'},{label:'解决方案',group:true,children:[{label:'行业解决方案',group:true,children:['电商','网站','游戏','安全'].map(name=>({label:name+'解决方案',href:'/help.html'}))},{label:'通用解决方案',group:true,children:[{label:'工业解决方案',href:'/help.html'}]}]}]}}}}));});
 test('TC-141a: 桌面悬停、键盘聚焦与选中状态及减少动态效果',async({page},info)=>{
  const site=new ActivationIndicatorPage(page);await site.open('/certificate.html');await site.hoverTitle('解决方案');const panel=site.panel();await expect(panel).toBeVisible();const category=panel.getByRole('button',{name:'行业解决方案',exact:true});
  const measure=(element=category)=>site.measure(element);
  await site.enterPanel();const idle=await measure();expect(idle.width).toBeCloseTo(idle.full,0);expect(idle.full).toBeGreaterThan(0);expect(idle.duration).toBe('0.3s');
  await site.hover(category);await expect.poll(async()=>Math.round((await measure()).width)).toBe(Math.round(idle.full));const hovered=await measure();expect(hovered.width).toBeCloseTo(hovered.full,0);await site.captureIndicator(info.outputPath('desktop-hover.png'));
  await site.enterPanel();await site.focus(panel.locator('.lina-nav-close'));expect((await measure()).width).toBeCloseTo(idle.full,0);await site.focus(category);await expect.poll(async()=>Math.round((await measure()).width)).toBe(Math.round(idle.full));await site.captureIndicator(info.outputPath('keyboard-focus.png'));
  const other=panel.getByRole('button',{name:'通用解决方案',exact:true});await site.focus(other);await site.click(other);await expect(other).toHaveAttribute('aria-pressed','true');await expect(category).toHaveAttribute('aria-pressed','false');await site.enterPanel();await expect.poll(async()=>Math.round((await measure()).width)).toBe(0);expect((await measure(other)).width).toBeGreaterThan(0);
  await site.reduce();expect((await measure(other)).duration).toBe('0s');await site.captureIndicator(info.outputPath('reduced-motion.png'));
 });
 test('TC-141c: 通用配置、临时状态收回、Tab键和重复渲染无脚本错误',async({page},info)=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
  const site=new ActivationIndicatorPage(page);await site.open('/certificate.html');
  const counts=await site.renderAgain();expect(counts).toBeGreaterThan(2);
  await expect(site.movingLine()).toHaveCount(1);
  await site.clickTitle('解决方案');await expect.poll(async()=>(await site.measureTitle('解决方案')).scale).toBe(1);const probe=await site.installProbe();
  expect((await site.measure(probe)).width).toBe(0);
  await site.hover(probe);
  const intermediate=await probe.evaluate(async e=>{const a=e.getAnimations({subtree:true}).find(a=>a.effect instanceof KeyframeEffect&&a.effect.pseudoElement==='::after');if(!a)return null;a.pause();a.currentTime=200;await new Promise(requestAnimationFrame);const scale=new DOMMatrixReadOnly(getComputedStyle(e,'::after').transform).a;a.play();return scale;});expect(intermediate).not.toBeNull();expect(intermediate!).toBeGreaterThan(0);expect(intermediate!).toBeLessThan(1);
  await expect.poll(async()=>Math.round((await site.measure(probe)).width)).toBe(Math.round((await site.measure(probe)).full));
  const active=await site.measure(probe);expect(active.duration).toBe('0.4s');expect(active.height).toBe('4px');expect(active.color).toBe('rgb(180, 20, 70)');expect(parseFloat(active.origin)).toBeCloseTo(active.full,0);
  await site.enterPanel();await expect.poll(async()=>Math.round((await site.measure(probe)).width)).toBe(0);
  await site.focus(site.category('通用解决方案'));await site.keyboard('Tab');await expect(probe).toBeFocused();
  await expect.poll(async()=>Math.round((await site.measure(probe)).width)).toBe(Math.round(active.full));
  await site.captureIndicator(info.outputPath('generic-keyboard.png'));
  await site.keyboard('Tab');await expect.poll(async()=>Math.round((await site.measure(probe)).width)).toBe(0);
  await site.reduce();await site.hover(probe);expect((await site.measure(probe)).duration).toBe('0s');expect((await site.measure(probe)).width).toBeCloseTo(active.full,0);
  expect(errors).toEqual([]);
 });
 test('TC-141b: 手机点击分类显示选中指示线且无横向溢出',async({page},info)=>{
  const site=new ActivationIndicatorPage(page);await site.setViewport(390,844);await site.open('/');await site.toggleMenu();await site.clickTitle('解决方案');const panel=site.panel();await expect(panel).toBeVisible();const category=panel.getByRole('button',{name:'行业解决方案',exact:true});await site.click(category);await page.waitForTimeout(350);await expect(category).toHaveAttribute('aria-pressed','true');const result=await category.evaluate(e=>({line:(getComputedStyle(e,'::after').transform==='none'?1:Math.abs(new DOMMatrixReadOnly(getComputedStyle(e,'::after').transform).a))*e.getBoundingClientRect().width,width:e.getBoundingClientRect().width,scroll:document.documentElement.scrollWidth,viewport:innerWidth}));expect(result.line).toBeCloseTo(result.width,0);expect(await panel.evaluate(e=>e.scrollWidth<=e.clientWidth+1)).toBe(true);await site.captureIndicator(info.outputPath('mobile-selected.png'));
 });
});




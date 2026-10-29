import {test,expect} from "../support/source-site";
import {HomeDetailPage} from "../pages/HomeDetailPage";
import {previewBrand,previewResponse} from "../support/home-preview-data.mjs";
test.use({baseURL:"http://localhost:6680",viewport:{width:1440,height:900},reducedMotion:"reduce"});
test.describe("TC-47 共用页脚字段与安全链接",()=>{
 test("TC-47a: 十页共用四列，长值、空值和后台导航更新使用本方字段",async({page},info)=>{
  let mode="full";await page.route("**/x/**",async route=>{const req=route.request(),url=new URL(req.url()),r=previewResponse(url,req.method());if(url.pathname.endsWith("/brand"))r.data={brand:{...previewBrand,footer:{zhCN:mode==="empty"?"":"本方公司".repeat(30)},logoUrl:mode==="empty"?"":"/static/picture/sidebar-newspaper.svg",contactPhone:mode==="empty"?"":"000-TEST-ONLY",contactEmail:mode==="empty"?"":"our-test@example.test",footerNavigation:mode==="updated"?[{group:true,label:{zhCN:"本方后台栏目"},children:[{label:{zhCN:"本方公告"},href:"/announcements.html"},{label:{zhCN:"恶意地址"},href:"javascript:alert(1)"}]}]:[]}};await route.fulfill({status:r.status,json:{code:r.status===200?0:r.status,data:r.data}});});
  const site=new HomeDetailPage(page);
  for(const path of ["index","products","configure","news","announcements","help","downloads","content","about","contact"]){
   await site.open("/"+path+".html");const footer=page.locator(".sky-footer-container");await expect(footer).toHaveCSS("background-color","rgb(10, 22, 51)");await expect(footer.locator(".sky-footer-nav-heading")).toHaveText(["服务指南","代理系统","帮助中心","关于我们"]);await expect(footer.locator("[data-brand-footer]")).toHaveText("本方公司".repeat(30));await expect(footer.locator("[data-footer-contact=phone] a")).toHaveAttribute("href","tel:000-TEST-ONLY");await expect(footer.locator("[data-footer-contact=email] a")).toHaveAttribute("href","mailto:our-test@example.test");await expect(footer.locator(".filing-itme:visible,.sky-footer-ewm:visible")).toHaveCount(0);await expect(footer).not.toContainText(/小白云|江苏白猿|1531335070|苏ICP备/);await site.captureSection(".sky-footer-container",info.outputPath(path+"-footer.png"));expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  }
  mode="empty";await site.open("/index.html");await expect(page.locator("[data-footer-logo],[data-brand-footer],[data-footer-contact]").filter({visible:true})).toHaveCount(0);
  mode="updated";await site.open("/index.html");await expect(page.locator("[data-footer-navigation]")).toContainText("本方后台栏目");await expect(page.locator("[data-footer-navigation] a").first()).toHaveAttribute("href",/announcements[.]html/);await expect(page.locator("[data-footer-navigation] a").nth(1)).not.toHaveAttribute("href",/.+/);await site.captureSection(".sky-footer-container",info.outputPath("updated-footer.png"));
 });
});


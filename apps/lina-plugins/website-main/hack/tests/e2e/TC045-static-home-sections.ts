import {test,expect} from "../support/source-site";
import {readFileSync} from "node:fs";
import {HomeDetailPage} from "../pages/HomeDetailPage";
import {previewResponse,previewBrand} from "../support/home-preview-data.mjs";
const expected=JSON.parse(readFileSync(new URL("../support/static-solutions-fixture.json",import.meta.url),"utf8"));
const benefits=JSON.parse(readFileSync(new URL("../support/static-benefits-fixture.json",import.meta.url),"utf8"));
test.use({baseURL:"http://localhost:6680",viewport:{width:1440,height:900},reducedMotion:"reduce"});
test.describe("TC-45 附件静态方案和能力区",()=>{
 for(const mode of ["configured","empty","failed"]){
  test("TC-45"+mode+": 七方案和四能力不受后台覆盖",async({page},info)=>{
   await page.route("**/x/**",async route=>{const req=route.request(),url=new URL(req.url()),r=previewResponse(url,req.method());if(url.pathname.endsWith("/brand")){if(mode==="failed")return route.fulfill({status:503,json:{code:503}});r.data={brand:{...previewBrand,templateConfig:mode==="empty"?{}:{...previewBrand.templateConfig,solutions:"不应覆盖 | 不应覆盖 | - | -",benefits:"不应覆盖 | 不应覆盖 | -"}}};}await route.fulfill({status:r.status,json:{code:r.status===200?0:r.status,data:r.data}});});
   const site=new HomeDetailPage(page);await site.open("/index.html");
   const section=page.locator(".container_bg15");await expect(section.locator(".sk-title")).toHaveText(expected.heading);await expect(section.locator(".sk-desc")).toHaveText(expected.description);
   await expect(section.locator(".main-option-name span")).toHaveText(expected.solutions.map((x:any)=>x.tab));
   for(const [i,item] of expected.solutions.entries()){
    if(i%2)await site.solutionByKeyboard(i);else await site.solution(i);
    const profile=section.locator(".sidebar-profile:visible"),detail=section.locator(".main-option-line:visible");
    await expect(profile.locator(".name")).toHaveText(item.title);await expect(profile.locator(".title")).toHaveText(item.subtitle);await expect(detail.locator(".reviews span")).toHaveText(item.body);await expect(detail.locator("li")).toHaveText(item.features);
    await expect(profile.locator("img")).toHaveAttribute("src","static/homepage/"+item.image);await expect.poll(()=>profile.locator("img").evaluate((n:HTMLImageElement)=>n.complete&&n.naturalWidth>0)).toBe(true);
    await site.captureSection(".container_bg15",info.outputPath(mode+"-solution-"+i+".png"));
   }
   await expect(section.locator("a[href]")).toHaveCount(0);await expect(section.locator(".social-links, .social-link")).toHaveCount(0);await expect(section.locator(".profile-position")).toHaveCount(7);
   const cards=page.locator(".chooseUs-itme");await expect(cards).toHaveCount(4);await expect(page.locator(".chooseUs-container .sk-title")).toHaveText(benefits.heading);await expect(page.locator(".chooseUs-container .sk-desc")).toHaveText(benefits.description);
   for(const [i,item] of benefits.cards.entries()){await expect(cards.nth(i).locator(".chooseUs-itme-title")).toHaveText(item.title);await expect(cards.nth(i).locator(".chooseUs-itme-desc")).toHaveText(item.description);await expect(cards.nth(i).locator(".chooseUs-itme-tag")).toHaveText(item.labels);await expect.poll(()=>cards.nth(i).locator("img").evaluate((n:HTMLImageElement)=>n.complete&&n.naturalWidth===200)).toBe(true);}
   await site.captureSection(".chooseUs-container",info.outputPath(mode+"-benefits.png"));expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  });
 }
});

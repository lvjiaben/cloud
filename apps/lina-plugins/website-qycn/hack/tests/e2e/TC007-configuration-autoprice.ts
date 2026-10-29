import { test, expect, brand } from "../support/fixtures";

test.describe("TC-7 品牌首屏与自动选配", () => {
  test("TC-7a: 品牌慢返回和无脚本首屏均不闪现示例 Logo 与名称", async ({site, page}) => {
    const html = await (await page.request.get("/")).text();
    expect(html).not.toContain("云启网络");
    expect(html).not.toContain("CLOUD NETWORK");
    let release!: () => void;
    const gate = new Promise<void>((resolve) => { release = resolve; });
    await page.route("**/public/brand", async (route) => {
      await gate;
      await route.fulfill({json:{code:0,data:{brand:{...brand,logoUrl:"/media/world-map.jpg"}}}});
    });
    await site.open();
    await expect(page.locator(".brand [data-brand]")).toBeEmpty();
    await expect(page.locator(".brand img, .brand svg")).toHaveCount(0);
    await expect(page.locator("body")).not.toContainText("云启网络");
    await site.screenshot("brand-pending");
    release();
    await expect(page.locator(".brand [data-brand]")).toHaveText("星河云");
    await expect(page.locator(".brand img")).toHaveAttribute("src","/media/world-map.jpg");
    await expect(page.locator(".pause")).toHaveCount(0);
  });

  test("TC-7b: 默认配置自动报价，连续改配只显示最新价格，失败可重试", async ({site,page}) => {
    const requests: any[] = [];
    let fail = false;
    const view = {ready:true,configurationRevision:"R".repeat(43),fields:[
      {fieldToken:"cpu",type:"integer",labelZhCN:"处理器核心数",visible:true,required:false,minimumInteger:"1",maximumInteger:"64"},
      {fieldToken:"region",type:"single_select",labelZhCN:"地区",visible:true,required:true,options:[{optionToken:"XA",labelZhCN:"西安"},{optionToken:"CD",labelZhCN:"成都"}]},
    ],values:[{fieldToken:"region",type:"single_select",optionTokens:["XA"]}]};
    // A broken customer login iframe must not hold up anonymous product pricing.
    await page.route("**/__website/session", r => r.fulfill({json:{checked:false}}));
    await page.route("http://127.0.0.1:6032/auth/website-session?**", r => r.fulfill({contentType:"text/html",body:"<html></html>"}));
    await page.route("**/x/idc-core/api/v1/**", async route => {
      const path=new URL(route.request().url()).pathname;
      let data: any;
      if(path.endsWith("/catalog/products/auto")) data={product:{nameZhCN:"弹性云",currencies:["CNY"]}};
      else if(path.endsWith("/purchase-configuration")) data=view;
      else if(path.endsWith("/evaluate")) {
        const input=route.request().postDataJSON();requests.push(input);
        data={...view,values:input.values.some((v:any)=>v.fieldToken==="cpu")?input.values:[...input.values,{fieldToken:"cpu",type:"integer",integerValue:"1"}],valid:true};
      } else if(path.endsWith("/quote")) {
        if(fail) return route.fulfill({status:503,json:{code:503}});
        const input=route.request().postDataJSON(),cpu=input.values.find((v:any)=>v.fieldToken==="cpu")?.integerValue;
        if(cpu==="4") await new Promise(r=>setTimeout(r,900));
        data={configurationRevision:view.configurationRevision,currencyCode:"CNY",terms:[{billingTermToken:"MONTH",labelZhCN:"月付",minimumPurchaseCycles:1,status:"final",totalAmount:String(Number(cpu||1)*1000)}]};
      } else return route.fallback();
      await route.fulfill({json:{code:0,data}});
    });
    await site.open("/configure.html?product=auto");
    await expect(page.locator("#purchase-price")).toHaveText("CNY 10.00",{timeout:5000});
    expect(requests[0].values).toContainEqual({fieldToken:"cpu",type:"integer",integerValue:"1"});
    await expect(page.getByLabel("处理器核心数",{exact:true})).toHaveValue("1");
    await expect(page.locator(".purchase-selection")).not.toContainText("套餐默认");
    await expect(page.getByRole("button",{name:"核对配置和价格"})).toHaveCount(0);
    await site.configureNumber("处理器核心数","4");
    await expect.poll(()=>requests.some(r=>r.values.some((v:any)=>v.integerValue==="4"))).toBe(true);
    await site.configureNumber("处理器核心数","8");
    await expect(page.locator("#purchase-price")).toHaveText("CNY 80.00");
    await expect(page.locator(".purchase-add")).toBeEnabled();
    await site.chooseConfigurationRadio("地区", "成都");
    await expect.poll(()=>requests.at(-1).values.find((v:any)=>v.fieldToken==="region")?.optionTokens[0]).toBe("CD");
    await expect(page.locator(".purchase-add")).toBeEnabled();
    await site.screenshot("autoprice-desktop");
    await site.configureNumber("处理器核心数", "");
    await expect(page.getByLabel("处理器核心数",{exact:true})).toHaveValue("1");
    await expect(page.locator("#purchase-price")).toHaveText("CNY 10.00");
    expect(requests.at(-1).values.some((v:any)=>v.fieldToken==="cpu")).toBe(false);
    fail=true;
    await site.configureNumber("处理器核心数","16");
    await expect(page.locator(".price-status")).toContainText("价格暂时无法更新");
    await expect(page.locator(".purchase-add")).toBeDisabled();
    await expect(page.locator("#purchase-price")).toHaveText("—");
    fail=false;
    await site.retryConfigurationPrice();
    await expect(page.locator("#purchase-price")).toHaveText("CNY 160.00");
    await site.mobile();
    await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await expect(page.locator(".purchase-add")).toBeInViewport();
    await site.screenshot("autoprice-mobile",true);
    await site.selectLanguage("en-US");
    await expect(page.getByRole("heading",{name:"Product configuration",exact:true})).toBeVisible();
    await expect(page.getByRole("heading",{name:"Order summary",exact:true})).toBeVisible();
    await expect(page.locator("#purchase-price")).toHaveText("CNY 10.00");
    await expect(page.getByRole("button",{name:"Add to cart",exact:true})).toBeEnabled();
  });

  test("TC-7c: 配置接口失败时错误和重试始终可见", async ({site,page}) => {
    let failed=true;
    await page.route("**/catalog/products/broken",route=>route.fulfill({json:{code:0,data:{product:{nameZhCN:"云主机",currencies:["CNY"]}}}}));
    await page.route("**/catalog/products/broken/purchase-configuration",route=>route.fulfill(failed?{status:503,json:{code:503}}:{json:{code:0,data:{ready:true,configurationRevision:"R".repeat(43),fields:[],values:[]}}}));
    await site.open("/configure.html?product=broken");
    await expect(page.locator("#commerce-page .status")).toContainText("加载暂时失败");
    await expect(page.locator("#commerce-page .status")).toBeVisible();
    await site.screenshot("configuration-error");
    failed=false;
    await site.retryConfigurationLoading();
    await expect(page.locator("#commerce-page h1")).toHaveText("云主机");
    await expect(page.getByRole("combobox",{name:"币种",exact:true})).toBeVisible();
  });
});

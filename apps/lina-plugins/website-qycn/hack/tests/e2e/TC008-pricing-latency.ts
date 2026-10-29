import { test, expect } from "../support/fixtures";

const view = {
  ready: true, configurationRevision: "R".repeat(43),
  fields: [{fieldToken:"cpu",type:"integer",labelZhCN:"处理器核心数",visible:true,required:true,minimumInteger:"1",maximumInteger:"64"}],
  values: [{fieldToken:"cpu",type:"integer",integerValue:"4"}],
};
const quote = (cpu: string) => ({configurationRevision:view.configurationRevision,currencyCode:"CNY",terms:[
  {billingTermToken:"MONTH",labelZhCN:"月付",minimumPurchaseCycles:1,status:"final",totalAmount:String(Number(cpu)*1000)},
]});

test.describe("TC-8 选配报价等待时间", () => {
  test("TC-8a: 评估未完成时已发起报价，同值失焦不重复请求", async ({site,page}) => {
    let release!: () => void;
    const gate = new Promise<void>(resolve => { release=resolve; });
    let evaluations=0, quotes=0;
    await page.route("**/__website/session",r=>r.fulfill({json:{checked:true}}));
    await page.route("**/catalog/products/fast**",async route=>{
      const path=new URL(route.request().url()).pathname;
      let data:any;
      if(path.endsWith("/fast")) data={product:{nameZhCN:"快速选配",currencies:["CNY"]}};
      else if(path.endsWith("/purchase-configuration")) data=view;
      else if(path.endsWith("/evaluate")) {
        evaluations++; await gate;
        data={...view,values:route.request().postDataJSON().values,valid:true};
      } else if(path.endsWith("/quote")) {quotes++;data=quote(route.request().postDataJSON().values[0].integerValue);}
      else return route.fallback();
      await route.fulfill({json:{code:0,data}});
    });
    await site.open("/configure.html?product=fast");
    try {
      // A serial implementation never reaches quote while evaluation is gated.
      await expect.poll(()=>({evaluations,quotes})).toEqual({evaluations:1,quotes:1});
      await expect(page.locator("#purchase-price")).toHaveText("—");
      await expect(page.locator(".purchase-add")).toBeDisabled();
    } finally {release();}
    await expect(page.locator("#purchase-price")).toHaveText("CNY 40.00");
    await site.configureNumber("处理器核心数","8");
    await expect(page.locator("#purchase-price")).toHaveText("CNY 80.00");
    await expect.poll(()=>({evaluations,quotes})).toEqual({evaluations:2,quotes:2});
    await site.finishConfigurationEdit();
    // Cover the debounce window deterministically without real-time sleeps.
    await page.clock.install();
    await page.clock.runFor(500);
    expect({evaluations,quotes}).toEqual({evaluations:2,quotes:2});
    await expect(page.locator(".purchase-add")).toBeEnabled();
    await site.screenshot("parallel-pricing");
  });

  test("TC-8b: 服务端调整配置后丢弃预取价格，取得新配置的报价", async ({site,page}) => {
    const amounts:string[]=[];
    let release!:()=>void;
    const gate=new Promise<void>(resolve=>{release=resolve;});
    await page.route("**/__website/session",r=>r.fulfill({json:{checked:true}}));
    await page.route("**/catalog/products/normalize**",async route=>{
      const path=new URL(route.request().url()).pathname;
      let data:any;
      if(path.endsWith("/normalize")) data={product:{nameZhCN:"联动选配",currencies:["CNY"]}};
      else if(path.endsWith("/purchase-configuration")) data=view;
      else if(path.endsWith("/evaluate")) data={...view,valid:true,values:[{...view.values[0],integerValue:"8"}]};
      else if(path.endsWith("/quote")) {
        const cpu=route.request().postDataJSON().values[0].integerValue;
        amounts.push(cpu);
        if(cpu==="8") await gate;
        data=quote(cpu);
      } else return route.fallback();
      await route.fulfill({json:{code:0,data}});
    });
    await site.open("/configure.html?product=normalize");
    try {
      await expect.poll(()=>amounts).toEqual(["4","8"]);
      await expect(page.getByLabel("处理器核心数",{exact:false})).toHaveValue("8");
      await expect(page.locator("#purchase-price")).toHaveText("—");
      await expect(page.locator(".purchase-add")).toBeDisabled();
    } finally {release();}
    await expect(page.locator("#purchase-price")).toHaveText("CNY 80.00");
    await expect(page.locator(".purchase-add")).toBeEnabled();
  });

  test("TC-8c: 报价先返回也不能绕过无效配置", async ({site,page}) => {
    let release!:()=>void;
    const gate=new Promise<void>(resolve=>{release=resolve;});
    let quoted=false;
    await page.route("**/__website/session",r=>r.fulfill({json:{checked:true}}));
    await page.route("**/catalog/products/invalid**",async route=>{
      const path=new URL(route.request().url()).pathname;
      let data:any;
      if(path.endsWith("/invalid")) data={product:{nameZhCN:"无效选配",currencies:["CNY"]}};
      else if(path.endsWith("/purchase-configuration")) data=view;
      else if(path.endsWith("/evaluate")) {await gate;data={...view,valid:false,issues:[{fieldToken:"cpu",issueCode:"invalid_value"}]};}
      else if(path.endsWith("/quote")) {quoted=true;data=quote("4");}
      else return route.fallback();
      await route.fulfill({json:{code:0,data}});
    });
    await site.open("/configure.html?product=invalid");
    try {await expect.poll(()=>quoted).toBe(true);} finally {release();}
    await expect(page.locator(".price-status")).toContainText("处理器核心数");
    await expect(page.locator("#purchase-price")).toHaveText("—");
    await expect(page.locator(".purchase-add")).toBeDisabled();
    await site.screenshot("parallel-pricing-invalid");
  });
});

import { test, expect, resourceSurface, sslSurface, menus } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
const now = new Date("2026-10-25T12:00:00+08:00").getTime(),
  day = 86400000;
test.describe("TC-24 插件拥有已购内容", () => {
  test("TC-24a 服务器生命周期圆点、日期分色及系统图标由资源插件展示", async ({
    consolePage: page,
  }) => {
    await page.clock.install({ time: new Date(now) });
    await page.route("https://api.iconify.design/**/*.svg?*", (route) =>
      route.fulfill({
        contentType: "image/svg+xml",
        body: '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><path d="M2 2h12v12H2z"/></svg>',
      }),
    );
    const items = [31, 30, 15, 14].map((days) => ({
      entitlementNo: `active-${days}`,
      productNameZhCn: `激活${days}`,
      statusCode: "active",
      expiresAt: now + days * day,
      orderNo: "ORD-test",
      entitlementOwnerId: "idc-resource",
    }));
    items.push({
      entitlementNo: "paused",
      productNameZhCn: "暂停服务器",
      statusCode: "suspended",
      expiresAt: now + 14 * day,
      orderNo: "ORD-test",
      entitlementOwnerId: "idc-resource",
    });
    await page.route("**/product-directories/cloud/services?*", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            items,
            total: items.length,
            providers: [{ pluginId: "idc-resource", total: items.length }],
          },
        },
      }),
    );
    await page.route("**/account/native-server-facts?*", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            items: items.map((item) => ({
              entitlementNo: item.entitlementNo,
              hostname: "test-host",
              osName: "CentOS 9",
              addresses: ["192.0.2.8"],
              addressesConfirmed: true,
              lastObservedState: "stopped",
            })),
          },
        },
      }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/product-directories/cloud");
    for (const [days, tone] of [
      [31, "success"],
      [30, "warning"],
      [15, "warning"],
      [14, "danger"],
    ] as const) {
      const row = view.serviceRows.filter({ hasText: `激活${days}` });
      await expect(row.locator(".status")).toHaveText("已激活");
      await expect(row.locator(".status .dot")).toBeVisible();
      await expect(row.locator(`.subline.${tone}`)).toHaveText(
        `[${days}天后过期]`,
      );
      await expect(row.locator(`.subline.${tone}`)).toHaveCSS(
        "color",
        (
          {
            success: "rgb(0, 168, 112)",
            warning: "rgb(227, 152, 0)",
            danger: "rgb(227, 77, 89)",
          } as const
        )[tone],
      );
      await expect(row).toContainText(/2026-\d{2}-\d{2}/);
      await expect(row.locator(".system img")).toHaveAttribute(
        "src",
        /simple-icons\/centos/,
      );
    }
    const paused = view.serviceRows.filter({ hasText: "暂停服务器" });
    await expect(paused.locator(".status")).toHaveText("已暂停");
    await expect(paused).not.toContainText("天后过期");
    await expect(view.serviceBody).not.toContainText("已关机");
    await view.screenshot("plugin-server-status-expiry-system");
  });
  test("TC-24b 同一菜单分块展示服务器与证书各自的列，证书请求限制当前菜单及插件", async ({
    consolePage: page,
  }) => {
    const calls: { directoryNo: string | null; pluginId: string | null }[] = [];
    await page.route("**/plugins/dynamic", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            list: [
              {
                id: "idc-resource",
                version: "v0.1.77",
                installed: 1,
                enabled: 1,
                customerSurfaces: [resourceSurface],
              },
              {
                id: "yunsu-ssl",
                version: "v0.3.8",
                installed: 1,
                enabled: 1,
                customerSurfaces: [sslSurface],
              },
            ],
          },
        },
      }),
    );
    await page.route("**/product-directories/cloud/services?*", (route) => {
      const native =
        new URL(route.request().url()).searchParams.get("providerId") ===
        "idc-resource";
      return route.fulfill({
        json: {
          code: 0,
          data: {
            items: [
              {
                entitlementNo: "native-1",
                productNameZhCn: "服务器专有内容",
                statusCode: "active",
                orderNo: "ORD-test",
                expiresAt: now + day,
              },
            ],
            total: native ? 1 : 2,
            providers: native
              ? [{ pluginId: "idc-resource", total: 1 }]
              : [
                  { pluginId: "idc-resource", total: 1 },
                  { pluginId: "yunsu-ssl", total: 1 },
                ],
          },
        },
      });
    });
    await page.route("**/plugins/page-bridge/grant", (route) => {
      expect(route.request().postDataJSON()).toMatchObject({
        pluginId: "yunsu-ssl",
        surfaceKey: "certificates",
        capability: "service-entitlements.customer.v1",
        method: "GET",
        path: "account/service-entitlements",
      });
      return route.fulfill({
        json: {
          code: 0,
          data: { grant: "isolated-surface-grant", ownerPluginId: "idc-core" },
        },
      });
    });
    await page.route("**/account/service-entitlements?*", (route) => {
      const url = new URL(route.request().url());
      calls.push({
        directoryNo: url.searchParams.get("directoryNo"),
        pluginId: url.searchParams.get("pluginId"),
      });
      expect(route.request().headers()["x-lina-plugin-page-grant"]).toBe(
        "isolated-surface-grant",
      );
      return route.fulfill({
        json: {
          code: 0,
          data: {
            items: [
              {
                entitlementNo: "cert-1",
                primaryDomain: "service.example.com",
                productNameZhCn: "通配符证书",
                statusCode: "issued",
                validationMethod: "dns",
                validation: [],
                availableActions: [],
                expiresAt: now + 365 * day,
                updatedAt: now,
                issuedAt: now,
              },
            ],
            nextCursor: "",
          },
        },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/product-directories/cloud");
    await expect(view.serviceBody).toContainText("服务器专有内容");
    const cert = view.pluginBody("yunsu-ssl");
    await expect(cert).toContainText("service.example.com");
    await expect(
      cert.getByRole("columnheader").filter({ hasText: "域名" }),
    ).toBeVisible();
    await expect(
      cert.getByRole("columnheader").filter({ hasText: /IP 地址|系统/ }),
    ).toHaveCount(0);
    expect(calls).toEqual([{ directoryNo: "cloud", pluginId: "yunsu-ssl" }]);
    await view.screenshot("mixed-plugin-owned-service-lists");
  });
  test("TC-24c 插件缺失时不套用服务器列，读取失败保留重试", async ({
    consolePage: page,
  }) => {
    await page.route("**/product-directories/cloud/services?*", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            items: [],
            total: 1,
            providers: [{ pluginId: "unavailable-plugin", total: 1 }],
          },
        },
      }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/product-directories/cloud");
    await expect(view.main).toContainText("商品所属插件暂未提供可用的已购页面");
    await expect(view.main.getByRole("columnheader")).toHaveCount(0);
    await view.screenshot("missing-plugin-service-page");
  });

  test("TC-24d 插件读取限制当前菜单且无法读取会话或其他插件服务", async ({
    consolePage: page,
  }) => {
    let grants = 0,
      foreignReads = 0;
    await page.route("**/plugins/dynamic", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            list: [
              {
                id: "yunsu-ssl",
                version: "v0.3.8",
                installed: 1,
                enabled: 1,
                customerSurfaces: [sslSurface],
              },
            ],
          },
        },
      }),
    );
    await page.route("**/product-directories/cloud/services?*", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            items: [],
            total: 1,
            providers: [{ pluginId: "yunsu-ssl", total: 1 }],
          },
        },
      }),
    );
    await page.route(
      "**/x-assets/yunsu-ssl/v0.3.8/customer/certificates/index.html",
      (route) =>
        route.fulfill({
          contentType: "text/html",
          body: `<!doctype html><button id="foreign">读取其他服务</button><p id="result"></p><p id="scope"></p><script>
      const envelope={protocol:"linapro.plugin-page-bridge",version:"1.0",direction:"plugin-to-host",pluginId:"yunsu-ssl",surfaceKey:"certificates"};
      onmessage=e=>{if(e.source!==parent)return;const m=e.data;if(m.capability==="host.context.v1"){document.getElementById("scope").textContent=JSON.stringify(m.payload);parent.postMessage({...envelope,capability:"service-entitlements.customer.v1",method:"request",requestId:"list",payload:{method:"GET",path:"account/service-entitlements",params:{directoryNo:"foreign",pluginId:"other"}}},"*");}if(m.requestId==="foreign")document.getElementById("result").textContent=m.ok?"错误放行":m.result.message;};
      document.getElementById("foreign").onclick=()=>{parent.postMessage({...envelope,capability:"service-entitlements.customer.v1",method:"request",requestId:"foreign",payload:{method:"GET",path:"account/service-entitlements/foreign-service"}},"*");};
      try{document.getElementById("result").textContent=parent.sessionStorage.getItem("customer-tdesign.session")||"可读取会话";}catch{document.getElementById("result").textContent="会话不可读取";}
      parent.postMessage({...envelope,capability:"host.view.v1",method:"ready",payload:{}},"*");
    </script>`,
        }),
    );
    await page.route("**/plugins/page-bridge/grant", (route) => {
      grants++;
      return route.fulfill({
        json: {
          code: 0,
          data: { grant: "safe-grant", ownerPluginId: "idc-core" },
        },
      });
    });
    await page.route("**/account/service-entitlements?*", (route) => {
      const url = new URL(route.request().url());
      expect(url.searchParams.get("directoryNo")).toBe("cloud");
      expect(url.searchParams.get("pluginId")).toBe("yunsu-ssl");
      return route.fulfill({
        json: {
          code: 0,
          data: { items: [{ entitlementNo: "own-service" }], nextCursor: "" },
        },
      });
    });
    await page.route(
      "**/account/service-entitlements/foreign-service",
      (route) => {
        foreignReads++;
        return route.fulfill({ json: { code: 0, data: {} } });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/product-directories/cloud");
    const body = view.pluginBody("yunsu-ssl");
    await expect(body).toContainText("会话不可读取");
    await expect(body.locator("#scope")).not.toContainText(
      "isolated-ui-fixture",
    );
    await view.pluginButton("yunsu-ssl", "读取其他服务");
    await expect(body).toContainText("该内容不在当前插件列表中");
    expect(foreignReads).toBe(0);
    expect(grants).toBe(1);
    await view.screenshot("plugin-service-scope-rejection");
  });
  test("TC-24e 空物理机保留紧凑搜索与满高表格，窗口变化后分页仍可达", async ({ consolePage: page }) => {
    const keywords: string[] = [];
    await page.route("**/account/product-directories?*", route => route.fulfill({json:{code:0,data:{items:[...menus,{directoryNo:"physical",nameZhCn:"物理机",navigationType:"product",targetUrl:"",sortOrder:2}],total:menus.length+1}}}));
    await page.route("**/product-directories/physical/services?*", route => {
      const params = new URL(route.request().url()).searchParams;
      if (params.get("providerId")) keywords.push(params.get("keyword") || "");
      return route.fulfill({json:{code:0,data:{items:[],total:0,providers:[{pluginId:"idc-resource",total:0}]}}});
    });
    const view = new CustomerPage(page);
    await view.open("/account/product-directories/physical");
    await expect(view.serviceBody).toContainText("暂无数据");
    await expect(view.serviceBody.getByRole("columnheader")).toHaveCount(8);
    await expect(view.serviceBody.getByRole("button", {name:/刷\s*新/})).toHaveCount(0);
    await expect(view.serviceBody.locator(".vxe-toolbar")).toBeHidden();
    const input=view.serviceBody.getByRole("textbox",{name:"搜索服务",exact:true});
    await expect(input).toHaveAttribute("placeholder","搜索产品名称、服务编号或备注");
    const search=view.serviceBody.getByRole("button",{name:/搜\s*索/});
    const inputBox=await input.boundingBox(), searchBox=await search.boundingBox();
    expect(Math.abs(inputBox!.y-searchBox!.y)).toBeLessThan(4);
    expect(searchBox!.x-inputBox!.x-inputBox!.width).toBeLessThan(150);
    const assertHeight=async(height:number)=>{
      await expect.poll(async()=>{const box=await view.serviceBody.locator(".vxe-grid").boundingBox();return Math.abs(height-24-box!.y-box!.height)}).toBeLessThan(24);
      await expect(view.serviceBody.locator(".vxe-pager")).toBeInViewport();
    };
    await assertHeight(960);
    await view.searchServices("没有这个物理机");
    await expect.poll(()=>keywords.at(-1)).toBe("没有这个物理机");
    await expect(view.serviceBody).toContainText("暂无数据");
    await expect(view.serviceBody).toContainText("共 0 条记录");
    await view.screenshot("empty-physical-full-height");
    await page.setViewportSize({width:1280,height:780});
    await assertHeight(780);
    await view.screenshot("empty-physical-resized");
  });

});

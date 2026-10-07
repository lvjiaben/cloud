import type { Page, Route } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

function envelope(data: unknown) {
  return JSON.stringify({ code: 0, data, message: 'OK' });
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({
    body: envelope(data),
    contentType: 'application/json',
    status: 200,
  });
}

export async function installCustomerPluginSurfaceMock(
  page: Page,
  options: {
    enabled?: boolean;
    locale?: 'en-US' | 'zh-CN';
    newBusinessState?: 'accepting' | 'stopped';
    pluginReportsError?: boolean;
    slowAssetMs?: number;
  } = {},
) {
  let assetRequests = 0;
  let bridgeRequestHeaders: Record<string, string> = {};
  let grantRequestBody: Record<string, unknown> = {};
  await mockCustomerAPIs(page, { locale: options.locale });

  await page.route('**/api/v1/plugins/dynamic', async (route) => {
    await fulfill(route, {
      list: [
        {
          customerSurfaces: [
            {
              bridgeCapabilities: ['phone-orders.query-own'],
              entry: 'customer/phone-orders/index.html',
              entryUrl:
                '/x-assets/phone-provider/v1.0.0/customer/phone-orders/index.html',
              icon: 'lucide:smartphone',
              key: 'phone-orders',
              pluginId: 'phone-provider',
              pluginType: 'dynamic',
              routePath: '/account/plugins/phone-provider/phone-orders',
              slot: 'customer.products',
              status: 'active',
              title: options.locale === 'en-US' ? 'Phone orders' : '手机订单',
              version: 'v1',
            },
          ],
          enabled: options.enabled === false ? 0 : 1,
          generation: 1,
          id: 'phone-provider',
          installed: 1,
          newBusinessState: options.newBusinessState ?? 'accepting',
          runtimeState: 'normal',
          statusKey: 'sys_plugin.status:phone-provider',
          type: 'dynamic',
          version: 'v1.0.0',
        },
      ],
    });
  });

  await page.route('**/api/v1/plugins/page-bridge/grant', async (route) => {
    grantRequestBody = route.request().postDataJSON() as Record<
      string,
      unknown
    >;
    await fulfill(route, {
      expiresAt: Math.floor(Date.now() / 1000) + 120,
      grant: 'server-signed-opaque-grant',
    });
  });

  await page.route('**/x/phone-provider/api/v1/phone-orders', async (route) => {
    bridgeRequestHeaders = route.request().headers();
    await fulfill(route, { owner: 'current-customer', status: 'shipped' });
  });
  const lifecycleSignal = options.pluginReportsError
    ? "send('host.view.v1','error')"
    : "send('host.view.v1','ready');send('host.view.v1','resize',{height:777})";

  await page.route(
    '**/x-assets/phone-provider/v1.0.0/customer/phone-orders/index.html',
    async (route) => {
      assetRequests += 1;
      if (options.slowAssetMs) {
        await new Promise((resolve) =>
          setTimeout(resolve, options.slowAssetMs),
        );
      }
      await route.fulfill({
        body: `<!doctype html><html><body style="margin:0"><main data-testid="phone-orders-plugin-page">手机订单内容</main><p data-testid="sandbox-result"></p><p data-testid="bridge-context"></p><p data-testid="bridge-api"></p><p data-testid="bridge-denied"></p><script>
const protocol='linapro.plugin-page-bridge';const version='1.0';const pluginId='phone-provider';const surfaceKey='phone-orders';let routeSynced=false;
function send(capability,method,payload,requestId){parent.postMessage({protocol,version,direction:'plugin-to-host',pluginId,surfaceKey,capability,method,payload,requestId},'*')}
let result='未隔离';try{void parent.localStorage.getItem('token')}catch{result='已隔离'}document.querySelector('[data-testid=sandbox-result]').textContent=result;
addEventListener('message',(event)=>{const data=event.data;if(data?.protocol!==protocol||data?.direction!=='host-to-plugin')return;if(data.capability==='host.context.v1'){const theme=data.payload.theme;const english=data.payload.locale==='en-US';document.documentElement.lang=data.payload.locale;document.documentElement.dataset.theme=theme.mode;for(const [name,value] of Object.entries(theme.cssVariables||{})){document.documentElement.style.setProperty(name,value)}document.body.style.background='hsl(var(--background))';document.body.style.color='hsl(var(--foreground))';document.body.style.fontFamily=theme.fontFamily;document.body.style.fontSize=theme.fontSize+'px';document.querySelector('[data-testid=phone-orders-plugin-page]').textContent=english?'Phone order content':'手机订单内容';document.querySelector('[data-testid=bridge-context]').textContent=data.payload.locale+'|'+theme.mode;send('host.view.v1','set-title',{title:english?'Phone order details':'手机订单详情'});if(!routeSynced){routeSynced=true;send('host.view.v1','navigate',{path:'orders/active'})}}if(data.requestId==='allowed'){document.querySelector('[data-testid=bridge-api]').textContent=data.ok?data.result.owner+'|'+data.result.status:data.error.code}if(data.requestId==='denied'){document.querySelector('[data-testid=bridge-denied]').textContent=data.error.code}});
${lifecycleSignal};
send('phone-orders.query-own','request',{method:'GET',path:'phone-orders'},'allowed');
send('other-plugin.read','request',{method:'GET',path:'phone-orders'},'denied');
</script></body></html>`,
        contentType: 'text/html; charset=utf-8',
        status: 200,
      });
    },
  );

  return {
    getAssetRequests: () => assetRequests,
    getBridgeRequestHeaders: () => bridgeRequestHeaders,
    getGrantRequestBody: () => grantRequestBody,
  };
}

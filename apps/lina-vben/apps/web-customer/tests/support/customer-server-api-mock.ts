import type { Page } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

type Body = Record<string, unknown>;

const now = 1_787_184_000_000;
export const serverServiceNo = 'SVC-SERVER-0001';
export const reinstallOperationNo = 'OP-REINSTALL-0001';

function envelope(data: unknown) {
  return { code: 0, data, message: 'OK' };
}

// Fixed-package UI fixture; no payment or upstream server action is performed.
export async function installCustomerTrafficPackageMock(page: Page) {
  const state = { quotes: [] as Body[], checkouts: [] as Body[], failCheckout: false };
  await page.route(/\/account\/servers\/[^/]+\/traffic-packages(?:\?|$)/u, async route => {
    await route.fulfill({ json: envelope({ total: 2, items: [
      { packageNo: 'tpk_customer_100', nameZhCN: '加量 100 GB', nameEnUS: '100 GB add-on', trafficGb: 100, currencyCode: 'CNY', amountMinor: 2000, available: true },
      { packageNo: 'tpk_customer_sold_out', nameZhCN: '售罄套餐', nameEnUS: 'Sold out package', trafficGb: 500, currencyCode: 'CNY', amountMinor: 8000, available: false },
    ] }) });
  });
  await page.route(/\/account\/servers\/[^/]+\/accessory-quotes$/u, async route => {
    state.quotes.push(route.request().postDataJSON() as Body);
    await route.fulfill({ json: envelope({ quote: {
      action: 'traffic_add', kind: 'traffic', quoteNo: 'QUOTE-TRAFFIC-0001', requestedQuantity: 1, chargedQuantity: 1,
      packageNameZhCN: '已确认 100 GB 套餐', packageNameEnUS: 'Confirmed 100 GB package', trafficGb: 100,
      currencyCode: 'CNY', totalAmountMinor: 2500, expiresAt: Date.now() + 300_000,
    } }) });
  });
  await page.route(/\/account\/server-accessory-quotes\/QUOTE-TRAFFIC-0001\/checkout$/u, async route => {
    state.checkouts.push(route.request().postDataJSON() as Body);
    if (state.failCheckout) {
      state.failCheckout = false;
      await route.fulfill({ status: 503, json: { code: 50, message: '下单结果暂时不可用，请重试' } });
      return;
    }
    await route.fulfill({ json: envelope({ checkout: { serviceNo: serverServiceNo, invoiceNo: 'INV-TRAFFIC-0001', orderNo: 'ORDER-TRAFFIC-0001', operationNo: '', amountMinor: 2500, currencyCode: 'CNY', paymentRequired: true, status: 'awaiting_financial_settlement' } }) });
  });
  return state;
}

function server(runtimeStatus = 'running', availableActions?: string[]) {
  return {
    availableActions:
      availableActions ||
      (runtimeStatus === 'stopped'
        ? [
            'start',
            'reset_password',
            'reinstall',
            'snapshot_create',
            'snapshot_restore',
          ]
        : [
            'stop',
            'reboot',
            'reset_password',
            'console',
            'reinstall',
            'snapshot_create',
            'traffic_add',
          ]),
    capabilityObservedAt: now,
    entitlementExpiresAt: now + 365 * 86_400_000,
    entitlementStartedAt: now - 30 * 86_400_000,
    entitlementStatus: 'active',
    periodCount: 1,
    periodUnit: 'month',
    productNameEnUS: 'Production Cloud Server',
    productNameZhCN: '生产云服务器',
    productType: 'cloud_server',
    providerTaskStatus: 'succeeded',
    runtimeStatus,
    serviceNo: serverServiceNo,
    updatedAt: now,
    version: 8,
  };
}

function detail(
  runtimeStatus: string,
  operations: Body[],
  availableActions?: string[],
) {
  return {
    accessories: [
      {
        kind: 'disk',
        name: 'System disk',
        objectNo: 'disk-0001',
        projection: { sizeGiB: 80 },
        state: 'attached',
      },
      {
        kind: 'snapshot',
        name: 'Before upgrade',
        objectNo: 'snapshot-0001',
        projection: {},
        state: 'available',
      },
      {
        kind: 'ip',
        name: 'Primary IP',
        objectNo: 'ip-0001',
        projection: { address: '203.0.113.10' },
        state: 'active',
      },
    ],
    accessoriesExpiresAt: Date.now() + 300_000,
    accessoriesObservedAt: Date.now(),
    configuration: [
      {
        componentType: 'cpu',
        groupNameEnUS: 'CPU',
        groupNameZhCN: '处理器',
        normalizedValue: '4',
        optionNameEnUS: '4 cores',
        optionNameZhCN: '4 核',
        quantity: 1,
      },
    ],
    operations,
    reinstallImages: [
      {
        arch: 'x86_64',
        bootMode: 'UEFI',
        group: 'Linux',
        name: 'Ubuntu 24.04 LTS',
        osType: 'Linux',
        reference: '101',
        selectable: true,
      },
      {
        arch: 'x86_64',
        bootMode: 'BIOS',
        group: 'Legacy',
        name: 'Retired image',
        osType: 'Linux',
        reference: '102',
        selectable: false,
      },
    ],
    reinstallImagesObservedAt: now,
    server: server(runtimeStatus, availableActions),
  };
}

export interface CustomerServerMockState {
  authorizationBodies: Body[];
  checkoutBodies: Body[];
  confirmedTransfers: Body[];
  createBodies: Body[];
  outputRequests: number;
  quoteBodies: Body[];
  resizeCheckoutBodies: Body[];
  resizeQuoteBodies: Body[];
  transferBodies: Body[];
}

export async function installCustomerServerMock(
  page: Page,
  options: {
    availableActions?: string[];
    locale?: 'en-US' | 'zh-CN';
    runtimeStatus?: 'running' | 'stopped';
  } = {},
) {
  await mockCustomerAPIs(page, options);
  const state: CustomerServerMockState = {
    authorizationBodies: [],
    checkoutBodies: [],
    confirmedTransfers: [],
    createBodies: [],
    outputRequests: 0,
    quoteBodies: [],
    resizeCheckoutBodies: [],
    resizeQuoteBodies: [],
    transferBodies: [],
  };
  let runtimeStatus = options.runtimeStatus || 'running';
  const operations: Body[] = [];

  await page.route(
    '**/x/idc-core/api/v1/account/server**',
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const method = request.method();

      if (
        path.endsWith('/account/server-transfers/incoming') &&
        method === 'GET'
      ) {
        await route.fulfill({
          json: envelope({
            items: [
              {
                expiresAt: now + 24 * 60 * 60 * 1_000,
                serviceNo: 'SVC-INCOMING-0002',
                transferNo: 'TRF-INCOMING-0002',
              },
            ],
          }),
        });
        return;
      }
      if (
        path.endsWith(
          '/account/server-transfers/TRF-INCOMING-0002/confirmations',
        ) &&
        method === 'POST'
      ) {
        state.confirmedTransfers.push(request.postDataJSON() as Body);
        await route.fulfill({
          json: envelope({
            reused: false,
            serviceNo: 'SVC-INCOMING-0002',
            status: 'confirmed',
            transferNo: 'TRF-INCOMING-0002',
          }),
        });
        return;
      }

      if (path.endsWith('/account/servers') && method === 'GET') {
        await route.fulfill({
          json: envelope({
            items: [server(runtimeStatus, options.availableActions)],
            nextCursor: '',
          }),
        });
        return;
      }
      if (path.endsWith(`/${serverServiceNo}/transfers`) && method === 'POST') {
        state.transferBodies.push(request.postDataJSON() as Body);
        await route.fulfill({
          json: envelope({ accepted: true, reused: false }),
        });
        return;
      }
      if (
        path.endsWith(`/${serverServiceNo}/resize-options`) &&
        method === 'GET'
      ) {
        await route.fulfill({
          json: envelope({
            currencyCode: 'CNY',
            currentSelections: [
              { optionNo: 'OPT-CPU-4', quantity: 1 },
              { optionNo: 'OPT-DISK-80', quantity: 1 },
            ],
            groups: [
              {
                groupNo: 'GROUP-CPU',
                maximumSelected: 1,
                minimumSelected: 1,
                nameEnUS: 'CPU',
                nameZhCN: '处理器',
                options: [
                  {
                    componentType: 'cpu',
                    defaultQuantity: 1,
                    maximumQuantity: 1,
                    minimumQuantity: 1,
                    nameEnUS: '4 cores',
                    nameZhCN: '4 核',
                    optionNo: 'OPT-CPU-4',
                  },
                  {
                    componentType: 'cpu',
                    defaultQuantity: 1,
                    maximumQuantity: 1,
                    minimumQuantity: 1,
                    nameEnUS: '8 cores',
                    nameZhCN: '8 核',
                    optionNo: 'OPT-CPU-8',
                  },
                ],
                selectionMode: 'single',
              },
              {
                groupNo: 'GROUP-DISK',
                maximumSelected: 1,
                minimumSelected: 1,
                nameEnUS: 'System disk',
                nameZhCN: '系统盘',
                options: [
                  {
                    componentType: 'system_disk',
                    defaultQuantity: 1,
                    maximumQuantity: 1,
                    minimumQuantity: 1,
                    nameEnUS: '80 GiB',
                    nameZhCN: '80 GiB',
                    optionNo: 'OPT-DISK-80',
                  },
                ],
                selectionMode: 'single',
              },
            ],
            publicSlug: 'production-cloud-server',
            revision: 'revision-signed',
            rules: [],
          }),
        });
        return;
      }
      if (
        path.endsWith(`/${serverServiceNo}/resize-quotes`) &&
        method === 'POST'
      ) {
        state.resizeQuoteBodies.push(request.postDataJSON() as Body);
        await route.fulfill({
          json: envelope({
            quote: {
              chargeAmountMinor: 1250,
              currencyCode: 'CNY',
              currentRecurringAmountMinor: 5000,
              direction: 'upgrade',
              effectiveAt: now,
              expiresAt: now + 300_000,
              issuedAt: now,
              quoteNo: 'RESIZE-QUOTE-0001',
              serviceNo: serverServiceNo,
              targetRecurringAmountMinor: 8000,
            },
          }),
        });
        return;
      }
      if (
        path.endsWith(
          '/account/server-resize-quotes/RESIZE-QUOTE-0001/checkout',
        ) &&
        method === 'POST'
      ) {
        state.resizeCheckoutBodies.push(request.postDataJSON() as Body);
        await route.fulfill({
          json: envelope({
            checkout: {
              chargeAmountMinor: 1250,
              currencyCode: 'CNY',
              direction: 'upgrade',
              effectiveAt: now,
              invoiceNo: 'INV-RESIZE-0001',
              operationNo: 'OP-RESIZE-0001',
              orderNo: 'ORDER-RESIZE-0001',
              serviceNo: serverServiceNo,
              status: 'awaiting_financial_settlement',
            },
          }),
        });
        return;
      }
      if (
        path.endsWith(`/${serverServiceNo}/accessory-quotes`) &&
        method === 'POST'
      ) {
        const body = request.postDataJSON() as Body;
        state.quoteBodies.push(body);
        await route.fulfill({
          json: envelope({
            quote: {
              action: body.action,
              chargedQuantity: 0,
              currencyCode: 'CNY',
              expiresAt: now + 300_000,
              freeCountPerServer: 2,
              kind: 'snapshot',
              periodCount: 0,
              periodUnitCode: 'one_time',
              quoteNo: 'QUOTE-0001',
              requestedQuantity: 1,
              totalAmountMinor: 0,
            },
          }),
        });
        return;
      }
      if (
        path.endsWith('/account/server-accessory-quotes/QUOTE-0001/checkout') &&
        method === 'POST'
      ) {
        state.checkoutBodies.push(request.postDataJSON() as Body);
        await route.fulfill({
          json: envelope({
            checkout: {
              amountMinor: 0,
              currencyCode: 'CNY',
              invoiceNo: '',
              operationNo: 'OP-SNAPSHOT-0001',
              paymentDueAt: 0,
              paymentRequired: false,
              serviceNo: serverServiceNo,
              status: 'ready',
            },
          }),
        });
        return;
      }
      if (
        path.endsWith(`/${serverServiceNo}/operation-authorizations`) &&
        method === 'POST'
      ) {
        const body = request.postDataJSON() as Body;
        state.authorizationBodies.push(body);
        const required = body.action === 'reinstall';
        await route.fulfill({
          json: envelope({
            expiresAt: required ? now + 120_000 : undefined,
            proofToken: required ? 'server-operation-proof' : '',
            required,
          }),
        });
        return;
      }
      if (
        path.endsWith(
          `/${serverServiceNo}/operations/${reinstallOperationNo}/confidential-output`,
        ) &&
        method === 'POST'
      ) {
        state.outputRequests += 1;
        operations[0]!.confidentialOutputAvailable = false;
        await route.fulfill({
          headers: {
            'cache-control': 'no-store, max-age=0',
            pragma: 'no-cache',
          },
          json: envelope({
            payload: {
              password: 'Temporary-Password-1!',
              port: 22,
              username: 'root',
            },
          }),
        });
        return;
      }
      if (
        path.endsWith(`/${serverServiceNo}/operations`) &&
        method === 'POST'
      ) {
        const body = request.postDataJSON() as Body;
        state.createBodies.push(body);
        const action = String(body.action || '');
        const operationNo =
          action === 'reinstall' ? reinstallOperationNo : 'OP-START-0001';
        if (action === 'start') runtimeStatus = 'running';
        operations.unshift({
          action,
          completedAt: now + 1_000,
          confidentialOutputAvailable: action === 'reinstall',
          createdAt: now,
          failureCode: '',
          operationNo,
          status: 'succeeded',
          updatedAt: now + 1_000,
        });
        await route.fulfill({
          json: envelope({ operation: operations[0] }),
        });
        return;
      }
      if (path.endsWith(`/${serverServiceNo}`) && method === 'GET') {
        await route.fulfill({
          json: envelope({
            detail: detail(runtimeStatus, operations, options.availableActions),
          }),
        });
        return;
      }
      await route.fulfill({ json: envelope({}), status: 404 });
    },
  );

  return state;
}

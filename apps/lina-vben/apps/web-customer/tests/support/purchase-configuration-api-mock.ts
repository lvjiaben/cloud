import type { Page, Route } from '@playwright/test';
import { mockCustomerAPIs } from './customer-api-mock';

const envelope = (data: unknown) =>
  JSON.stringify({ code: 0, data, message: 'OK' });
async function ok(route: Route, data: unknown) {
  await route.fulfill({
    body: envelope(data),
    contentType: 'application/json',
    status: 200,
  });
}

export async function installPurchaseConfigurationMock(page: Page) {
  await mockCustomerAPIs(page);
  const state = {
    cartFrozen: false as boolean,
    evaluationFileTokens: [] as string[][],
    quoteRequests: 0,
    revoked: [] as string[],
    uploaded: [] as string[],
  };
  let uploadIndex = 0;
  const toggleField = {
    descriptionEnUS: 'Attach a certificate request',
    descriptionZhCN: '需要证书申请文件',
    disabled: false,
    fieldToken: 'opaque-toggle-field',
    labelEnUS: 'Certificate request requirement',
    labelZhCN: '证书文件要求',
    required: false,
    type: 'boolean',
    visible: true,
  };
  const field = {
    descriptionEnUS: 'Upload the certificate request',
    descriptionZhCN: '上传证书申请文件',
    disabled: false,
    fieldToken: 'opaque-file-field',
    labelEnUS: 'Certificate request',
    labelZhCN: '证书申请文件',
    required: true,
    type: 'file',
    uploadPolicy: {
      allowedExtensions: ['.pdf'],
      allowedMimeTypes: ['application/pdf'],
      maximumFileBytes: 1048576,
      maximumFiles: 1,
      maximumTotalBytes: 1048576,
      safetyScanRequired: true,
    },
    visible: true,
  };
  const commerceQuote = (frozen: boolean, quoteNo = 'quote-one') => ({
    cartNo: 'cart-one',
    cartVersion: 1,
    currencyCode: 'CNY',
    discountMinor: '0',
    expiresAt: Date.now() + 3_600_000,
    issuedAt: Date.now(),
    items: [
      {
        adjustments: [],
        cartItemNo: 'cart-item-one',
        configurationFrozen: frozen,
        configurationRequired: true,
        configurationRevision: frozen ? 'opaque-frozen-revision' : '',
        discountMinor: '0',
        productNameEnUS: 'SSL certificate',
        productNameZhCN: 'SSL 证书',
        publicSlug: 'ssl-certificate',
        purchaseCycles: 1,
        quoteItemNo: `${quoteNo}-item`,
        quantity: 1,
        subtotalMinor: '1000',
        taxAmountMinor: '0',
        totalAmountMinor: '1000',
      },
    ],
    netAmountMinor: '1000',
    quoteNo,
    subtotalMinor: '1000',
    taxAmountMinor: '0',
    totalAmountMinor: '1000',
  });
  const commerceCart = () => ({
    cartNo: 'cart-one',
    currencyCode: 'CNY',
    items: [
      {
        cartItemNo: 'cart-item-one',
        configuration: [],
        configurationFrozen: state.cartFrozen,
        configurationRequired: true,
        configurationRevision: state.cartFrozen ? 'opaque-frozen-revision' : '',
        issueCode: '',
        productNameEnUS: 'SSL certificate',
        productNameZhCN: 'SSL 证书',
        publicSlug: 'ssl-certificate',
        purchaseCycles: 1,
        repairAction: '',
        quantity: 1,
        statusCode: 'active',
        version: 1,
      },
    ],
    latestQuote: commerceQuote(state.cartFrozen),
    version: 1,
  });
  await page.route('**/x/idc-core/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();
    if (path.endsWith('/account/commerce/cart') && method === 'GET')
      return ok(route, { cart: commerceCart() });
    if (path.endsWith('/account/commerce/quotes') && method === 'POST') {
      state.quoteRequests += 1;
      state.cartFrozen = false;
      return ok(route, { quote: commerceQuote(false, 'quote-reconfigured') });
    }
    if (path.endsWith('/public/catalog/products') && method === 'GET')
      return ok(route, {
        nextCursor: '',
        products: [
          {
            availability: 'available',
            categoryNameEnUS: 'Certificates',
            categoryNameZhCN: '证书',
            currencies: ['CNY'],
            nameEnUS: 'SSL certificate',
            nameZhCN: 'SSL 证书',
            publicSlug: 'ssl-certificate',
            summaryEnUS: 'Issued automatically',
            summaryZhCN: '自动签发',
          },
        ],
      });
    if (
      path.endsWith(
        '/public/catalog/products/ssl-certificate/purchase-configuration',
      ) &&
      method === 'GET'
    )
      return ok(route, {
        configurationRevision: 'opaque-revision',
        fields: [toggleField, field],
        values: [
          {
            booleanValue: true,
            fieldToken: toggleField.fieldToken,
            type: 'boolean',
          },
        ],
      });
    if (
      path.endsWith('/purchase-configuration/evaluate') &&
      method === 'POST'
    ) {
      const body = request.postDataJSON() as {
        values: Array<{
          booleanValue?: boolean;
          fieldToken: string;
          fileTokens?: string[];
        }>;
      };
      const enabled =
        body.values.find((row) => row.fieldToken === toggleField.fieldToken)
          ?.booleanValue === true;
      const tokens =
        body.values.find((row) => row.fieldToken === field.fieldToken)
          ?.fileTokens ?? [];
      state.evaluationFileTokens.push(tokens);
      return ok(route, {
        configurationRevision: 'opaque-revision',
        fields: [toggleField, { ...field, visible: enabled }],
        issues: [],
        valid: !enabled || tokens.length === 1,
        values: body.values,
      });
    }
    if (path.endsWith('/purchase-configuration/quote') && method === 'POST')
      return ok(route, {
        configurationRevision: 'opaque-revision',
        currencyCode: 'CNY',
        terms: [
          {
            billingTermToken: 'term',
            labelEnUS: 'Monthly',
            labelZhCN: '月付',
            minimumPurchaseCycles: 1,
            setupAmount: '0',
            status: 'final',
            subtotalAmount: '1000',
            taxAmount: '0',
            totalAmount: '1000',
          },
        ],
      });
    if (path.endsWith('/file-uploads') && method === 'POST') {
      uploadIndex += 1;
      const uploadToken = `CFU_${String(uploadIndex).padStart(32, 'A')}`.slice(
        0,
        36,
      );
      return ok(route, { expiresAt: Date.now() + 3600000, uploadToken });
    }
    if (path.endsWith('/content') && method === 'PUT') {
      const token = path.split('/').at(-2)!;
      state.uploaded.push(token);
      return ok(route, {
        expiresAt: Date.now() + 604800000,
        fileName: 'request.pdf',
        fileToken: token,
        mediaType: 'application/pdf',
        sizeBytes: 24,
        status: 'ready',
      });
    }
    if (path.includes('/file-uploads/') && method === 'DELETE') {
      const token = path.split('/').at(-1)!;
      state.revoked.push(token);
      return ok(route, {
        expiresAt: Date.now(),
        fileName: 'request.pdf',
        fileToken: token,
        mediaType: 'application/pdf',
        sizeBytes: 24,
        status: 'revoked',
      });
    }
    await route.fallback();
  });
  return state;
}

import type { Page, Route } from '@playwright/test';

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

async function fulfillFile(route: Route, body: string) {
  await route.fulfill({
    body,
    contentType: 'text/plain; charset=utf-8',
    status: 200,
  });
}

async function reject(route: Route) {
  await route.fulfill({
    body: JSON.stringify({ code: 403, data: null, message: 'expired' }),
    contentType: 'application/json',
    status: 403,
  });
}

function supportCustomFields() {
  return [
    {
      descriptionEnUS: 'Short environment name',
      descriptionZhCN: '填写简短的环境名称',
      fieldNo: 'CF-TEXT',
      fieldType: 'single_line',
      nameEnUS: 'Environment',
      nameZhCN: '环境名称',
      options: [],
      required: true,
      sortOrder: 10,
    },
    {
      descriptionEnUS: 'Describe the expected result',
      descriptionZhCN: '说明期望结果',
      fieldNo: 'CF-MULTI-LINE',
      fieldType: 'multi_line',
      nameEnUS: 'Expected result',
      nameZhCN: '期望结果',
      options: [],
      required: false,
      sortOrder: 20,
    },
    {
      fieldNo: 'CF-NUMBER',
      fieldType: 'number',
      nameEnUS: 'Affected servers',
      nameZhCN: '影响服务器数量',
      options: [],
      required: true,
      sortOrder: 30,
    },
    {
      fieldNo: 'CF-SINGLE',
      fieldType: 'single_select',
      nameEnUS: 'Business area',
      nameZhCN: '业务区域',
      options: [
        {
          nameEnUS: 'East China',
          nameZhCN: '华东',
          optionNo: 'OPT-EAST',
          sortOrder: 10,
        },
        {
          nameEnUS: 'South China',
          nameZhCN: '华南',
          optionNo: 'OPT-SOUTH',
          sortOrder: 20,
        },
      ],
      required: true,
      sortOrder: 40,
    },
    {
      fieldNo: 'CF-MULTI-SELECT',
      fieldType: 'multi_select',
      nameEnUS: 'Affected functions',
      nameZhCN: '影响功能',
      options: [
        {
          nameEnUS: 'Network',
          nameZhCN: '网络',
          optionNo: 'OPT-NETWORK',
          sortOrder: 10,
        },
        {
          nameEnUS: 'Storage',
          nameZhCN: '存储',
          optionNo: 'OPT-STORAGE',
          sortOrder: 20,
        },
      ],
      required: false,
      sortOrder: 50,
    },
    {
      fieldNo: 'CF-DATE',
      fieldType: 'date',
      nameEnUS: 'First observed date',
      nameZhCN: '首次发现日期',
      options: [],
      required: true,
      sortOrder: 60,
    },
    {
      fieldNo: 'CF-BOOLEAN',
      fieldType: 'boolean',
      nameEnUS: 'Production affected',
      nameZhCN: '是否影响生产',
      options: [],
      required: true,
      sortOrder: 70,
    },
  ];
}

function ticketDetail(messages = 1, rated = false) {
  return {
    allowedActions: rated ? ['reply', 'close'] : ['reply', 'close', 'rate'],
    customFields: [
      {
        fieldNo: 'CF-SINGLE',
        fieldType: 'single_select',
        nameEnUS: 'Business area at creation',
        nameZhCN: '创建时业务区域',
        normalizedValue: 'OPT-RETIRED',
        required: true,
        selectedOptions: [
          {
            nameEnUS: 'Retired region',
            nameZhCN: '已停用区域',
            optionNo: 'OPT-RETIRED',
            sortOrder: 10,
          },
        ],
        sortOrder: 10,
      },
    ],
    messages: [
      {
        attachments: [
          {
            attachmentNo: 'ATT-ACCOUNT-1',
            fileName: 'network-trace.log',
            mediaType: 'text/plain',
            sizeBytes: 18,
          },
        ],
        author: 'customer',
        automated: false,
        body: '服务器无法连接，请协助检查。',
        createdAt: 1_786_320_000_000,
        kind: 'reply',
        messageNo: 'message-1',
      },
      ...(messages > 1
        ? [
            {
              attachments: [],
              author: 'visitor',
              automated: false,
              body: '请同时说明是否支持自定义硬盘。',
              createdAt: 1_786_320_700_000,
              kind: 'reply',
              messageNo: 'message-2',
            },
          ]
        : []),
    ],
    reopenDeadline: 1_786_924_800_000,
    ...(rated
      ? {
          rating: {
            feedback: '仍需观察',
            roundNo: 2,
            stars: 2,
            submittedAt: 1_786_320_900_000,
          },
        }
      : {}),
    ticket: {
      category: {
        categoryNo: 'network',
        nameEnUS: 'Network',
        nameZhCN: '网络问题',
      },
      createdAt: 1_786_320_000_000,
      currentRound: 2,
      lastActivityAt: 1_786_320_600_000,
      priority: 'normal',
      relatedEntitlement: {
        associationReference: 'sar_0123456789abcdef0123456789abcdef',
        currentAvailable: true,
        currentStatus: 'active',
        currentNameEnUS: 'Current cloud server',
        currentNameZhCN: '当前云服务器',
        currentVersion: 4,
        snapshotStatus: 'active',
        snapshotNameEnUS: 'Cloud server at creation',
        snapshotNameZhCN: '创建时云服务器',
        snapshotVersion: 3,
      },
      status: 'resolved',
      subject: '服务器网络异常',
      ticketNo: 'TKT-2026-0001',
      updatedAt: 1_786_320_600_000,
      version: messages > 1 ? 4 : 3,
    },
  };
}

type PresalesDetailState = {
  closed?: boolean;
  feedback?: string;
  rated?: boolean;
  round?: number;
  stars?: number;
  version?: number;
};

function presalesDetail(
  messages = 1,
  replyAttachmentReady = false,
  state: PresalesDetailState = {},
) {
  const detail = ticketDetail(messages);
  const round = state.round ?? 2;
  return {
    ...detail,
    allowedActions: state.closed
      ? ['reopen', ...(state.rated ? [] : ['rate'])]
      : ['reply', 'close', ...(state.rated ? [] : ['rate'])],
    messages: detail.messages.map((message, index) => {
      if (index === 0) {
        return {
          ...message,
          attachments: [
            {
              attachmentNo: 'ATT-PRESALES-1',
              fileName: 'server-spec.txt',
              mediaType: 'text/plain',
              sizeBytes: 16,
            },
          ],
        };
      }
      return replyAttachmentReady
        ? {
            ...message,
            attachments: [
              {
                attachmentNo: 'ATT-PRESALES-REPLY-1',
                fileName: 'presales-note.txt',
                mediaType: 'text/plain',
                sizeBytes: 22,
              },
            ],
          }
        : message;
    }),
    ...(state.rated
      ? {
          rating: {
            feedback: state.feedback ?? '',
            roundNo: round,
            stars: state.stars ?? 5,
            submittedAt: 1_786_320_900_000,
          },
        }
      : {}),
    ticket: {
      ...detail.ticket,
      category: {
        categoryNo: 'sales',
        nameEnUS: 'Purchase inquiry',
        nameZhCN: '购买咨询',
      },
      currentRound: round,
      status: state.closed ? 'closed' : 'resolved',
      subject: '裸金属服务器交付咨询',
      ticketNo: 'PRE-2026-0001',
      relatedEntitlement: undefined,
      version: state.version ?? detail.ticket.version,
    },
  };
}

export async function mockCustomerAPIs(
  page: Page,
  options: {
    externalMenus?: Array<{
      enabled: boolean;
      icon: string;
      id: number;
      nameEn: string;
      nameZh: string;
      sortOrder: number;
      url: string;
      version: number;
    }>;
    locale?: 'en-US' | 'zh-CN';
  } = {},
) {
  const runtimeLocale = options.locale || 'zh-CN';
  let accountRated = false;
  let marketingNotificationsEnabled = true;
  let notificationPreferenceVersion = 2;
  let notificationQuietHours = {
    enabled: true,
    endMinute: 420,
    startMinute: 1320,
    timezoneName: 'Asia/Shanghai',
  };
  let supportNotificationsEnabled = true;
  let presalesMessages = 1;
  let presalesReplyAttachmentReady = false;
  let presalesClosed = false;
  let presalesFeedback = '';
  let presalesRated = false;
  let presalesRound = 2;
  let presalesStars = 0;
  let presalesVersion = 3;
  let presalesChallengeSequence = 0;
  let presalesVerificationSequence = 0;

  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith('/config/public/frontend')) {
      await fulfill(route, {
        app: { name: 'IDC 客户中心' },
        auth: {},
        ui: {},
        workspace: { basePath: '/' },
      });
      return;
    }
    if (url.pathname.endsWith('/i18n/runtime/locales')) {
      await fulfill(route, {
        enabled: true,
        items: [
          {
            isDefault: runtimeLocale === 'zh-CN',
            locale: 'zh-CN',
            name: '简体中文',
            nativeName: '简体中文',
          },
          {
            isDefault: runtimeLocale === 'en-US',
            locale: 'en-US',
            name: 'English',
            nativeName: 'English',
          },
        ],
        locale: runtimeLocale,
      });
      return;
    }
    if (url.pathname.endsWith('/i18n/runtime/messages')) {
      await fulfill(route, { messages: {} });
      return;
    }
    if (url.pathname.endsWith('/auth/login')) {
      await fulfill(route, {
        accessToken: 'customer-e2e-token',
        refreshToken: 'customer-e2e-refresh',
        tenants: [],
      });
      return;
    }
    if (url.pathname.endsWith('/user/info')) {
      await fulfill(route, {
        avatar: '',
        homePath: '/account',
        id: 1001,
        realName: '测试客户',
        roles: [],
        username: 'customer',
      });
      return;
    }
    if (url.pathname.endsWith('/account/customer/profile')) {
      await fulfill(route, {
        item: {
          countryCode: 'CN',
          customerCode: '10000',
          customerNo: 'CUS_00000000000000000000000000000001',
          displayName: '测试客户',
          identifierMasked: '310***********1234',
          legalNameMasked: '测**',
          preferredLocale: runtimeLocale,
          status: 'active',
          type: 'individual',
          verification: 'verified',
          version: 3,
        },
      });
      return;
    }
    if (url.pathname.endsWith('/account/billing/overview')) {
      await fulfill(route, {
        overview: {
          automaticRenewalEnabled: 1,
          balances: [
            {
              accountTypeCode: 'customer_balance',
              availableBalanceMinor: '12850',
              balanceMinor: '13000',
              currencyCode: 'CNY',
              heldBalanceMinor: '150',
              statusCode: 'active',
              updatedAt: 1_786_406_400_000,
              withdrawable: false,
            },
          ],
          openInvoiceCount: 2,
          openReceivables: [{ currencyCode: 'CNY', outstandingMinor: '8800' }],
          rechargeAvailable: true,
          rechargeReasonCode: '',
        },
      });
      return;
    }
    if (url.pathname.endsWith('/account/servers')) {
      await fulfill(route, {
        items: [
          {
            availableActions: ['reboot', 'stop'],
            entitlementExpiresAt: Date.now() + 30 * 86_400_000,
            entitlementStatus: 'active',
            productNameEnUS: 'Production Cloud Server',
            productNameZhCN: '生产云服务器',
            productType: 'cloud_server',
            runtimeStatus: 'running',
            serviceNo: 'SVC-SERVER-0001',
            updatedAt: Date.now(),
            version: 3,
          },
        ],
        nextCursor: '',
      });
      return;
    }
    if (url.pathname.endsWith('/customer/external-menus')) {
      await fulfill(route, { items: options.externalMenus || [] });
      return;
    }
    await fulfill(route, {});
  });

  await page.route('**/x/idc-core/api/v1/account/support/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.pathname.endsWith('/support/categories')) {
      await fulfill(route, {
        items: [
          {
            categoryNo: 'network',
            nameEnUS: 'Network',
            nameZhCN: '网络问题',
            priority: 'normal',
          },
        ],
        nextCursor: '',
      });
      return;
    }
    if (url.pathname.endsWith('/support/custom-fields')) {
      await fulfill(route, { items: supportCustomFields() });
      return;
    }
    if (url.pathname.endsWith('/support/entitlement-candidates')) {
      await fulfill(route, {
        items: [
          {
            associationReference: 'sar_0123456789abcdef0123456789abcdef',
            status: 'active',
            nameEnUS: 'Production cloud server',
            nameZhCN: '生产云服务器',
          },
        ],
        nextCursor: '',
      });
      return;
    }
    if (url.pathname.endsWith('/support/attachments/ATT-ACCOUNT-1/content')) {
      await fulfillFile(route, 'account attachment');
      return;
    }
    if (
      url.pathname.endsWith('/support/initial-attachment-uploads') &&
      request.method() === 'POST'
    ) {
      await fulfill(route, {
        expiresAt: 1_786_327_200_000,
        nextOffset: 0,
        uploadNo: 'UP-INITIAL-1',
        version: 1,
      });
      return;
    }
    if (
      url.pathname.endsWith(
        '/support/initial-attachment-uploads/UP-INITIAL-1/content',
      ) &&
      request.method() === 'PUT'
    ) {
      await fulfill(route, {
        fileName: 'initial-note.txt',
        mediaType: 'text/plain',
        ready: true,
        sizeBytes: 18,
        uploadNo: 'UP-INITIAL-1',
        version: 2,
      });
      return;
    }
    if (
      url.pathname.endsWith('/tickets/TKT-2026-0001/ratings') &&
      request.method() === 'POST'
    ) {
      accountRated = true;
      await fulfill(route, { item: ticketDetail(1, accountRated) });
      return;
    }
    if (url.pathname.endsWith('/tickets/TKT-2026-0001')) {
      await fulfill(route, { item: ticketDetail(1, accountRated) });
      return;
    }
    if (url.pathname.endsWith('/tickets') && request.method() === 'GET') {
      await fulfill(route, {
        nextCursor: '',
        tickets: [ticketDetail().ticket],
      });
      return;
    }
    if (url.pathname.endsWith('/tickets') && request.method() === 'POST') {
      await fulfill(route, { item: ticketDetail() });
      return;
    }
    await fulfill(route, { item: ticketDetail() });
  });

  await page.route(
    '**/x/idc-notify/api/v1/account/notifications**',
    async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname.endsWith('/deliveries')) {
        await fulfill(route, {
          items: [
            {
              channelCode: 'in_app',
              completedAt: 1_786_320_650_000,
              queuedAt: 1_786_320_600_000,
              status: 'delivered',
              taskNo: 'notification-task-1',
              topic: 'support',
            },
          ],
          nextCursor: '',
        });
        return;
      }
      if (url.pathname.endsWith('/preferences')) {
        if (route.request().method() === 'PUT') {
          const input = route.request().postDataJSON() as {
            channelCode?: string;
            contactNo?: string;
            enabled?: boolean;
            expectedVersion: number;
            locale?: string;
            marketingConsentAction?: string;
            policyReference?: string;
            policyVersion?: string;
            quietHours?: {
              enabled: boolean;
              endMinute: number;
              startMinute: number;
              timezoneName: string;
            };
            topic?: string;
          };
          if (input.expectedVersion === notificationPreferenceVersion) {
            if (
              input.quietHours &&
              input.channelCode === undefined &&
              input.contactNo === undefined &&
              input.enabled === undefined &&
              input.locale === undefined &&
              input.marketingConsentAction === undefined &&
              input.policyReference === undefined &&
              input.policyVersion === undefined &&
              input.topic === undefined
            ) {
              notificationQuietHours = { ...input.quietHours };
              notificationPreferenceVersion += 1;
            } else if (
              input.topic === 'support' &&
              typeof input.enabled === 'boolean'
            ) {
              supportNotificationsEnabled = input.enabled;
              notificationPreferenceVersion += 1;
            } else if (
              input.topic === 'marketing' &&
              input.contactNo === 'contact-1' &&
              input.channelCode === 'email' &&
              ((input.marketingConsentAction === 'granted' &&
                input.enabled &&
                input.locale === 'zh-CN' &&
                input.policyReference === 'privacy-notice' &&
                input.policyVersion === '2026-08-15') ||
                (input.marketingConsentAction === 'withdrawn' &&
                  !input.enabled &&
                  input.locale === undefined &&
                  input.policyReference === undefined &&
                  input.policyVersion === undefined))
            ) {
              marketingNotificationsEnabled = input.enabled === true;
              notificationPreferenceVersion += 1;
            }
          }
          await fulfill(route, {
            appliedAt: 1_786_320_900_000,
            evidenceDigest:
              '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
            reused: false,
            version: notificationPreferenceVersion,
          });
          return;
        }
        await fulfill(route, {
          customerNo: 'customer-1',
          marketingConsents: [
            {
              channelCode: 'email',
              contactNo: 'contact-1',
              locale: 'zh-CN',
              policyReference: 'privacy-notice',
              policyVersion: '2026-08-15',
              status: marketingNotificationsEnabled ? 'opted_in' : 'opted_out',
              version: notificationPreferenceVersion,
            },
          ],
          preferences: [
            {
              channelCode: 'in_app',
              contactNo: 'contact-1',
              enabled: supportNotificationsEnabled,
              topic: 'support',
              version: 2,
            },
            {
              channelCode: 'email',
              contactNo: 'contact-1',
              enabled: true,
              topic: 'security',
              version: 1,
            },
            {
              channelCode: 'email',
              contactNo: 'contact-1',
              enabled: marketingNotificationsEnabled,
              topic: 'marketing',
              version: notificationPreferenceVersion,
            },
          ],
          preferredLocale: 'zh-CN',
          quietHours: {
            ...notificationQuietHours,
            crossesMidnight:
              notificationQuietHours.enabled &&
              notificationQuietHours.startMinute >
                notificationQuietHours.endMinute,
            version: notificationPreferenceVersion,
          },
          version: notificationPreferenceVersion,
        });
        return;
      }
      await fulfill(route, {});
    },
  );

  await page.route('**/x/idc-core/api/v1/public/support/**', async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path.endsWith('/public/support/categories')) {
      await fulfill(route, {
        items: [
          {
            categoryNo: 'sales',
            nameEnUS: 'Purchase inquiry',
            nameZhCN: '购买咨询',
            priority: 'normal',
          },
        ],
        nextCursor: '',
      });
      return;
    }
    if (path.endsWith('/presales/custom-fields')) {
      await fulfill(route, { items: supportCustomFields() });
      return;
    }
    if (path.endsWith('/presales/access-verifications')) {
      const payload = request.postDataJSON() as { accessToken?: string };
      if (payload.accessToken === 'expired-token') {
        await reject(route);
        return;
      }
      await fulfill(route, {
        expiresAt: 1_786_327_200_000,
        item: presalesDetail(presalesMessages, false, {
          closed: presalesClosed,
          feedback: presalesFeedback,
          rated: presalesRated,
          round: presalesRound,
          stars: presalesStars,
          version: presalesVersion,
        }),
      });
      return;
    }
    if (
      path.endsWith('/presales/attachment-uploads') &&
      request.method() === 'POST'
    ) {
      const payload = request.postDataJSON() as {
        accessToken?: string;
        fileName?: string;
        sizeBytes?: number;
      };
      await fulfill(route, {
        expiresAt: 1_786_327_200_000,
        nextOffset: 0,
        uploadNo: 'UP-PRESALES-1',
        version: 1,
        ...(payload.accessToken && payload.fileName && payload.sizeBytes
          ? {}
          : { invalid: true }),
      });
      return;
    }
    if (
      path.endsWith('/presales/attachment-uploads/UP-PRESALES-1/content') &&
      request.method() === 'PUT'
    ) {
      const accessToken = new URL(request.url()).searchParams.get(
        'accessToken',
      );
      presalesReplyAttachmentReady =
        accessToken === 'active-token' &&
        request.postDataBuffer()?.toString() === 'presales attachment';
      await fulfill(route, {
        fileName: 'presales-note.txt',
        mediaType: 'text/plain',
        ready: presalesReplyAttachmentReady,
        sizeBytes: 22,
        uploadNo: 'UP-PRESALES-1',
        version: 2,
      });
      return;
    }
    if (path.endsWith('/presales/replies')) {
      const payload = request.postDataJSON() as {
        uploadNos?: string[];
      };
      presalesMessages = 2;
      await fulfill(route, {
        expiresAt: 1_786_327_200_000,
        item: presalesDetail(
          presalesMessages,
          presalesReplyAttachmentReady &&
            payload.uploadNos?.[0] === 'UP-PRESALES-1',
          {
            closed: presalesClosed,
            feedback: presalesFeedback,
            rated: presalesRated,
            round: presalesRound,
            stars: presalesStars,
            version: ++presalesVersion,
          },
        ),
      });
      return;
    }
    if (path.endsWith('/presales/ratings')) {
      const payload = request.postDataJSON() as {
        feedback?: string;
        roundNo?: number;
        stars?: number;
      };
      presalesFeedback = payload.feedback ?? '';
      presalesRated = payload.roundNo === presalesRound;
      presalesStars = payload.stars ?? 0;
      presalesVersion += 1;
      await fulfill(route, {
        expiresAt: 1_786_327_200_000,
        item: presalesDetail(presalesMessages, false, {
          closed: presalesClosed,
          feedback: presalesFeedback,
          rated: presalesRated,
          round: presalesRound,
          stars: presalesStars,
          version: presalesVersion,
        }),
      });
      return;
    }
    if (path.endsWith('/presales/closures')) {
      presalesClosed = true;
      presalesVersion += 1;
      await fulfill(route, {
        expiresAt: 1_786_327_200_000,
        item: presalesDetail(presalesMessages, false, {
          closed: presalesClosed,
          feedback: presalesFeedback,
          rated: presalesRated,
          round: presalesRound,
          stars: presalesStars,
          version: presalesVersion,
        }),
      });
      return;
    }
    if (path.endsWith('/presales/reopenings')) {
      presalesClosed = false;
      presalesRated = false;
      presalesFeedback = '';
      presalesStars = 0;
      presalesRound += 1;
      presalesVersion += 1;
      await fulfill(route, {
        expiresAt: 1_786_327_200_000,
        item: presalesDetail(presalesMessages, false, {
          closed: presalesClosed,
          rated: presalesRated,
          round: presalesRound,
          version: presalesVersion,
        }),
      });
      return;
    }
    if (
      path.endsWith('/presales/attachments/ATT-PRESALES-1/content') &&
      request.method() === 'POST'
    ) {
      await fulfillFile(route, 'presales attachment');
      return;
    }
    await fulfill(route, {});
  });

  await page.route(
    '**/x/idc-notify/api/v1/public/support/presales/**',
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const payload = (request.postDataJSON() || {}) as Record<string, string>;
      if (path.endsWith('/human-challenges')) {
        presalesChallengeSequence += 1;
        const common = {
          challengeNo: `CH-${presalesChallengeSequence}`,
          expiresAt: Date.now() + 300_000,
          providerCode: 'local',
        };
        if (payload.destination?.includes('turnstile')) {
          await fulfill(route, {
            ...common,
            kind: 'turnstile',
            providerAction: 'support_presales',
            siteKey: 'site-key',
          });
          return;
        }
        if (payload.presentationKind === 'accessible') {
          await fulfill(route, {
            ...common,
            accessiblePrompt: '2 + 3 = ?',
            kind: 'accessible',
          });
          return;
        }
        await fulfill(route, {
          ...common,
          imageDataUrl:
            'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="320" height="80"%3E%3Crect width="100%25" height="100%25" fill="white"/%3E%3Ctext x="28" y="52" font-size="32" fill="black"%3E2 + 3 = ?%3C/text%3E%3C/svg%3E',
          kind: 'image',
        });
        return;
      }
      if (path.endsWith('/human-challenge-completions')) {
        await fulfill(route, {
          expiresAt: Date.now() + 120_000,
          proofToken: `PROOF-${presalesChallengeSequence}`,
        });
        return;
      }
      if (path.endsWith('/verification-codes')) {
        presalesVerificationSequence += 1;
        await fulfill(route, {
          deliveryState: 'accepted',
          expiresAt: Date.now() + 600_000,
          maskedDestination:
            payload.destinationKind === 'sms'
              ? '138****8000'
              : 'b***@example.com',
          retryAfterAt: Date.now() + 60_000,
          verificationNo: `VER-${presalesVerificationSequence}`,
        });
        return;
      }
      if (path.endsWith('/tickets')) {
        await fulfill(route, {
          accessToken: 'created-token',
          expiresAt: Date.now() + 7_200_000,
          linkDeliveryState: payload.destination?.includes('unknown-link')
            ? 'result_unknown'
            : 'queued',
          ticketNo: 'PRE-2026-0001',
        });
        return;
      }
      if (path.endsWith('/access-renewals')) {
        await fulfill(route, {
          accessToken: 'renewed-token',
          expiresAt: Date.now() + 7_200_000,
          linkDeliveryState: 'queued',
          ticketNo: payload.ticketNo,
        });
        return;
      }
      await fulfill(route, {});
    },
  );
}

import { beforeEach, describe, expect, it, vi } from 'vitest';

const requestClient = {
  delete: vi.fn(),
  download: vi.fn(),
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
};

vi.mock('#/api/request', () => ({
  pluginApiPath: (pluginID: string, path: string) =>
    `/x/${pluginID}/api/v1/${path}`,
  requestClient,
}));

const ticketDetail = {
  allowedActions: ['reply', 'close', 'rate'],
  messages: [
    {
      attachments: [],
      author: 'customer',
      automated: false,
      body: 'Network issue',
      createdAt: 1,
      messageNo: 'MSG-1',
    },
  ],
  ticket: {
    category: {
      categoryNo: 'network',
      nameEnUS: 'Network',
      nameZhCN: '网络',
    },
    createdAt: 1,
    currentRound: 2,
    lastActivityAt: 2,
    priority: 'normal',
    status: 'waiting_customer' as const,
    subject: 'Network issue',
    ticketNo: 'TKT-1',
    updatedAt: 2,
    version: 3,
  },
};

describe('customer IDC API boundaries', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('uses the real cursor-based support ticket list envelope', async () => {
    const { listCustomerTickets } = await import('./support');
    requestClient.get.mockResolvedValue({ nextCursor: '', tickets: [] });

    await listCustomerTickets({ cursor: 'next-page', limit: 10 });

    expect(requestClient.get).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/support/tickets',
      { params: { cursor: 'next-page', limit: 10 } },
    );
  });

  it('loads real account categories and creates with reserved initial uploads', async () => {
    const {
      createCustomerTicket,
      listCustomerTicketCategories,
      uploadInitialCustomerTicketAttachments,
    } = await import('./support');
    requestClient.get.mockResolvedValue({ items: [], nextCursor: '' });
    requestClient.post.mockImplementation((url: string) =>
      url.endsWith('/initial-attachment-uploads')
        ? Promise.resolve({
            expiresAt: 100,
            nextOffset: 0,
            uploadNo: 'UP-1',
            version: 1,
          })
        : Promise.resolve({ item: ticketDetail }),
    );
    requestClient.put.mockResolvedValue({
      fileName: 'trace.log',
      mediaType: 'text/plain',
      ready: true,
      sizeBytes: 5,
      uploadNo: 'UP-1',
      version: 2,
    });

    await listCustomerTicketCategories({ limit: 100 });
    const uploadNos = await uploadInitialCustomerTicketAttachments([
      new File(['trace'], 'trace.log', { type: 'text/plain' }),
    ]);
    await createCustomerTicket({
      body: 'Network issue',
      categoryNo: 'network',
      customFields: [
        { fieldNo: 'CF-NUMBER', numberValue: '2' },
        { fieldNo: 'CF-BOOLEAN', booleanValue: true },
      ],
      associationReference: 'sar_0123456789abcdef0123456789abcdef',
      subject: 'Network issue',
      uploadNos,
    });

    expect(requestClient.get).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/support/categories',
      { params: { limit: 100 } },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      '/x/idc-core/api/v1/account/support/initial-attachment-uploads',
      { fileName: 'trace.log', sizeBytes: 5 },
      { silentErrorMessage: true },
    );
    expect(requestClient.put).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/support/initial-attachment-uploads/UP-1/content',
      expect.any(File),
      {
        headers: { 'Content-Type': 'text/plain' },
        silentErrorMessage: true,
        timeout: 120_000,
      },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/account/support/tickets',
      {
        body: 'Network issue',
        categoryNo: 'network',
        customFields: [
          { fieldNo: 'CF-NUMBER', numberValue: '2' },
          { fieldNo: 'CF-BOOLEAN', booleanValue: true },
        ],
        associationReference: 'sar_0123456789abcdef0123456789abcdef',
        subject: 'Network issue',
        uploadNos: ['UP-1'],
      },
      { silentErrorMessage: true },
    );
  });

  it('loads bounded customer fields, eligible entitlements, and public presales fields from their real routes', async () => {
    const {
      listCustomerSupportCustomFields,
      listCustomerSupportEntitlementCandidates,
    } = await import('./support');
    const { listPresalesCustomFields } = await import('./presales');
    requestClient.get
      .mockResolvedValueOnce({ items: [] })
      .mockResolvedValueOnce({ items: [], nextCursor: '' })
      .mockResolvedValueOnce({ items: [] });

    await listCustomerSupportCustomFields();
    await listCustomerSupportEntitlementCandidates({
      cursor: 'next',
      keyword: 'cloud',
      limit: 25,
    });
    await listPresalesCustomFields();

    expect(requestClient.get).toHaveBeenNthCalledWith(
      1,
      '/x/idc-core/api/v1/account/support/custom-fields',
    );
    expect(requestClient.get).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/account/support/entitlement-candidates',
      { params: { cursor: 'next', keyword: 'cloud', limit: 25 } },
    );
    expect(requestClient.get).toHaveBeenNthCalledWith(
      3,
      '/x/idc-core/api/v1/public/support/presales/custom-fields',
    );
  });

  it('keeps frozen customer-visible field and related-entitlement snapshots from ticket details', async () => {
    const { mapCustomerTicketDetail } = await import('./support');
    const mapped = mapCustomerTicketDetail({
      ...ticketDetail,
      customFields: [
        {
          fieldNo: 'CF-SINGLE',
          fieldType: 'single_select',
          nameEnUS: 'Region at creation',
          nameZhCN: '创建时区域',
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
      ticket: {
        ...ticketDetail.ticket,
        relatedEntitlement: {
          associationReference: 'sar_fedcba9876543210fedcba9876543210',
          currentAvailable: false,
          snapshotNameEnUS: 'Original entitlement',
          snapshotNameZhCN: '创建时权益',
          snapshotStatus: 'active',
          snapshotVersion: 3,
        },
      },
    });

    expect(mapped.customFields[0]?.selectedOptions[0]?.nameZhCN).toBe(
      '已停用区域',
    );
    expect(mapped.relatedEntitlement).toMatchObject({
      currentAvailable: false,
      snapshotNameZhCN: '创建时权益',
    });
  });

  it('sends ticket version and confirmation to the real closure route', async () => {
    const { closeCustomerTicket } = await import('./support');
    requestClient.post.mockResolvedValue({ item: ticketDetail });

    const result = await closeCustomerTicket('TKT-1', 3);

    expect(requestClient.post).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/support/tickets/TKT-1/closures',
      { confirmed: true, ticketVersion: 3 },
    );
    expect(result.actions).toMatchObject({ canClose: true, canReply: true });
  });

  it('uses currentRound for the real rating contract and exposes rating only when safe', async () => {
    const { mapCustomerTicketDetail, rateCustomerTicket } =
      await import('./support');
    requestClient.post.mockResolvedValue({ item: ticketDetail });

    expect(mapCustomerTicketDetail(ticketDetail).actions.canRate).toBe(true);
    await rateCustomerTicket('TKT-1', 2, 2, 3, 'Still unstable');

    expect(requestClient.post).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/account/support/tickets/TKT-1/ratings',
      {
        feedback: 'Still unstable',
        roundNo: 2,
        stars: 2,
        ticketVersion: 3,
      },
    );
  });

  it('downloads customer and anonymous attachments through safe scoped routes', async () => {
    const { downloadCustomerTicketAttachment } = await import('./support');
    const { downloadPresalesAttachment } = await import('./presales');
    requestClient.download.mockResolvedValue(new Blob(['file']));

    await downloadCustomerTicketAttachment('ATT-1');
    await downloadPresalesAttachment('short-token', {
      attachmentNo: 'ATT-2',
    });

    expect(requestClient.download).toHaveBeenNthCalledWith(
      1,
      '/x/idc-core/api/v1/account/support/attachments/ATT-1/content',
      { silentErrorMessage: true },
    );
    expect(requestClient.download).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/public/support/presales/attachments/ATT-2/content',
      {
        data: { accessToken: 'short-token' },
        method: 'POST',
        silentErrorMessage: true,
      },
    );
  });

  it('uses the real delivery and preference read routes', async () => {
    const {
      getCustomerNotificationPreferences,
      listCustomerNotificationDeliveries,
    } = await import('./notifications');
    requestClient.get.mockResolvedValue({ items: [], nextCursor: '' });

    await listCustomerNotificationDeliveries({ limit: 10 });
    await getCustomerNotificationPreferences();

    expect(requestClient.get).toHaveBeenNthCalledWith(
      1,
      '/x/idc-notify/api/v1/account/notifications/deliveries',
      { params: { limit: 10 } },
    );
    expect(requestClient.get).toHaveBeenNthCalledWith(
      2,
      '/x/idc-notify/api/v1/account/notifications/preferences',
    );
  });

  it('saves one ordinary preference without inventing marketing evidence', async () => {
    const { updateCustomerNotificationPreference } =
      await import('./notifications');
    const input = {
      channelCode: 'in_app' as const,
      contactNo: 'contact-1',
      enabled: false,
      expectedVersion: 2,
      idempotencyKey: 'customer-choice-1',
      topic: 'support' as const,
    };
    requestClient.put.mockResolvedValue({ version: 3 });

    await updateCustomerNotificationPreference(input);

    expect(requestClient.put).toHaveBeenCalledWith(
      '/x/idc-notify/api/v1/account/notifications/preferences',
      input,
    );
  });

  it('uses authoritative consent evidence when opting in and omits it when opting out', async () => {
    const { updateCustomerNotificationPreference } =
      await import('./notifications');
    requestClient.put.mockResolvedValue({ version: 4 });

    const granted = {
      channelCode: 'email' as const,
      contactNo: 'contact-1',
      enabled: true,
      expectedVersion: 2,
      idempotencyKey: 'marketing-granted-1',
      locale: 'zh-CN' as const,
      marketingConsentAction: 'granted' as const,
      policyReference: 'privacy-notice',
      policyVersion: '2026-08-15',
      topic: 'marketing' as const,
    };
    const withdrawn = {
      channelCode: 'email' as const,
      contactNo: 'contact-1',
      enabled: false,
      expectedVersion: 3,
      idempotencyKey: 'marketing-withdrawn-1',
      marketingConsentAction: 'withdrawn' as const,
      topic: 'marketing' as const,
    };

    await updateCustomerNotificationPreference(granted);
    await updateCustomerNotificationPreference(withdrawn);

    expect(requestClient.put).toHaveBeenNthCalledWith(
      1,
      '/x/idc-notify/api/v1/account/notifications/preferences',
      granted,
    );
    expect(requestClient.put).toHaveBeenNthCalledWith(
      2,
      '/x/idc-notify/api/v1/account/notifications/preferences',
      withdrawn,
    );
    expect(withdrawn).not.toHaveProperty('policyReference');
    expect(withdrawn).not.toHaveProperty('policyVersion');
    expect(withdrawn).not.toHaveProperty('locale');
  });

  it('saves customer quiet hours without rewriting a contact or consent', async () => {
    const { updateCustomerNotificationPreference } =
      await import('./notifications');
    const input = {
      expectedVersion: 4,
      idempotencyKey: 'quiet-hours-1',
      quietHours: {
        enabled: true,
        endMinute: 480,
        startMinute: 1380,
        timezoneName: 'Asia/Shanghai',
      },
    };
    requestClient.put.mockResolvedValue({ version: 5 });

    await updateCustomerNotificationPreference(input);

    expect(requestClient.put).toHaveBeenCalledWith(
      '/x/idc-notify/api/v1/account/notifications/preferences',
      input,
    );
  });

  it('verifies and replies through the real anonymous short-link contract', async () => {
    const { replyPresales, verifyPresalesAccess } = await import('./presales');
    requestClient.post.mockResolvedValue({
      expiresAt: 7_200_000,
      item: ticketDetail,
    });

    const verified = await verifyPresalesAccess('short-token');
    const replied = await replyPresales('short-token', 'More details', 3);

    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      '/x/idc-core/api/v1/public/support/presales/access-verifications',
      { accessToken: 'short-token' },
      { silentErrorMessage: true },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/public/support/presales/replies',
      {
        accessToken: 'short-token',
        body: 'More details',
        ticketVersion: 3,
        uploadNos: [],
      },
      { silentErrorMessage: true },
    );
    expect(verified.accessExpiresAt).toBe(7_200_000);
    expect(verified.actions).toMatchObject({
      canClose: true,
      canRate: true,
      canReopen: false,
      canReply: true,
    });
    expect(replied.accessExpiresAt).toBe(7_200_000);
  });

  it('uses the real anonymous close, reopen, and current-round rating contracts', async () => {
    const { closePresales, ratePresales, reopenPresales } =
      await import('./presales');
    requestClient.post.mockResolvedValue({
      expiresAt: 7_200_000,
      item: ticketDetail,
    });

    await closePresales('short-token', 3);
    await reopenPresales('short-token', 4);
    await ratePresales('short-token', 5, 2, 2, 'Still needs attention');

    const prefix = '/x/idc-core/api/v1/public/support/presales';
    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      `${prefix}/closures`,
      { accessToken: 'short-token', ticketVersion: 3 },
      { silentErrorMessage: true },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      `${prefix}/reopenings`,
      { accessToken: 'short-token', ticketVersion: 4 },
      { silentErrorMessage: true },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      3,
      `${prefix}/ratings`,
      {
        accessToken: 'short-token',
        feedback: 'Still needs attention',
        roundNo: 2,
        stars: 2,
        ticketVersion: 5,
      },
      { silentErrorMessage: true },
    );
  });

  it('reserves and uploads anonymous reply attachments as raw bytes before replying with opaque upload numbers', async () => {
    const { replyPresales, uploadPresalesReplyAttachments } =
      await import('./presales');
    requestClient.post
      .mockResolvedValueOnce({
        expiresAt: 7_200_000,
        nextOffset: 0,
        uploadNo: 'UP-PRESALES-1',
        version: 1,
      })
      .mockResolvedValueOnce({
        expiresAt: 7_200_000,
        item: ticketDetail,
      });
    requestClient.put.mockResolvedValue({
      fileName: 'question.txt',
      mediaType: 'text/plain',
      ready: true,
      sizeBytes: 8,
      uploadNo: 'UP-PRESALES-1',
      version: 2,
    });
    const file = new File(['question'], 'question.txt', { type: 'text/plain' });

    const uploadNos = await uploadPresalesReplyAttachments('short-token', [
      file,
    ]);
    await replyPresales('short-token', 'See the attachment', 3, uploadNos);

    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      '/x/idc-core/api/v1/public/support/presales/attachment-uploads',
      {
        accessToken: 'short-token',
        fileName: 'question.txt',
        sizeBytes: 8,
      },
      { silentErrorMessage: true },
    );
    expect(requestClient.put).toHaveBeenCalledWith(
      '/x/idc-core/api/v1/public/support/presales/attachment-uploads/UP-PRESALES-1/content?accessToken=short-token',
      file,
      {
        headers: { 'Content-Type': 'text/plain' },
        silentErrorMessage: true,
        timeout: 120_000,
      },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      '/x/idc-core/api/v1/public/support/presales/replies',
      {
        accessToken: 'short-token',
        body: 'See the attachment',
        ticketVersion: 3,
        uploadNos: ['UP-PRESALES-1'],
      },
      { silentErrorMessage: true },
    );
  });

  it('uses the frozen anonymous presales verification and creation contracts', async () => {
    const {
      completePresalesHumanChallenge,
      createPresalesTicket,
      renewPresalesAccess,
      requestPresalesHumanChallenge,
      requestPresalesVerificationCode,
    } = await import('./presales');
    const binding = {
      browserNonce: 'browser-nonce',
      destination: 'buyer@example.com',
      destinationKind: 'email' as const,
      locale: 'en-US' as const,
      operation: 'create' as const,
    };
    requestClient.post
      .mockResolvedValueOnce({
        challengeNo: 'CH-1',
        expiresAt: 300_000,
        imageDataUrl: 'data:image/png;base64,AA==',
        kind: 'image',
        providerCode: 'local',
      })
      .mockResolvedValueOnce({ expiresAt: 120_000, proofToken: 'PROOF-1' })
      .mockResolvedValueOnce({
        deliveryState: 'accepted',
        expiresAt: 600_000,
        maskedDestination: 'b***@example.com',
        retryAfterAt: 60_000,
        verificationNo: 'VER-1',
      })
      .mockResolvedValueOnce({
        accessToken: 'ACCESS-1',
        expiresAt: 7_200_000,
        linkDeliveryState: 'queued',
        ticketNo: 'TKT-1',
      })
      .mockResolvedValueOnce({
        accessToken: 'ACCESS-2',
        expiresAt: 14_400_000,
        linkDeliveryState: 'result_unknown',
        ticketNo: 'TKT-1',
      });

    await requestPresalesHumanChallenge({
      ...binding,
      presentationKind: 'image',
    });
    await completePresalesHumanChallenge({
      ...binding,
      answer: '5',
      challengeNo: 'CH-1',
    });
    await requestPresalesVerificationCode({
      ...binding,
      idempotencyKey: 'send-1',
      proofToken: 'PROOF-1',
    });
    await createPresalesTicket({
      body: 'Need a bare metal server',
      browserNonce: binding.browserNonce,
      categoryNo: 'sales',
      code: '123456',
      customFields: [{ fieldNo: 'CF-PROJECT', textValue: 'New region' }],
      destination: binding.destination,
      destinationKind: binding.destinationKind,
      locale: binding.locale,
      operationNo: 'create-1',
      subject: 'Purchase inquiry',
      verificationNo: 'VER-1',
    });
    await renewPresalesAccess({
      browserNonce: binding.browserNonce,
      code: '654321',
      destination: binding.destination,
      destinationKind: binding.destinationKind,
      locale: binding.locale,
      operationNo: 'renew-1',
      ticketNo: 'TKT-1',
      verificationNo: 'VER-2',
    });

    const prefix = '/x/idc-notify/api/v1/public/support/presales';
    expect(requestClient.post).toHaveBeenNthCalledWith(
      1,
      `${prefix}/human-challenges`,
      { ...binding, presentationKind: 'image' },
      { silentErrorMessage: true },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      2,
      `${prefix}/human-challenge-completions`,
      { ...binding, answer: '5', challengeNo: 'CH-1' },
      { silentErrorMessage: true },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      3,
      `${prefix}/verification-codes`,
      { ...binding, idempotencyKey: 'send-1', proofToken: 'PROOF-1' },
      { silentErrorMessage: true },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      4,
      `${prefix}/tickets`,
      expect.objectContaining({
        browserNonce: 'browser-nonce',
        categoryNo: 'sales',
        customFields: [{ fieldNo: 'CF-PROJECT', textValue: 'New region' }],
        operationNo: 'create-1',
        verificationNo: 'VER-1',
      }),
      { silentErrorMessage: true },
    );
    expect(requestClient.post).toHaveBeenNthCalledWith(
      5,
      `${prefix}/access-renewals`,
      expect.objectContaining({
        browserNonce: 'browser-nonce',
        operationNo: 'renew-1',
        ticketNo: 'TKT-1',
        verificationNo: 'VER-2',
      }),
      { silentErrorMessage: true },
    );
  });
});

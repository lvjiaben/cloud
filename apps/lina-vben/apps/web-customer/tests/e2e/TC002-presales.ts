import { expect, test } from '@playwright/test';

import { PresalesPage } from '../pages/PresalesPage';
import { mockCustomerAPIs } from '../support/customer-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

test.describe('TC002 匿名售前咨询', () => {
  test.beforeEach(async ({ page }) => {
    await mockCustomerAPIs(page);
  });

  test('TC002a: 图片人机验证后创建咨询并立即进入两小时短链详情', async ({
    page,
  }) => {
    const presales = new PresalesPage(page);
    const requests: Array<Record<string, unknown>> = [];
    let challengeRequests = 0;
    page.on('request', (request) => {
      if (
        request.method() === 'POST' &&
        request.url().includes('/public/support/presales/') &&
        request.postData()
      ) {
        requests.push(request.postDataJSON() as Record<string, unknown>);
        if (request.url().endsWith('/human-challenges')) {
          challengeRequests += 1;
        }
      }
    });

    await presales.openCreate();
    await expect(page).toHaveURL(/\/auth\/presales$/);
    await expect(page.getByText(/访问链接最长有效 2 小时/)).toBeVisible();
    await presales.fillInquiry('buyer@example.com');
    await expect(page.getByTestId('ticket-entitlement')).toHaveCount(0);
    await presales.requestChallenge();
    await expect(
      page.getByText('请检查补充信息中的必填项和格式'),
    ).toBeVisible();
    expect(challengeRequests).toBe(0);
    await presales.fillSevenCustomFieldTypes();
    await presales.requestChallenge();
    expect(challengeRequests).toBe(1);
    await expect(presales.challengeImage).toBeVisible();
    await presales.challengeImage.scrollIntoViewIfNeeded();
    await captureCustomerScreenshot(page, 'presales-image-challenge');

    await presales.answerChallenge();
    await expect(presales.codePanel).toContainText('b***@example.com');
    await expect(presales.retryStatus).toContainText('页面不会自动申请');
    await expect(page.getByTestId('presales-resend-code')).toBeDisabled();
    await presales.submitCode();

    await expect(page).toHaveURL(
      /\/auth\/presales\/access\/created-token\?ticket=PRE-2026-0001$/,
    );
    await expect(presales.detail).toContainText('裸金属服务器交付咨询');
    await expect(presales.detail).not.toContainText('内部部门');
    await presales.waitForIdle();
    await captureCustomerScreenshot(page, 'presales-created-detail');

    const boundRequests = requests.filter((item) => item.browserNonce);
    expect(boundRequests.length).toBeGreaterThanOrEqual(4);
    expect(new Set(boundRequests.map((item) => item.browserNonce)).size).toBe(
      1,
    );
    const browserNonce = boundRequests[0]?.browserNonce || '';
    const persisted = await page.evaluate(() => [
      ...Object.values(localStorage),
      ...Object.values(sessionStorage),
    ]);
    expect(persisted.join('\n')).not.toContain(String(browserNonce));
    const ticketRequest = requests.find((item) =>
      Array.isArray(item.customFields),
    );
    expect(ticketRequest).toMatchObject({
      customFields: [
        { fieldNo: 'CF-TEXT', textValue: '新建项目' },
        {
          fieldNo: 'CF-MULTI-LINE',
          textValue: '需要确认交付区域和存储配置。',
        },
        { fieldNo: 'CF-NUMBER', numberValue: '3' },
        { fieldNo: 'CF-SINGLE', optionNos: ['OPT-EAST'] },
        {
          fieldNo: 'CF-MULTI-SELECT',
          optionNos: ['OPT-NETWORK', 'OPT-STORAGE'],
        },
        { dateValue: '2026-08-15', fieldNo: 'CF-DATE' },
        { booleanValue: true, fieldNo: 'CF-BOOLEAN' },
      ],
    });
    expect(ticketRequest).not.toHaveProperty('entitlementNo');
  });

  test('TC002b: 可主动切换无障碍算术题，Turnstile 未接入时明确失败关闭', async ({
    page,
  }) => {
    const presales = new PresalesPage(page);
    await presales.openCreate();
    await presales.fillInquiry('accessible@example.com');
    await presales.fillSevenCustomFieldTypes();
    await presales.requestChallenge();
    await presales.selectAccessibleChallenge();
    await expect(presales.accessiblePrompt).toHaveText('2 + 3 = ?');
    await presales.accessiblePrompt.scrollIntoViewIfNeeded();
    await captureCustomerScreenshot(page, 'presales-accessible-challenge');

    await page.reload();
    await presales.fillInquiry('turnstile@example.com');
    await presales.fillSevenCustomFieldTypes();
    await presales.requestChallenge();
    await expect(presales.challengeUnavailable).toContainText(
      '人机验证暂时不可用',
    );
    await expect(page.getByTestId('presales-send-code')).toHaveCount(0);
  });

  test('TC002c: 验证码请求结果未知时不自动重发', async ({ page }) => {
    let verificationRequests = 0;
    await page.route('**/verification-codes', async (route) => {
      verificationRequests += 1;
      await route.fulfill({
        body: JSON.stringify({
          code: 504,
          data: null,
          message: 'result unknown',
        }),
        contentType: 'application/json',
        status: 504,
      });
    });
    const presales = new PresalesPage(page);
    await presales.openCreate();
    await presales.fillInquiry('unknown-code@example.com');
    await presales.fillSevenCustomFieldTypes();
    await presales.requestChallenge();
    await presales.answerChallenge();
    await expect(page.getByTestId('presales-code-uncertain')).toContainText(
      '发送结果暂时无法确认',
    );
    await page.waitForTimeout(1500);
    expect(verificationRequests).toBe(1);
    await expect(page.getByTestId('presales-submit')).toHaveCount(0);
  });

  test('TC002d: 链接发送结果未知仍保留当前浏览器访问且不自动补发', async ({
    page,
  }) => {
    let ticketRequests = 0;
    page.on('request', (request) => {
      if (request.url().endsWith('/public/support/presales/tickets')) {
        ticketRequests += 1;
      }
    });
    const presales = new PresalesPage(page);
    await presales.openCreate();
    await presales.fillInquiry('unknown-link@example.com');
    await presales.fillSevenCustomFieldTypes();
    await presales.requestChallenge();
    await presales.answerChallenge();
    await presales.submitCode();
    await expect(presales.detail).toBeVisible();
    await expect(presales.linkDeliveryUncertain).toContainText(
      '系统不会自动重发',
    );
    await page.waitForTimeout(1500);
    expect(ticketRequests).toBe(1);
  });

  test('TC002e: 有效短链显示服务端到期时间并安全下载附件', async ({ page }) => {
    const reservationBodies: Array<Record<string, unknown>> = [];
    const replyBodies: Array<Record<string, unknown>> = [];
    const rawUploads: string[] = [];
    page.on('request', (request) => {
      if (
        request.method() === 'POST' &&
        request.url().endsWith('/presales/attachment-uploads')
      ) {
        reservationBodies.push(
          request.postDataJSON() as Record<string, unknown>,
        );
      }
      if (
        request.method() === 'PUT' &&
        request.url().includes('/presales/attachment-uploads/')
      ) {
        rawUploads.push(request.postDataBuffer()?.toString() || '');
      }
      if (
        request.method() === 'POST' &&
        request.url().endsWith('/presales/replies')
      ) {
        replyBodies.push(request.postDataJSON() as Record<string, unknown>);
      }
    });
    const presales = new PresalesPage(page);
    await presales.openAccess('active-token', 'PRE-2026-0001');
    await expect(presales.detail).toContainText('裸金属服务器交付咨询');
    await expect(
      page.getByTestId('presales-custom-field-snapshots'),
    ).toContainText('创建时业务区域');
    await expect(
      page.getByTestId('presales-custom-field-snapshots'),
    ).toContainText('已停用区域');
    await expect(page.getByTestId('ticket-related-entitlement')).toHaveCount(0);
    await expect(page.getByTestId('presales-access-expires-at')).toHaveText(
      '本链接可使用至 2026-08-10 10:00',
    );

    const download = await presales.downloadAttachment('server-spec.txt');
    await expect(download.suggestedFilename()).toBe('server-spec.txt');
    await presales.selectReplyAttachment({
      buffer: Buffer.from('presales attachment'),
      mimeType: 'text/plain',
      name: 'presales-note.txt',
    });
    await presales.reply('请说明是否支持自定义硬盘。');
    await expect(page.getByTestId('presales-timeline')).toContainText(
      '请同时说明是否支持自定义硬盘。',
    );
    await expect(page.getByTestId('presales-timeline')).toContainText(
      'presales-note.txt',
    );
    await expect(presales.replyAttachments).not.toContainText(
      'presales-note.txt',
    );
    expect(reservationBodies).toEqual([
      {
        accessToken: 'active-token',
        fileName: 'presales-note.txt',
        sizeBytes: 19,
      },
    ]);
    expect(rawUploads).toEqual(['presales attachment']);
    expect(replyBodies).toEqual([
      {
        accessToken: 'active-token',
        body: '请说明是否支持自定义硬盘。',
        ticketVersion: 3,
        uploadNos: ['UP-PRESALES-1'],
      },
    ]);
    await captureCustomerScreenshot(page, 'presales-reply-with-attachment');
  });

  test('TC002f: 过期链接只在网址带安全咨询编号时允许重新验证', async ({
    page,
  }) => {
    const presales = new PresalesPage(page);
    await presales.openAccess('expired-token');
    await expect(presales.accessUnavailable).toBeVisible();
    await expect(presales.renewalNeedsSafeTicket).toContainText(
      '页面不会从失败信息中猜测编号',
    );
    await expect(presales.renewal).toHaveCount(0);

    await presales.openAccess('expired-token', 'PRE-2026-0001');
    await expect(presales.renewal).toBeVisible();
    await presales.renewWithAccessibleChallenge('buyer@example.com');
    await expect(page).toHaveURL(
      /\/auth\/presales\/access\/renewed-token\?ticket=PRE-2026-0001$/,
    );
    await expect(presales.detail).toContainText('裸金属服务器交付咨询');
    await presales.waitForIdle();
    await captureCustomerScreenshot(page, 'presales-access-renewed');
  });

  test('TC002g: 附件上传失败时保留回复和文件，重试后才发送', async ({
    page,
  }) => {
    let rejectFirstReservation = true;
    let replyRequests = 0;
    await page.route('**/presales/attachment-uploads', async (route) => {
      if (!rejectFirstReservation) {
        await route.fallback();
        return;
      }
      rejectFirstReservation = false;
      await route.fulfill({
        body: JSON.stringify({
          code: 400,
          data: null,
          message: 'attachment rejected',
        }),
        contentType: 'application/json',
        status: 422,
      });
    });
    page.on('request', (request) => {
      if (request.url().endsWith('/presales/replies')) replyRequests += 1;
    });

    const presales = new PresalesPage(page);
    await presales.openAccess('active-token', 'PRE-2026-0001');
    await presales.selectReplyAttachment({
      buffer: Buffer.from('presales attachment'),
      mimeType: 'text/plain',
      name: 'presales-note.txt',
    });
    await presales.reply('这段回复和文件需要保留。');

    await expect(
      page.getByText(/发送失败，回复内容和已选文件仍然保留/),
    ).toBeVisible();
    await expect(presales.replyInput).toHaveValue('这段回复和文件需要保留。');
    await expect(presales.replyAttachments).toContainText('presales-note.txt');
    expect(replyRequests).toBe(0);

    await presales.submitReply();
    await expect(page.getByTestId('presales-timeline')).toContainText(
      '请同时说明是否支持自定义硬盘。',
    );
    expect(replyRequests).toBe(1);
  });

  test('TC002h: 上传时短链过期统一回到不可访问提示且不发送回复', async ({
    page,
  }) => {
    let replyRequests = 0;
    await page.route('**/presales/attachment-uploads', async (route) => {
      await route.fulfill({
        body: JSON.stringify({
          code: 403,
          data: null,
          errorCode: 'IDC_SUPPORT_TICKET_NOT_ACCESSIBLE',
          message: 'expired',
        }),
        contentType: 'application/json',
        status: 403,
      });
    });
    page.on('request', (request) => {
      if (request.url().endsWith('/presales/replies')) replyRequests += 1;
    });

    const presales = new PresalesPage(page);
    await presales.openAccess('active-token', 'PRE-2026-0001');
    await presales.selectReplyAttachment({
      buffer: Buffer.from('presales attachment'),
      mimeType: 'text/plain',
      name: 'presales-note.txt',
    });
    await presales.reply('上传时链接已经过期。');

    await expect(presales.accessUnavailable).toContainText(
      '这个访问链接无法继续使用',
    );
    await expect(presales.accessUnavailable).toContainText(
      '页面不会显示任何咨询内容',
    );
    expect(replyRequests).toBe(0);
  });

  test('TC002i: 匿名访客按服务端动作评价、关闭并在新一轮重新打开', async ({
    page,
  }) => {
    const presales = new PresalesPage(page);
    await presales.openAccess('valid-token', 'PRE-2026-0001');
    await presales.waitForIdle();

    await expect(page.getByTestId('presales-rate')).toBeVisible();
    await expect(page.getByTestId('presales-close')).toBeVisible();

    const ratingRequest = page.waitForRequest((request) =>
      request.url().endsWith('/public/support/presales/ratings'),
    );
    await presales.rate(2, '仍需继续协助');
    expect((await ratingRequest).postDataJSON()).toEqual({
      accessToken: 'valid-token',
      feedback: '仍需继续协助',
      roundNo: 2,
      stars: 2,
      ticketVersion: 3,
    });
    await expect(page.getByTestId('presales-rating-summary')).toContainText(
      '已提交 2 星评价',
    );
    await expect(page.getByTestId('presales-rate')).toHaveCount(0);

    const closeRequest = page.waitForRequest((request) =>
      request.url().endsWith('/public/support/presales/closures'),
    );
    await presales.close();
    expect((await closeRequest).postDataJSON()).toEqual({
      accessToken: 'valid-token',
      ticketVersion: 4,
    });
    await expect(page.getByTestId('presales-reopen')).toBeVisible();
    await expect(page.getByTestId('presales-reply')).toHaveCount(0);

    const reopenRequest = page.waitForRequest((request) =>
      request.url().endsWith('/public/support/presales/reopenings'),
    );
    await presales.reopen();
    expect((await reopenRequest).postDataJSON()).toEqual({
      accessToken: 'valid-token',
      ticketVersion: 5,
    });
    await expect(page.getByTestId('presales-close')).toBeVisible();
    await expect(page.getByTestId('presales-rate')).toBeVisible();
    await expect(page.getByTestId('presales-rating-summary')).toHaveCount(0);
  });
});

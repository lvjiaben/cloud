import { Buffer } from 'node:buffer';

import { expect, test } from '@playwright/test';

import { CustomerNotificationsPage } from '../pages/CustomerNotificationsPage';
import { CustomerSupportPage } from '../pages/CustomerSupportPage';
import { mockCustomerAPIs } from '../support/customer-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

test.describe('TC001 客户工单与通知', () => {
  test.beforeEach(async ({ page }) => {
    await mockCustomerAPIs(page);
    await page.goto('/auth/login');
    await page.getByPlaceholder('请输入用户名').fill('customer');
    await page.getByPlaceholder('请输入密码').fill('customer-password');
    await page.getByRole('button', { name: 'login' }).click();
    await page.waitForURL(/\/(?:account|profile)(?:$|\?)/);
  });

  test('TC001i: 从工单列表用大抽屉提交新工单，不跳到独立新建页', async ({
    page,
  }) => {
    await page.goto('/account/support');
    await page.getByRole('button', { name: '新建工单' }).click();
    await expect(
      page.getByRole('dialog', { name: '提交新工单' }),
    ).toBeVisible();
    await expect(page.getByTestId('ticket-custom-field-group')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/account/support');
    await captureCustomerScreenshot(page, 'customer-ticket-create-drawer');
  });

  test('TC001a: 客户按服务端轮次评价并安全下载自己工单的附件', async ({
    page,
  }) => {
    await page.goto('/account/support');
    const list = page.getByTestId('ticket-list');
    await expect(list).toContainText('服务器网络异常');
    await expect(list).toContainText('网络问题');
    await expect(list).not.toContainText('等待上游处理');
    await captureCustomerScreenshot(page, 'customer-ticket-list');

    await list.getByRole('button').click();
    await page.waitForURL('**/account/support/TKT-2026-0001');
    await page.locator('.ant-spin-spinning').waitFor({ state: 'detached' });
    await expect(page.getByTestId('ticket-timeline')).toContainText(
      '服务器无法连接',
    );
    await expect(page.getByText('网络问题', { exact: true })).toBeVisible();

    const accountDownload = page.waitForEvent('download');
    await page
      .getByRole('button', { name: '下载附件 network-trace.log' })
      .click();
    const downloadedAttachment = await accountDownload;
    await expect(downloadedAttachment.suggestedFilename()).toBe(
      'network-trace.log',
    );

    await page.getByTestId('ticket-rate').click();
    await page
      .getByTestId('ticket-rating-stars')
      .locator('.ant-rate-star')
      .nth(1)
      .click();
    await expect(
      page.getByText('如需继续处理，请重新打开工单。'),
    ).toBeVisible();
    await page.getByTestId('ticket-rating-feedback').fill('仍需观察');
    await captureCustomerScreenshot(page, 'customer-ticket-rating-modal');
    const ratingRequest = page.waitForRequest(
      (request) =>
        request.method() === 'POST' &&
        request.url().endsWith('/tickets/TKT-2026-0001/ratings'),
    );
    await page.getByRole('button', { name: '提交评价' }).click();
    const savedRatingRequest = await ratingRequest;
    const ratingInput = savedRatingRequest.postDataJSON();
    expect(ratingInput).toMatchObject({
      feedback: '仍需观察',
      roundNo: 2,
      stars: 2,
      ticketVersion: 3,
    });
    await expect(page.getByText('已提交 2 星评价')).toBeVisible();
    await expect(page.getByTestId('ticket-rate')).toHaveCount(0);
    await expect(page.getByTestId('ticket-rating-stars')).toHaveCount(0);
    await captureCustomerScreenshot(page, 'customer-ticket-rated');
  });

  test('TC001b: 普通通知、营销同意和免打扰按权威版本保存，安全通知保持锁定', async ({
    page,
  }) => {
    const notifications = new CustomerNotificationsPage(page);
    await notifications.open();
    await expect(page.getByTestId('notification-list')).toContainText(
      '工单消息',
    );
    await expect(page.getByTestId('notification-list')).toContainText('站内信');
    await expect(page.getByTestId('notification-list')).toContainText('已送达');
    await captureCustomerScreenshot(page, 'customer-notification-deliveries');
    await notifications.openPreferences();
    await expect(page.getByText('通知接收设置')).toBeVisible();
    await expect(notifications.preferences).toContainText('工单消息');
    await expect(notifications.securityPreference).toContainText(
      '这类通知不能关闭',
    );
    await expect(
      notifications.securityPreference.getByRole('switch'),
    ).toBeChecked();
    await expect(
      notifications.securityPreference.getByRole('switch'),
    ).toBeDisabled();
    await expect(
      notifications.marketingPreference.getByRole('switch'),
    ).toBeEnabled();
    await expect(
      notifications.marketingPreference.getByRole('switch'),
    ).toBeChecked();
    await expect(notifications.quietHoursStart).toHaveValue('22:00');
    await expect(notifications.quietHoursEnd).toHaveValue('07:00');

    const saveRequest = page.waitForRequest(
      (request) =>
        request.method() === 'PUT' &&
        request.url().endsWith('/account/notifications/preferences'),
    );
    await notifications.toggleSupport();
    const savedPreferenceRequest = await saveRequest;
    const input = savedPreferenceRequest.postDataJSON();
    expect(input).toMatchObject({
      channelCode: 'in_app',
      contactNo: 'contact-1',
      enabled: false,
      expectedVersion: 2,
      topic: 'support',
    });
    expect(input.idempotencyKey).toMatch(/^pref-[0-9a-f-]{36}$/u);
    expect(input).not.toHaveProperty('locale');
    expect(input).not.toHaveProperty('marketingConsentAction');
    await expect(notifications.supportPreference).toContainText('已保存');
    await expect(
      notifications.supportPreference.getByRole('switch'),
    ).not.toBeChecked();

    const unsubscribeRequest = page.waitForRequest(
      (request) =>
        request.method() === 'PUT' &&
        request.url().endsWith('/account/notifications/preferences'),
    );
    await notifications.toggleMarketing();
    const savedUnsubscribeRequest = await unsubscribeRequest;
    const unsubscribeInput = savedUnsubscribeRequest.postDataJSON();
    expect(unsubscribeInput).toMatchObject({
      channelCode: 'email',
      contactNo: 'contact-1',
      enabled: false,
      expectedVersion: 3,
      marketingConsentAction: 'withdrawn',
      topic: 'marketing',
    });
    expect(unsubscribeInput).not.toHaveProperty('locale');
    expect(unsubscribeInput).not.toHaveProperty('policyReference');
    expect(unsubscribeInput).not.toHaveProperty('policyVersion');
    await expect(
      notifications.marketingPreference.getByRole('switch'),
    ).not.toBeChecked();

    const optInRequest = page.waitForRequest(
      (request) =>
        request.method() === 'PUT' &&
        request.url().endsWith('/account/notifications/preferences'),
    );
    await notifications.toggleMarketing();
    const savedOptInRequest = await optInRequest;
    expect(savedOptInRequest.postDataJSON()).toMatchObject({
      channelCode: 'email',
      contactNo: 'contact-1',
      enabled: true,
      expectedVersion: 4,
      locale: 'zh-CN',
      marketingConsentAction: 'granted',
      policyReference: 'privacy-notice',
      policyVersion: '2026-08-15',
      topic: 'marketing',
    });

    const quietHoursRequest = page.waitForRequest(
      (request) =>
        request.method() === 'PUT' &&
        request.url().endsWith('/account/notifications/preferences'),
    );
    await notifications.saveQuietHours('23:00', '08:00');
    const savedQuietHoursRequest = await quietHoursRequest;
    const quietHoursInput = savedQuietHoursRequest.postDataJSON();
    expect(quietHoursInput).toMatchObject({
      expectedVersion: 5,
      quietHours: {
        enabled: true,
        endMinute: 480,
        startMinute: 1380,
        timezoneName: 'Asia/Shanghai',
      },
    });
    expect(Object.keys(quietHoursInput).sort()).toEqual([
      'expectedVersion',
      'idempotencyKey',
      'quietHours',
    ]);
    await expect(notifications.quietHours).toContainText('已保存');
    await captureCustomerScreenshot(page, 'customer-notification-preferences');
  });

  test('TC001d: 版本冲突保留客户选择，读取最新版本后才允许再次保存', async ({
    page,
  }) => {
    let preferenceReads = 0;
    let preferenceWrites = 0;
    let firstOperationKey = '';
    await page.route(
      '**/x/idc-notify/api/v1/account/notifications/preferences',
      async (route) => {
        const request = route.request();
        if (request.method() === 'GET') {
          preferenceReads += 1;
          const version = preferenceReads === 1 ? 2 : 3;
          await route.fulfill({
            body: JSON.stringify({
              code: 0,
              data: {
                customerNo: 'customer-1',
                marketingConsents: [],
                preferences: [
                  {
                    channelCode: 'in_app',
                    contactNo: 'contact-1',
                    enabled: true,
                    topic: 'support',
                    version,
                  },
                ],
                preferredLocale: 'zh-CN',
                version,
              },
              message: 'OK',
            }),
            contentType: 'application/json',
            status: 200,
          });
          return;
        }

        preferenceWrites += 1;
        const input = request.postDataJSON() as {
          enabled: boolean;
          expectedVersion: number;
          idempotencyKey: string;
        };
        if (preferenceWrites === 1) {
          firstOperationKey = input.idempotencyKey;
          await route.fulfill({
            body: JSON.stringify({
              code: 409,
              data: null,
              message: 'settings changed',
            }),
            contentType: 'application/json',
            status: 409,
          });
          return;
        }
        expect(input).toMatchObject({ enabled: false, expectedVersion: 3 });
        expect(input.idempotencyKey).not.toBe(firstOperationKey);
        await route.fulfill({
          body: JSON.stringify({
            code: 0,
            data: {
              appliedAt: 1_786_320_900_000,
              evidenceDigest:
                '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
              reused: false,
              version: 4,
            },
            message: 'OK',
          }),
          contentType: 'application/json',
          status: 200,
        });
      },
    );

    const notifications = new CustomerNotificationsPage(page);
    await notifications.open();
    await notifications.openPreferences();
    await notifications.toggleSupport();
    await expect(
      notifications.supportPreference.getByRole('switch'),
    ).not.toBeChecked();
    await expect(notifications.supportPreference).toContainText(
      '你刚才的选择已保留',
    );

    const readsBeforeRefresh = preferenceReads;
    await notifications.loadLatestSettings();
    expect(preferenceReads).toBe(readsBeforeRefresh + 1);
    await expect(
      notifications.supportPreference.getByRole('switch'),
    ).not.toBeChecked();
    await expect(notifications.supportPreference).toContainText(
      '已读取最新设置，并保留了你的选择',
    );
    await notifications.saveAgain();
    await expect(notifications.supportPreference).toContainText('已保存');
    expect(preferenceReads).toBeGreaterThanOrEqual(2);
    expect(preferenceWrites).toBe(2);
  });

  test('TC001e: 刷新失败时保留旧快照但禁用修改', async ({ page }) => {
    let preferenceReads = 0;
    await page.route(
      '**/x/idc-notify/api/v1/account/notifications/preferences',
      async (route) => {
        preferenceReads += 1;
        if (preferenceReads > 1) {
          await route.fulfill({
            body: JSON.stringify({
              code: 503,
              data: null,
              message: 'unavailable',
            }),
            contentType: 'application/json',
            status: 503,
          });
          return;
        }
        await route.fallback();
      },
    );

    const notifications = new CustomerNotificationsPage(page);
    await notifications.open();
    await notifications.openPreferences();
    await notifications.loadLatestSettings();
    await expect(
      page.getByText('暂时无法读取通知偏好，请稍后重试'),
    ).toBeVisible();
    await expect(
      notifications.supportPreference.getByRole('switch'),
    ).toBeDisabled();
    await expect(
      notifications.supportPreference.getByRole('switch'),
    ).toBeChecked();
  });

  test('TC001f: 缺少当前营销政策证据只阻止营销同意，免打扰仍独立保存', async ({
    page,
  }) => {
    await page.route(
      '**/x/idc-notify/api/v1/account/notifications/preferences',
      async (route) => {
        if (route.request().method() === 'PUT') {
          await route.fulfill({
            body: JSON.stringify({
              code: 0,
              data: {
                appliedAt: 1_786_320_900_000,
                evidenceDigest:
                  '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
                reused: false,
                version: 3,
              },
              message: 'OK',
            }),
            contentType: 'application/json',
            status: 200,
          });
          return;
        }
        await route.fulfill({
          body: JSON.stringify({
            code: 0,
            data: {
              customerNo: 'customer-1',
              marketingConsents: [
                {
                  channelCode: 'email',
                  contactNo: 'contact-1',
                  locale: 'zh-CN',
                  policyReference: '',
                  policyVersion: '',
                  status: 'opted_in',
                  version: 2,
                },
              ],
              preferences: [
                {
                  channelCode: 'email',
                  contactNo: 'contact-1',
                  enabled: true,
                  topic: 'marketing',
                  version: 2,
                },
              ],
              preferredLocale: 'zh-CN',
              quietHours: {
                crossesMidnight: true,
                enabled: true,
                endMinute: 420,
                startMinute: 1320,
                timezoneName: 'Asia/Shanghai',
                version: 2,
              },
              version: 2,
            },
            message: 'OK',
          }),
          contentType: 'application/json',
          status: 200,
        });
      },
    );

    const notifications = new CustomerNotificationsPage(page);
    await notifications.open();
    await notifications.openPreferences();
    await expect(
      notifications.marketingPreference.getByRole('switch'),
    ).toBeDisabled();
    await expect(notifications.marketingPreference).toContainText(
      '当前授权信息不完整',
    );
    const quietHoursRequest = page.waitForRequest(
      (request) =>
        request.method() === 'PUT' &&
        request.url().endsWith('/account/notifications/preferences'),
    );
    await notifications.saveQuietHours('23:30', '08:30');
    const input = (await quietHoursRequest).postDataJSON();
    expect(Object.keys(input).sort()).toEqual([
      'expectedVersion',
      'idempotencyKey',
      'quietHours',
    ]);
    await expect(notifications.quietHours).toContainText('已保存');
    await captureCustomerScreenshot(
      page,
      'customer-notification-quiet-hours-missing-consent',
    );
  });

  test('TC001g: 首次没有免打扰记录时显示空白表单并只保存客户级设置', async ({
    page,
  }) => {
    await page.route(
      '**/x/idc-notify/api/v1/account/notifications/preferences',
      async (route) => {
        if (route.request().method() === 'PUT') {
          await route.fulfill({
            body: JSON.stringify({
              code: 0,
              data: {
                appliedAt: 1_786_320_900_000,
                evidenceDigest:
                  '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
                reused: false,
                version: 2,
              },
              message: 'OK',
            }),
            contentType: 'application/json',
            status: 200,
          });
          return;
        }
        await route.fulfill({
          body: JSON.stringify({
            code: 0,
            data: {
              customerNo: 'customer-1',
              marketingConsents: [],
              preferences: [],
              preferredLocale: 'zh-CN',
              version: 1,
            },
            message: 'OK',
          }),
          contentType: 'application/json',
          status: 200,
        });
      },
    );

    const notifications = new CustomerNotificationsPage(page);
    await notifications.open();
    await notifications.openPreferences();
    await expect(
      page.getByText('当前没有可用的免打扰设置，本页没有替你新建默认值。'),
    ).toBeVisible();
    await expect(notifications.quietHoursTimezone).toHaveValue('');
    await expect(notifications.quietHoursStart).toHaveValue('');
    await expect(notifications.quietHoursEnd).toHaveValue('');
    await expect(notifications.quietHoursSave).toBeDisabled();
    await notifications.quietHours.scrollIntoViewIfNeeded();
    await captureCustomerScreenshot(
      page,
      'customer-notification-quiet-hours-first-empty',
    );

    const quietHoursRequest = page.waitForRequest(
      (request) =>
        request.method() === 'PUT' &&
        request.url().endsWith('/account/notifications/preferences'),
    );
    await notifications.saveNewQuietHours('Asia/Shanghai', '21:30', '07:30');
    const input = (await quietHoursRequest).postDataJSON();
    expect(input).toMatchObject({
      expectedVersion: 1,
      quietHours: {
        enabled: true,
        endMinute: 450,
        startMinute: 1290,
        timezoneName: 'Asia/Shanghai',
      },
    });
    expect(Object.keys(input).sort()).toEqual([
      'expectedVersion',
      'idempotencyKey',
      'quietHours',
    ]);
    await expect(notifications.quietHours).toContainText('已保存');
    await captureCustomerScreenshot(
      page,
      'customer-notification-quiet-hours-first-saved',
    );
  });

  test('TC001h: 多条营销同意不会阻止客户级免打扰保存', async ({ page }) => {
    await page.route(
      '**/x/idc-notify/api/v1/account/notifications/preferences',
      async (route) => {
        if (route.request().method() === 'PUT') {
          await route.fulfill({
            body: JSON.stringify({
              code: 0,
              data: {
                appliedAt: 1_786_320_900_000,
                evidenceDigest:
                  '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
                reused: false,
                version: 5,
              },
              message: 'OK',
            }),
            contentType: 'application/json',
            status: 200,
          });
          return;
        }
        await route.fulfill({
          body: JSON.stringify({
            code: 0,
            data: {
              customerNo: 'customer-1',
              marketingConsents: [
                {
                  channelCode: 'email',
                  contactNo: 'contact-1',
                  locale: 'zh-CN',
                  policyReference: 'privacy-notice',
                  policyVersion: '2026-08-15',
                  status: 'opted_in',
                  version: 4,
                },
                {
                  channelCode: 'sms',
                  contactNo: 'contact-2',
                  locale: 'zh-CN',
                  policyReference: 'privacy-notice',
                  policyVersion: '2026-08-15',
                  status: 'opted_in',
                  version: 4,
                },
              ],
              preferences: [],
              preferredLocale: 'zh-CN',
              quietHours: {
                crossesMidnight: true,
                enabled: true,
                endMinute: 420,
                startMinute: 1320,
                timezoneName: 'Asia/Shanghai',
                version: 4,
              },
              version: 4,
            },
            message: 'OK',
          }),
          contentType: 'application/json',
          status: 200,
        });
      },
    );

    const notifications = new CustomerNotificationsPage(page);
    await notifications.open();
    await notifications.openPreferences();
    await expect(notifications.quietHoursStart).toBeEnabled();
    await expect(notifications.quietHoursEnd).toBeEnabled();

    const quietHoursRequest = page.waitForRequest(
      (request) =>
        request.method() === 'PUT' &&
        request.url().endsWith('/account/notifications/preferences'),
    );
    await notifications.saveQuietHours('22:30', '07:30');
    const input = (await quietHoursRequest).postDataJSON();
    expect(input).toMatchObject({ expectedVersion: 4 });
    expect(Object.keys(input).sort()).toEqual([
      'expectedVersion',
      'idempotencyKey',
      'quietHours',
    ]);
    await expect(notifications.quietHours).toContainText('已保存');
    await captureCustomerScreenshot(
      page,
      'customer-notification-quiet-hours-multiple-consents',
    );
  });

  test('TC001c: 客户填写七类补充信息、可选权益和初始附件后创建工单', async ({
    page,
  }) => {
    const support = new CustomerSupportPage(page);
    await support.openCreate();
    await support.fillBaseTicket();
    await support.chooseEntitlement('生产云服务器');
    await support.fillSevenCustomFieldTypes();
    await expect(
      support.entitlement.locator('.ant-select-selection-item'),
    ).toHaveText('生产云服务器 · 有效');
    await page
      .getByTestId('ticket-initial-attachments')
      .locator('input[type="file"]')
      .setInputFiles({
        buffer: Buffer.from('initial attachment'),
        mimeType: 'text/plain',
        name: 'initial-note.txt',
      });
    await expect(page.getByText('initial-note.txt')).toBeVisible();

    const reserveRequest = page.waitForRequest(
      (request) =>
        request.method() === 'POST' &&
        request.url().endsWith('/support/initial-attachment-uploads'),
    );
    const uploadRequest = page.waitForRequest(
      (request) =>
        request.method() === 'PUT' &&
        request
          .url()
          .endsWith('/support/initial-attachment-uploads/UP-INITIAL-1/content'),
    );
    const createRequest = page.waitForRequest(
      (request) =>
        request.method() === 'POST' &&
        request.url().endsWith('/account/support/tickets'),
    );
    await support.submit();
    await reserveRequest;
    await uploadRequest;
    const savedCreateRequest = await createRequest;
    expect(savedCreateRequest.postDataJSON()).toMatchObject({
      customFields: [
        { fieldNo: 'CF-TEXT', textValue: '生产环境' },
        {
          fieldNo: 'CF-MULTI-LINE',
          textValue: '恢复网络并确认连接稳定。',
        },
        { fieldNo: 'CF-NUMBER', numberValue: '2' },
        { fieldNo: 'CF-SINGLE', optionNos: ['OPT-EAST'] },
        {
          fieldNo: 'CF-MULTI-SELECT',
          optionNos: ['OPT-NETWORK', 'OPT-STORAGE'],
        },
        { dateValue: '2026-08-15', fieldNo: 'CF-DATE' },
        { booleanValue: true, fieldNo: 'CF-BOOLEAN' },
      ],
      associationReference: 'sar_0123456789abcdef0123456789abcdef',
      uploadNos: ['UP-INITIAL-1'],
    });
    expect(savedCreateRequest.postDataJSON()).not.toHaveProperty(
      'entitlementNo',
    );
    await page.waitForURL('**/account/support/TKT-2026-0001');
    await expect(page.getByTestId('ticket-timeline')).toContainText(
      '服务器无法连接',
    );
    await expect(
      page.getByTestId('ticket-custom-field-snapshots'),
    ).toContainText('创建时业务区域');
    await expect(
      page.getByTestId('ticket-custom-field-snapshots'),
    ).toContainText('已停用区域');
    await expect(page.getByTestId('ticket-related-entitlement')).toContainText(
      '创建时云服务器',
    );
    await expect(page.getByTestId('ticket-related-entitlement')).toContainText(
      '当前云服务器',
    );
    await expect(page.locator('body')).not.toContainText('ENT-ACTIVE');
    await expect(page.locator('.ant-spin-spinning')).toHaveCount(0);
    await page
      .getByTestId('ticket-related-entitlement')
      .scrollIntoViewIfNeeded();
    await captureCustomerScreenshot(page, 'customer-ticket-created-with-file');
  });
});

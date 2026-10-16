import { expect, test } from "@host-tests/fixtures/auth";

import { NotificationCenterPage } from "../pages/NotificationCenterPage";
import {
  assertNotificationMockClean,
  installNotificationCenterMock,
  type NotificationCenterMockState,
} from "../support/notification-center-mock";

test.describe("TC001 通知中心", () => {
  let state: NotificationCenterMockState | undefined;

  test.beforeEach(() => {
    state = undefined;
  });

  test.afterEach(async ({ adminPage }) => {
    if (state) assertNotificationMockClean(state);
    await adminPage.unrouteAll({ behavior: "wait" });
  });

  test("TC001a: 五个已冻结渠道只显示安全状态且企业微信没有 UserID 入口", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();

    for (const name of [
      "站内信",
      "电子邮件",
      "短信",
      "微信公众号",
      "企业微信",
    ]) {
      await expect(notification.channelCard(name)).toBeVisible();
    }
    await expect(notification.pageRoot).not.toContainText(
      /smtp|tencent_cloud_sms|wechat_work|builtin/iu,
    );
    await expect(notification.pageRoot).not.toContainText(
      /public-id|mailer@example\.com|smtp\.notify\.example/iu,
    );
    await expect(notification.pageRoot).not.toContainText(
      /password|secretid|secretkey|appsecret/iu,
    );
    await expect(notification.channelCard("电子邮件")).toContainText(
      "凭据尾号 7a9c",
    );
    await expect(
      notification.pageRoot.getByRole("button", { name: /删除|Delete/iu }),
    ).toHaveCount(0);
    const channelDirectoryWidth = await notification.pageRoot
      .locator(".channel-layout > .panel")
      .first()
      .evaluate((element) => element.getBoundingClientRect().width);
    expect(channelDirectoryWidth).toBeGreaterThan(900);
    await notification.screenshot("notification-channels-zh-cn");

    await notification.openChannelConfiguration("电子邮件");
    await expect(
      notification.channelWizard().getByLabel("邮件服务器地址"),
    ).toBeVisible();

    await notification.openChannelConfiguration("短信");
    await expect(
      notification.channelWizard().getByLabel("短信服务商"),
    ).toHaveValue("tencent_cloud_sms_v20210111");
    await expect(
      notification.channelWizard().getByLabel("腾讯云访问账号"),
    ).toBeVisible();

    await notification.openChannelConfiguration("微信公众号");
    await expect(
      notification.channelWizard().getByLabel("公众号固定回调地址"),
    ).toBeVisible();

    await notification.openChannelConfiguration("企业微信");
    const wizard = notification.channelWizard();
    await expect(wizard).toContainText("员工需登录后台后自行扫码绑定企业微信");
    await expect(wizard.getByLabel(/UserID|用户编号|员工账号/iu)).toHaveCount(
      0,
    );
    await expect(wizard.getByLabel(/企业编号|Corporation ID/iu)).toBeVisible();

    await expect(
      notification
        .channelCard("企业微信")
        .getByRole("button", { name: "测试发送" }),
    ).toHaveCount(1);
    await notification.screenshot(
      "notification-channels-wecom-binding-required",
    );
  });

  test("TC001b: 无权限时菜单隐藏且直接请求也被拒绝", async ({ adminPage }) => {
    state = await installNotificationCenterMock(adminPage, {
      canManage: false,
    });
    const notification = new NotificationCenterPage(adminPage);
    await adminPage.reload({ waitUntil: "domcontentloaded" });

    await expect(
      adminPage.getByText(/通知渠道|Notification channels/iu),
    ).toHaveCount(0);
    const direct = await adminPage.evaluate(async () => {
      const response = await fetch(
        "/x/idc-notify/api/v1/admin/notifications/channels",
      );
      return { status: response.status, text: await response.text() };
    });
    expect(direct.status).toBe(403);
    expect(direct.text).toContain("notification permission denied");
    await notification.screenshot("notification-permission-denied");
  });

  test("TC001c: SMTP 配置不回显秘密并且近期登录门禁拒绝旧登录", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();
    await notification.openChannelConfiguration("电子邮件");
    await notification.fillSMTPConfiguration("smtp-e2e-secret-never-echo");
    await notification.screenshot(
      "notification-smtp-configuration-before-save",
    );
    await notification.saveConfiguration();

    await expect(notification.pageRoot.getByRole("status")).toContainText(
      "配置已安全保存",
    );
    expect(state?.configurationRequests).toHaveLength(1);
    expect(state?.configurationRequests[0]?.path).toBe(
      "/x/idc-notify/api/v1/admin/notifications/channels/email/smtp-configuration",
    );
    expect(state?.configurationRequests[0]?.body).toMatchObject({
      fromAddress: "notice@example.com",
      host: "smtp.notify.example",
      password: "smtp-e2e-secret-never-echo",
      port: 465,
      security: "implicit_tls",
    });
    await expect(notification.pageRoot).not.toContainText(
      "smtp-e2e-secret-never-echo",
    );
    await notification.screenshot("notification-smtp-configuration-saved");

    await adminPage.unrouteAll({ behavior: "wait" });
    assertNotificationMockClean(state);
    state = await installNotificationCenterMock(adminPage, {
      recentLogin: false,
    });
    await notification.gotoChannels();
    await notification.openChannelConfiguration("电子邮件");
    await notification.fillSMTPConfiguration("rejected-secret-never-echo");
    await notification.saveConfiguration();
    await expect(notification.pageRoot.getByRole("alert")).toContainText(
      "配置没有保存",
    );
    await expect(notification.pageRoot).not.toContainText(
      "rejected-secret-never-echo",
    );
    expect(state?.configurationRequests).toHaveLength(0);
    await notification.screenshot("notification-recent-login-rejected");
  });

  test("TC001d: 渠道停用、启用、清除都需要确认且只发送一次", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();

    const disableMessage = await notification.confirmNextDialog(() =>
      notification
        .channelCard("电子邮件")
        .getByRole("button", { name: "停用" })
        .click(),
    );
    expect(disableMessage).toContain("确认执行");
    await expect(notification.channelCard("电子邮件")).toContainText("已停用");

    const enableMessage = await notification.confirmNextDialog(() =>
      notification
        .channelCard("电子邮件")
        .getByRole("button", { name: "重新启用" })
        .click(),
    );
    expect(enableMessage).toContain("确认执行");
    await expect(notification.channelCard("电子邮件")).toContainText("已启用");

    await notification.confirmNextDialog(() =>
      notification
        .channelCard("电子邮件")
        .getByRole("button", { name: "停用" })
        .click(),
    );
    const clearMessage = await notification.confirmNextDialog(() =>
      notification
        .channelCard("电子邮件")
        .getByRole("button", { name: "清除配置" })
        .click(),
    );
    expect(clearMessage).toContain("确认执行");
    await expect(notification.channelCard("电子邮件")).toContainText("未配置");
    expect(state?.actionRequests.map((item) => item.path)).toEqual([
      "/x/idc-notify/api/v1/admin/notifications/channels/email/disablement",
      "/x/idc-notify/api/v1/admin/notifications/channels/email/enablement",
      "/x/idc-notify/api/v1/admin/notifications/channels/email/disablement",
      "/x/idc-notify/api/v1/admin/notifications/channels/email/configuration-clearance",
    ]);
    expect(
      state?.actionRequests.every((item) => item.body.confirmed === true),
    ).toBe(true);
    await notification.screenshot("notification-channel-cleared");
  });

  test("TC001v: 整个平台只使用后台选中的一家短信服务商且密钥不回显", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();
    await notification.openChannelConfiguration("短信");

    const wizard = notification.channelWizard();
    const provider = wizard.getByLabel("短信服务商");
    await expect(provider.locator("option")).toHaveText([
      "腾讯云短信",
      "阿里云短信",
      "短信宝",
    ]);

    await provider.selectOption("aliyun_sms_v20170525");
    await wizard.getByLabel("已审核短信签名").fill("LinaIDC");
    await wizard.getByLabel("阿里云访问账号").fill("aliyun-e2e-account");
    await wizard
      .getByLabel("阿里云访问密钥")
      .fill("aliyun-e2e-secret-never-echo");
    await notification.saveConfiguration();
    await expect(notification.pageRoot.getByRole("status")).toContainText(
      "配置已安全保存",
    );
    expect(state?.configurationRequests[0]?.path).toBe(
      "/x/idc-notify/api/v1/admin/notifications/channels/sms/aliyun-configuration",
    );
    expect(state?.configurationRequests[0]?.body).toMatchObject({
      accessKeyId: "aliyun-e2e-account",
      accessKeySecret: "aliyun-e2e-secret-never-echo",
      signName: "LinaIDC",
    });
    await expect(notification.pageRoot).not.toContainText(
      "aliyun-e2e-secret-never-echo",
    );

    await provider.selectOption("smsbao_http_v1");
    await expect(wizard).toContainText("不接收模板编号");
    await wizard.getByLabel("已审核短信签名").fill("LinaIDC");
    await wizard.getByLabel("专用产品编号（选填）").fill("9");
    await wizard.getByLabel("短信宝账号").fill("smsbao-e2e-account");
    await wizard
      .getByLabel(/短信宝 API Key/iu)
      .fill("smsbao-e2e-secret-never-echo");
    await notification.saveConfiguration();
    expect(state?.configurationRequests[1]?.path).toBe(
      "/x/idc-notify/api/v1/admin/notifications/channels/sms/smsbao-configuration",
    );
    expect(state?.configurationRequests[1]?.body).toMatchObject({
      apiKey: "smsbao-e2e-secret-never-echo",
      productId: "9",
      signName: "LinaIDC",
      username: "smsbao-e2e-account",
    });
    await expect(notification.pageRoot).not.toContainText(
      "smsbao-e2e-secret-never-echo",
    );
    await expect(notification.channelCard("短信")).toContainText("短信宝");
    await notification.screenshot("notification-sms-provider-selection");
  });

  test("TC001e: 模板页面保留短信顺序和公众号字段映射", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoTemplates();

    const workspaceTabs = notification.pageRoot.getByRole("tablist", {
      exact: true,
      name: "通知模板 / 发送规则",
    });
    await expect(workspaceTabs.getByRole("tab")).toHaveText([
      "通知模板",
      "发送规则",
    ]);
    await expect(
      workspaceTabs.getByRole("tab", { name: "通知模板", exact: true }),
    ).toHaveAttribute("aria-selected", "true");

    const channelTabs = notification.pageRoot.getByRole("tablist", {
      exact: true,
      name: "通知模板",
    });
    await expect(channelTabs.getByRole("tab")).toHaveText([
      "全部渠道",
      "站内信",
      "电子邮件",
      "短信",
      "微信公众号",
      "企业微信",
      "签名 Webhook",
      "钉钉",
      "飞书",
    ]);
    await channelTabs
      .getByRole("tab", { name: "电子邮件", exact: true })
      .click();
    await expect(
      channelTabs.getByRole("tab", { name: "电子邮件", exact: true }),
    ).toHaveAttribute("aria-selected", "true");
    const emailRows = notification.pageRoot.locator(".template-scroll .row");
    await expect(emailRows).toHaveCount(2);
    await expect(emailRows).toHaveText([/电子邮件/u, /电子邮件/u]);
    await expect(notification.pageRoot.locator(".template-scroll")).toHaveCSS(
      "max-height",
      "440px",
    );
    await expect(notification.pageRoot.locator(".template-scroll")).toHaveCSS(
      "overflow-y",
      "auto",
    );
    await channelTabs
      .getByRole("tab", { name: "全部渠道", exact: true })
      .click();

    await workspaceTabs
      .getByRole("tab", { name: "发送规则", exact: true })
      .click();
    await expect(
      notification.pageRoot.locator(".policy-scroll .row"),
    ).toHaveCount(2);
    await expect(notification.pageRoot.locator(".policy-scroll")).toHaveCSS(
      "max-height",
      "520px",
    );
    await expect(notification.pageRoot.locator(".policy-scroll")).toHaveCSS(
      "overflow-y",
      "auto",
    );
    await expect(notification.pageRoot.locator("form.editor")).toHaveCount(0);
    await workspaceTabs
      .getByRole("tab", { name: "通知模板", exact: true })
      .click();

    const rows = notification.pageRoot
      .locator(".panel")
      .first()
      .locator(".row");
    const smsRow = rows.filter({ hasText: "服务到期提醒" });
    await smsRow.getByRole("button", { name: "查看详情" }).click();
    const editor = notification.pageRoot.locator("form.editor");
    await expect(editor.getByText("当前短信服务商模板对应关系")).toBeVisible();
    await expect(editor.getByLabel("服务商模板编号")).toHaveValue("2299001");
    const smsOrder = editor.locator("fieldset.mapping select");
    await expect(smsOrder).toHaveCount(2);
    await expect(smsOrder.nth(0)).toHaveValue("service_no");
    await expect(smsOrder.nth(1)).toHaveValue("expires_at");
    await editor.getByLabel("服务商模板编号").fill("2299002");
    await editor.getByRole("button", { name: "保存" }).click();
    await expect(notification.pageRoot.getByRole("status")).toContainText(
      "模板已保存",
    );
    expect(state?.templateRequests[0]).toMatchObject({
      channelCode: "sms",
      expectedVersion: 3,
      sms: {
        dataMapping: {},
        providerCode: "tencent_cloud_sms_v20210111",
        templateId: "2299002",
        variableOrder: ["service_no", "expires_at"],
      },
    });

    const wechatRow = rows.filter({ hasText: "支付结果" });
    await wechatRow.getByRole("button", { name: "查看详情" }).click();
    await expect(editor.getByText("公众号模板对应关系")).toBeVisible();
    await expect(editor.getByLabel("服务商模板编号")).toHaveValue(
      "wechat-template-public-id",
    );
    await expect(editor.getByPlaceholder("公众号模板字段名")).toHaveCount(3);
    await editor.getByRole("button", { name: "保存" }).click();
    expect(state?.templateRequests[1]).toMatchObject({
      channelCode: "wechat_official",
      expectedVersion: 4,
      weChatOfficial: {
        dataMapping: {
          amount1: "amount",
          character_string1: "invoice_no",
          thing2: "currency",
        },
        templateId: "wechat-template-public-id",
      },
    });
    await notification.screenshot("notification-template-provider-mappings");
  });

  test("TC001f: 短信测试明确费用、一次确认、目标不留在页面", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();
    await notification.openChannelTest("短信");

    const wizard = notification.pageRoot.locator(".channel-wizard");
    await expect(wizard).toContainText("可能产生当前短信服务商费用");
    await notification.screenshot("notification-sms-test-confirmation");
    await wizard.getByLabel(/一次性手机号/iu).fill("+8613800008000");
    await wizard.getByLabel("服务编号").fill("SERVICE-001");
    await wizard.getByLabel("到期时间").fill("2026-09-01");
    const confirmation = await notification.confirmNextDialog(() =>
      wizard.getByRole("button", { name: "确认并真实发送一次" }).click(),
    );
    expect(confirmation).toContain("可能产生费用");
    expect(state?.testRequests).toHaveLength(1);
    expect(state?.testRequests[0]?.body).toMatchObject({
      confirmed: true,
      target: "+8613800008000",
      templateNo: "NTF_SMS_EXPIRY_ZH",
      templateVersion: 3,
      variables: { expires_at: "2026-09-01", service_no: "SERVICE-001" },
    });
    await expect(wizard.getByLabel(/一次性手机号/iu)).toHaveValue("");
    await expect(notification.pageRoot).not.toContainText("+8613800008000");
    await expect(notification.pageRoot).toContainText("+86 138****8000");
    await notification.screenshot("notification-sms-test-masked-result");
  });

  test("TC001g: 投递记录支持游标、重发资格、人工结案和自动重试状态", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoDeliveries();

    await expect(notification.pageRoot).toContainText("等待重试");
    await notification.screenshot("notification-deliveries-initial");
    await notification.pageRoot
      .getByRole("button", { name: "加载更多" })
      .click();
    expect(state?.deliveryListRequests.at(-1)?.searchParams.get("cursor")).toBe(
      "delivery-page-2",
    );
    await expect(notification.pageRoot).toContainText("密码重置");

    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "服务到期提醒" })
      .click();
    const detail = notification.pageRoot.locator("aside.detail");
    await expect(detail).toContainText("尚未到下一次可操作时间");
    await expect(
      detail.getByRole("button", { name: "重新发送" }),
    ).toBeDisabled();

    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "工单更新" })
      .click();
    await expect(
      detail.getByRole("heading", { name: "工单更新" }),
    ).toBeVisible();
    await detail.getByLabel("请填写操作原因").fill("人工确认收件信息后重发");
    await expect(
      detail.getByRole("button", { name: "重新发送" }),
    ).toBeEnabled();
    await notification.confirmNextDialog(() =>
      detail.getByRole("button", { name: "重新发送" }).click(),
    );
    expect(state?.actionRequests.at(-1)?.body).toMatchObject({
      confirmed: true,
      expectedVersion: 5,
      reason: "人工确认收件信息后重发",
    });
    await expect(notification.pageRoot).toContainText("排队中");

    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "支付结果" })
      .click();
    await expect(
      detail.getByRole("button", { name: /人工结案/iu }),
    ).toBeVisible();
    await detail.getByLabel("请填写操作原因").fill("服务商回调窗口已结束");
    await notification.confirmNextDialog(() =>
      detail.getByRole("button", { name: /人工结案/iu }).click(),
    );
    expect(state?.actionRequests.at(-1)?.body).toMatchObject({
      confirmed: true,
      expectedVersion: 2,
      reason: "服务商回调窗口已结束",
    });
    await expect(detail).toContainText("结果不明确，已人工结案");
    await notification.screenshot("notification-delivery-manual-close");
  });

  test("TC001h: 切换英文后渠道、模板和状态都使用英文业务文案", async ({
    adminPage,
    mainLayout,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();
    await mainLayout.switchLanguage("English");

    await expect(notification.pageRoot).toContainText(
      "External channels use your company accounts. Passwords and keys are never shown again after saving.",
    );
    await expect(notification.channelCard("In-app message")).toContainText(
      "Enabled",
    );
    await expect(notification.channelCard("WeCom")).toContainText("Enabled");
    await expect(notification.pageRoot).not.toContainText(
      /titleChannels|channel_wechat|plugin\.linapro/iu,
    );

    // 通知配置已经统一收进“系统配置”，不再依赖被隐藏的独立菜单。
    await notification.gotoTemplates();
    await mainLayout.switchLanguage("English");
    await expect(
      notification.pageRoot.getByRole("tablist", {
        exact: true,
        name: "Notification templates / Sending rules",
      }),
    ).toBeVisible();
    await expect(notification.pageRoot).toContainText(
      "Service expiry reminder",
    );
    await notification.pageRoot
      .locator(".panel")
      .first()
      .locator(".row")
      .filter({ hasText: "Payment result" })
      .getByRole("button", { name: "View details" })
      .click();
    await expect(notification.pageRoot).toContainText(
      "Official Account template mapping",
    );

    await notification.gotoDeliveries();
    await mainLayout.switchLanguage("English");
    await expect(notification.pageRoot).toContainText("Waiting to retry");
    await expect(notification.pageRoot).toContainText("Manual review");
    await notification.screenshot("notification-center-en-us");
  });

  test("TC001i: 当前员工只能扫码绑定本人且接口始终走通知插件前缀", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage, {
      canManage: false,
    });
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoWeComBinding();

    await expect(
      notification.pageRoot.getByRole("heading", { name: "当前绑定状态" }),
    ).toBeVisible();
    await expect(notification.pageRoot).toContainText("只能绑定你本人的账号");
    await expect(
      notification.pageRoot.getByLabel(/UserID|用户编号|员工账号/iu),
    ).toHaveCount(0);
    await notification.pageRoot
      .getByRole("button", { name: "扫码绑定" })
      .click();

    await expect(
      notification.pageRoot.getByRole("link", { name: "打开企业微信扫码页" }),
    ).toHaveAttribute("href", /^https:\/\/login\.work\.weixin\.qq\.com\//u);
    await expect(
      notification.pageRoot.getByRole("link", { name: "在企业微信中继续授权" }),
    ).toHaveAttribute("href", /^https:\/\/open\.weixin\.qq\.com\//u);
    expect(state?.weComBindingRequests).toHaveLength(1);
    expect(state?.weComBindingRequests[0]?.path).toBe(
      "/x/idc-notify/api/v1/staff/notifications/wechat-work/oauth-authorizations",
    );
    expect(state?.weComBindingRequests[0]?.body).not.toHaveProperty("userId");
    await notification.screenshot("notification-wecom-self-binding");
  });

  test("TC001j: 企业微信测试只发给当前操作员本人绑定且不接受 UserID", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage, {
      weComBound: true,
    });
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();
    await notification.openChannelTest("企业微信");

    const wizard = notification.channelWizard();
    await expect(wizard).toContainText(
      "测试消息发到你已绑定的企业微信",
    );
    await expect(
      wizard.getByLabel(/UserID|用户编号|员工账号|收件人/iu),
    ).toHaveCount(0);
    await expect(wizard).toContainText("工单编号");
    await notification.fillWeComSelfTest("TICKET-E2E-001", "企业微信本人测试");

    const prompt = await notification.submitRealChannelTest();
    expect(prompt).toContain("你本人绑定的企业微信");
    await expect(notification.pageRoot.getByRole("status")).toContainText(
      "测试请求已完成",
    );
    await expect(wizard).toContainText("当前操作员本人的企业微信绑定");
    await expect(wizard).not.toContainText("current_staff_binding");

    expect(state?.testRequests).toHaveLength(1);
    expect(state?.testRequests[0]?.path).toBe(
      "/x/idc-notify/api/v1/admin/notifications/channels/wechat-work/tests",
    );
    expect(state?.testRequests[0]?.body).not.toHaveProperty("target");
    expect(state?.testRequests[0]?.body).not.toHaveProperty("userId");
    await notification.screenshot("notification-wecom-self-test");
  });

  test("TC001k: 当前操作员没有企业微信绑定时测试失败且不伪造发送", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();
    await notification.openChannelTest("企业微信");
    await notification.fillWeComSelfTest("TICKET-E2E-002", "未绑定不发送");

    await notification.submitRealChannelTest();

    await expect(notification.pageRoot.getByRole("alert")).toContainText(
      "测试没有成功完成",
    );
    expect(state?.testRequests).toHaveLength(0);
  });

  test("TC001l: 签名 Webhook 只能配置安全目标并由事件策略明确订阅", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();
    await notification.openChannelConfiguration("签名 Webhook");

    const wizard = notification.channelWizard();
    await expect(wizard).toContainText("只允许公网 HTTPS");
    await expect(
      wizard.getByLabel(/允许内网|跳过证书|关闭安全检查/iu),
    ).toHaveCount(0);
    await wizard
      .getByLabel(/Webhook 公网地址|Public Webhook endpoint/iu)
      .fill("https://127.0.0.1/lina/events");
    await wizard
      .getByLabel("签名密钥")
      .fill("signed-webhook-secret-at-least-32-bytes");
    await notification.saveConfiguration();

    await expect(notification.pageRoot.getByRole("alert")).toContainText(
      "配置没有保存",
    );
    expect(state?.configurationRequests).toHaveLength(0);
    await wizard
      .getByLabel(/Webhook 公网地址|Public Webhook endpoint/iu)
      .fill("https://hooks.example.com/lina/events");
    await wizard
      .getByLabel("签名密钥")
      .fill("signed-webhook-secret-at-least-32-bytes");
    await notification.saveConfiguration();

    expect(state?.configurationRequests.at(-1)).toMatchObject({
      path: "/x/idc-notify/api/v1/admin/notifications/channels/signed-webhook/configuration",
      body: {
        endpointUrl: "https://hooks.example.com/lina/events",
        signingSecret: "signed-webhook-secret-at-least-32-bytes",
      },
    });
    await expect(notification.pageRoot).not.toContainText(
      "signed-webhook-secret-at-least-32-bytes",
    );

    await notification.gotoTemplates();
    await notification.pageRoot
      .getByRole("tab", { name: "发送规则" })
      .click();
    const policy = notification.pageRoot
      .locator(".row")
      .filter({ hasText: "签名 Webhook" });
    await expect(policy).toContainText("工单更新");
    await policy.getByRole("button", { name: "重新启用" }).click();
    expect(state?.eventPolicyRequests).toHaveLength(1);
    expect(state?.eventPolicyRequests[0]).toMatchObject({
      path: "/x/idc-notify/api/v1/admin/notifications/event-policies/support.ticket.updated/customer_contact/signed_webhook",
      body: { enabled: true, expectedVersion: 1 },
    });
    await expect(policy.getByRole("button", { name: "停用" })).toBeVisible();

    const stalePolicy = await adminPage.evaluate(async () => {
      const response = await fetch(
        "/x/idc-notify/api/v1/admin/notifications/event-policies/support.ticket.updated/customer_contact/signed_webhook",
        {
          body: JSON.stringify({
            enabled: false,
            expectedVersion: 1,
            idempotencyKey: "event-policy-stale-e2e-0001",
          }),
          headers: { "Content-Type": "application/json" },
          method: "PUT",
        },
      );
      return response.status;
    });
    expect(stalePolicy).toBe(409);
    expect(state?.eventPolicyRequests).toHaveLength(1);
  });

  test("TC001m: 待核对任务显示安全数量并同时阻止页面和直接清除", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage, {
      pendingEmailQueries: 2,
    });
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();

    const email = notification.channelCard("电子邮件");
    await expect(email).toContainText("待核对任务");
    await expect(email).toContainText("2");
    await expect(email).toContainText("还有待核对的外部发送结果");
    await expect(
      email.getByRole("button", { name: "清除配置" }),
    ).toBeDisabled();

    const direct = await adminPage.evaluate(async () => {
      const response = await fetch(
        "/x/idc-notify/api/v1/admin/notifications/channels/email/configuration-clearance",
        {
          body: JSON.stringify({
            confirmed: true,
            expectedVersion: 3,
            idempotencyKey: "clear-pending-e2e-0001",
          }),
          headers: { "Content-Type": "application/json" },
          method: "POST",
        },
      );
      return { status: response.status, text: await response.text() };
    });
    expect(direct.status).toBe(409);
    expect(direct.text).toContain("pending provider result checks exist");
  });

  test("TC001n: 取消确认零写入，停用取消未越界任务且重启不补发", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage, {
      includeQueuedEmail: true,
    });
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();

    const cancelledDialog = adminPage.waitForEvent("dialog");
    const cancelledClick = notification
      .channelCard("电子邮件")
      .getByRole("button", { name: "停用" })
      .click();
    const opened = await cancelledDialog;
    await opened.dismiss();
    await cancelledClick;
    expect(state?.actionRequests).toHaveLength(0);

    await notification.confirmNextDialog(() =>
      notification
        .channelCard("电子邮件")
        .getByRole("button", { name: "停用" })
        .click(),
    );
    await notification.gotoDeliveries();
    const cancelledTask = notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "已取消" });
    await expect(cancelledTask).toHaveCount(3);

    await notification.gotoChannels();
    await notification.confirmNextDialog(() =>
      notification
        .channelCard("电子邮件")
        .getByRole("button", { name: "重新启用" })
        .click(),
    );
    await notification.gotoDeliveries();
    await expect(
      notification.pageRoot
        .locator("button.row-button")
        .filter({ hasText: "已取消" }),
    ).toHaveCount(3);

    const rejected = await adminPage.evaluate(async () => {
      const request = async (confirmed: boolean, expectedVersion: number) => {
        const response = await fetch(
          "/x/idc-notify/api/v1/admin/notifications/channels/email/disablement",
          {
            body: JSON.stringify({
              confirmed,
              expectedVersion,
              idempotencyKey: `disable-gate-${confirmed}-${expectedVersion}`,
            }),
            headers: { "Content-Type": "application/json" },
            method: "POST",
          },
        );
        return response.status;
      };
      return {
        stale: await request(true, 1),
        unconfirmed: await request(false, 5),
      };
    });
    expect(rejected).toEqual({ stale: 409, unconfirmed: 400 });

    await adminPage.unrouteAll({ behavior: "wait" });
    assertNotificationMockClean(state);
    state = await installNotificationCenterMock(adminPage, {
      recentLogin: false,
    });
    const expiredLogin = await adminPage.evaluate(async () => {
      const response = await fetch(
        "/x/idc-notify/api/v1/admin/notifications/channels/email/disablement",
        {
          body: JSON.stringify({
            confirmed: true,
            expectedVersion: 3,
            idempotencyKey: "disable-expired-login-e2e-0001",
          }),
          headers: { "Content-Type": "application/json" },
          method: "POST",
        },
      );
      return response.status;
    });
    expect(expiredLogin).toBe(403);
    expect(state.actionRequests).toHaveLength(0);
  });

  test("TC001o: 页面分别显示 1、5、30 分钟重试批次且结果不明时不显示自动重试", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoDeliveries();
    const detail = notification.pageRoot.locator("aside.detail");

    for (const [eventName, sequence] of [
      ["密码重置", "1/3"],
      ["服务到期提醒", "2/3"],
      ["工单创建", "3/3"],
    ] as const) {
      await notification.pageRoot
        .locator("button.row-button")
        .filter({ hasText: eventName })
        .first()
        .click();
      await expect(detail).toContainText(sequence);
      await expect(detail).toContainText("下一次自动重试");
    }

    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "支付结果" })
      .click();
    await expect(detail).toContainText("等待人工核对");
    await expect(detail).not.toContainText("下一次自动重试");
  });

  test("TC001p: 模板必填变量缺失时页面不产生测试发送请求", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage);
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoChannels();
    await notification.openChannelTest("短信");
    const wizard = notification.channelWizard();
    await wizard.getByLabel(/一次性手机号/iu).fill("+8613800008000");
    await expect(wizard.getByLabel("服务编号")).toHaveValue("");
    await expect(wizard.getByLabel("到期时间")).toHaveValue("");
    await wizard.getByRole("button", { name: "确认并真实发送一次" }).click();
    expect(state?.testRequests).toHaveLength(0);
  });

  test("TC001q: 人工重发冷却和三次上限都保留灰色按钮并可重新查资格", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage, {
      resendMode: "cooldown",
    });
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoDeliveries();
    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "工单更新" })
      .click();
    const detail = notification.pageRoot.locator("aside.detail");
    await expect(detail).toContainText("尚未到下一次可操作时间");
    await expect(detail).toContainText("最早可再操作时间");
    await detail.getByLabel("请填写操作原因").fill("等待冷却时间");
    await expect(
      detail.getByRole("button", { name: "重新发送" }),
    ).toBeDisabled();
    await detail.getByRole("button", { name: "重新检查" }).click();
    expect(
      state?.eligibilityRequests.filter((task) => task === "TASK-FAILED-001"),
    ).toHaveLength(2);

    await adminPage.unrouteAll({ behavior: "wait" });
    assertNotificationMockClean(state);
    state = await installNotificationCenterMock(adminPage, {
      resendMode: "limit",
    });
    await notification.gotoDeliveries();
    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "工单更新" })
      .click();
    await expect(detail).toContainText("人工重发次数已用完");
    await expect(detail).toContainText("剩余人工重发次数");
    await expect(detail).toContainText(/0/u);
    await expect(
      detail.getByRole("button", { name: "重新发送" }),
    ).toBeDisabled();
  });

  test("TC001r: 人工重发重复提交只产生一个后继任务，人工结案保留原因和操作证据", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage, {
      resendMode: "allowed",
    });
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoDeliveries();
    const before = state.getDeliveryCount();
    const replay = await adminPage.evaluate(async () => {
      const path =
        "/x/idc-notify/api/v1/admin/notifications/deliveries/TASK-FAILED-001/resends";
      const body = {
        confirmed: true,
        expectedVersion: 5,
        operationNo: "resend-idempotent-e2e-0001",
        reason: "provider confirmed no receipt",
      };
      const send = async () => {
        const response = await fetch(path, {
          body: JSON.stringify(body),
          headers: { "Content-Type": "application/json" },
          method: "POST",
        });
        return response.json();
      };
      return [await send(), await send()];
    });
    expect(replay[0].data.item.taskNo).toBe(replay[1].data.item.taskNo);
    expect(state.getDeliveryCount()).toBe(before + 1);

    await notification.gotoDeliveries();
    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "支付结果" })
      .click();
    const detail = notification.pageRoot.locator("aside.detail");
    await expect(
      detail.getByRole("button", { name: /人工结案/iu }),
    ).toBeDisabled();
    await detail.getByLabel("请填写操作原因").fill("服务商回调窗口已结束");
    const dialog = adminPage.waitForEvent("dialog");
    const click = detail.getByRole("button", { name: /人工结案/iu }).click();
    (await dialog).dismiss();
    await click;
    expect(state.closureHistory).toHaveLength(0);

    await notification.confirmNextDialog(() =>
      detail.getByRole("button", { name: /人工结案/iu }).click(),
    );
    expect(state.closureHistory).toEqual([
      expect.objectContaining({
        actor: 1,
        reason: "服务商回调窗口已结束",
        taskNo: "TASK-UNKNOWN-001",
      }),
    ]);
    await expect(detail).toContainText("结果不明确，已人工结案");
  });

  test("TC001s: 资格原因保持安全顺序，检查期间防重复且失败保留旧结果", async ({
    adminPage,
    mainLayout,
  }) => {
    state = await installNotificationCenterMock(adminPage, {
      eligibilityDelayCalls: [2, 5],
      eligibilityFailureCalls: [3, 6],
      eligibilityReasons: [
        "task_state_changed",
        "channel_disabled",
        "insufficient_balance",
        "recipient_unsubscribed",
        "unavailable",
      ],
      resendMode: "cooldown",
    });
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoDeliveries();
    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "工单更新" })
      .click();
    const detail = notification.pageRoot.locator("aside.detail");
    await expect(detail.locator(".reason-list p")).toHaveText([
      "任务状态已变化",
      "渠道已停用",
      "渠道余额不足",
      "收件人已退订",
      "当前不能重新发送，请稍后重试或联系管理员",
    ]);
    await expect(detail).toContainText(
      /还需等待 10 分 0 秒|还需等待 9 分 59 秒/u,
    );

    const recheck = detail.getByRole("button", { name: "重新检查" });
    await recheck.click();
    await expect(
      detail.getByRole("button", { name: "检查中…" }),
    ).toBeDisabled();
    await detail
      .getByRole("button", { name: "检查中…" })
      .click({ force: true });
    await expect(
      detail.getByRole("button", { name: "重新检查" }),
    ).toBeEnabled();
    expect(state.eligibilityRequests).toHaveLength(2);

    await recheck.click();
    await expect(notification.pageRoot.getByRole("alert")).toHaveText(
      "检查失败，请稍后重试",
    );
    await expect(detail.locator(".reason-list p")).toHaveCount(5);
    expect(state.eligibilityRequests).toHaveLength(3);

    await mainLayout.switchLanguage("English");
    await adminPage
      .getByText("Notification Deliveries", { exact: true })
      .first()
      .click();
    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "Ticket update" })
      .click();
    const englishCheck = detail.getByRole("button", { name: "Check again" });
    await englishCheck.click();
    await expect(
      detail.getByRole("button", { name: "Checking…" }),
    ).toBeDisabled();
    await expect(
      detail.getByRole("button", { name: "Check again" }),
    ).toBeEnabled();
    await englishCheck.click();
    await expect(notification.pageRoot.getByRole("alert")).toHaveText(
      "Check failed. Try again later.",
    );
    await expect(detail.locator(".reason-list p")).toHaveText([
      "The task state has changed.",
      "The channel is disabled.",
      "The channel balance is insufficient.",
      "The recipient has unsubscribed.",
      "Resending is currently unavailable. Try again later or contact an administrator.",
    ]);
    expect(state.eligibilityRequests).toHaveLength(6);
  });

  test("TC001t: 离开原投递后迟到的资格结果不会覆盖当前详情", async ({
    adminPage,
  }) => {
    const installedState = await installNotificationCenterMock(adminPage, {
      eligibilityDelayCalls: [1],
      resendMode: "allowed",
    });
    state = installedState;
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoDeliveries();
    const delayedEligibility = adminPage.waitForResponse((response) =>
      response.url().includes("/deliveries/TASK-FAILED-001/resend-eligibility"),
    );
    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "工单更新" })
      .click();
    await expect.poll(() => installedState.eligibilityRequests.length).toBe(1);
    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "支付结果" })
      .click();

    const detail = notification.pageRoot.locator("aside.detail");
    await expect(detail).toContainText("支付结果");
    await detail.getByLabel("请填写操作原因").fill("只验证当前详情状态");
    await expect(
      detail.getByRole("button", { name: "重新发送" }),
    ).toBeDisabled();
    await delayedEligibility;
    await adminPage.evaluate(
      () =>
        new Promise<void>((resolve) => requestAnimationFrame(() => resolve())),
    );
    await expect(detail).toContainText("支付结果");
    await expect(
      detail.getByRole("button", { name: "重新发送" }),
    ).toBeDisabled();
    expect(installedState.eligibilityRequests).toEqual([
      "TASK-FAILED-001",
      "TASK-UNKNOWN-001",
    ]);
  });

  test("TC001u: 资格重查时失去数据权限会退出详情且不保留旧原因", async ({
    adminPage,
  }) => {
    state = await installNotificationCenterMock(adminPage, {
      eligibilityDeniedCalls: [2],
      eligibilityReasons: ["channel_disabled", "recipient_unsubscribed"],
      resendMode: "cooldown",
    });
    const notification = new NotificationCenterPage(adminPage);
    await notification.gotoDeliveries();
    await notification.pageRoot
      .locator("button.row-button")
      .filter({ hasText: "工单更新" })
      .click();
    const detail = notification.pageRoot.locator("aside.detail");
    await expect(detail.locator(".reason-list p")).toHaveCount(2);

    await detail.getByRole("button", { name: "重新检查" }).click();
    await expect(detail).toContainText("请选择一条记录查看详情");
    await expect(detail.locator(".reason-list p")).toHaveCount(0);
    await expect(detail.getByRole("button", { name: "重新发送" })).toHaveCount(
      0,
    );
    expect(state.eligibilityRequests).toEqual([
      "TASK-FAILED-001",
      "TASK-FAILED-001",
    ]);
  });
});

import { readFile } from "node:fs/promises";
import {
  expect,
  test as base,
} from "../../../../../../hack/tests/fixtures/auth";
import { adminStorageStatePath } from "../../../../../../hack/tests/fixtures/auth-state";
import {
  config,
  workspacePath,
} from "../../../../../../hack/tests/fixtures/config";

// Reuse the authorized login for an isolated frontend origin without rewriting shared auth files.
const test = base.extend({
  adminContext: async ({ browser }, use) => {
    const storageState = JSON.parse(
      await readFile(adminStorageStatePath, "utf8"),
    );
    storageState.origins = storageState.origins.map(
      (entry: { origin: string }) => ({
        ...entry,
        origin: new URL(config.baseURL).origin,
      }),
    );
    const context = await browser.newContext({
      baseURL: config.baseURL,
      storageState,
    });
    await use(context);
    await context.close();
  },
});

import { ContentManagementPage } from "../pages/ContentManagementPage";
import {
  contentManagementNo,
  installContentCRUDMock,
  installContentManagementMock,
} from "../support/content-management-mock";

test.describe("TC008 内容上传前安全限制", () => {
  test("TC008a: 需检查文件超过当前实例上限时不会创建上传会话", async ({
    adminPage,
  }) => {
    const state = await installContentManagementMock(adminPage);
    const content = new ContentManagementPage(adminPage);
    await content.goto(contentManagementNo);

    expect(state.policyRequests).toBeGreaterThanOrEqual(1);
    await expect(content.policySummary).toContainText("1.0 KiB 上限");
    await content.chooseFile("oversized.zip", 1025);
    await expect(
      adminPage
        .getByTestId("content-editor")
        .getByRole("alert")
        .filter({ hasText: /不能超过|暂不可用|current instance limit/ }),
    ).toContainText("不能超过当前实例的 1.0 KiB 上限");
    await expect(content.startUpload).toHaveCount(0);
    expect(state.uploadSessionRequests).toHaveLength(0);
    await expect(content.fileInput).toHaveAttribute("accept", /\.zip/);
    await expect(content.fileInput).not.toHaveAttribute("accept", /\.svg/);
    await expect(content.fileInput).not.toHaveAttribute("accept", /\.od[st]/);
    await content.screenshot("content-upload-policy-zh-cn");
  });

  test("TC008b: 当前实例检查能力不可用时需检查文件失败关闭", async ({
    adminPage,
  }) => {
    const state = await installContentManagementMock(adminPage, {
      uploadPolicy: {
        inspectionRequiredAvailable: false,
        inspectionRequiredMaxSizeBytes: 0,
      },
    });
    const content = new ContentManagementPage(adminPage);
    await content.goto(contentManagementNo);

    await expect(content.inspectionUnavailable).toContainText(
      "ZIP、7Z、EXE、MSI 和 ISO 暂时不能上传",
    );
    await content.chooseFile("installer.exe", 64);
    await expect(
      adminPage
        .getByTestId("content-editor")
        .getByRole("alert")
        .filter({ hasText: /不能超过|暂不可用|current instance limit/ }),
    ).toContainText("这类文件需要安全检查，但当前暂不可用");
    expect(state.uploadSessionRequests).toHaveLength(0);
  });

  test("TC008c: 策略读取失败时普通小文件仍按稳定边界继续上传", async ({
    adminPage,
  }) => {
    const state = await installContentManagementMock(adminPage, {
      failUploadPolicy: true,
    });
    const content = new ContentManagementPage(adminPage);
    await content.goto(contentManagementNo);

    expect(state.policyRequests).toBeGreaterThanOrEqual(1);
    await expect(content.inspectionUnavailable).toBeVisible();
    await expect(content.policySummary).toContainText("5.0 GiB");
    await content.chooseFile("policy-unavailable.zip", 32);
    await expect(
      adminPage
        .getByTestId("content-editor")
        .getByRole("alert")
        .filter({ hasText: /不能超过|暂不可用|current instance limit/ }),
    ).toContainText("这类文件需要安全检查，但当前暂不可用");
    expect(state.uploadSessionRequests).toHaveLength(0);
    await content.chooseFile("ordinary.pdf", 32);
    await content.startUpload.click();
    await expect.poll(() => state.uploadSessionRequests.length).toBe(1);
    expect(state.uploadSessionRequests[0]).toMatchObject({
      contentVersion: 3,
      fileName: "ordinary.pdf",
      sizeBytes: 32,
    });
  });

  test("TC008d: 英文界面显示真实实例上限并在上传前拒绝", async ({
    adminPage,
    mainLayout,
  }) => {
    const state = await installContentManagementMock(adminPage);
    const content = new ContentManagementPage(adminPage);
    await content.goto(contentManagementNo);
    await adminPage.getByRole("button", { name: /取\s*消/ }).click();
    await mainLayout.switchLanguage("English");
    await content.goto(contentManagementNo);

    await expect(content.pageRoot).toHaveAttribute("data-locale", "en-US");
    await expect(content.pageRoot).toContainText("安全下载测试内容");
    await expect(content.policySummary).toContainText(
      "verified limit of 1.0 KiB",
    );
    await adminPage.locator(".tiptap-toolbar button").first().hover();
    await expect(adminPage.getByRole("tooltip")).toHaveText("Undo");
    await expect(adminPage.getByTestId("content-editor")).not.toContainText(
      "pages.editor.",
    );
    await content.chooseFile("oversized.iso", 1025);
    await expect(
      adminPage
        .getByTestId("content-editor")
        .getByRole("alert")
        .filter({ hasText: /不能超过|暂不可用|current instance limit/ }),
    ).toContainText("current instance limit of 1.0 KiB");
    expect(state.uploadSessionRequests).toHaveLength(0);
    await content.screenshot("content-upload-policy-en-us");
  });

  test("TC008e: 内容表单没有审核、定时发布、关键词和手工规范链接", async ({
    adminPage,
  }) => {
    await installContentManagementMock(adminPage);
    const content = new ContentManagementPage(adminPage);
    await content.goto(contentManagementNo);
    await adminPage.getByRole("button", { name: /取\s*消/ }).click();
    await adminPage.getByTestId("content-create").click();

    const editor = adminPage.getByTestId("content-editor");
    await expect(editor.getByLabel("正文标题")).toBeVisible();
    await expect(editor.getByLabel("描述")).toBeVisible();
    await expect(editor.getByText(/SEO|关键词|规范链接/)).toHaveCount(0);
    await expect(editor.locator('[contenteditable="true"]')).toBeVisible();
    await expect(editor.getByText(/审核|定时发布/u)).toHaveCount(0);
    await expect(
      adminPage.getByRole("button", { name: "保存草稿" }),
    ).toBeVisible();
    await expect(
      adminPage.getByRole("dialog").getByRole("button", { name: /发\s*布/ }),
    ).toBeVisible();
  });

  test("TC008f: 普通内容类型正确且表单列表搜索均无可见范围", async ({
    adminPage,
  }) => {
    await installContentManagementMock(adminPage);
    const content = new ContentManagementPage(adminPage);
    await content.goto(contentManagementNo);
    await content.startNew();
    const typeChoices = await content.openTypeChoices();
    await expect(typeChoices.filter({ hasText: /^公告$/u })).toHaveCount(0);
    await expect(typeChoices.filter({ hasText: /^新闻$/u })).toBeVisible();
    await typeChoices.filter({ hasText: /^新闻$/u }).click();
    await expect(adminPage.getByText("可见范围", { exact: true })).toHaveCount(
      0,
    );
    await expect(
      adminPage.getByText("指定可见对象", { exact: true }),
    ).toHaveCount(0);
    await content.screenshot("content-no-visibility-editor");
    await adminPage.getByRole("button", { name: /取\s*消/ }).click();
    await expect(
      content.pageRoot.getByText("可见范围", { exact: true }),
    ).toHaveCount(0);
    await content.screenshot("content-no-visibility-table-search");
  });
  test("TC008g: 标准表格、分类、富文本增改删及保存失败恢复", async ({
    adminPage,
  }) => {
    const state = await installContentCRUDMock(adminPage);
    await adminPage.goto(workspacePath("/idc/content"));
    const root = adminPage.getByTestId("content-management-page");
    await expect(
      root.locator("aside .ant-tree").getByText("公司新闻", { exact: true }),
    ).toBeVisible();
    await expect(root.locator(".vxe-pager--total")).toContainText("12");
    expect(state.authenticated).toBe(true);
    await new ContentManagementPage(adminPage).screenshot("content-table");
    await root.locator(".vxe-pager--next-btn").click();
    await expect.poll(() => state.queries.at(-1)?.get("page")).toBe("2");
    await expect(root.getByTestId("content-item-CNT_NEWS_11")).toBeVisible();
    await root
      .locator("aside .ant-tree")
      .getByText("公司新闻", { exact: true })
      .click();
    await expect
      .poll(() => state.queries.at(-1)?.get("categoryNo"))
      .toBe("CAT_NEWS");
    await root.getByTestId("content-create").click();
    const editor = adminPage.getByTestId("content-editor");
    const drawer = adminPage.locator(".content-editor-drawer");
    await expect(drawer).toHaveCSS("box-shadow", /rgba?\(/);
    await expect(
      adminPage.locator("[data-dismissable-drawer].bg-overlay").first(),
    ).toBeVisible();
    await editor.locator(".tiptap-toolbar button").first().hover();
    await expect(adminPage.getByRole("tooltip")).toHaveText("撤销");
    await editor.getByLabel("正文标题").fill("富文本测试");
    await editor.getByLabel("描述", { exact: true }).fill("内容描述");
    await editor.locator('[contenteditable="true"]').fill("正文段落");
    await new ContentManagementPage(adminPage).screenshot(
      "content-editor-drawer",
    );
    state.failSave = true;
    await adminPage
      .getByRole("button", { name: "保存草稿", exact: true })
      .click();
    await expect(editor.getByRole("alert")).toContainText("本次更改未保存");
    await expect(editor.getByLabel("正文标题")).toHaveValue("富文本测试");
    await new ContentManagementPage(adminPage).screenshot(
      "content-save-failed",
    );
    state.failSave = false;
    await adminPage
      .getByRole("button", { name: "保存草稿", exact: true })
      .click();
    await expect(editor).not.toBeVisible();
    expect(state.writes.at(-1)).toMatchObject({
      title: "富文本测试",
      summary: "内容描述",
      body: "<p>正文段落</p>",
      seoTitle: "",
      seoDescription: "",
    });
    await root.getByTestId("content-item-CNT_CREATED").click();
    await editor.getByLabel("正文标题").fill("修改后标题");
    await adminPage
      .getByRole("dialog")
      .getByRole("button", { name: /发\s*布/ })
      .click();
    await expect(editor).not.toBeVisible();
    expect(
      state.items.find((item) => item.contentNo === "CNT_CREATED"),
    ).toMatchObject({ title: "修改后标题", status: "published" });
    expect(state.actions.at(-1)).toMatchObject({
      action: "publications",
      version: 2,
    });
    const row = root.locator(
      '.vxe-table--fixed-right-wrapper [rowid="CNT_CREATED"]',
    );
    await root
      .locator('.vxe-table--fixed-right-wrapper [rowid="CNT_NEWS_1"]')
      .getByRole("button", { name: /发\s*布/ })
      .click();
    await expect
      .poll(
        () =>
          state.items.find((item) => item.contentNo === "CNT_NEWS_1")?.status,
      )
      .toBe("published");
    const actionButtons = row.getByRole("button");
    const bounds = await actionButtons.evaluateAll((buttons) =>
      buttons.map((button) => {
        const { left, right } = button.getBoundingClientRect();
        return { left, right };
      }),
    );
    expect(bounds[1].left - bounds[0].right).toBeGreaterThanOrEqual(7);
    await row.getByRole("button", { name: /下\s*架/ }).click();
    await expect(row.getByRole("button", { name: /上\s*架/ })).toBeVisible();
    expect(state.actions.at(-1)?.action).toBe("unpublications");
    await row.getByRole("button", { name: /上\s*架/ }).click();
    await expect(row.getByRole("button", { name: /下\s*架/ })).toBeVisible();
    expect(state.actions.at(-1)?.action).toBe("republications");
    state.failDetailNo = "CNT_CREATED";
    await row.getByRole("button", { name: /删\s*除/ }).click();
    await adminPage
      .locator(".ant-popconfirm")
      .getByRole("button", { name: /确\s*定/ })
      .click();
    await expect(root.getByRole("alert")).toContainText("内容加载失败");
    expect(state.deletedNos).toHaveLength(0);
    state.failDetailNo = "";
    await row.getByRole("button", { name: /删\s*除/ }).click();
    await expect(
      adminPage.getByText(
        "删除后不能通过普通内容管理恢复，原公开链接也会失效。",
      ),
    ).toBeVisible();
    await adminPage
      .locator(".ant-popconfirm")
      .getByRole("button", { name: /取\s*消/ })
      .click();
    expect(state.items.some((item) => item.contentNo === "CNT_CREATED")).toBe(
      true,
    );
    await row.getByRole("button", { name: /删\s*除/ }).click();
    await adminPage
      .locator(".ant-popconfirm")
      .getByRole("button", { name: /确\s*定/ })
      .click();
    await expect(root.getByTestId("content-item-CNT_CREATED")).toHaveCount(0);
    expect(state.items.some((item) => item.contentNo === "CNT_CREATED")).toBe(
      false,
    );
    await root.getByTestId("content-category-create").click();
    const categoryModal = adminPage.locator(".content-category-modal");
    await expect(categoryModal).toHaveAttribute("role", "dialog");
    await expect(categoryModal).toHaveCSS("width", "480px");
    const modalBounds = await categoryModal.boundingBox();
    expect(modalBounds!.width).toBeLessThanOrEqual(500);
    expect(modalBounds!.x).toBeGreaterThan(100);
    await adminPage.getByLabel("分类名称").fill("新增新闻分类");
    await new ContentManagementPage(adminPage).screenshot(
      "content-category-modal",
    );
    await adminPage
      .getByRole("dialog")
      .getByRole("button", { name: /确\s*认/ })
      .click();
    await expect(root.getByText("新增新闻分类", { exact: true })).toBeVisible();
    await root.getByText("新增新闻分类", { exact: true }).click();
    await expect
      .poll(() => state.queries.at(-1)?.get("categoryNo"))
      .toBe("CAT_NEW");
    await new ContentManagementPage(adminPage).screenshot(
      "content-category-filter",
    );
  });
  test("TC008h: 官网配置入口使用实际中文并打开同页内容管理", async ({
    adminPage,
  }) => {
    const state = await installContentCRUDMock(adminPage);
    await adminPage.goto(workspacePath("/system/website-configuration"));
    const entry = adminPage.getByTestId("website-content-management");
    await expect(entry).toHaveText("内容管理");
    await entry.click();
    await expect(
      adminPage.getByTestId("content-management-page"),
    ).toBeVisible();
    await expect.poll(() => state.queries.length).toBeGreaterThan(0);
    expect(state.authenticated).toBe(true);
    await new ContentManagementPage(adminPage).screenshot(
      "content-website-configuration-entry",
    );
  });

  test("TC008i: 真实接口完成草稿编辑发布刷新与删除", async ({ adminPage }) => {
    let contentNo = "",
      authorization = "";
    const basePath = "/x/idc-core/api/v1/admin/content/items";
    const title = `内容管理验收-${Date.now()}`;
    try {
      await adminPage.goto(workspacePath("/idc/content"));
      const root = adminPage.getByTestId("content-management-page");
      await root.getByTestId("content-create").click();
      const editor = adminPage.getByTestId("content-editor");
      await editor.getByLabel("正文标题").fill(title);
      await editor.getByLabel("描述", { exact: true }).fill("真实保存描述");
      await editor.locator('[contenteditable="true"]').fill("真实正文");
      const creation = adminPage.waitForResponse(
        (response) =>
          response.url().endsWith(basePath) &&
          response.request().method() === "POST",
      );
      await adminPage
        .getByRole("dialog")
        .getByRole("button", { name: "保存草稿" })
        .click();
      const response = await creation;
      const created = await response.json();
      expect(created.code).toBe(0);
      contentNo = created.data.item.contentNo;
      authorization = response.request().headers().authorization;
      await expect(editor).not.toBeVisible();
      await root.getByTestId(`content-item-${contentNo}`).click();
      await editor.getByLabel("正文标题").fill(title + "-修改");
      await adminPage
        .getByRole("dialog")
        .getByRole("button", { name: /发\s*布/ })
        .click();
      await expect(editor).not.toBeVisible();
      await adminPage.reload();
      await expect(root.getByTestId(`content-item-${contentNo}`)).toHaveText(
        title + "-修改",
      );
      const read = await adminPage.request.get(`${basePath}/${contentNo}`, {
        headers: { Authorization: authorization },
      });
      const detail = (await read.json()).data.item;
      expect(detail).toMatchObject({
        title: title + "-修改",
        summary: "真实保存描述",
        body: "<p>真实正文</p>",
        status: "published",
        publicPath: created.data.item.publicPath,
      });
      await new ContentManagementPage(adminPage).screenshot(
        "content-real-persisted",
      );
      const row = root.locator(
        `.vxe-table--fixed-right-wrapper [rowid="${contentNo}"]`,
      );
      await row.getByRole("button", { name: /删\s*除/ }).click();
      await adminPage
        .locator(".ant-popconfirm")
        .getByRole("button", { name: /确\s*定/ })
        .click();
      await expect(root.getByTestId(`content-item-${contentNo}`)).toHaveCount(
        0,
      );
      await new ContentManagementPage(adminPage).screenshot(
        "content-real-deleted",
      );
    } finally {
      if (contentNo && authorization) {
        const response = await adminPage.request.get(
          `${basePath}/${contentNo}`,
          { headers: { Authorization: authorization } },
        );
        const payload = await response.json();
        if (payload.code === 0 && payload.data?.item?.status !== "deleted") {
          const cleanup = await adminPage.request.delete(
            `${basePath}/${contentNo}`,
            {
              headers: { Authorization: authorization },
              data: { confirmed: true, version: payload.data.item.version },
            },
          );
          expect((await cleanup.json()).code).toBe(0);
        }
      }
    }
  });
  test("TC008j: 隐藏可见范围后保留原权限并完整翻译编辑器提示", async ({
    adminPage,
    mainLayout,
  }) => {
    const state = await installContentCRUDMock(adminPage);
    Object.assign(state.items[0], {
      visibility: "customer_group",
      targetCustomerGroupNos: ["GROUP_EXISTING"],
    });
    await adminPage.goto(workspacePath("/idc/content"));
    const root = adminPage.getByTestId("content-management-page");
    await root.getByTestId("content-item-CNT_NEWS_1").click();
    const editor = adminPage.getByTestId("content-editor");
    await expect(editor.getByText("可见范围", { exact: true })).toHaveCount(0);
    const labels = [
      [
        "撤销",
        "重做",
        "加粗",
        "斜体",
        "下划线",
        "删除线",
        "标题1",
        "标题2",
        "标题3",
        "无序列表",
        "有序列表",
        "引用",
        "代码块",
        "分割线",
        "图片",
        "链接",
      ],
      [
        "Undo",
        "Redo",
        "Bold",
        "Italic",
        "Underline",
        "Strikethrough",
        "Heading 1",
        "Heading 2",
        "Heading 3",
        "Bulleted List",
        "Numbered List",
        "Blockquote",
        "Code Block",
        "Divider",
        "Image",
        "Link",
      ],
    ];
    for (const [index, expected] of labels.entries()) {
      if (index) {
        await mainLayout.switchLanguage("English");
        await root.getByTestId("content-item-CNT_NEWS_1").click();
      }
      const buttons = editor.locator(".tiptap-toolbar button");
      await expect(buttons).toHaveCount(expected.length);
      for (const [buttonIndex, label] of expected.entries()) {
        await buttons.nth(buttonIndex).hover();
        await expect(
          adminPage.getByRole("tooltip", { name: label, exact: true }),
        ).toHaveText(label);
      }
      await buttons.last().click();
      const linkModal = adminPage.locator(".ant-modal-content");
      await expect(adminPage.locator(".ant-modal-wrap:visible")).toHaveCSS(
        "background-color",
        "rgba(0, 0, 0, 0)",
      );
      await expect(linkModal.locator(".ant-modal-title")).toHaveText(
        index ? "Insert Link" : "插入链接",
      );
      await expect(linkModal.locator("input")).toHaveAttribute(
        "placeholder",
        index ? "Please enter the link URL" : "请输入链接地址",
      );
      await new ContentManagementPage(adminPage).screenshot(
        index ? "content-editor-link-en" : "content-editor-link-zh",
      );
      await linkModal
        .getByRole("button", { name: index ? "Cancel" : /取\s*消/ })
        .click();
      if (!index) {
        await editor.getByLabel("正文标题").fill("保留原有权限");
        await adminPage.getByRole("button", { name: /^保\s*存$/ }).click();
        await expect(editor).not.toBeVisible();
        expect(state.writes.at(-1)).toMatchObject({
          title: "保留原有权限",
          visibility: "customer_group",
          targetCustomerGroupNos: ["GROUP_EXISTING"],
        });
      }
    }
  });
});

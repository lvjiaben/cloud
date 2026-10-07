import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ContentManagementPage } from "../pages/ContentManagementPage";
import {
  contentManagementNo,
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
    await expect(content.pageRoot.getByRole("alert")).toContainText(
      "不能超过当前实例的 1.0 KiB 上限",
    );
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
    await expect(content.pageRoot.getByRole("alert")).toContainText(
      "这类文件需要安全检查，但当前暂不可用",
    );
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
    await expect(content.pageRoot.getByRole("alert")).toContainText(
      "这类文件需要安全检查，但当前暂不可用",
    );
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
    await mainLayout.switchLanguage("English");

    await expect(content.pageRoot).toHaveAttribute("data-locale", "en-US");
    await expect(content.pageRoot).toContainText("安全下载测试内容");
    await expect(content.policySummary).toContainText(
      "verified limit of 1.0 KiB",
    );
    await content.chooseFile("oversized.iso", 1025);
    await expect(content.pageRoot.getByRole("alert")).toContainText(
      "current instance limit of 1.0 KiB",
    );
    expect(state.uploadSessionRequests).toHaveLength(0);
    await content.screenshot("content-upload-policy-en-us");
  });

  test("TC008e: 内容表单没有审核、定时发布、关键词和手工规范链接", async ({
    adminPage,
  }) => {
    await installContentManagementMock(adminPage);
    const content = new ContentManagementPage(adminPage);
    await content.goto(contentManagementNo);
    await adminPage.getByTestId("content-create").click();

    const editor = adminPage.getByTestId("content-editor");
    await expect(editor.getByLabel("正文标题")).toHaveAttribute("required", "");
    await expect(editor.getByLabel("摘要（可选）")).toBeVisible();
    await expect(editor.getByLabel("SEO 标题（可选）")).toBeVisible();
    await expect(editor.getByLabel("SEO 描述（可选）")).toBeVisible();
    await expect(editor).toContainText("不需要关键词和手工规范链接");
    await expect(editor.getByText(/审核|定时发布/u)).toHaveCount(0);
    await expect(editor.getByRole("button", { name: "保存草稿" })).toBeVisible();
    await expect(editor.getByRole("button", { name: "发布", exact: true })).toBeVisible();
  });

  test("TC008f: 公告不显示分类并提供精确客户和服务器服务范围", async ({
    adminPage,
  }) => {
    await installContentManagementMock(adminPage);
    const content = new ContentManagementPage(adminPage);
    await content.goto(contentManagementNo);
    await adminPage.getByTestId("content-create").click();

    const editor = adminPage.getByTestId("content-editor");
    await editor.getByLabel("内容类型").selectOption("announcement");
    await expect(editor.getByLabel("分类")).toHaveCount(0);
    const visibility = editor.getByLabel("可见范围");
    await expect(visibility.locator('option[value="specific_customers"]')).toHaveCount(1);
    await expect(visibility.locator('option[value="specific_services"]')).toHaveCount(1);
  });
});

import { expect, test } from "../../fixtures/auth";
import { DashboardPage } from "../../pages/DashboardPage";

const chineseTextPattern = /[\u3400-\u9fff]/u;

test.describe("TC-4 Workbench English i18n regression", () => {
  test("TC-4a: English workbench uses complete business copy", async ({
    adminPage,
    mainLayout,
  }) => {
    const dashboardPage = new DashboardPage(adminPage);
    await adminPage.setViewportSize({ width: 1366, height: 900 });
    await mainLayout.switchLanguage("English");
    await dashboardPage.gotoWorkspace();

    await expect(dashboardPage.workspaceProjects).toContainText(
      "Today's business focus",
    );
    await expect(dashboardPage.workspaceQuickNav).toContainText(
      "Quick navigation",
    );
    await expect(dashboardPage.workspaceTodos).toContainText("To-do items");
    await expect(dashboardPage.workspaceQuickNav).toContainText(
      "Customer management",
    );
    expect(await dashboardPage.workspacePage.innerText()).not.toMatch(
      chineseTextPattern,
    );
  });
});

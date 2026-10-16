import { test, expect } from "@host-tests/fixtures/auth";

import { PermissionHelpPage } from "../../pages/PermissionHelpPage";

const cases = [
  {
    topic: "user",
    title: "用户管理怎么用",
    note: "不用再单独申请员工资格",
    english: "How to manage users",
    englishNote: "No separate staff qualification is needed.",
  },
  {
    topic: "role",
    title: "角色管理怎么用",
    note: "多个角色会叠加权限",
    english: "How to manage roles",
    englishNote: "Permissions from multiple roles are combined",
  },
  {
    topic: "dept",
    title: "部门管理怎么用",
    note: "加入部门不等于获得操作权限",
    english: "How to manage departments",
    englishNote: "Joining a department does not grant action permissions",
  },
  {
    topic: "post",
    title: "岗位管理怎么用",
    note: "岗位不授予任何功能权限",
    english: "How to manage positions",
    englishNote: "Positions do not grant permissions.",
  },
  {
    topic: "menu",
    title: "菜单管理怎么用",
    note: "隐藏菜单只是不在导航里显示，不代表禁止访问",
    english: "How to manage menus",
    englishNote: "Hiding a menu only removes it from navigation",
  },
];

type Menu = { path: string; children?: Menu[] };
function find(nodes: Menu[], name: string): Menu | undefined {
  for (const node of nodes) {
    if (node.path.replace(/^\//, "") === name) return node;
    const child = find(node.children ?? [], name);
    if (child) return child;
  }
}

test.describe("TC-9 权限菜单归并与大白话帮助", () => {
  test("TC-9a: 五页帮助说明关键权限边界，关闭后保留原页面", async ({
    adminPage,
    mainLayout,
  }) => {
    const help = new PermissionHelpPage(adminPage);
    await help.goto("user");
    await mainLayout.switchLanguage("简体中文");
    for (const item of cases) {
      await help.goto(item.topic);
      await expect(help.button).toHaveAccessibleName("使用帮助");
      const buttonBox = await help.button.boundingBox();
      const viewport = adminPage.viewportSize()!;
      expect(buttonBox!.x).toBeGreaterThan(viewport.width - 90);
      expect(buttonBox!.y).toBeGreaterThan(viewport.height - 180);
      await expect(help.button).toHaveText("");
      await help.open();
      await expect(help.drawer).toContainText(item.title);
      await expect(
        help.drawer.getByRole("button", { name: "确认", exact: true }),
      ).toHaveCount(0);
      await expect(help.content).toContainText(item.note);
      await expect(help.content.locator("li")).toHaveCount(4);
      if (item.topic === "role") {
        await expect(help.content).toContainText("本部门");
        await expect(help.content).toContainText("本人”范围不能接新单");
        await expect(help.content).toContainText("管理本部门工单");
      }
      expect(await help.content.innerText()).not.toMatch(
        /plugin\.org-core\.permissionHelp/,
      );
      await help.screenshot(`${item.topic}-zh`);
      await help.close();
      await expect(help.button).toBeVisible();
    }
  });

  test("TC-9b: 英文帮助翻译完整，窄屏可阅读，非权限页不显示", async ({
    adminPage,
    mainLayout,
  }) => {
    const help = new PermissionHelpPage(adminPage);
    await help.goto("user");
    await mainLayout.switchLanguage("English");
    for (const item of cases) {
      await help.goto(item.topic);
      await expect(help.button).toHaveAccessibleName("Usage help");
      await help.open();
      await expect(help.drawer).toContainText(item.english);
      await expect(help.content).toContainText(item.englishNote);
      expect(await help.content.innerText()).not.toMatch(
        /plugin\.org-core\.permissionHelp/,
      );
      await help.close();
    }
    await adminPage.setViewportSize({ width: 430, height: 900 });
    await help.goto("role");
    await help.open();
    await expect(help.content).toContainText(
      "Permissions from multiple roles are combined",
    );
    const box = await help.drawer.boundingBox();
    expect(box?.width).toBeLessThanOrEqual(430);
    await help.screenshot("role-en-mobile");
    await help.close();
    await adminPage.goto("/profile");
    await expect(help.button).toHaveCount(0);
  });

  test("TC-9c: 部门岗位与用户角色菜单归入权限管理", async ({ adminPage }) => {
    const help = new PermissionHelpPage(adminPage);
    await help.goto("dept");
    const payload = await help.menuTree();
    expect(payload.code).toBe(0);
    const menus: Menu[] = payload.data.list;
    const iam = find(menus, "iam");
    expect(iam).toBeTruthy();
    const children = iam?.children ?? [];
    for (const name of [
      "system/user",
      "system/role",
      "system/dept",
      "system/post",
    ]) {
      expect(find(children, name), name).toBeTruthy();
    }
    expect(find(menus, "org")).toBeUndefined();
    expect(JSON.stringify(menus)).not.toMatch(/staff-access|员工权限/);
    await help.screenshot("unified-menu");
  });
  test("TC-9d: 悬浮问号随明暗主题变化并保持圆形阴影", async ({
    adminPage,
    mainLayout,
  }) => {
    const help = new PermissionHelpPage(adminPage);
    await help.goto("user");
    await mainLayout.switchLanguage("简体中文");
    await mainLayout.prepareThemeModeWithoutAnimation("light");
    await expect(help.button).toHaveAccessibleName("使用帮助");
    const light = await help.appearance();
    expect(light.radius).toBe("50%");
    expect(light.position).toBe("fixed");
    expect(light.shadow).not.toBe("none");
    await help.screenshot("floating-light");
    await mainLayout.prepareThemeModeWithoutAnimation("dark");
    await expect
      .poll(async () => (await help.appearance()).background)
      .not.toBe(light.background);
    expect((await help.appearance()).radius).toBe("50%");
    await help.screenshot("floating-dark");
    await help.open();
    await expect(help.content).toContainText("不用再单独申请员工资格");
    await help.screenshot("drawer-dark");
    await help.close();
    await mainLayout.prepareThemeModeWithoutAnimation("light");
  });
});

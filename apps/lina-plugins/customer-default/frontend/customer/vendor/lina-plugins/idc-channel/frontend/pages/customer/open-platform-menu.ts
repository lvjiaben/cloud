// Source-plugin initialization uses public router readiness and store setters only.
// The menu is limited to the manifest-declared route; API ownership remains authoritative.
import type { Router } from "vue-router";
import { effectScope, onScopeDispose, watch } from "vue";
import { preferences } from "@vben/preferences";
import { useAccessStore, useTabbarStore, useUserStore } from "@vben/stores";
import { pluginApiPath, requestClient } from "#/api/request";
import { channelText as t } from "../channel-i18n";

let installed = false;
export function bindOpenPlatformMenu(router: Router) {
  if (installed) return;
  installed = true;
  const scope = effectScope(true);
  if (import.meta.hot)
    import.meta.hot.dispose(() => {
      scope.stop();
      installed = false;
    });
  scope.run(() => {
    const access = useAccessStore();
    const user = useUserStore();
    const tabbar = useTabbarStore();
    const path = "/account/plugins/idc-channel/open-platform";
    let authorized = false;
    let generation = 0;
    let updating = false;
    let pendingRevision: number | undefined;
    type Menu = (typeof access.accessMenus)[number];
    function updateMenu() {
      if (updating || !router.currentRoute.value.path.startsWith("/account"))
        return;
      updating = true;
      try {
        const clean = (menus: Menu[]): Menu[] =>
          menus
            .filter((menu) => menu.path !== path)
            .map((menu) =>
              menu.children
                ? { ...menu, children: clean(menu.children) }
                : menu,
            );
        const menus = clean(access.accessMenus);
        const registered = router
          .getRoutes()
          .find((item) => item.path === path);
        if (registered) registered.meta.title = t("platform.title");
        if (router.currentRoute.value.path === path)
          router.currentRoute.value.meta.title = t("platform.title");
        const ownTab = tabbar.getTabs.find((tab) => tab.path === path);
        if (ownTab && ownTab.meta.newTabTitle !== t("platform.title"))
          void tabbar.setTabTitle(ownTab, t("platform.title"));
        if (authorized && registered) {
          const group = menus.find((menu) => menu.path === "/account/products");
          if (group)
            group.children = [
              ...(group.children ?? []),
              {
                path,
                name: t("platform.title"),
                icon: "lucide:code",
                parent: group.path,
                parents: [group.path],
                order: 100,
                show: true,
              },
            ];
          else menus.push({ path, name: t("platform.title"), icon: "lucide:code", order: 100, show: true });
        }
        if (JSON.stringify(menus) !== JSON.stringify(access.accessMenus))
          access.setAccessMenus(menus);
      } finally {
        updating = false;
      }
    }
    async function refresh() {
      if (
        pendingRevision === generation ||
        !user.userInfo ||
        !router.currentRoute.value.path.startsWith("/account") ||
        !router.getRoutes().some((item) => item.path === path)
      )
        return;
      const revision = generation;
      pendingRevision = revision;
      try {
        const result = await requestClient.get<{ authorized: boolean }>(
          pluginApiPath("idc-channel", "account/open-platform"),
          { silentErrorMessage: true },
        );
        if (revision === generation) {
          authorized = result.authorized;
          updateMenu();
        }
      } catch {
        if (revision === generation) {
          authorized = false;
          updateMenu();
        }
      } finally {
        if (pendingRevision === revision) pendingRevision = undefined;
      }
    }
    watch(
      () => user.userInfo?.userId,
      () => {
        generation++;
        authorized = false;
        updateMenu();
        void refresh();
      },
      { immediate: true },
    );
    watch(
      () => access.accessMenus,
      () => {
        updateMenu();
        void refresh();
      },
      { deep: true },
    );
    watch(
      () => router.currentRoute.value.path,
      () => {
        updateMenu();
        void refresh();
      },
    );
    watch(() => preferences.app.locale, updateMenu);
    window.addEventListener("focus", refresh);
    onScopeDispose(() => {
      generation++;
      window.removeEventListener("focus", refresh);
    });
  });
}

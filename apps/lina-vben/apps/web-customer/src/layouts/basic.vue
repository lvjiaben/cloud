<script lang="ts" setup>
import type { MenuRecordRaw } from '@vben/types';

import type { CustomerExternalMenu } from '#/api/idc/external-menu';

import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import { AuthenticationLoginExpiredModal } from '@vben/common-ui';
import { useWatermark } from '@vben/hooks';
import { BasicLayout, LockScreen, UserDropdown } from '@vben/layouts';
import { preferences, updatePreferences } from '@vben/preferences';
import { useAccessStore, useTabbarStore, useUserStore } from '@vben/stores';

import { listCustomerExternalMenus } from '#/api/idc/external-menu';
import { $t } from '#/locales';
import { refreshAccessibleState } from '#/router/access-refresh';
import { useAuthStore } from '#/store';
import LoginForm from '#/views/_core/authentication/login.vue';

const userStore = useUserStore();
const authStore = useAuthStore();
const accessStore = useAccessStore();
const router = useRouter();
const { destroyWatermark, updateWatermark } = useWatermark();
const externalMenus = ref<CustomerExternalMenu[]>([]);
const externalMenuGroupPath = '/account/external-services';
const assistanceNow = ref(Date.now());
let assistanceClock: ReturnType<typeof setInterval> | undefined;

const assistanceRemainingMinutes = computed(() => {
  const expiresAt = authStore.customerAssistanceSession?.expiresAt ?? 0;
  return Math.max(0, Math.ceil((expiresAt - assistanceNow.value) / 60_000));
});

const avatar = computed(() => {
  return userStore.userInfo?.avatar || preferences.app.defaultAvatar;
});

const displayRealName = computed(() => {
  return userStore.userInfo?.realName || userStore.userInfo?.username || '';
});

const accountMenus = computed(() => [
  {
    handler: () => router.push('/account/profile'),
    icon: 'lucide:user-round-cog',
    text: $t('page.customer.avatarMenu.profile'),
  },
  {
    handler: () => router.push('/account/addresses'),
    icon: 'lucide:map-pin-house',
    text: $t('page.customer.avatarMenu.addresses'),
  },
  {
    handler: () => router.push('/account/data-rights'),
    icon: 'lucide:file-lock-2',
    text: $t('page.customer.avatarMenu.dataRights'),
  },
  {
    handler: () => router.push('/account/privacy'),
    icon: 'lucide:shield-check',
    text: $t('page.customer.avatarMenu.privacy'),
  },
]);

async function handleLogout() {
  await authStore.logout(false);
}

function applyExternalMenus() {
  const localMenus = accessStore.accessMenus.filter(
    (menu) => menu.path !== externalMenuGroupPath,
  );
  if (externalMenus.value.length === 0) {
    accessStore.setAccessMenus(localMenus);
    return;
  }
  const useChinese = preferences.app.locale === 'zh-CN';
  const children: MenuRecordRaw[] = externalMenus.value.map((item) => ({
    icon: item.icon,
    name: useChinese ? item.nameZh : item.nameEn,
    order: item.sortOrder,
    path: item.url,
  }));
  accessStore.setAccessMenus([
    ...localMenus,
    {
      children,
      icon: 'lucide:external-link',
      name: $t('menu.customerGroups.external'),
      order: 90,
      path: externalMenuGroupPath,
    },
  ]);
}

async function loadExternalMenus() {
  try {
    const result = await listCustomerExternalMenus();
    externalMenus.value = (result.items || []).filter(
      (item) => item.enabled && /^https?:\/\//i.test(item.url),
    );
    applyExternalMenus();
  } catch {
    externalMenus.value = [];
    applyExternalMenus();
  }
}

watch(
  () => ({
    content: preferences.app.watermarkContent,
    enable: preferences.app.watermark,
  }),
  async ({ content, enable }) => {
    if (enable) {
      await updateWatermark({
        content:
          content ||
          `${userStore.userInfo?.username || ''} - ${displayRealName.value}`,
      });
      return;
    }
    destroyWatermark();
  },
  { immediate: true },
);

watch(
  () => preferences.app.locale,
  async () => {
    await refreshAccessibleState(router, {
      refreshUserInfo: false,
      skipRouteNavigation: true,
    });
    const tabbar = useTabbarStore();
    await Promise.all(
      tabbar.getTabs
        .filter((tab) => tab.meta.productDirectoryNo)
        .map((tab) =>
          tabbar.setTabTitle(
            tab,
            String(router.resolve(tab.fullPath).meta.title || tab.meta.title),
          ),
        ),
    );
    await router.replace({
      path: router.currentRoute.value.path,
      query: router.currentRoute.value.query,
      hash: router.currentRoute.value.hash,
      force: true,
    });
    applyExternalMenus();
  },
);

watch(
  () =>
    [
      preferences.app.isMobile,
      preferences.sidebar.collapsed,
      preferences.sidebar.expandOnHover,
      preferences.sidebar.width,
    ] as const,
  ([isMobile, collapsed, expandOnHover, width]) => {
    // Vben owns the temporary mobile drawer state. Forcing the desktop value
    // while mobile immediately re-opened the drawer after its close event and
    // allowed it to cover page content.
    if (isMobile) return;
    if (!collapsed && expandOnHover && width === 204) return;
    updatePreferences({
      sidebar: { collapsed: false, expandOnHover: true, width: 204 },
    });
  },
  { immediate: true },
);

onMounted(() => {
  void loadExternalMenus();
  assistanceClock = setInterval(() => {
    assistanceNow.value = Date.now();
    const expiresAt = authStore.customerAssistanceSession?.expiresAt ?? 0;
    if (expiresAt > 0 && expiresAt <= assistanceNow.value) {
      void authStore.clearSession(false);
    }
  }, 15_000);
});

onBeforeUnmount(() => {
  if (assistanceClock) clearInterval(assistanceClock);
});
</script>

<template>
  <BasicLayout
    close-mobile-sidebar-on-select
    lock-sidebar-menu-open
    @clear-preferences-and-logout="handleLogout"
  >
    <template v-if="authStore.customerAssistanceSession" #header-left-60>
      <div class="assistance-banner" data-testid="customer-assistance-banner">
        <span class="assistance-banner__dot" aria-hidden="true"></span>
        <span class="hidden sm:inline">
          {{
            $t('assistance.banner', {
              customer: authStore.customerAssistanceSession.customerName,
              minutes: assistanceRemainingMinutes,
            })
          }}
        </span>
        <span class="sm:hidden">
          {{
            $t('assistance.bannerMobile', {
              minutes: assistanceRemainingMinutes,
            })
          }}
        </span>
      </div>
    </template>
    <template #user-dropdown>
      <UserDropdown
        :avatar
        :menus="accountMenus"
        :text="displayRealName"
        :description="userStore.userInfo?.email || ''"
        :tag-text="userStore.userInfo?.username"
        @logout="handleLogout"
      />
    </template>
    <template #extra>
      <AuthenticationLoginExpiredModal
        v-model:open="accessStore.loginExpired"
        :avatar
      >
        <LoginForm />
      </AuthenticationLoginExpiredModal>
    </template>
    <template #lock-screen>
      <LockScreen :avatar @to-login="handleLogout" />
    </template>
  </BasicLayout>
</template>

<style scoped>
.assistance-banner {
  display: inline-flex;
  height: 30px;
  max-width: min(520px, 45vw);
  align-items: center;
  gap: 8px;
  overflow: hidden;
  border: 1px solid #ffb65d;
  border-radius: 0;
  background: #fff7e8;
  padding: 0 10px;
  color: #873800;
  font-size: 12px;
  font-weight: 500;
  line-height: 28px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.assistance-banner__dot {
  width: 7px;
  height: 7px;
  flex: none;
  background: #f77234;
}

@media (max-width: 639px) {
  .assistance-banner {
    max-width: 128px;
    padding: 0 8px;
  }
}
</style>

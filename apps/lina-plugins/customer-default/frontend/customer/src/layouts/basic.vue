<script lang="ts" setup>

import type { CustomerExternalMenu } from '#/api/idc/external-menu';

import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import { AuthenticationLoginExpiredModal } from '@vben/common-ui';
import { useWatermark } from '@vben/hooks';
import { BasicLayout, LockScreen, UserDropdown } from '@vben/layouts';
import { preferences, updatePreferences } from '@vben/preferences';
import { useAccessStore, useTabbarStore, useUserStore } from '@vben/stores';

import { listCustomerExternalMenus } from '#/api/idc/external-menu';
import { listCustomerProductDirectories, type CustomerProductDirectory } from '#/api/idc/product-directories';
import { $t } from '#/locales';
import { refreshAccessibleState } from '#/router/access-refresh';
import { useAuthStore } from '#/store';
import LoginForm from '#/views/_core/authentication/login.vue';
import CustomerAnnouncements from '#/components/customer-announcements.vue';

const userStore = useUserStore();
const authStore = useAuthStore();
const accessStore = useAccessStore();
const router = useRouter();
const { destroyWatermark, updateWatermark } = useWatermark();
const externalMenus = ref<CustomerExternalMenu[]>([]);
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
]);

async function handleLogout() {
  await authStore.logout(false);
}

const memberNavigation = ref<CustomerProductDirectory[]>([]);
function applyExternalMenus() {
  const useChinese = preferences.app.locale === 'zh-CN';
  const managed = memberNavigation.value.filter((item) => item.targetUrl !== '/account/notifications').map((item) => ({
    icon: item.icon || 'lucide:folder',
    name: useChinese ? item.nameZhCn || item.nameEnUs : item.nameEnUs || item.nameZhCn,
    order: item.sortOrder,
    path: item.navigationType && item.navigationType !== 'product' ? item.targetUrl || '/account' : '/account/product-directories/' + encodeURIComponent(item.directoryNo),
  }));
  const links = externalMenus.value.map((item) => ({icon:item.icon,name:useChinese?item.nameZh:item.nameEn,order:item.sortOrder,path:item.url}));
  // The successful server snapshot is authoritative, including an intentionally empty tree.
  accessStore.setAccessMenus([...managed,...links]);
}
async function loadExternalMenus() {
 try {
  const [first, links] = await Promise.all([listCustomerProductDirectories(1,100),listCustomerExternalMenus().catch(() => ({items:[]}))]);
  const all=[...first.items];
  for(let page=2;page<=Math.ceil(first.total/100);page+=1){const result=await listCustomerProductDirectories(page,100);all.push(...result.items);}
  memberNavigation.value=all;
  externalMenus.value=(links.items||[]).filter((item)=>item.enabled && /^https?:/.test(item.url));
  applyExternalMenus();
 } catch { /* Preserve working built-in access when navigation is temporarily unavailable. */ }
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
    <template #header-right-59>
      <CustomerAnnouncements />
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

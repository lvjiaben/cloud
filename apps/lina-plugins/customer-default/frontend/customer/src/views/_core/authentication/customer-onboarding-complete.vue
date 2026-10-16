<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { useAccessStore, useUserStore } from '@vben/stores';

import { Button, Result, Spin } from 'ant-design-vue';

import {
  getCustomerOnboardingStatus,
  getOrCreateRegistrationDeviceID,
  redeemCustomerOnboarding,
} from '#/api/idc/customer-onboarding';
import { $t } from '#/locales';

defineOptions({ name: 'CustomerOnboardingComplete' });

const tokenKey = 'linapro.customer.onboarding.token';
const browserKey = 'linapro.customer.onboarding.browser';
const userKey = 'linapro.customer.onboarding.user';
const expiryKey = 'linapro.customer.onboarding.expires';

const router = useRouter();
const accessStore = useAccessStore();
const userStore = useUserStore();
const state = ref<'failed' | 'ready' | 'redeeming' | 'success'>('ready');
const loggedIn = computed(() => Boolean(accessStore.accessToken));

function clearIntent() {
  for (const key of [tokenKey, browserKey, userKey, expiryKey]) {
    sessionStorage.removeItem(key);
  }
}

async function complete() {
  if (!loggedIn.value) {
    state.value = 'ready';
    return;
  }
  const token = sessionStorage.getItem(tokenKey)?.trim() || '';
  const browserNonce = sessionStorage.getItem(browserKey)?.trim() || '';
  const expiresAt = Number(sessionStorage.getItem(expiryKey) || 0);
  const registeredUserID = Number(sessionStorage.getItem(userKey) || 0);
  const currentUserID = Number(userStore.userInfo?.userId || 0);
  if (
    token.length !== 43 ||
    browserNonce.length < 16 ||
    !currentUserID ||
    (registeredUserID > 0 && registeredUserID !== currentUserID) ||
    (expiresAt > 0 && expiresAt <= Date.now())
  ) {
    clearIntent();
    state.value = 'failed';
    return;
  }
  state.value = 'redeeming';
  try {
    const status = await getCustomerOnboardingStatus();
    if (!status.required && status.customerNo) {
      clearIntent();
      state.value = 'success';
      return;
    }
    await redeemCustomerOnboarding({
      browserNonce,
      deviceId: getOrCreateRegistrationDeviceID(),
      hostRegistrationUserId: currentUserID,
      token,
    });
    clearIntent();
    state.value = 'success';
  } catch {
    state.value = 'failed';
  }
}

function goToLogin() {
  void router.push({
    path: '/auth/login',
    query: {
      redirect: encodeURIComponent('/auth/customer-onboarding-complete'),
    },
  });
}

function enterCustomerCenter() {
  window.location.assign('/account');
}

onMounted(complete);
</script>

<template>
  <section class="onboarding-card" data-testid="customer-onboarding-complete">
    <div v-if="state === 'redeeming'" class="onboarding-loading" role="status">
      <Spin size="large" />
      <h1>{{ $t('customerOnboarding.redeemingTitle') }}</h1>
      <p>{{ $t('customerOnboarding.redeemingDescription') }}</p>
    </div>
    <Result
      v-else-if="state === 'success'"
      status="success"
      :sub-title="$t('customerOnboarding.successDescription')"
      :title="$t('customerOnboarding.successTitle')"
    >
      <template #extra>
        <Button type="primary" @click="enterCustomerCenter">
          {{ $t('customerOnboarding.enter') }}
        </Button>
      </template>
    </Result>
    <Result
      v-else-if="state === 'failed'"
      status="warning"
      :sub-title="$t('customerOnboarding.failedDescription')"
      :title="$t('customerOnboarding.failedTitle')"
    >
      <template #extra>
        <Button type="primary" @click="goToLogin">
          {{ $t('customerOnboarding.loginAgain') }}
        </Button>
      </template>
    </Result>
    <Result
      v-else
      status="info"
      :sub-title="$t('customerOnboarding.readyDescription')"
      :title="$t('customerOnboarding.readyTitle')"
    >
      <template #extra>
        <Button type="primary" @click="goToLogin">
          {{ $t('customerOnboarding.login') }}
        </Button>
      </template>
    </Result>
  </section>
</template>

<style scoped>
.onboarding-card {
  width: min(100%, 620px);
  border: 1px solid hsl(var(--border));
  border-radius: 12px;
  background: hsl(var(--card));
  box-shadow: 0 18px 50px rgb(15 23 42 / 8%);
}
.onboarding-loading {
  display: grid;
  min-height: 360px;
  place-items: center;
  align-content: center;
  gap: 16px;
  padding: 48px 28px;
  text-align: center;
}
.onboarding-loading h1 { margin: 8px 0 0; font-size: 24px; font-weight: 650; }
.onboarding-loading p { margin: 0; color: hsl(var(--muted-foreground)); }
</style>

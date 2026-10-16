<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { useAccessStore } from '@vben/stores';

import { Button, Result, Spin } from 'ant-design-vue';

import { redeemCustomerActivation } from '#/api/idc/customer-activation';
import { $t } from '#/locales';

defineOptions({ name: 'CustomerActivation' });

const tokenStorageKey = 'linapro.customer.activation.token';
const route = useRoute();
const router = useRouter();
const accessStore = useAccessStore();
const state = ref<'failed' | 'ready' | 'redeeming' | 'success'>('ready');

const loggedIn = computed(() => Boolean(accessStore.accessToken));

function readToken() {
  const queryToken =
    typeof route.query.token === 'string' ? route.query.token.trim() : '';
  if (queryToken) {
    sessionStorage.setItem(tokenStorageKey, queryToken);
  }
  return queryToken || sessionStorage.getItem(tokenStorageKey)?.trim() || '';
}

function stripSensitiveQuery() {
  if (!('token' in route.query)) return;
  const safeUrl = new URL(window.location.href);
  safeUrl.searchParams.delete('token');
  window.history.replaceState(
    window.history.state,
    '',
    `${safeUrl.pathname}${safeUrl.search}${safeUrl.hash}`,
  );
}

async function redeem() {
  const token = readToken();
  stripSensitiveQuery();
  if (token.length < 32 || token.length > 128) {
    sessionStorage.removeItem(tokenStorageKey);
    state.value = 'failed';
    return;
  }
  if (!loggedIn.value) {
    state.value = 'ready';
    return;
  }
  state.value = 'redeeming';
  try {
    await redeemCustomerActivation(token);
    sessionStorage.removeItem(tokenStorageKey);
    state.value = 'success';
  } catch {
    state.value = 'failed';
  }
}

function authQuery() {
  return { redirect: encodeURIComponent('/auth/customer-activation') };
}

function goToLogin() {
  void router.push({ path: '/auth/login', query: authQuery() });
}

function goToRegister() {
  void router.push({ path: '/auth/register', query: authQuery() });
}

function enterCustomerCenter() {
  window.location.assign('/account');
}

onMounted(redeem);
</script>

<template>
  <section class="activation-card" data-testid="customer-activation-page">
    <div v-if="state === 'redeeming'" class="activation-loading" role="status">
      <Spin size="large" />
      <h1>{{ $t('customerActivation.redeemingTitle') }}</h1>
      <p>{{ $t('customerActivation.redeemingDescription') }}</p>
    </div>

    <Result
      v-else-if="state === 'success'"
      status="success"
      :sub-title="$t('customerActivation.successDescription')"
      :title="$t('customerActivation.successTitle')"
    >
      <template #extra>
        <Button
          data-testid="customer-activation-enter"
          type="primary"
          @click="enterCustomerCenter"
        >
          {{ $t('customerActivation.enter') }}
        </Button>
      </template>
    </Result>

    <Result
      v-else-if="state === 'failed'"
      status="warning"
      :sub-title="$t('customerActivation.failedDescription')"
      :title="$t('customerActivation.failedTitle')"
    >
      <template #extra>
        <Button
          data-testid="customer-activation-login-again"
          type="primary"
          @click="goToLogin"
        >
          {{ $t('customerActivation.useAnotherAccount') }}
        </Button>
      </template>
    </Result>

    <Result
      v-else
      status="info"
      :sub-title="$t('customerActivation.readyDescription')"
      :title="$t('customerActivation.readyTitle')"
    >
      <template #extra>
        <div class="activation-actions">
          <Button
            data-testid="customer-activation-login"
            type="primary"
            @click="goToLogin"
          >
            {{ $t('customerActivation.login') }}
          </Button>
          <Button
            data-testid="customer-activation-register"
            @click="goToRegister"
          >
            {{ $t('customerActivation.register') }}
          </Button>
        </div>
      </template>
    </Result>
  </section>
</template>

<style scoped>
.activation-card {
  width: min(100%, 620px);
  border: 1px solid hsl(var(--border));
  border-radius: 12px;
  background: hsl(var(--card));
  box-shadow: 0 18px 50px rgb(15 23 42 / 8%);
}

.activation-loading {
  display: grid;
  min-height: 360px;
  place-items: center;
  align-content: center;
  gap: 16px;
  padding: 48px 28px;
  text-align: center;
}

.activation-loading h1 {
  margin: 8px 0 0;
  font-size: 24px;
  font-weight: 650;
}

.activation-loading p {
  margin: 0;
  color: hsl(var(--muted-foreground));
}

.activation-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
}
</style>

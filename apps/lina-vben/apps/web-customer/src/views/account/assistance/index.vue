<script lang="ts" setup>
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

import { Button, Result, Spin } from 'ant-design-vue';

import { verifyCustomerAssistance } from '#/api/idc/customer-assistance';
import { $t } from '#/locales';
import { useAuthStore } from '#/store';

const route = useRoute();
const authStore = useAuthStore();
const loading = ref(true);
const unavailable = ref(false);

function consumeLaunchToken() {
  const value = typeof route.query.token === 'string' ? route.query.token : '';
  window.history.replaceState(window.history.state, '', '/account/assistance');
  return /^[A-Za-z0-9_-]{32,128}$/u.test(value) ? value : '';
}

async function exchange() {
  const launchToken = consumeLaunchToken();
  if (!launchToken) {
    loading.value = false;
    unavailable.value = true;
    return;
  }
  try {
    const result = await verifyCustomerAssistance(launchToken);
    if (!result.accessToken || !result.sessionNo) {
      throw new Error('customer assistance exchange is incomplete');
    }
    await authStore.completeCustomerAssistance(
      result.accessToken,
      { expiresAt: result.expiresAt, sessionNo: result.sessionNo },
      result.homePath || '/account',
    );
  } catch {
    unavailable.value = true;
  } finally {
    loading.value = false;
  }
}

function closePage() {
  window.close();
}

onMounted(() => void exchange());
</script>

<template>
  <main class="assistance-exchange" data-testid="customer-assistance-page">
    <div v-if="loading" class="assistance-state">
      <Spin size="large" />
      <p>{{ $t('assistance.loading') }}</p>
    </div>
    <Result
      v-else-if="unavailable"
      status="warning"
      :sub-title="$t('assistance.unavailableDescription')"
      :title="$t('assistance.unavailableTitle')"
    >
      <template #extra>
        <Button type="primary" @click="closePage">
          {{ $t('assistance.close') }}
        </Button>
      </template>
    </Result>
  </main>
</template>

<style scoped>
.assistance-exchange {
  display: grid;
  min-height: 100vh;
  place-items: center;
  background: #f2f3f5;
}

.assistance-state {
  display: grid;
  gap: 16px;
  justify-items: center;
  color: #4e5969;
}
</style>

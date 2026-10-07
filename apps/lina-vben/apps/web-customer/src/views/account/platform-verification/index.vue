<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';

import { Alert, Skeleton, TabPane, Tabs } from 'ant-design-vue';

import { getPersonalVerification } from '#/api/idc/personal-verification';
import { $t } from '#/locales';

const route = useRoute();
const router = useRouter();
const loading = ref(true);
const personalVerified = ref(false);
const loadFailed = ref(false);

const activeTab = computed({
  get: () => (route.path.endsWith('/enterprise') ? 'enterprise' : 'personal'),
  set: (value: string) => {
    const target =
      value === 'enterprise' && personalVerified.value
        ? '/account/verification/enterprise'
        : '/account/verification';
    void router.push(target);
  },
});

async function loadGate() {
  loading.value = true;
  loadFailed.value = false;
  try {
    const result = await getPersonalVerification();
    personalVerified.value = result.item?.statusCode === 'verified';
    if (!personalVerified.value && route.path.endsWith('/enterprise')) {
      await router.replace('/account/verification');
    }
  } catch {
    loadFailed.value = true;
    if (route.path.endsWith('/enterprise')) {
      await router.replace('/account/verification');
    }
  } finally {
    loading.value = false;
  }
}

onMounted(loadGate);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('identity.platform.title')">
    <div class="verification-center" data-testid="platform-verification-center">
      <Alert
        :description="$t('identity.platform.description')"
        :message="$t('identity.platform.personalFirst')"
        show-icon
        type="info"
      />
      <Alert
        v-if="loadFailed"
        :message="$t('identity.platform.gateFailed')"
        show-icon
        type="warning"
      />
      <Skeleton v-if="loading" active :paragraph="{ rows: 8 }" />
      <template v-else>
        <Tabs v-model:active-key="activeTab" class="verification-tabs">
          <TabPane key="personal" :tab="$t('identity.platform.personalTab')" />
          <TabPane
            key="enterprise"
            :disabled="!personalVerified"
            :tab="$t('identity.platform.enterpriseTab')"
          />
        </Tabs>
        <RouterView />
      </template>
    </div>
  </Page>
</template>

<style scoped>
.verification-center {
  display: grid;
  gap: 16px;
}

.verification-tabs {
  border: 1px solid hsl(var(--border));
  background: hsl(var(--card));
  padding-inline: 20px;
}
</style>

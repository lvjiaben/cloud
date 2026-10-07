<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';

import { Alert, TabPane, Tabs } from 'ant-design-vue';

import { getCustomerProfile } from '#/api/idc/customer-account';
import { $t } from '#/locales';

const route = useRoute();
const router = useRouter();
const enterpriseCustomer = ref(false);

const activeTab = computed({
  get: () => (route.path.endsWith('/signers') ? 'signers' : 'contracts'),
  set: (value: string) => {
    void router.push(
      value === 'signers' && enterpriseCustomer.value
        ? '/account/contracts/signers'
        : '/account/contracts',
    );
  },
});

async function loadProfile() {
  try {
    const result = await getCustomerProfile();
    enterpriseCustomer.value = result.item.type === 'enterprise';
    if (!enterpriseCustomer.value && route.path.endsWith('/signers')) {
      await router.replace('/account/contracts');
    }
  } catch {
    enterpriseCustomer.value = false;
  }
}

onMounted(loadProfile);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('contracts.center.title')">
    <div class="contract-center" data-testid="customer-contract-center">
      <Tabs v-model:active-key="activeTab" class="contract-tabs">
        <TabPane key="contracts" :tab="$t('contracts.center.contracts')" />
        <TabPane
          key="signers"
          :disabled="!enterpriseCustomer"
          :tab="$t('contracts.center.signers')"
        />
      </Tabs>
      <Alert
        v-if="!enterpriseCustomer"
        :message="$t('contracts.center.enterpriseRequired')"
        show-icon
        type="info"
      />
      <RouterView />
    </div>
  </Page>
</template>

<style scoped>
.contract-center {
  display: grid;
  gap: 16px;
}

.contract-tabs {
  border: 1px solid hsl(var(--border));
  background: hsl(var(--card));
  padding-inline: 20px;
}
</style>

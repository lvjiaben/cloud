<script lang="ts" setup>
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';
import { Alert, Button, Card, Empty, Input, Spin, Typography } from 'ant-design-vue';

import {
  askCustomerAssistant,
  getCustomerAssistantStatus,
  type CustomerAssistantReply,
} from '#/api/idc/ai';
import { $t } from '#/locales';

const router = useRouter();
const prompt = ref('');
const loading = ref(false);
const checking = ref(true);
const enabled = ref(false);
const error = ref(false);
const reply = ref<CustomerAssistantReply>();

async function loadStatus() {
  checking.value = true;
  try {
    enabled.value = (await getCustomerAssistantStatus()).enabled;
  } catch {
    error.value = true;
  } finally {
    checking.value = false;
  }
}

async function ask() {
  const value = prompt.value.trim();
  if (!value || loading.value) return;
  loading.value = true;
  error.value = false;
  try {
    reply.value = await askCustomerAssistant(value);
  } catch {
    error.value = true;
  } finally {
    loading.value = false;
  }
}

function openNavigation() {
  if (reply.value?.navigation) void router.push(reply.value.navigation);
}

onMounted(loadStatus);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('aiAssistant.title')">
    <Spin :spinning="checking">
      <Alert v-if="error" class="mb-4" type="error" show-icon>
        {{ $t('aiAssistant.error') }}
      </Alert>
      <Card v-if="!checking && !enabled" :bordered="false">
        <Empty :description="$t('aiAssistant.disabled')" />
      </Card>
      <Card v-else :bordered="false">
        <Typography.Paragraph type="secondary">
          {{ $t('aiAssistant.description') }}
        </Typography.Paragraph>
        <Input.TextArea
          v-model:value="prompt"
          :auto-size="{ minRows: 3, maxRows: 8 }"
          :maxlength="2000"
          show-count
          :placeholder="$t('aiAssistant.placeholder')"
          @press-enter="(event: KeyboardEvent) => { if (!event.shiftKey) { event.preventDefault(); void ask(); } }"
        />
        <div class="mt-3 flex justify-end">
          <Button type="primary" :loading="loading" @click="ask">
            {{ $t('aiAssistant.ask') }}
          </Button>
        </div>
        <Card v-if="reply" class="mt-5" size="small">
          <Typography.Paragraph>{{ reply.answer }}</Typography.Paragraph>
          <Typography.Paragraph v-if="reply.dataAsOf" type="secondary">
            {{ $t('aiAssistant.dataAsOf', { value: reply.dataAsOf }) }}
          </Typography.Paragraph>
          <Typography.Paragraph v-if="reply.uncertainty" type="warning">
            {{ reply.uncertainty }}
          </Typography.Paragraph>
          <Button v-if="reply.navigation" type="link" class="px-0" @click="openNavigation">
            {{ $t('aiAssistant.openRelated') }}
          </Button>
        </Card>
      </Card>
    </Spin>
  </Page>
</template>

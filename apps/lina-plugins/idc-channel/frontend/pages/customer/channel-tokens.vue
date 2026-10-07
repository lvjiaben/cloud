<script lang="ts">
export const pluginPageMeta = {
  routePath: 'channel-tokens',
  title: 'Channel access tokens',
};
</script>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Col,
  Empty,
  Row,
  Space,
  Spin,
  Tag,
  message,
} from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { pluginApiPath, requestClient } from '#/api/request';
import { $t } from '#/locales';

type ChannelToken = {
  createdAt: number;
  expiresAt?: number;
  id: number;
  revokedAt?: number;
  scopes: string[];
  sourceCidrs: string[];
};

type TokenList = { list: ChannelToken[] };
type TokenCreated = { token: string };

// Resolve lazily so this page cannot call the shared request module while that
// module is still being initialized through the authentication store.
const apiPath = () =>
  pluginApiPath('idc-channel', 'account/channel/tokens');
const locale = ref(document.documentElement.lang || navigator.language || 'zh-CN');
const isEnglish = computed(() => locale.value.toLowerCase().startsWith('en'));
const text = (zh: string, en: string) => (isEnglish.value ? en : zh);
const loading = ref(true);
const submitting = ref(false);
const blocked = ref(false);
const tokens = ref<ChannelToken[]>([]);
const oneTimeToken = ref('');
const defaultScopes = [
  'catalog.read',
  'order.create',
  'order.read',
  'service.read',
  'settlement.read',
];

function scopeOptions() {
  return [
    { label: text('查看可售商品', 'View catalog'), value: 'catalog.read' },
    { label: text('创建订单', 'Create orders'), value: 'order.create' },
    { label: text('查看订单', 'View orders'), value: 'order.read' },
    { label: text('查看服务', 'View services'), value: 'service.read' },
    { label: text('查看结算', 'View settlement'), value: 'settlement.read' },
  ];
}

function tokenFormSchema() {
  return [
    {
      component: 'CheckboxGroup' as const,
      componentProps: { options: scopeOptions() },
      defaultValue: defaultScopes,
      fieldName: 'scopes',
      label: text('允许的操作', 'Allowed actions'),
      rules: z.array(z.string()).min(1),
    },
    {
      component: 'Textarea' as const,
      componentProps: { rows: 3 },
      defaultValue: '',
      fieldName: 'sourceCidrs',
      label: text('来源网段（每行一个，可留空）', 'Source CIDRs (one per line, optional)'),
    },
    {
      component: 'DatePicker' as const,
      componentProps: {
        format: 'YYYY-MM-DD HH:mm',
        showTime: true,
        valueFormat: 'x',
      },
      defaultValue: '',
      fieldName: 'expiresAt',
      label: text('到期时间（可留空）', 'Expiry time (optional)'),
    },
  ];
}

const [TokenForm, tokenFormApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    formItemClass: 'col-span-1',
  },
  layout: 'vertical',
  schema: tokenFormSchema(),
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1',
});

const splitLines = (value: string) =>
  value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);

const formatTime = (value?: number) =>
  value ? new Date(value).toLocaleString(locale.value) : text('不限', 'No limit');

function formatScopes(scopes: string[]) {
  const labels = new Map(scopeOptions().map((item) => [item.value, item.label]));
  const values = scopes.map((scope) => labels.get(scope) ?? text('其他已授权操作', 'Other authorized action'));
  return [...new Set(values)].join(text('、', ', '));
}

async function loadTokens() {
  loading.value = true;
  try {
    const result = await requestClient.get<TokenList>(apiPath(), {
      silentErrorMessage: true,
    });
    tokens.value = result.list ?? [];
    blocked.value = false;
  } catch {
    blocked.value = true;
    tokens.value = [];
  } finally {
    loading.value = false;
  }
}

async function createToken() {
  const { valid } = await tokenFormApi.validate();
  if (!valid) return;
  const form = await tokenFormApi.getValues<{
    expiresAt?: string;
    scopes?: string[];
    sourceCidrs?: string;
  }>();
  submitting.value = true;
  try {
    const result = await requestClient.post<TokenCreated>(
      apiPath(),
      {
        expiresAt: form.expiresAt ? Number(form.expiresAt) : undefined,
        scopes: form.scopes ?? [],
        sourceCidrs: splitLines(form.sourceCidrs ?? ''),
      },
      { silentErrorMessage: true },
    );
    oneTimeToken.value = result.token;
    message.success($t('plugin.idc-channel.customerTokenCreated'));
    await loadTokens();
  } catch {
    message.error($t('plugin.idc-channel.customerTokenCreateFailed'));
  } finally {
    submitting.value = false;
  }
}

async function revokeToken(id: number) {
  try {
    await requestClient.delete(`${apiPath()}/${id}`, {
      silentErrorMessage: true,
    });
    message.success($t('plugin.idc-channel.customerTokenRevoked'));
    await loadTokens();
  } catch {
    message.error($t('plugin.idc-channel.customerTokenRevokeFailed'));
  }
}

const localeObserver = new MutationObserver(() => {
  locale.value = document.documentElement.lang || 'zh-CN';
  void tokenFormApi.setState({ schema: tokenFormSchema() });
});

onMounted(() => {
  localeObserver.observe(document.documentElement, {
    attributeFilter: ['lang'],
    attributes: true,
  });
  void loadTokens();
});
onBeforeUnmount(() => localeObserver.disconnect());
</script>

<template>
  <Page :auto-content-height="true" data-testid="channel-customer-token-page">
    <Card :bordered="false" class="channel-token-page">
      <div class="page-heading">
        <div>
          <h1>{{ text('渠道访问令牌', 'Channel access tokens') }}</h1>
          <p>
            {{
              text(
                '仅商家已批准且完成实名的企业渠道可用。令牌原文只在创建时显示一次。',
                'Available only to merchant-approved, identity-verified enterprise channels. A token is shown only once when created.',
              )
            }}
          </p>
        </div>
        <Button @click="loadTokens">{{ text('刷新', 'Refresh') }}</Button>
      </div>

      <Alert
        v-if="blocked"
        show-icon
        type="warning"
        :message="text('此页面只对获批企业渠道开放', 'Available only to approved enterprise channels')"
        :description="text('普通个人、普通企业、待激活客户和未获批渠道不能创建或查看渠道令牌。', 'Individuals, ordinary enterprises, pending customers, and unapproved channels cannot manage channel tokens.')"
      />

      <Spin v-else :spinning="loading">
        <Row :gutter="16">
          <Col :lg="12" :xs="24">
            <Card size="small" :title="text('创建令牌', 'Create token')">
              <TokenForm />
              <Button data-testid="create-own-token" html-type="button" type="primary" :loading="submitting" @click="createToken">
                {{ text('创建并显示一次', 'Create and show once') }}
              </Button>
              <div class="secret-box" data-testid="one-time-token">
                {{ oneTimeToken || text('创建后在这里显示一次', 'The new token will appear here once') }}
              </div>
            </Card>
          </Col>

          <Col :lg="12" :xs="24">
            <Card size="small" :title="text('现有令牌', 'Existing tokens')">
              <p class="muted">{{ text('这里不会回显令牌原文。', 'Stored token values are never shown again.') }}</p>
              <Empty v-if="!tokens.length" :description="text('暂无令牌', 'No tokens')" />
              <Space v-else direction="vertical" size="middle" class="token-list">
                <div v-for="token in tokens" :key="token.id" class="token-item">
                  <div class="token-item__head">
                    <strong>{{ text('访问令牌', 'Access token') }}</strong>
                    <Tag :color="token.revokedAt ? 'default' : 'success'">
                      {{ token.revokedAt ? text('已撤销', 'Revoked') : text('有效', 'Active') }}
                    </Tag>
                  </div>
                  <div>{{ text('允许的操作', 'Allowed actions') }}：{{ formatScopes(token.scopes) }}</div>
                  <div>{{ text('来源', 'Sources') }}：{{ token.sourceCidrs.join(', ') || text('不限', 'No limit') }}</div>
                  <div>{{ text('创建', 'Created') }}：{{ formatTime(token.createdAt) }}</div>
                  <div>{{ text('到期', 'Expires') }}：{{ formatTime(token.expiresAt) }}</div>
                  <Button v-if="!token.revokedAt" danger size="small" :data-testid="`revoke-own-token-${token.id}`" @click="revokeToken(token.id)">
                    {{ text('撤销', 'Revoke') }}
                  </Button>
                </div>
              </Space>
            </Card>
          </Col>
        </Row>
      </Spin>
    </Card>
  </Page>
</template>

<style scoped>
.channel-token-page {
  min-width: 0;
  min-height: 100%;
}

.channel-token-page :deep(.ant-card-body) {
  padding: 16px;
}

.channel-token-page :deep(.ant-row) {
  row-gap: 16px;
}

.channel-token-page :deep(.ant-col) {
  min-width: 0;
}

.page-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.page-heading > div {
  min-width: 0;
}

.page-heading h1 {
  margin: 0 0 6px;
  font-size: 18px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.page-heading p,
.muted,
.token-item div {
  color: hsl(var(--muted-foreground));
}

.page-heading p,
.muted {
  margin: 0;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.secret-box {
  min-height: 46px;
  margin-top: 16px;
  padding: 11px 12px;
  border-radius: 0;
  background: hsl(var(--muted));
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  overflow-wrap: anywhere;
}

.token-list {
  width: 100%;
}

.token-item {
  display: grid;
  min-width: 0;
  gap: 8px;
  padding: 12px 16px;
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  overflow-wrap: anywhere;
}

.token-item__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 16px;
}

@media (max-width: 991px) {
  .page-heading {
    flex-direction: column;
  }
}
</style>

<script lang="ts" setup>
import type { SSHPublicKeyItem } from '#/api/idc/ssh-public-keys';

import { Page, useVbenModal } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  message,
  Popconfirm,
  Tag,
} from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { useVbenForm, z } from '#/adapter/form';
import {
  createSSHPublicKeyApi,
  listSSHPublicKeysApi,
  revokeSSHPublicKeyApi,
} from '#/api/idc/ssh-public-keys';
import { $t } from '#/locales';

const [CreateForm, createFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: 'w-full' }, labelWidth: 110 },
  schema: [
    {
      component: 'Input',
      componentProps: { autocomplete: 'off', maxlength: 80 },
      fieldName: 'name',
      label: $t('sshKeys.fields.name'),
      rules: z.string().trim().min(1).max(80),
    },
    {
      component: 'Textarea',
      componentProps: {
        'auto-size': { maxRows: 10, minRows: 5 },
        autocomplete: 'off',
        maxlength: 16_384,
        placeholder: $t('sshKeys.create.placeholder'),
      },
      fieldName: 'authorizedKey',
      label: $t('sshKeys.fields.publicKey'),
      rules: z.string().trim().min(1).max(16_384),
    },
  ],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1',
});

const [CreateModal, createModalApi] = useVbenModal({
  cancelText: $t('sshKeys.actions.cancel'),
  confirmText: $t('sshKeys.actions.save'),
  onClosed: () => createFormApi.resetForm(),
  onConfirm: createKey,
  title: $t('sshKeys.create.title'),
});

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: [
      { field: 'name', minWidth: 160, title: $t('sshKeys.fields.name') },
      {
        field: 'fingerprint',
        minWidth: 300,
        slots: { default: 'fingerprint' },
        title: $t('sshKeys.fields.fingerprint'),
      },
      {
        field: 'algorithm',
        minWidth: 140,
        title: $t('sshKeys.fields.algorithm'),
      },
      {
        field: 'status',
        minWidth: 110,
        slots: { default: 'status' },
        title: $t('sshKeys.fields.status'),
      },
      {
        field: 'actions',
        fixed: 'right',
        slots: { default: 'actions' },
        title: $t('sshKeys.fields.actions'),
        width: 120,
      },
    ],
    height: '100%',
    pagerConfig: { enabled: false },
    proxyConfig: {
      ajax: {
        query: async () => {
          const response = await listSSHPublicKeysApi();
          const items = response.items || [];
          return { items, total: items.length };
        },
      },
    },
    rowConfig: { keyField: 'keyNo' },
    showOverflow: true,
    id: 'customer-ssh-public-keys',
  },
});

function openCreate() {
  createFormApi.resetForm();
  createModalApi.open();
}

async function createKey() {
  const { valid } = await createFormApi.validate();
  if (!valid) return false;
  const values = await createFormApi.getValues<{
    authorizedKey: string;
    name: string;
  }>();
  const name = values.name.trim();
  const authorizedKey = values.authorizedKey.trim();
  if (/PRIVATE KEY|PUTTY-USER-KEY-FILE-/i.test(authorizedKey)) {
    message.error($t('sshKeys.messages.privateRejected'));
    return false;
  }
  createModalApi.lock(true);
  try {
    await createSSHPublicKeyApi({
      authorizedKey,
      name,
      requestNo: `ssh-key-${crypto.randomUUID()}`,
    });
    message.success($t('sshKeys.messages.created'));
    createModalApi.close();
    await gridApi.query();
  } finally {
    createModalApi.lock(false);
  }
}

async function revoke(record: Record<string, any>) {
  const item = record as SSHPublicKeyItem;
  await revokeSSHPublicKeyApi(item.keyNo, {
    expectedVersion: item.version,
    requestNo: `ssh-revoke-${crypto.randomUUID()}`,
  });
  message.success($t('sshKeys.messages.revoked'));
  await gridApi.query();
}
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :auto-content-height="true" :title="$t('sshKeys.title')">
    <Card class="ssh-key-card h-full" data-testid="customer-ssh-keys-page">
      <Alert
        class="mb-4"
        show-icon
        type="info"
        :message="$t('sshKeys.notice.title')"
        :description="$t('sshKeys.notice.description')"
      />
      <div class="mb-4 flex justify-end">
        <Button data-testid="ssh-key-create" type="primary" @click="openCreate">
          {{ $t('sshKeys.actions.add') }}
        </Button>
      </div>
      <Grid data-testid="ssh-key-table">
        <template #fingerprint="{ row }">
          <code>{{ row.fingerprint }}</code>
        </template>
        <template #status="{ row }">
          <Tag :color="row.status === 'active' ? 'green' : 'default'">
            {{ $t(`sshKeys.status.${row.status}`) }}
          </Tag>
        </template>
        <template #actions="{ row }">
          <Popconfirm
            v-if="row.status === 'active'"
            :title="$t('sshKeys.confirm.revoke')"
            :ok-text="$t('sshKeys.actions.revoke')"
            :cancel-text="$t('sshKeys.actions.cancel')"
            @confirm="revoke(row)"
          >
            <Button danger type="link">
              {{ $t('sshKeys.actions.revoke') }}
            </Button>
          </Popconfirm>
        </template>
      </Grid>
    </Card>

    <CreateModal data-testid="ssh-key-create-modal">
      <Alert
        class="mb-4"
        show-icon
        type="warning"
        :message="$t('sshKeys.create.warning')"
      />
      <CreateForm />
    </CreateModal>
  </Page>
</template>

<style scoped>
.ssh-key-card :deep(.ant-card-body) {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.ssh-key-card :deep(.vxe-grid) {
  flex: 1;
  min-height: 320px;
}
</style>

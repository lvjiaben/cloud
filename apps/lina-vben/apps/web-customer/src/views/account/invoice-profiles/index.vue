<script setup lang="ts">
import type {
  BankAccountAction,
  InvoiceProfileItem,
  InvoiceProfileTitleType,
} from '#/api/idc/invoice-profiles';

import { computed, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Empty,
  message,
  Modal as AntModal,
  Space,
  Spin,
  Tag,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  createInvoiceProfile,
  deactivateInvoiceProfile,
  listInvoiceProfiles,
  setDefaultInvoiceProfile,
  updateInvoiceProfile,
} from '#/api/idc/invoice-profiles';
import { $t } from '#/locales';
import { useVbenForm } from '#/adapter/form';
import VbenModalCompat from '#/components/vben-modal-compat.vue';

const loading = ref(false);
const loadingMore = ref(false);
const loadFailed = ref(false);
const items = ref<InvoiceProfileItem[]>([]);
const nextCursor = ref('');
const editorOpen = ref(false);
const saving = ref(false);
const editing = ref<InvoiceProfileItem>();
const actionBusy = ref('');

const form = reactive({
  bankAccount: '',
  bankAccountAction: 'keep' as BankAccountAction,
  bankName: '',
  invoiceTitle: '',
  isDefault: false,
  registeredAddress: '',
  registeredPhone: '',
  taxRegistrationNo: '',
  titleTypeCode: 'enterprise' as InvoiceProfileTitleType,
});

const isEnterprise = computed(() => form.titleTypeCode === 'enterprise');
const isEditing = computed(() => Boolean(editing.value));
const hasSavedBankAccount = computed(
  () => editing.value?.bankAccountConfigured === true,
);
const canSave = computed(() => {
  if (!form.invoiceTitle.trim()) return false;
  if (isEnterprise.value && !form.taxRegistrationNo.trim()) return false;
  if (form.bankAccountAction === 'replace' && !form.bankAccount.trim()) {
    return false;
  }
  return true;
});

const [InvoiceForm, invoiceFormApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    labelWidth: 128,
  },
  handleValuesChange(values) {
    form.bankAccount = String(values.bankAccount || '');
    form.bankAccountAction = (values.bankAccountAction || 'keep') as BankAccountAction;
    form.bankName = String(values.bankName || '');
    form.invoiceTitle = String(values.invoiceTitle || '');
    form.isDefault = Boolean(values.isDefault);
    form.registeredAddress = String(values.registeredAddress || '');
    form.registeredPhone = String(values.registeredPhone || '');
    form.taxRegistrationNo = String(values.taxRegistrationNo || '');
    form.titleTypeCode = (values.titleTypeCode || 'enterprise') as InvoiceProfileTitleType;
  },
  schema: [
    {
      component: 'RadioGroup',
      componentProps: {
        buttonStyle: 'solid',
        onChange: (event: { target?: { value?: InvoiceProfileTitleType } }) =>
          changeTitleType(event?.target?.value || 'enterprise'),
        optionType: 'button',
        options: [
          { label: $t('invoiceProfiles.type.enterprise'), value: 'enterprise' },
          { label: $t('invoiceProfiles.type.individual'), value: 'individual' },
        ],
      },
      fieldName: 'titleTypeCode',
      label: $t('invoiceProfiles.form.type'),
      rules: 'selectRequired',
    },
    {
      component: 'Input',
      componentProps: {
        'data-testid': 'invoice-profile-title',
        maxlength: 512,
      },
      fieldName: 'invoiceTitle',
      label: $t('invoiceProfiles.form.title'),
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: {
        'data-testid': 'invoice-profile-tax-no',
        maxlength: 128,
      },
      dependencies: {
        required: (values) => values.titleTypeCode === 'enterprise',
        show: (values) => values.titleTypeCode === 'enterprise',
        triggerFields: ['titleTypeCode'],
      },
      fieldName: 'taxRegistrationNo',
      label: $t('invoiceProfiles.form.taxNo'),
    },
    {
      component: 'Input',
      componentProps: {
        'data-testid': 'invoice-profile-address',
        maxlength: 512,
      },
      dependencies: {
        show: (values) => values.titleTypeCode === 'enterprise',
        triggerFields: ['titleTypeCode'],
      },
      fieldName: 'registeredAddress',
      label: $t('invoiceProfiles.form.address'),
    },
    {
      component: 'Input',
      componentProps: {
        'data-testid': 'invoice-profile-phone',
        maxlength: 32,
      },
      dependencies: {
        show: (values) => values.titleTypeCode === 'enterprise',
        triggerFields: ['titleTypeCode'],
      },
      fieldName: 'registeredPhone',
      label: $t('invoiceProfiles.form.phone'),
    },
    {
      component: 'Input',
      componentProps: {
        'data-testid': 'invoice-profile-bank-name',
        maxlength: 256,
      },
      dependencies: {
        show: (values) => values.titleTypeCode === 'enterprise',
        triggerFields: ['titleTypeCode'],
      },
      fieldName: 'bankName',
      label: $t('invoiceProfiles.form.bankName'),
    },
    {
      component: 'RadioGroup',
      componentProps: () => ({
        options: [
          { label: $t('invoiceProfiles.bankAction.keep'), value: 'keep' },
          { label: $t('invoiceProfiles.bankAction.replace'), value: 'replace' },
          {
            disabled: !hasSavedBankAccount.value,
            label: $t('invoiceProfiles.bankAction.clear'),
            value: 'clear',
          },
        ],
      }),
      dependencies: {
        show: (values) =>
          values.titleTypeCode === 'enterprise' && isEditing.value,
        triggerFields: ['titleTypeCode'],
      },
      fieldName: 'bankAccountAction',
      label: $t('invoiceProfiles.form.bankAction'),
      rules: 'selectRequired',
    },
    {
      component: 'InputPassword',
      componentProps: {
        autocomplete: 'new-password',
        'data-testid': 'invoice-profile-bank-account',
        maxlength: 128,
      },
      dependencies: {
        required: (values) =>
          isEditing.value && values.bankAccountAction === 'replace',
        show: (values) =>
          values.titleTypeCode === 'enterprise' &&
          (!isEditing.value || values.bankAccountAction === 'replace'),
        triggerFields: ['bankAccountAction', 'titleTypeCode'],
      },
      description: $t('invoiceProfiles.form.bankAccountHelp'),
      fieldName: 'bankAccount',
      label: $t('invoiceProfiles.form.bankAccount'),
    },
    {
      component: 'Checkbox',
      dependencies: {
        show: () => !isEditing.value,
        triggerFields: ['titleTypeCode'],
      },
      fieldName: 'isDefault',
      label: $t('invoiceProfiles.form.makeDefault'),
    },
  ],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1',
});

function randomValue(prefix: string) {
  const value =
    typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : Array.from(crypto.getRandomValues(new Uint8Array(24)), (item) =>
          item.toString(16).padStart(2, '0'),
        ).join('');
  return `${prefix}-${value}`;
}

function formatTime(value?: number) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—';
}

function replaceItem(item: InvoiceProfileItem) {
  const index = items.value.findIndex(
    (candidate) => candidate.profileNo === item.profileNo,
  );
  if (index >= 0) items.value[index] = item;
  else items.value.unshift(item);
}

async function load(reset = true) {
  if (reset ? loading.value : loadingMore.value) return;
  if (reset) loading.value = true;
  else loadingMore.value = true;
  loadFailed.value = false;
  try {
    const result = await listInvoiceProfiles({
      cursor: reset ? undefined : nextCursor.value,
      limit: 20,
    });
    items.value = reset
      ? result.items || []
      : [...items.value, ...(result.items || [])];
    nextCursor.value = result.nextCursor || '';
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
    loadingMore.value = false;
  }
}

function clearForm() {
  form.titleTypeCode = 'enterprise';
  form.invoiceTitle = '';
  form.taxRegistrationNo = '';
  form.registeredAddress = '';
  form.registeredPhone = '';
  form.bankName = '';
  form.bankAccount = '';
  form.bankAccountAction = 'keep';
  form.isDefault = false;
  invoiceFormApi.setValues({ ...form });
}

function openCreate() {
  editing.value = undefined;
  clearForm();
  refreshBankAccountSchema();
  editorOpen.value = true;
}

function openEdit(item: InvoiceProfileItem) {
  editing.value = item;
  form.titleTypeCode = item.titleTypeCode;
  form.invoiceTitle = item.invoiceTitle;
  form.taxRegistrationNo = item.taxRegistrationNo || '';
  form.registeredAddress = item.registeredAddress || '';
  form.registeredPhone = item.registeredPhone || '';
  form.bankName = item.bankName || '';
  form.bankAccount = '';
  form.bankAccountAction = 'keep';
  form.isDefault = item.isDefault;
  invoiceFormApi.setValues({ ...form });
  refreshBankAccountSchema();
  editorOpen.value = true;
}

// VbenForm evaluates dependency visibility while the schema is first mounted.
// Refresh the two bank-account fields after switching between create/edit so
// the edit-only action selector is rendered with the current profile state.
function refreshBankAccountSchema() {
  void invoiceFormApi.updateSchema([
    {
      dependencies: {
        show: (values) =>
          values.titleTypeCode === 'enterprise' && isEditing.value,
        triggerFields: ['titleTypeCode'],
      },
      fieldName: 'bankAccountAction',
    },
    {
      dependencies: {
        required: (values) =>
          isEditing.value && values.bankAccountAction === 'replace',
        show: (values) =>
          values.titleTypeCode === 'enterprise' &&
          (!isEditing.value || values.bankAccountAction === 'replace'),
        triggerFields: ['bankAccountAction', 'titleTypeCode'],
      },
      fieldName: 'bankAccount',
    },
  ]);
}

function changeTitleType(value: InvoiceProfileTitleType) {
  form.titleTypeCode = value;
  if (value === 'individual') {
    form.taxRegistrationNo = '';
    form.registeredAddress = '';
    form.registeredPhone = '';
    form.bankName = '';
    form.bankAccount = '';
    form.bankAccountAction = hasSavedBankAccount.value ? 'clear' : 'keep';
    invoiceFormApi.setValues({ ...form });
  }
}

async function save() {
  if (!canSave.value || saving.value) return;
  saving.value = true;
  try {
    if (editing.value) {
      const result = await updateInvoiceProfile(editing.value.profileNo, {
        bankAccount:
          form.bankAccountAction === 'replace'
            ? form.bankAccount.trim()
            : undefined,
        bankAccountAction: form.bankAccountAction,
        bankName: isEnterprise.value ? form.bankName.trim() : '',
        expectedVersion: editing.value.version,
        idempotencyKey: randomValue('invoice-profile-update'),
        invoiceTitle: form.invoiceTitle.trim(),
        registeredAddress: isEnterprise.value
          ? form.registeredAddress.trim()
          : '',
        registeredPhone: isEnterprise.value
          ? form.registeredPhone.trim()
          : '',
        taxRegistrationNo: isEnterprise.value
          ? form.taxRegistrationNo.trim()
          : '',
        titleTypeCode: form.titleTypeCode,
      });
      replaceItem(result.item);
      message.success($t('invoiceProfiles.messages.updated'));
    } else {
      const result = await createInvoiceProfile({
        bankAccount: isEnterprise.value
          ? form.bankAccount.trim() || undefined
          : undefined,
        bankName: isEnterprise.value ? form.bankName.trim() : '',
        idempotencyKey: randomValue('invoice-profile-create'),
        invoiceTitle: form.invoiceTitle.trim(),
        isDefault: form.isDefault,
        registeredAddress: isEnterprise.value
          ? form.registeredAddress.trim()
          : '',
        registeredPhone: isEnterprise.value
          ? form.registeredPhone.trim()
          : '',
        taxRegistrationNo: isEnterprise.value
          ? form.taxRegistrationNo.trim()
          : '',
        titleTypeCode: form.titleTypeCode,
      });
      await load();
      replaceItem(result.item);
      message.success($t('invoiceProfiles.messages.created'));
    }
    editorOpen.value = false;
  } finally {
    saving.value = false;
  }
}

async function setDefault(item: InvoiceProfileItem) {
  if (actionBusy.value) return;
  actionBusy.value = item.profileNo;
  try {
    await setDefaultInvoiceProfile(item.profileNo, item.version);
    await load();
    message.success($t('invoiceProfiles.messages.defaultSet'));
  } finally {
    actionBusy.value = '';
  }
}

function deactivate(item: InvoiceProfileItem) {
  AntModal.confirm({
    cancelText: $t('invoiceProfiles.deactivate.cancel'),
    content: item.isDefault
      ? $t('invoiceProfiles.deactivate.defaultDescription')
      : $t('invoiceProfiles.deactivate.description'),
    okText: $t('invoiceProfiles.deactivate.confirm'),
    onOk: async () => {
      actionBusy.value = item.profileNo;
      try {
        const result = await deactivateInvoiceProfile(
          item.profileNo,
          item.version,
        );
        replaceItem(result.item);
        message.success($t('invoiceProfiles.messages.deactivated'));
      } finally {
        actionBusy.value = '';
      }
    },
    title: $t('invoiceProfiles.deactivate.title'),
  });
}

onMounted(() => load());
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('invoiceProfiles.title')">
    <div class="profile-page" data-testid="invoice-profile-page">
      <Card :bordered="false" class="page-toolbar">
        <div class="page-toolbar-content">
          <div>
            <strong>{{ $t('invoiceProfiles.heading') }}</strong>
            <p>{{ $t('invoiceProfiles.subtitle') }}</p>
          </div>
          <Space wrap>
            <Button :loading="loading" @click="load()">
              {{ $t('invoiceProfiles.refresh') }}
            </Button>
            <Button data-testid="invoice-profile-add" type="primary" @click="openCreate">
              {{ $t('invoiceProfiles.add') }}
            </Button>
          </Space>
        </div>
      </Card>

      <Alert
        :description="$t('invoiceProfiles.securityDescription')"
        :message="$t('invoiceProfiles.securityTitle')"
        show-icon
        type="info"
      />
      <Alert v-if="loadFailed" :message="$t('invoiceProfiles.loadFailed')" show-icon type="error">
        <template #action>
          <Button size="small" @click="load()">{{ $t('invoiceProfiles.retry') }}</Button>
        </template>
      </Alert>

      <Spin :spinning="loading">
        <Empty
          v-if="!loading && !loadFailed && items.length === 0"
          :description="$t('invoiceProfiles.empty')"
        />
        <div v-else class="profile-grid" data-testid="invoice-profile-list">
          <Card
            v-for="item in items"
            :key="item.profileNo"
            :data-testid="`invoice-profile-${item.profileNo}`"
            class="profile-card"
            :class="{ 'profile-card--archived': item.statusCode === 'archived' }"
          >
            <div class="profile-heading">
              <div>
                <div class="profile-name">{{ item.invoiceTitle }}</div>
                <div class="profile-number">{{ item.profileNo }}</div>
              </div>
              <Space wrap>
                <Tag v-if="item.isDefault" color="blue">{{ $t('invoiceProfiles.default') }}</Tag>
                <Tag :color="item.statusCode === 'active' ? 'success' : 'default'">
                  {{ $t(`invoiceProfiles.status.${item.statusCode}`) }}
                </Tag>
              </Space>
            </div>

            <dl class="profile-facts">
              <div>
                <dt>{{ $t('invoiceProfiles.fields.type') }}</dt>
                <dd>{{ $t(`invoiceProfiles.type.${item.titleTypeCode}`) }}</dd>
              </div>
              <div v-if="item.titleTypeCode === 'enterprise'">
                <dt>{{ $t('invoiceProfiles.fields.taxNo') }}</dt>
                <dd>{{ item.taxRegistrationNo || '—' }}</dd>
              </div>
              <div v-if="item.titleTypeCode === 'enterprise'">
                <dt>{{ $t('invoiceProfiles.fields.bank') }}</dt>
                <dd>{{ item.bankName || '—' }}</dd>
              </div>
              <div v-if="item.titleTypeCode === 'enterprise'">
                <dt>{{ $t('invoiceProfiles.fields.bankAccount') }}</dt>
                <dd>{{ item.bankAccountMasked || $t('invoiceProfiles.notConfigured') }}</dd>
              </div>
              <div>
                <dt>{{ $t('invoiceProfiles.fields.updated') }}</dt>
                <dd>{{ formatTime(item.updatedAt) }}</dd>
              </div>
            </dl>

            <Space v-if="item.statusCode === 'active'" wrap>
              <Button data-testid="invoice-profile-edit" @click="openEdit(item)">
                {{ $t('invoiceProfiles.actions.edit') }}
              </Button>
              <Button
                v-if="!item.isDefault"
                :data-testid="`invoice-profile-default-${item.profileNo}`"
                :loading="actionBusy === item.profileNo"
                @click="setDefault(item)"
              >
                {{ $t('invoiceProfiles.actions.setDefault') }}
              </Button>
              <Button
                danger
                :data-testid="`invoice-profile-deactivate-${item.profileNo}`"
                :loading="actionBusy === item.profileNo"
                @click="deactivate(item)"
              >
                {{ $t('invoiceProfiles.actions.deactivate') }}
              </Button>
            </Space>
          </Card>
        </div>
      </Spin>

      <div v-if="nextCursor" class="load-more">
        <Button :loading="loadingMore" @click="load(false)">
          {{ $t('invoiceProfiles.loadMore') }}
        </Button>
      </div>
    </div>

    <VbenModalCompat
      v-model:open="editorOpen"
      :cancel-text="$t('invoiceProfiles.form.cancel')"
      :confirm-loading="saving"
      :ok-button-props="{ disabled: !canSave }"
      :ok-text="$t('invoiceProfiles.form.save')"
      :title="isEditing ? $t('invoiceProfiles.form.editTitle') : $t('invoiceProfiles.form.createTitle')"
      width="680px"
      @ok="save"
    >
      <div data-testid="invoice-profile-form">
        <InvoiceForm />
        <Alert
          v-if="isEditing && !isEnterprise && hasSavedBankAccount"
          :description="$t('invoiceProfiles.form.individualClearsBankDescription')"
          :message="$t('invoiceProfiles.form.individualClearsBankTitle')"
          show-icon
          type="warning"
        />
        <Alert
          v-else-if="isEditing && isEnterprise"
          :message="hasSavedBankAccount
            ? $t('invoiceProfiles.form.savedBank', { masked: editing?.bankAccountMasked || '****' })
            : $t('invoiceProfiles.form.noSavedBank')"
          show-icon
          type="info"
        />
      </div>
    </VbenModalCompat>
  </Page>
</template>

<style scoped>
.profile-page {
  display: grid;
  gap: 16px;
}

.page-toolbar {
  border: 1px solid color-mix(in srgb, hsl(var(--primary)) 24%, hsl(var(--border)));
}

.page-toolbar-content {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 24px;
}
.page-toolbar-content strong {
  display: block;
  margin-bottom: 6px;
  font-size: 18px;
}
.page-toolbar-content p {
  max-width: 720px;
  margin: 0;
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
}

.profile-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.profile-card {
  border-top: 3px solid hsl(var(--primary));
}

.profile-card--archived {
  border-top-color: hsl(var(--muted-foreground));
  opacity: 0.78;
}

.profile-heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 14px;
}

.profile-name {
  font-size: 18px;
  font-weight: 700;
}

.profile-number,
.field-help {
  margin-top: 4px;
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.profile-number {
  overflow-wrap: anywhere;
}

.profile-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 18px 0;
}

.profile-facts div {
  min-width: 0;
  padding: 11px;
  border: 1px solid hsl(var(--border));
  border-radius: 9px;
}

.profile-facts dt {
  margin-bottom: 4px;
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.profile-facts dd {
  margin: 0;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.load-more {
  display: flex;
  justify-content: center;
}

@media (max-width: 900px) {
  .profile-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .page-toolbar-content {
    align-items: stretch;
    flex-direction: column;
  }

  .profile-facts {
    grid-template-columns: 1fr;
  }
}
</style>

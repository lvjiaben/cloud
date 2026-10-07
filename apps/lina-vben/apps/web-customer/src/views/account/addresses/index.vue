<script setup lang="ts">
import type {
  CustomerAddress,
  CustomerAddressSaveInput,
  CustomerAddressType,
} from '#/api/idc/customer-account';

import { computed, onMounted, reactive, ref } from 'vue';

import { Page, useVbenModal } from '@vben/common-ui';
import { preferences } from '@vben/preferences';

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

import {
  archiveCustomerAddress,
  createCustomerAddress,
  createCustomerIdempotencyKey,
  listCustomerAddresses,
  updateCustomerAddress,
} from '#/api/idc/customer-account';
import { $t } from '#/locales';
import { useVbenForm } from '#/adapter/form';
import { mainlandRegionOptions } from '#/utils/address-region-options';
import { buildCountryOptions } from '#/utils/country-options';

const items = ref<CustomerAddress[]>([]);
const loading = ref(false);
const saving = ref(false);
const action = ref('');
const editing = ref<CustomerAddress>();
const regionPath = ref<string[]>([]);
const form = reactive({
  addressLine1: '',
  addressLine2: '',
  city: '',
  countryCode: 'CN',
  default: false,
  district: '',
  label: '',
  postalCode: '',
  province: '',
  recipientName: '',
  recipientPhone: '',
  type: 'contact' as CustomerAddressType,
});

const typeOptions = computed(() =>
  (['contact', 'billing', 'registered', 'service'] as const).map((value) => ({
    label: $t(`customerAddress.types.${value}`),
    value,
  })),
);
const countryOptions = computed(() =>
  buildCountryOptions(preferences.app.locale),
);
const usesMainlandRegions = computed(() => form.countryCode === 'CN');
const canSave = computed(
  () =>
    /^[A-Za-z]{2}$/.test(form.countryCode.trim()) &&
    form.addressLine1.trim().length > 0 &&
    (!usesMainlandRegions.value || regionPath.value.length === 3),
);

const [AddressForm, addressFormApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    labelWidth: 120,
  },
  handleValuesChange(values) {
    Object.assign(form, {
      addressLine1: String(values.addressLine1 || ''),
      addressLine2: String(values.addressLine2 || ''),
      countryCode: String(values.countryCode || ''),
      default: Boolean(values.default),
      label: String(values.label || ''),
      postalCode: String(values.postalCode || ''),
      recipientName: String(values.recipientName || ''),
      recipientPhone: String(values.recipientPhone || ''),
      type: (values.type || 'contact') as CustomerAddressType,
    });
    changeRegion(values.regionPath);
  },
  schema: [
    {
      component: 'Select',
      componentProps: () => ({ options: typeOptions.value }),
      fieldName: 'type',
      label: $t('customerAddress.type'),
      rules: 'selectRequired',
    },
    {
      component: 'Input',
      componentProps: { maxlength: 128 },
      fieldName: 'label',
      label: $t('customerAddress.label'),
    },
    {
      component: 'Input',
      componentProps: { maxlength: 256 },
      fieldName: 'recipientName',
      label: $t('customerAddress.recipientName'),
    },
    {
      component: 'Input',
      componentProps: { maxlength: 32 },
      fieldName: 'recipientPhone',
      label: $t('customerAddress.recipientPhone'),
    },
    {
      component: 'Select',
      componentProps: () => ({
        'data-testid': 'customer-address-country',
        onChange: changeCountry,
        optionFilterProp: 'label',
        options: countryOptions.value,
        showSearch: true,
      }),
      fieldName: 'countryCode',
      label: $t('customerAddress.countryCode'),
      rules: 'selectRequired',
    },
    {
      component: 'Cascader',
      componentProps: {
        'data-testid': 'customer-address-region',
        options: mainlandRegionOptions,
        placeholder: $t('customerAddress.regionPlaceholder'),
        showSearch: true,
      },
      dependencies: {
        required: (values) => values.countryCode === 'CN',
        show: (values) => values.countryCode === 'CN',
        triggerFields: ['countryCode'],
      },
      fieldName: 'regionPath',
      label: $t('customerAddress.region'),
    },
    {
      component: 'Input',
      componentProps: {
        'data-testid': 'customer-address-line1',
        maxlength: 512,
      },
      fieldName: 'addressLine1',
      label: $t('customerAddress.addressLine1'),
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { maxlength: 512 },
      fieldName: 'addressLine2',
      label: $t('customerAddress.addressLine2'),
    },
    {
      component: 'Input',
      componentProps: { maxlength: 32 },
      fieldName: 'postalCode',
      label: $t('customerAddress.postalCode'),
    },
    {
      component: 'Switch',
      fieldName: 'default',
      label: $t('customerAddress.makeDefault'),
    },
  ],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1 md:grid-cols-2',
});

const [EditorModal, editorModalApi] = useVbenModal({
  cancelText: $t('customerAddress.cancel'),
  class: 'w-[min(96vw,720px)]',
  confirmText: $t('customerAddress.save'),
  onConfirm: save,
});

async function load() {
  if (loading.value) return;
  loading.value = true;
  try {
    items.value = (await listCustomerAddresses()).items || [];
  } finally {
    loading.value = false;
  }
}

function resetForm() {
  regionPath.value = [];
  Object.assign(form, {
    addressLine1: '',
    addressLine2: '',
    city: '',
    countryCode: 'CN',
    default: false,
    district: '',
    label: '',
    postalCode: '',
    province: '',
    recipientName: '',
    recipientPhone: '',
    type: 'contact',
  });
  addressFormApi.setValues({ ...form, regionPath: [] });
}

function showEditor(item?: CustomerAddress) {
  editing.value = item;
  if (item) {
    regionPath.value = [item.province, item.city, item.district].filter(Boolean);
    Object.assign(form, {
      addressLine1: item.addressLine1,
      addressLine2: item.addressLine2,
      city: item.city,
      countryCode: item.countryCode,
      default: item.default,
      district: item.district,
      label: item.label,
      postalCode: item.postalCode,
      province: item.province,
      recipientName: item.recipientName,
      recipientPhone: item.recipientPhone,
      type: item.type,
    });
    addressFormApi.setValues({ ...form, regionPath: regionPath.value });
  } else {
    resetForm();
  }
  editorModalApi.setState({
    title: item
      ? $t('customerAddress.editTitle')
      : $t('customerAddress.createTitle'),
  });
  editorModalApi.open();
}

function changeCountry(value: unknown) {
  form.countryCode = String(value || '');
  regionPath.value = [];
  form.province = '';
  form.city = '';
  form.district = '';
  addressFormApi.setFieldValue('regionPath', []);
}

function changeRegion(value: unknown) {
  const values = Array.isArray(value) ? value.map(String) : [];
  regionPath.value = values;
  form.province = values[0] ?? '';
  form.city = values[1] ?? '';
  form.district = values[2] ?? '';
}

function saveInput(): CustomerAddressSaveInput {
  return {
    addressLine1: form.addressLine1.trim(),
    addressLine2: form.addressLine2.trim(),
    city: form.city.trim(),
    countryCode: form.countryCode.trim().toUpperCase(),
    default: form.default,
    district: form.district.trim(),
    idempotencyKey: createCustomerIdempotencyKey('customer-address'),
    label: form.label.trim(),
    postalCode: form.postalCode.trim(),
    province: form.province.trim(),
    recipientName: form.recipientName.trim(),
    recipientPhone: form.recipientPhone.trim(),
    type: form.type,
  };
}

async function save() {
  if (!canSave.value || saving.value) return false;
  saving.value = true;
  editorModalApi.lock(true);
  try {
    const input = saveInput();
    if (editing.value) {
      await updateCustomerAddress(editing.value.addressNo, {
        ...input,
        expectedVersion: editing.value.version,
      });
    } else {
      await createCustomerAddress(input);
    }
    editorModalApi.close();
    await load();
    message.success($t('customerAddress.saved'));
  } finally {
    saving.value = false;
    editorModalApi.lock(false);
  }
}

function archive(item: CustomerAddress) {
  AntModal.confirm({
    cancelText: $t('customerAddress.cancel'),
    content: $t('customerAddress.archiveDescription'),
    okButtonProps: { danger: true },
    okText: $t('customerAddress.archiveConfirm'),
    title: $t('customerAddress.archiveTitle'),
    onOk: async () => {
      action.value = item.addressNo;
      try {
        await archiveCustomerAddress(item.addressNo, {
          expectedVersion: item.version,
          idempotencyKey: createCustomerIdempotencyKey(
            'customer-address-archive',
          ),
        });
        await load();
        message.success($t('customerAddress.archivedMessage'));
      } finally {
        action.value = '';
      }
    },
  });
}

onMounted(load);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('customerAddress.title')">
    <div class="address-workspace" data-testid="customer-address-page">
      <Card :bordered="false" class="page-toolbar">
        <div>
          <p>{{ $t('customerAddress.subtitle') }}</p>
        </div>
        <Space wrap>
          <Button :loading="loading" @click="load">{{ $t('customerAddress.refresh') }}</Button>
          <Button data-testid="customer-address-add" type="primary" @click="showEditor()">
            {{ $t('customerAddress.add') }}
          </Button>
        </Space>
      </Card>

      <Spin :spinning="loading">
        <Empty v-if="!loading && items.length === 0" :description="$t('customerAddress.empty')" />
        <div v-else class="address-grid" data-testid="customer-address-list">
          <Card
            v-for="item in items"
            :key="item.addressNo"
            :class="{ archived: item.status === 'archived' }"
            size="small"
          >
            <div class="address-title">
              <div>
                <strong>{{ item.label || $t(`customerAddress.types.${item.type}`) }}</strong>
                <small>{{ $t(`customerAddress.types.${item.type}`) }}</small>
              </div>
              <Space wrap>
                <Tag v-if="item.default" color="blue">{{ $t('customerAddress.default') }}</Tag>
                <Tag v-if="item.status === 'archived'">{{ $t('customerAddress.archived') }}</Tag>
              </Space>
            </div>
            <address>
              <span v-if="item.recipientName">{{ item.recipientName }} {{ item.recipientPhone }}</span>
              <span>{{ item.countryCode }} {{ item.province }} {{ item.city }} {{ item.district }}</span>
              <span>{{ item.addressLine1 }} {{ item.addressLine2 }}</span>
              <span v-if="item.postalCode">{{ item.postalCode }}</span>
            </address>
            <Space v-if="item.status === 'active'" wrap>
              <Button data-testid="customer-address-edit" @click="showEditor(item)">{{ $t('customerAddress.edit') }}</Button>
              <Button danger :loading="action === item.addressNo" @click="archive(item)">{{ $t('customerAddress.archive') }}</Button>
            </Space>
          </Card>
        </div>
      </Spin>
    </div>

    <EditorModal>
      <div data-testid="customer-address-form">
        <AddressForm />
        <Alert :message="$t('customerAddress.defaultHint')" show-icon type="info" />
      </div>
    </EditorModal>
  </Page>
</template>

<style scoped>
.address-workspace { display: grid; gap: 16px; }
.page-toolbar { border: 1px solid hsl(var(--border)); }
.page-toolbar :deep(.ant-card-body) { display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.page-toolbar strong { display: block; margin-bottom: 6px; font-size: 18px; }
.page-toolbar p { max-width: 720px; margin: 0; color: hsl(var(--muted-foreground)); line-height: 1.7; }
.address-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.address-grid .ant-card { border-left: 3px solid hsl(var(--primary)); }
.address-grid .archived { border-left-color: hsl(var(--muted-foreground)); opacity: 0.72; }
.address-title { display: flex; align-items: start; justify-content: space-between; gap: 12px; }
.address-title strong, .address-title small, address span { display: block; }
.address-title small { margin-top: 4px; color: hsl(var(--muted-foreground)); }
address { min-height: 96px; margin: 18px 0; font-style: normal; line-height: 1.7; overflow-wrap: anywhere; }
@media (max-width: 760px) {
  .address-grid { grid-template-columns: 1fr; }
  .page-toolbar :deep(.ant-card-body) { align-items: stretch; flex-direction: column; }
}
</style>

<script lang="ts" setup>
import type { VbenFormSchema } from '#/adapter/form';
import type {
  CustomerSupportCustomFieldDefinition,
  CustomerSupportEntitlementCandidate,
  CustomerTicketCustomFieldSubmission,
} from '#/api/idc/support';
import type { UploadFile, UploadProps } from 'ant-design-vue';

import { onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';
import { preferences } from '@vben/preferences';

import { Alert, Button, Card, Space, Upload, message } from 'ant-design-vue';

import {
  createCustomerTicket,
  listCustomerSupportCustomFields,
  listCustomerSupportEntitlementCandidates,
  listCustomerTicketCategories,
  uploadInitialCustomerTicketAttachments,
} from '#/api/idc/support';
import { useVbenForm } from '#/adapter/form';
import { $t } from '#/locales';
import SupportCustomFields from '#/components/idc/SupportCustomFields.vue';

const props = withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
});
const emit = defineEmits<{
  cancel: [];
  created: [ticketNo: string];
}>();

const router = useRouter();
const route = useRoute();
const loadingCategories = ref(false);
const categoriesFailed = ref(false);
const customFields = ref<CustomerSupportCustomFieldDefinition[]>([]);
const customFieldsFailed = ref(false);
const loadingCustomFields = ref(true);
const entitlementCandidates = ref<CustomerSupportEntitlementCandidate[]>([]);
const entitlementsFailed = ref(false);
const customFieldForm = ref<{
  submissions: () => CustomerTicketCustomFieldSubmission[];
  validate: () => boolean;
}>();
const submitting = ref(false);
const attachmentFiles = ref<UploadFile[]>([]);
const UploadDragger = Upload.Dragger;
const maxAttachmentBytes = 20 * 1024 * 1024;
let entitlementSearchTimer: ReturnType<typeof setTimeout> | undefined;
let entitlementSearchSequence = 0;

const beforeUpload: UploadProps['beforeUpload'] = (file) => {
  if (file.size > maxAttachmentBytes) {
    message.error($t('support.attachmentTooLarge'));
    return Upload.LIST_IGNORE;
  }
  return false;
};

function createSchema(): VbenFormSchema[] {
  return [
    {
      component: 'Select',
      componentProps: {
        allowClear: true,
        'data-testid': 'ticket-entitlement',
        filterOption: false,
        onSearch: scheduleEntitlementSearch,
        options: [],
        showSearch: true,
      },
      fieldName: 'associationReference',
      help: $t('support.entitlementOptionalHint'),
      label: $t('support.relatedEntitlement'),
    },
    {
      component: 'Select',
      componentProps: {
        'data-testid': 'ticket-category',
        options: [],
      },
      fieldName: 'categoryNo',
      label: $t('support.category'),
      rules: 'selectRequired',
    },
    {
      component: 'Input',
      componentProps: {
        'data-testid': 'ticket-subject',
        maxlength: 200,
      },
      fieldName: 'subject',
      label: $t('support.subject'),
      rules: 'required',
    },
    {
      component: 'Textarea',
      componentProps: {
        'auto-size': { maxRows: 14, minRows: 7 },
        'data-testid': 'ticket-body',
      },
      fieldName: 'body',
      label: $t('support.body'),
      rules: 'required',
    },
  ];
}

const [TicketForm, formApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    labelWidth: 120,
  },
  schema: [],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1',
});

async function loadCategories() {
  loadingCategories.value = true;
  categoriesFailed.value = false;
  try {
    const result = await listCustomerTicketCategories({ limit: 100 });
    const useEnglish = preferences.app.locale === 'en-US';
    await formApi.updateSchema([
      {
        componentProps: {
          'data-testid': 'ticket-category',
          loading: false,
          options: (result.items || []).map((item) => ({
            label: useEnglish ? item.nameEnUS : item.nameZhCN,
            value: item.categoryNo,
          })),
        },
        fieldName: 'categoryNo',
      },
    ]);
  } catch {
    categoriesFailed.value = true;
  } finally {
    loadingCategories.value = false;
  }
}

function entitlementName(item: CustomerSupportEntitlementCandidate) {
  const name =
    preferences.app.locale === 'en-US'
      ? item.nameEnUS || item.nameZhCN
      : item.nameZhCN || item.nameEnUS;
  return `${name} · ${$t(`support.entitlementStatus.${item.status}`)}`;
}

async function loadCustomFields() {
  loadingCustomFields.value = true;
  customFieldsFailed.value = false;
  try {
    const result = await listCustomerSupportCustomFields();
    customFields.value = result.items || [];
  } catch {
    customFields.value = [];
    customFieldsFailed.value = true;
  } finally {
    loadingCustomFields.value = false;
  }
}

async function loadEntitlementCandidates(
  keyword = '',
  associationReference = '',
) {
  const sequence = ++entitlementSearchSequence;
  entitlementsFailed.value = false;
  try {
    const result = await listCustomerSupportEntitlementCandidates({
      associationReference: associationReference || undefined,
      keyword: keyword.trim(),
      limit: 25,
    });
    if (sequence !== entitlementSearchSequence) return;
    entitlementCandidates.value = result.items || [];
    await formApi.updateSchema([
      {
        componentProps: {
          allowClear: true,
          'data-testid': 'ticket-entitlement',
          filterOption: false,
          notFoundContent: $t('support.noEntitlementCandidates'),
          onSearch: scheduleEntitlementSearch,
          options: entitlementCandidates.value.map((item) => ({
            label: entitlementName(item),
            value: item.associationReference,
          })),
          placeholder: $t('support.entitlementPlaceholder'),
          showSearch: true,
        },
        fieldName: 'associationReference',
      },
    ]);
  } catch {
    if (sequence !== entitlementSearchSequence) return;
    entitlementCandidates.value = [];
    entitlementsFailed.value = true;
    await formApi.updateSchema([
      {
        componentProps: {
          allowClear: true,
          'data-testid': 'ticket-entitlement',
          disabled: true,
          options: [],
          placeholder: $t('support.entitlementsLoadFailed'),
        },
        fieldName: 'associationReference',
      },
    ]);
  }
}

function scheduleEntitlementSearch(keyword: string) {
  if (entitlementSearchTimer) clearTimeout(entitlementSearchTimer);
  entitlementSearchTimer = setTimeout(() => {
    void loadEntitlementCandidates(keyword);
  }, 250);
}

async function loadFormData() {
  formApi.setState({ schema: createSchema() });
  const requestedAssociationReference = props.embedded
    ? ''
    : typeof route.query.associationReference === 'string'
      ? route.query.associationReference.trim().slice(0, 64)
      : '';
  await Promise.all([
    loadCategories(),
    loadCustomFields(),
    loadEntitlementCandidates('', requestedAssociationReference),
  ]);
  if (
    requestedAssociationReference &&
    entitlementCandidates.value.some(
      (candidate) =>
        candidate.associationReference === requestedAssociationReference,
    )
  ) {
    await formApi.setFieldValue(
      'associationReference',
      requestedAssociationReference,
    );
  }
}

async function submit() {
  const { valid } = await formApi.validate();
  const customFieldsValid = customFieldForm.value?.validate() ?? false;
  if (!valid || !customFieldsValid || customFieldsFailed.value) return;
  const values = await formApi.getValues<{
    body: string;
    categoryNo: string;
    associationReference?: string;
    subject: string;
  }>();
  submitting.value = true;
  try {
    const files = attachmentFiles.value.flatMap((file) =>
      file.originFileObj ? [file.originFileObj as File] : [],
    );
    const uploadNos = await uploadInitialCustomerTicketAttachments(files);
    const ticket = await createCustomerTicket({
      body: values.body.trim(),
      categoryNo: values.categoryNo,
      customFields: customFieldForm.value?.submissions() || [],
      associationReference: values.associationReference,
      subject: values.subject.trim(),
      uploadNos,
    });
    message.success($t('support.createSuccess'));
    if (props.embedded) {
      emit('created', ticket.ticketNo);
    } else {
      await router.push(`/account/support/${ticket.ticketNo}`);
    }
  } catch {
    message.error($t('support.submitFailed'));
  } finally {
    submitting.value = false;
  }
}

onMounted(loadFormData);
onBeforeUnmount(() => {
  if (entitlementSearchTimer) clearTimeout(entitlementSearchTimer);
});

function cancel() {
  if (props.embedded) emit('cancel');
  else void router.push('/account/support');
}
</script>

<template>
  <component
    :is="embedded ? 'div' : Page"
    :class="embedded ? 'customer-support-create' : 'customer-inner-page'"
    header-class="customer-inner-heading"
    content-class="customer-inner-content"
    :title="embedded ? undefined : $t('support.createTitle')"
  >
    <Card class="customer-support-form" :bordered="false">
      <p class="mb-5 text-sm leading-6 text-muted-foreground">
        {{ $t('support.createIntro') }}
      </p>
      <Alert
        v-if="categoriesFailed"
        class="mb-5"
        :message="$t('support.categoriesLoadFailed')"
        show-icon
        type="error"
      >
        <template #action>
          <Button size="small" @click="loadFormData">
            {{ $t('support.retry') }}
          </Button>
        </template>
      </Alert>
      <Alert
        v-if="customFieldsFailed"
        class="mb-5"
        :message="$t('support.customFieldsLoadFailed')"
        show-icon
        type="error"
      />
      <Alert
        v-if="entitlementsFailed"
        class="mb-5"
        :message="$t('support.entitlementsLoadFailedHint')"
        show-icon
        type="warning"
      />
      <TicketForm />
      <SupportCustomFields
        ref="customFieldForm"
        :definitions="customFields"
        test-id-prefix="ticket-custom-field"
      />
      <UploadDragger
        v-model:file-list="attachmentFiles"
        class="mt-4"
        data-testid="ticket-initial-attachments"
        :before-upload="beforeUpload"
        multiple
      >
        <p class="ant-upload-text">{{ $t('support.addAttachments') }}</p>
        <p class="ant-upload-hint">{{ $t('support.attachmentHint') }}</p>
      </UploadDragger>
      <Space class="mt-6 flex justify-end">
        <Button @click="cancel">
          {{ $t('support.back') }}
        </Button>
        <Button
          data-testid="ticket-submit"
          :disabled="
            categoriesFailed ||
            customFieldsFailed ||
            loadingCategories ||
            loadingCustomFields
          "
          :loading="submitting"
          type="primary"
          @click="submit"
        >
          {{ $t('support.create') }}
        </Button>
      </Space>
    </Card>
  </component>
</template>

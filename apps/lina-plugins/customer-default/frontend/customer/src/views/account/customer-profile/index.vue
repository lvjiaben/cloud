<script setup lang="ts">
import type {
  CustomerContact,
  CustomerProfile,
} from '#/api/idc/customer-account';

import { computed, onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Descriptions,
  Empty,
  message,
  Modal as AntModal,
  Space,
  Spin,
  Tag,
} from 'ant-design-vue';

import {
  archiveCustomerContact,
  createCustomerContact,
  createCustomerIdempotencyKey,
  getCustomerProfile,
  listCustomerContacts,
  updateCustomerContact,
  updateCustomerProfile,
} from '#/api/idc/customer-account';
import { $t } from '#/locales';
import { useVbenForm } from '#/adapter/form';
import VbenModalCompat from '#/components/vben-modal-compat.vue';
import {
  buildCountryOptions,
  callingCodeOptions,
} from '#/utils/country-options';

const loading = ref(false);
const saving = ref(false);
const failed = ref(false);
const profile = ref<CustomerProfile>();
const contacts = ref<CustomerContact[]>([]);
const contactsLoading = ref(false);
const contactSaving = ref(false);
const contactModalOpen = ref(false);
const editingContact = ref<CustomerContact>();
const contactAction = ref('');
const form = reactive({
  countryCode: '',
  displayName: '',
  preferredLocale: 'zh-CN',
});
const contactForm = reactive({
  displayName: '',
  email: '',
  phone: '',
  phoneCountryCode: '+86',
  preferredLocale: 'zh-CN',
});

const localeOptions = computed(() => [
  { label: $t('customerAccount.profile.localeZh'), value: 'zh-CN' },
  { label: $t('customerAccount.profile.localeEn'), value: 'en-US' },
]);
const countryOptions = computed(() =>
  buildCountryOptions(form.preferredLocale),
);
const canSave = computed(
  () =>
    Boolean(profile.value) &&
    form.displayName.trim().length > 0 &&
    /^[A-Za-z]{2}$/.test(form.countryCode.trim()) &&
    ['en-US', 'zh-CN'].includes(form.preferredLocale),
);
const canSaveContact = computed(
  () =>
    contactForm.displayName.trim().length > 0 &&
    (contactForm.email.trim().length > 0 || contactForm.phone.trim().length > 0) &&
    (contactForm.phone.trim().length === 0 || /^\+[1-9][0-9]{0,3}$/.test(contactForm.phoneCountryCode.trim())) &&
    (contactForm.phone.trim().length === 0 || /^[0-9]{4,20}$/.test(contactForm.phone.replaceAll(' ', ''))),
);

const [ProfileForm, profileFormApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    labelWidth: 120,
  },
  handleValuesChange(values) {
    form.displayName = String(values.displayName || '');
    form.countryCode = String(values.countryCode || '');
    form.preferredLocale = String(values.preferredLocale || 'zh-CN');
  },
  schema: [
    {
      component: 'Input',
      componentProps: {
        'data-testid': 'customer-profile-display-name',
        maxlength: 256,
      },
      fieldName: 'displayName',
      label: $t('customerAccount.profile.displayName'),
      rules: 'required',
    },
    {
      component: 'Select',
      componentProps: () => ({
        'data-testid': 'customer-profile-country',
        optionFilterProp: 'label',
        options: countryOptions.value,
        showSearch: true,
      }),
      fieldName: 'countryCode',
      label: $t('customerAccount.profile.countryCode'),
      rules: 'selectRequired',
    },
    {
      component: 'Select',
      componentProps: () => ({
        'data-testid': 'customer-profile-locale',
        options: localeOptions.value,
      }),
      fieldName: 'preferredLocale',
      label: $t('customerAccount.profile.locale'),
      rules: 'selectRequired',
    },
  ],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1',
});

const [ContactForm, contactFormApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    labelWidth: 120,
  },
  handleValuesChange(values) {
    contactForm.displayName = String(values.displayName || '');
    contactForm.email = String(values.email || '');
    contactForm.phoneCountryCode = String(values.phoneCountryCode || '+86');
    contactForm.phone = String(values.phone || '');
    contactForm.preferredLocale = String(values.preferredLocale || 'zh-CN');
  },
  schema: [
    {
      component: 'Input',
      componentProps: { maxlength: 256 },
      fieldName: 'displayName',
      label: $t('customerAccount.contacts.name'),
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { maxlength: 320, type: 'email' },
      fieldName: 'email',
      label: $t('customerAccount.contacts.email'),
    },
    {
      component: 'Select',
      componentProps: {
        optionFilterProp: 'label',
        options: callingCodeOptions,
        showSearch: true,
      },
      fieldName: 'phoneCountryCode',
      label: $t('customerAccount.contacts.callingCode'),
    },
    {
      component: 'Input',
      componentProps: { maxlength: 20 },
      fieldName: 'phone',
      label: $t('customerAccount.contacts.phone'),
    },
    {
      component: 'Select',
      componentProps: () => ({ options: localeOptions.value }),
      fieldName: 'preferredLocale',
      label: $t('customerAccount.profile.locale'),
      rules: 'selectRequired',
    },
  ],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1 md:grid-cols-2',
});

function fillForm(item: CustomerProfile) {
  form.displayName = item.displayName;
  form.countryCode = item.countryCode;
  form.preferredLocale = item.preferredLocale;
  profileFormApi.setValues({ ...form });
}

async function load() {
  if (loading.value) return;
  loading.value = true;
  failed.value = false;
  try {
    const result = await getCustomerProfile();
    profile.value = result.item;
    fillForm(result.item);
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

async function loadContacts() {
  if (contactsLoading.value) return;
  contactsLoading.value = true;
  try {
    contacts.value = (await listCustomerContacts()).items || [];
  } finally {
    contactsLoading.value = false;
  }
}

function resetContactForm() {
  contactForm.displayName = '';
  contactForm.email = '';
  contactForm.phoneCountryCode = '+86';
  contactForm.phone = '';
  contactForm.preferredLocale = profile.value?.preferredLocale || 'zh-CN';
  contactFormApi.setValues({ ...contactForm });
}

function openContact(item?: CustomerContact) {
  editingContact.value = item;
  if (item) {
    contactForm.displayName = item.displayName;
    contactForm.email = item.email;
    contactForm.phoneCountryCode = item.phoneCountryCode || '+86';
    contactForm.phone = item.phone;
    contactForm.preferredLocale = item.preferredLocale;
    contactFormApi.setValues({ ...contactForm });
  } else {
    resetContactForm();
  }
  contactModalOpen.value = true;
}

async function saveContact() {
  if (!canSaveContact.value || contactSaving.value) return;
  contactSaving.value = true;
  const input = {
    displayName: contactForm.displayName.trim(),
    email: contactForm.email.trim(),
    idempotencyKey: createCustomerIdempotencyKey('customer-contact'),
    phone: contactForm.phone.replaceAll(' ', ''),
    phoneCountryCode: contactForm.phone.trim()
      ? contactForm.phoneCountryCode.trim()
      : '',
    preferredLocale: contactForm.preferredLocale,
  };
  try {
    if (editingContact.value) {
      await updateCustomerContact(editingContact.value.contactNo, {
        ...input,
        expectedVersion: editingContact.value.version,
      });
      message.success($t('customerAccount.contacts.updated'));
    } else {
      await createCustomerContact(input);
      message.success($t('customerAccount.contacts.created'));
    }
    contactModalOpen.value = false;
    await loadContacts();
  } finally {
    contactSaving.value = false;
  }
}

function archiveContact(item: CustomerContact) {
  AntModal.confirm({
    cancelText: $t('customerAccount.contacts.archiveCancel'),
    content: $t('customerAccount.contacts.archiveDescription'),
    okButtonProps: { danger: true },
    okText: $t('customerAccount.contacts.archiveConfirm'),
    title: $t('customerAccount.contacts.archiveTitle'),
    onOk: async () => {
      contactAction.value = item.contactNo;
      try {
        await archiveCustomerContact(item.contactNo, {
          expectedVersion: item.version,
          idempotencyKey: createCustomerIdempotencyKey('customer-contact-archive'),
        });
        await loadContacts();
        message.success($t('customerAccount.contacts.archived'));
      } finally {
        contactAction.value = '';
      }
    },
  });
}

async function save() {
  if (!canSave.value || !profile.value || saving.value) return;
  saving.value = true;
  try {
    const result = await updateCustomerProfile({
      countryCode: form.countryCode.trim().toUpperCase(),
      displayName: form.displayName.trim(),
      expectedVersion: profile.value.version,
      idempotencyKey: createCustomerIdempotencyKey('customer-profile'),
      preferredLocale: form.preferredLocale,
    });
    profile.value = result.item;
    fillForm(result.item);
    message.success($t('customerAccount.profile.saved'));
  } finally {
    saving.value = false;
  }
}

onMounted(() => Promise.all([load(), loadContacts()]));
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('customerAccount.profile.title')">
    <div class="profile-workspace" data-testid="customer-profile-page">
      <Card :bordered="false" class="page-toolbar">
        <div class="intro-content">
          <div>
            <strong>{{ $t('customerAccount.profile.heading') }}</strong>
            <p>{{ $t('customerAccount.profile.subtitle') }}</p>
          </div>
          <Button :loading="loading" @click="load">
            {{ $t('customerAccount.common.refresh') }}
          </Button>
        </div>
      </Card>

      <Alert
        v-if="failed"
        :message="$t('customerAccount.profile.loadFailed')"
        show-icon
        type="error"
      >
        <template #action>
          <Button size="small" @click="load">{{ $t('customerAccount.common.retry') }}</Button>
        </template>
      </Alert>

      <Spin :spinning="loading">
        <div v-if="profile" class="profile-grid">
          <Card :title="$t('customerAccount.profile.accountCard')">
            <Descriptions :column="1" size="small">
              <Descriptions.Item :label="$t('customerAccount.profile.customerNo')">
                <span>{{ profile.customerCode }}</span>
              </Descriptions.Item>
              <Descriptions.Item :label="$t('customerAccount.profile.customerType')">
                {{ $t(`customerAccount.type.${profile.type}`) }}
              </Descriptions.Item>
              <Descriptions.Item :label="$t('customerAccount.profile.accountStatus')">
                <Tag color="blue">{{ $t(`customerAccount.status.${profile.status}`) }}</Tag>
              </Descriptions.Item>
              <Descriptions.Item :label="$t('customerAccount.profile.verification')">
                {{ $t(`customerAccount.verification.${profile.verification || 'unverified'}`) }}
              </Descriptions.Item>
              <Descriptions.Item v-if="profile.legalNameMasked" :label="$t('customerAccount.profile.legalName')">
                {{ profile.legalNameMasked }}
              </Descriptions.Item>
              <Descriptions.Item v-if="profile.identifierMasked" :label="$t('customerAccount.profile.identifier')">
                {{ profile.identifierMasked }}
              </Descriptions.Item>
            </Descriptions>
            <Alert
              class="legal-note"
              :message="$t('customerAccount.profile.legalNote')"
              show-icon
              type="info"
            />
          </Card>

          <Card :title="$t('customerAccount.profile.editCard')">
            <ProfileForm />
              <Space>
                <Button :disabled="!canSave" :loading="saving" type="primary" @click="save">
                  {{ $t('customerAccount.profile.save') }}
                </Button>
                <Button :disabled="saving" @click="profile && fillForm(profile)">
                  {{ $t('customerAccount.common.cancelChanges') }}
                </Button>
              </Space>
          </Card>
        </div>
      </Spin>

      <Card :title="$t('customerAccount.contacts.title')">
        <div class="contact-heading">
          <p>{{ $t('customerAccount.contacts.description') }}</p>
          <Space wrap>
            <Button :loading="contactsLoading" @click="loadContacts">
              {{ $t('customerAccount.common.refresh') }}
            </Button>
            <Button data-testid="customer-contact-add" type="primary" @click="openContact()">
              {{ $t('customerAccount.contacts.add') }}
            </Button>
          </Space>
        </div>
        <Alert
          :message="$t('customerAccount.contacts.authorityWarning')"
          show-icon
          type="info"
        />
        <Spin :spinning="contactsLoading">
          <Empty
            v-if="!contactsLoading && contacts.length === 0"
            :description="$t('customerAccount.contacts.empty')"
          />
          <div v-else class="contact-grid" data-testid="customer-contact-list">
            <Card
              v-for="item in contacts"
              :key="item.contactNo"
              class="contact-card"
              :class="{ 'contact-card--archived': item.status === 'archived' }"
              size="small"
            >
              <div class="contact-title">
                <div>
                  <strong>{{ item.displayName }}</strong>
                </div>
                <Tag :color="item.status === 'active' ? 'success' : 'default'">
                  {{ $t(`customerAccount.contacts.status.${item.status}`) }}
                </Tag>
              </div>
              <dl class="contact-facts">
                <div v-if="item.email">
                  <dt>{{ $t('customerAccount.contacts.email') }}</dt>
                  <dd>
                    {{ item.email }}
                    <Tag :color="item.emailVerified ? 'success' : 'warning'">
                      {{ $t(`customerAccount.contacts.verification.${item.emailVerified ? 'verified' : 'unverified'}`) }}
                    </Tag>
                  </dd>
                </div>
                <div v-if="item.phone">
                  <dt>{{ $t('customerAccount.contacts.phone') }}</dt>
                  <dd>
                    {{ item.phoneCountryCode }} {{ item.phone }}
                    <Tag :color="item.phoneVerified ? 'success' : 'warning'">
                      {{ $t(`customerAccount.contacts.verification.${item.phoneVerified ? 'verified' : 'unverified'}`) }}
                    </Tag>
                  </dd>
                </div>
              </dl>
              <Space v-if="item.status === 'active'" wrap>
                <Button data-testid="customer-contact-edit" @click="openContact(item)">
                  {{ $t('customerAccount.contacts.edit') }}
                </Button>
                <Button
                  danger
                  :loading="contactAction === item.contactNo"
                  @click="archiveContact(item)"
                >
                  {{ $t('customerAccount.contacts.archive') }}
                </Button>
              </Space>
            </Card>
          </div>
        </Spin>
      </Card>
    </div>

    <VbenModalCompat
      v-model:open="contactModalOpen"
      :cancel-text="$t('customerAccount.contacts.cancel')"
      :confirm-loading="contactSaving"
      :ok-button-props="{ disabled: !canSaveContact }"
      :ok-text="$t('customerAccount.contacts.save')"
      :title="editingContact ? $t('customerAccount.contacts.editTitle') : $t('customerAccount.contacts.createTitle')"
      width="620px"
      @ok="saveContact"
    >
      <div data-testid="customer-contact-form">
        <ContactForm />
        <Alert
          :message="$t('customerAccount.contacts.verificationReset')"
          show-icon
          type="warning"
        />
      </div>
    </VbenModalCompat>
  </Page>
</template>

<style scoped>
.profile-workspace {
  display: grid;
  gap: 16px;
}

.page-toolbar {
  border: 1px solid hsl(var(--border));
}

.intro-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}

.intro-content strong {
  display: block;
  margin-bottom: 6px;
  font-size: 18px;
}

.intro-content p {
  max-width: 720px;
  margin: 0;
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
}

.profile-grid {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
  gap: 16px;
}

.legal-note {
  margin-top: 18px;
}

.contact-heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 14px;
}

.contact-heading p {
  max-width: 720px;
  margin: 0;
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
}

.contact-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-top: 14px;
}

.contact-card {
  border-left: 3px solid hsl(var(--primary));
}

.contact-card--archived {
  border-left-color: hsl(var(--muted-foreground));
  opacity: 0.72;
}

.contact-title {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 12px;
}

.contact-title strong,
.contact-title small {
  display: block;
}

.contact-title small {
  margin-top: 4px;
  color: hsl(var(--muted-foreground));
  overflow-wrap: anywhere;
}

.contact-facts {
  display: grid;
  gap: 10px;
  margin: 16px 0;
}

.contact-facts dt {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.contact-facts dd {
  margin: 4px 0 0;
  overflow-wrap: anywhere;
}

@media (max-width: 860px) {
  .contact-grid,
  .profile-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .intro-content {
    align-items: stretch;
    flex-direction: column;
  }

  .contact-heading {
    flex-direction: column;
  }

}
</style>

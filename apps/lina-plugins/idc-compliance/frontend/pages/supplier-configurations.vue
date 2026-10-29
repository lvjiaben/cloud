<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/compliance/suppliers",
  title: "Invoice Provider",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import type {
  SupplierConfiguration,
  SupplierProviderCode,
} from "./supplier-configuration-client";

import { computed, onMounted, reactive, ref } from "vue";

import { Page } from "@vben/common-ui";
import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Space,
  TabPane,
  Tabs,
  Tag,
  message,
} from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import VbenModalCompat from "./components/vben-modal-compat.vue";

import { useAccess } from "./plugin-access";
import {
  supplierConfigurationGet,
  supplierCredentialUpdate,
} from "./supplier-configuration-client";

const providers: SupplierProviderCode[] = [
  "nuonuo_invoice",
];
const activeProvider = ref<SupplierProviderCode>("nuonuo_invoice");
const loading = ref(false);
const saving = ref(false);
const editorOpen = ref(false);
const configurations = reactive<
  Partial<Record<SupplierProviderCode, SupplierConfiguration>>
>({});
const { hasAccessByCodes } = useAccess();
const canWrite = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:supplier:credential:write"]),
);
const current = computed(() => configurations[activeProvider.value]);

function tr(key: string) {
  return $t(`plugin.idc-compliance.supplier.${key}`);
}

function providerName(provider: SupplierProviderCode) {
  return tr(`provider.${provider}`);
}

function statusColor(value: string) {
  if (value === "active" || value === "configured" || value === "succeeded")
    return "green";
  if (value === "retryable_failed" || value === "revoked") return "red";
  if (value === "binding" || value === "reopening" || value === "updating")
    return "orange";
  return "default";
}

function credentialFormSchema() {
  return [
    {
      component: "Textarea",
      componentProps: {
        autocomplete: "off",
        maxlength: 16_384,
        rows: 7,
      },
      fieldName: "credentialEnvelope",
      label: tr("credential.envelope"),
      rules: z.string().trim().min(16).max(16_384),
    },
  ];
}

const [CredentialForm, credentialFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 120 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});

async function load(provider = activeProvider.value) {
  loading.value = true;
  try {
    configurations[provider] = (await supplierConfigurationGet(provider)).item;
  } finally {
    loading.value = false;
  }
}

async function openEditor() {
  await credentialFormApi.setState({ schema: credentialFormSchema() });
  await credentialFormApi.resetForm();
  editorOpen.value = true;
}

async function saveCredential() {
  const item = current.value;
  const { valid } = await credentialFormApi.validate();
  if (!item || !valid) {
    message.warning(tr("credential.required"));
    return;
  }
  const { credentialEnvelope } = await credentialFormApi.getValues<{
    credentialEnvelope: string;
  }>();
  saving.value = true;
  try {
    configurations[activeProvider.value] = (
      await supplierCredentialUpdate(activeProvider.value, {
        credentialEnvelope: credentialEnvelope.trim(),
        expectedVersion: item.version,
        idempotencyKey: `supplier-credential-${crypto.randomUUID()}`,
      })
    ).item;
    editorOpen.value = false;
    message.success(tr("credential.saved"));
  } finally {
    await credentialFormApi.resetForm();
    saving.value = false;
  }
}

async function changeProvider(value: string | number) {
  if (!providers.includes(value as SupplierProviderCode)) return;
  activeProvider.value = value as SupplierProviderCode;
  if (!configurations[activeProvider.value]) await load(activeProvider.value);
}

onMounted(() => load());
</script>

<template>
  <Page class="plugin-admin-page" :auto-content-height="true" data-testid="supplier-configurations-page">
    <Card class="h-full" :loading="loading">
      <template #title>{{ tr("title") }}</template>
      <template #extra>
        <Space>
          <Button @click="load()">{{ tr("refresh") }}</Button>
          <Button v-if="canWrite" type="primary" @click="openEditor">
            {{ tr("credential.configure") }}
          </Button>
        </Space>
      </template>

      <Alert
        class="mb-4"
        :description="tr('description')"
        :message="tr('notice')"
        show-icon
        type="info"
      />

      <Tabs :active-key="activeProvider" @change="changeProvider">
        <TabPane
          v-for="provider in providers"
          :key="provider"
          :tab="providerName(provider)"
        />
      </Tabs>

      <template v-if="current">
        <Alert
          class="mb-4"
          :description="
            current.adapterEnabled
              ? tr('ready.description')
              : tr(`blocked.${current.capabilityEvidenceCode}`)
          "
          :message="
            current.adapterEnabled ? tr('ready.title') : tr('blocked.title')
          "
          show-icon
          :type="current.adapterEnabled ? 'success' : 'warning'"
        />

        <Descriptions bordered :column="2">
          <DescriptionsItem :label="tr('field.provider')">
            {{ providerName(current.providerCode) }}
          </DescriptionsItem>
          <DescriptionsItem :label="tr('field.adapter')">
            <Tag :color="current.adapterEnabled ? 'green' : 'orange'">
              {{ current.adapterEnabled ? tr("enabled") : tr("disabled") }}
            </Tag>
          </DescriptionsItem>
          <DescriptionsItem :label="tr('field.credential')">
            <Tag :color="statusColor(current.credentialStatus)">
              {{ tr(`status.${current.credentialStatus}`) }}
            </Tag>
            <span v-if="current.credentialMaskedTail" class="ml-2">
              {{ tr("credential.tail") }} {{ current.credentialMaskedTail }}
            </span>
          </DescriptionsItem>
          <DescriptionsItem :label="tr('field.subject')">
            <Tag :color="statusColor(current.subjectStatus)">
              {{ tr(`status.${current.subjectStatus}`) }}
            </Tag>
          </DescriptionsItem>
          <DescriptionsItem :label="tr('field.generation')">
            {{ current.credentialGeneration }}
          </DescriptionsItem>
          <DescriptionsItem :label="tr('field.updatedAt')">
            {{ current.updatedAt ? formatTimestamp(current.updatedAt) : "—" }}
          </DescriptionsItem>
        </Descriptions>
      </template>
    </Card>

    <VbenModalCompat
      v-model:open="editorOpen"
      :confirm-loading="saving"
      :title="tr('credential.title')"
      width="620px"
      @ok="saveCredential"
    >
      <Alert
        class="mb-4"
        :description="tr('credential.description')"
        :message="providerName(activeProvider)"
        show-icon
        type="warning"
      />
      <CredentialForm />
    </VbenModalCompat>
  </Page>
</template>

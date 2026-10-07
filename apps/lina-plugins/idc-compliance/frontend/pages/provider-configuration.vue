<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/compliance/providers/aliyun-face",
  title: "Identity Provider",
};
</script>

<script setup lang="ts">
import type {
  ProviderConfiguration,
  ProviderConfigurationStatus,
  ProviderValidationStatus,
} from "./provider-configuration-client";

import { computed, onMounted, ref } from "vue";

import { useAccess } from "./plugin-access";
import { Page, useVbenModal } from "@vben/common-ui";

import { Alert, message, Tag } from "ant-design-vue";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import ProviderCredentialModal from "./components/provider-credential-modal.vue";
import {
  providerConfigurationGet,
  providerConfigurationValidate,
} from "./provider-configuration-client";

const { hasAccessByCodes } = useAccess();
const configuration = ref<ProviderConfiguration | null>(null);
const loading = ref(true);
const validating = ref(false);

const configurationStatusKeys = {
  configured: "plugin.idc-compliance.provider.status.configuration.configured",
  disabled: "plugin.idc-compliance.provider.status.configuration.disabled",
  draft: "plugin.idc-compliance.provider.status.configuration.draft",
  updating: "plugin.idc-compliance.provider.status.configuration.updating",
} as const satisfies Record<ProviderConfigurationStatus, string>;
const configurationDescriptionKeys = {
  configured:
    "plugin.idc-compliance.provider.status.configurationDescription.configured",
  disabled:
    "plugin.idc-compliance.provider.status.configurationDescription.disabled",
  draft: "plugin.idc-compliance.provider.status.configurationDescription.draft",
  updating:
    "plugin.idc-compliance.provider.status.configurationDescription.updating",
} as const satisfies Record<ProviderConfigurationStatus, string>;
const validationStatusKeys = {
  reachable: "plugin.idc-compliance.provider.status.validation.reachable",
  rejected: "plugin.idc-compliance.provider.status.validation.rejected",
  unavailable: "plugin.idc-compliance.provider.status.validation.unavailable",
  untested: "plugin.idc-compliance.provider.status.validation.untested",
} as const satisfies Record<ProviderValidationStatus, string>;
const validationDescriptionKeys = {
  reachable:
    "plugin.idc-compliance.provider.status.validationDescription.reachable",
  rejected:
    "plugin.idc-compliance.provider.status.validationDescription.rejected",
  unavailable:
    "plugin.idc-compliance.provider.status.validationDescription.unavailable",
  untested:
    "plugin.idc-compliance.provider.status.validationDescription.untested",
} as const satisfies Record<ProviderValidationStatus, string>;
const validationResultKeys = {
  reachable: "plugin.idc-compliance.provider.validation.result.reachable",
  rejected: "plugin.idc-compliance.provider.validation.result.rejected",
  unavailable: "plugin.idc-compliance.provider.validation.result.unavailable",
  untested: "plugin.idc-compliance.provider.validation.result.untested",
} as const satisfies Record<ProviderValidationStatus, string>;

const canWrite = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:provider:write"]),
);
const canValidate = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:provider:validate"]),
);
const isUpdating = computed(() => configuration.value?.status === "updating");
const isConfigured = computed(
  () =>
    configuration.value?.status === "configured" &&
    configuration.value.credentialConfigured,
);

const [CredentialModal, credentialModalApi] = useVbenModal({
  connectedComponent: ProviderCredentialModal,
});

function configurationStatusLabel(status: ProviderConfigurationStatus) {
  return $t(configurationStatusKeys[status]);
}

function validationStatusLabel(status: ProviderValidationStatus) {
  return $t(validationStatusKeys[status]);
}

function configurationStatusDescription(status: ProviderConfigurationStatus) {
  return $t(configurationDescriptionKeys[status]);
}

function validationStatusDescription(status: ProviderValidationStatus) {
  return $t(validationDescriptionKeys[status]);
}

function configurationTagColor(status: ProviderConfigurationStatus) {
  if (status === "configured") return "green";
  if (status === "updating") return "orange";
  if (status === "disabled") return "red";
  return "default";
}

function validationTagColor(status: ProviderValidationStatus) {
  if (status === "reachable") return "green";
  if (status === "rejected") return "red";
  if (status === "unavailable") return "orange";
  return "default";
}

async function loadConfiguration() {
  loading.value = true;
  try {
    const result = await providerConfigurationGet();
    configuration.value = result.item;
  } finally {
    loading.value = false;
  }
}

function openCredential(mode: "configure" | "recover") {
  if (!configuration.value) return;
  credentialModalApi.setData({ configuration: configuration.value, mode });
  credentialModalApi.open();
}

async function validateConnectivity() {
  if (!configuration.value || validating.value) return;
  validating.value = true;
  try {
    const result = await providerConfigurationValidate();
    configuration.value = result.item;
    message.success($t(validationResultKeys[result.item.validationStatus]));
  } finally {
    validating.value = false;
  }
}

function credentialUpdated(item: ProviderConfiguration) {
  configuration.value = item;
}

onMounted(loadConfiguration);
</script>

<template>
  <Page
    :auto-content-height="true"
    :description="$t('plugin.idc-compliance.provider.subtitle')"
    :title="$t('plugin.idc-compliance.provider.title')"
    data-testid="aliyun-provider-page"
  >
    <div class="provider-shell" :class="{ loading }">
      <Alert
        v-if="isUpdating"
        data-testid="aliyun-provider-recovery-alert"
        :description="
          $t('plugin.idc-compliance.provider.recovery.bannerDescription')
        "
        :message="$t('plugin.idc-compliance.provider.recovery.bannerTitle')"
        show-icon
        type="warning"
      />

      <section v-if="configuration" class="provider-panel">
        <header class="panel-heading">
          <div>
            <h2>{{ $t("plugin.idc-compliance.provider.cards.state") }}</h2>
            <p>
              {{ configurationStatusDescription(configuration.status) }}
            </p>
          </div>
          <div class="action-buttons">
            <Tag
              data-testid="aliyun-provider-status"
              :color="configurationTagColor(configuration.status)"
              >{{ configurationStatusLabel(configuration.status) }}</Tag
            >
          </div>
        </header>

        <div class="provider-grid">
          <dl class="detail-list">
            <div>
              <dt>{{ $t("plugin.idc-compliance.provider.fields.sceneId") }}</dt>
              <dd data-testid="aliyun-provider-scene-value">
                {{ configuration.sceneId || "—" }}
              </dd>
            </div>
            <div>
              <dt>
                {{ $t("plugin.idc-compliance.provider.fields.credential") }}
              </dt>
              <dd data-testid="aliyun-provider-credential-tail">
                {{
                  configuration.credentialConfigured
                    ? configuration.credentialMaskedTail
                    : $t("plugin.idc-compliance.provider.fields.notStored")
                }}
              </dd>
            </div>
            <div>
              <dt>
                {{ $t("plugin.idc-compliance.provider.fields.generation") }}
              </dt>
              <dd>{{ configuration.credentialGeneration }}</dd>
            </div>
            <div>
              <dt>
                {{ $t("plugin.idc-compliance.provider.fields.updatedAt") }}
              </dt>
              <dd>
                {{
                  configuration.updatedAt
                    ? formatTimestamp(configuration.updatedAt)
                    : "—"
                }}
              </dd>
            </div>
          </dl>

          <div class="validation-panel">
            <div class="validation-heading">
              <strong>{{
                $t("plugin.idc-compliance.provider.cards.validation")
              }}</strong>
              <Tag
                data-testid="aliyun-provider-validation-status"
                :color="validationTagColor(configuration.validationStatus)"
                >{{
                  validationStatusLabel(configuration.validationStatus)
                }}</Tag
              >
            </div>
            <p>
              {{ validationStatusDescription(configuration.validationStatus) }}
            </p>
            <div class="validation-time">
              <span>{{
                $t("plugin.idc-compliance.provider.fields.validatedAt")
              }}</span>
              <strong>{{
                configuration.validatedAt
                  ? formatTimestamp(configuration.validatedAt)
                  : "—"
              }}</strong>
            </div>
          </div>
        </div>

        <footer class="action-rail">
          <div>
            <strong>{{
              $t("plugin.idc-compliance.provider.actions.title")
            }}</strong>
            <p>
              {{ $t("plugin.idc-compliance.provider.actions.description") }}
            </p>
          </div>
          <div class="action-buttons">
            <a-button
              v-if="canValidate && isConfigured"
              data-testid="aliyun-provider-validate"
              :loading="validating"
              @click="validateConnectivity"
              >{{
                $t("plugin.idc-compliance.provider.actions.validate")
              }}</a-button
            >
            <a-button
              v-if="canWrite && !isUpdating"
              data-testid="aliyun-provider-configure"
              type="primary"
              @click="openCredential('configure')"
              >{{
                $t(
                  isConfigured
                    ? "plugin.idc-compliance.provider.actions.replace"
                    : "plugin.idc-compliance.provider.actions.configure",
                )
              }}</a-button
            >
            <a-button
              v-if="canWrite && isUpdating"
              danger
              data-testid="aliyun-provider-recover"
              type="primary"
              @click="openCredential('recover')"
              >{{
                $t("plugin.idc-compliance.provider.actions.recover")
              }}</a-button
            >
          </div>
        </footer>
      </section>

      <Alert
        v-if="configuration"
        class="responsibility-note"
        :description="
          $t('plugin.idc-compliance.provider.responsibility.description')
        "
        :message="$t('plugin.idc-compliance.provider.responsibility.title')"
        show-icon
        type="info"
      />
    </div>
    <CredentialModal @success="credentialUpdated" />
  </Page>
</template>

<style scoped>
.provider-shell {
  display: grid;
  gap: 12px;
  min-height: 100%;
  transition: opacity 160ms ease;
}
.provider-shell.loading {
  opacity: 0.55;
}
.provider-panel {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  box-shadow: none;
}
.panel-heading,
.action-rail {
  align-items: center;
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
  border-bottom: 1px solid hsl(var(--border));
}
.panel-heading h2 {
  margin: 0;
  font-size: 16px;
  line-height: 1.4;
}
.panel-heading p,
.action-rail p,
.validation-panel p {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  line-height: 1.55;
  margin: 4px 0 0;
}
.provider-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(280px, 0.75fr);
}
.detail-list {
  display: grid;
  margin: 0;
  padding: 4px 20px;
}
.detail-list > div {
  align-items: center;
  border-bottom: 1px solid hsl(var(--border));
  display: flex;
  justify-content: space-between;
  gap: 24px;
  min-height: 44px;
}
.detail-list > div:last-child {
  border-bottom: 0;
}
.detail-list dt,
.validation-time span {
  color: hsl(var(--muted-foreground));
  font-size: 13px;
}
.detail-list dd,
.validation-time strong {
  color: hsl(var(--foreground));
  font-size: 13px;
  font-weight: 600;
  margin: 0;
}
.validation-panel {
  padding: 16px 20px;
  border-left: 1px solid hsl(var(--border));
  background: hsl(var(--muted) / 45%);
}
.validation-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 14px;
}
.validation-time {
  align-items: center;
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-top: 20px;
  padding-top: 12px;
  border-top: 1px solid hsl(var(--border));
}
.action-rail {
  border-top: 1px solid hsl(var(--border));
  border-bottom: 0;
}
.action-rail strong {
  font-size: 14px;
}
.action-buttons {
  display: flex;
  gap: 10px;
}
.responsibility-note {
  border-radius: 0;
}
@media (max-width: 1100px) {
  .provider-grid {
    grid-template-columns: 1fr;
  }
  .validation-panel {
    border-top: 1px solid hsl(var(--border));
    border-left: 0;
  }
}
@media (max-width: 720px) {
  .panel-heading,
  .action-rail {
    align-items: stretch;
    flex-direction: column;
    gap: 16px;
  }
  .action-buttons {
    flex-direction: column;
  }
}
</style>

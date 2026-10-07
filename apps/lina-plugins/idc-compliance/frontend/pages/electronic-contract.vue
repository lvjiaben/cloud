<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/compliance/electronic-contract",
  title: "Electronic Contract",
};
</script>

<script setup lang="ts">
import type {
  ElectronicContractOnboarding,
  OnboardingCapability,
  OnboardingState,
} from "./electronic-contract-onboarding-client";
import type { ElectronicContractTemplate } from "./electronic-contract-template-client";

import { computed, onMounted, ref } from "vue";

import { useAccess } from "./plugin-access";
import { Page, useVbenDrawer } from "@vben/common-ui";

import {
  Alert,
  Badge,
  Button,
  Card,
  message,
  Steps,
  TabPane,
  Tabs,
  Tag,
} from "ant-design-vue";

import { $t } from "#/locales";

import {
  electronicContractOnboardingGet,
  electronicContractOnboardingLinkCreate,
  electronicContractOnboardingRecheck,
} from "./electronic-contract-onboarding-client";
import ElectronicContractTemplateEditor from "./components/electronic-contract-template-editor.vue";
import ElectronicContractTemplateManager from "./components/electronic-contract-template-manager.vue";
import ElectronicContractAutoSignManager from "./components/electronic-contract-auto-sign-manager.vue";
import ElectronicContractManager from "./components/electronic-contract-manager.vue";

const { hasAccessByCodes } = useAccess();
const onboarding = ref<ElectronicContractOnboarding | null>(null);
const loading = ref(true);
const opening = ref(false);
const rechecking = ref(false);
const canViewOnboarding = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:supplier:query"]),
);
const canViewTemplates = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:contract-template:query"]),
);
const canManageTemplates = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:contract-template:manage"]),
);
const canManageAutoSign = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:contract-auto-sign:manage"]),
);
const canViewContracts = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:contract:query"]),
);
const canCreateContracts = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:contract:create"]),
);
const canReissueContracts = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:contract:reissue"]),
);
const canTerminateContracts = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:contract:terminate"]),
);
const canCancelFailedContractOrder = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:contract:cancel-order"]),
);
const canConfigureFailurePolicy = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:contract:failure-policy"]),
);
const activeTab = ref(
  canViewOnboarding.value
    ? "onboarding"
    : canViewTemplates.value
      ? "templates"
      : canViewContracts.value
        ? "contracts"
        : "auto-sign",
);
const editorMode = ref<"create" | "edit" | "view" | "">("");
const editorItem = ref<ElectronicContractTemplate>();
const templateRefreshToken = ref(0);
const [TemplateEditorDrawer, templateEditorDrawerApi] = useVbenDrawer({
  appendToMain: true,
  class: "w-full max-w-[1400px]",
  closeOnClickModal: false,
  destroyOnClose: true,
  footer: false,
  onClosed() {
    editorMode.value = "";
    editorItem.value = undefined;
  },
});

const stateTextKeys: Record<OnboardingState, string> = {
  active: "plugin.idc-compliance.contract.state.active",
  enterprise_pending: "plugin.idc-compliance.contract.state.enterprise_pending",
  not_configured: "plugin.idc-compliance.contract.state.not_configured",
  operator_pending: "plugin.idc-compliance.contract.state.operator_pending",
  ready: "plugin.idc-compliance.contract.state.ready",
  temporarily_unavailable:
    "plugin.idc-compliance.contract.state.temporarily_unavailable",
};
const reasonTextKeys: Record<string, string> = {
  local_gateway_unavailable:
    "plugin.idc-compliance.contract.reason.local_gateway_unavailable",
  merchant_profile_incomplete:
    "plugin.idc-compliance.contract.reason.merchant_profile_incomplete",
  tencent_app_not_configured:
    "plugin.idc-compliance.contract.reason.tencent_app_not_configured",
  tencent_temporarily_unavailable:
    "plugin.idc-compliance.contract.reason.tencent_temporarily_unavailable",
};
const capabilityStateTextKeys: Record<OnboardingCapability["state"], string> = {
  available: "plugin.idc-compliance.contract.capability.state.available",
  disabled: "plugin.idc-compliance.contract.capability.state.disabled",
  unavailable: "plugin.idc-compliance.contract.capability.state.unavailable",
};

const canOnboard = computed(() =>
  hasAccessByCodes(["idc:admin:compliance:supplier:subject:bind"]),
);
const profileComplete = computed(
  () =>
    Boolean(onboarding.value?.merchant.legalName) &&
    Boolean(onboarding.value?.merchant.countryCode) &&
    Boolean(onboarding.value?.merchant.merchantNo),
);
const temporarilyBlocked = computed(
  () =>
    onboarding.value?.state === "not_configured" ||
    onboarding.value?.state === "temporarily_unavailable",
);
const currentStep = computed(() => {
  if (onboarding.value?.state === "active") return 3;
  if (
    onboarding.value?.state === "enterprise_pending" ||
    onboarding.value?.state === "operator_pending"
  ) {
    return 2;
  }
  return profileComplete.value ? 1 : 0;
});
const stateTone = computed(() => {
  if (onboarding.value?.state === "active") return "success";
  if (temporarilyBlocked.value) return "error";
  if (onboarding.value?.state?.includes("pending")) return "warning";
  return "processing";
});

function key(prefix: string) {
  return prefix + "-" + crypto.randomUUID();
}

function stateText(
  state = onboarding.value?.state ?? "temporarily_unavailable",
) {
  return $t(stateTextKeys[state]);
}

function reasonText(reason = onboarding.value?.reasonCode ?? "") {
  if (!reason) return "";
  const reasonKey = reasonTextKeys[reason];
  return reasonKey ? $t(reasonKey) : reason;
}

function capability(code: OnboardingCapability["code"]) {
  return onboarding.value?.capabilities.find((item) => item.code === code);
}

function capabilityText(item?: OnboardingCapability) {
  if (!item) {
    return $t("plugin.idc-compliance.contract.capability.unchecked");
  }
  return $t(capabilityStateTextKeys[item.state]);
}

function capabilityColor(item?: OnboardingCapability) {
  if (item?.state === "available") return "green";
  if (item?.state === "unavailable") return "red";
  return "default";
}

async function load() {
  if (!canViewOnboarding.value) return;
  loading.value = true;
  try {
    onboarding.value = (await electronicContractOnboardingGet()).item;
  } finally {
    loading.value = false;
  }
}

function openTemplateEditor(payload: {
  item?: ElectronicContractTemplate;
  mode: "create" | "edit" | "view";
}) {
  editorItem.value = payload.item;
  editorMode.value = payload.mode;
  templateEditorDrawerApi.setState({
    title: $t(
      payload.mode === "create"
        ? "plugin.idc-compliance.contract.template.actions.create"
        : payload.mode === "view"
          ? "plugin.idc-compliance.contract.template.actions.preview"
          : "plugin.idc-compliance.contract.template.actions.edit",
    ),
  });
  templateEditorDrawerApi.open();
}

function closeTemplateEditor(payload: { saved: boolean }) {
  editorMode.value = "";
  editorItem.value = undefined;
  templateEditorDrawerApi.close();
  activeTab.value = "templates";
  if (payload.saved) templateRefreshToken.value += 1;
}

async function openTencent() {
  if (!canOnboard.value || opening.value) return;
  const externalWindow = window.open("about:blank", "_blank");
  if (!externalWindow) {
    message.warning($t("plugin.idc-compliance.contract.actions.popupBlocked"));
    return;
  }
  externalWindow.opener = null;
  opening.value = true;
  try {
    const result = await electronicContractOnboardingLinkCreate(
      key("tencent-onboarding"),
    );
    onboarding.value = result.item;
    if (!result.item.consoleUrl) {
      externalWindow.close();
      message.warning(reasonText(result.item.reasonCode) || stateText());
      return;
    }
    externalWindow.location.replace(result.item.consoleUrl);
  } catch (error) {
    externalWindow.close();
    throw error;
  } finally {
    opening.value = false;
  }
}

async function recheck() {
  if (!canOnboard.value || rechecking.value) return;
  rechecking.value = true;
  try {
    onboarding.value = (
      await electronicContractOnboardingRecheck(key("tencent-recheck"))
    ).item;
    message.success($t("plugin.idc-compliance.contract.actions.rechecked"));
  } finally {
    rechecking.value = false;
  }
}

onMounted(() => {
  if (canViewOnboarding.value) load();
});
</script>

<template>
  <Page :auto-content-height="true" class="min-w-0 w-full" content-class="min-h-0 min-w-0" data-testid="electronic-contract-page">
    <Tabs
      v-model:active-key="activeTab"
      class="contract-tabs"
      data-testid="electronic-contract-tabs"
    >
      <TabPane
        v-if="canViewOnboarding"
        key="onboarding"
        :tab="$t('plugin.idc-compliance.contract.tabs.onboarding')"
      >
        <div class="contract-shell" :class="{ loading }">
          <Card
            v-if="onboarding"
            :bordered="false"
            :title="$t('plugin.idc-compliance.contract.title')"
            class="contract-toolbar"
            size="small"
          >
            <template #extra>
              <Badge :status="stateTone" :text="stateText()" />
            </template>
          </Card>

          <Alert
            v-if="temporarilyBlocked && onboarding"
            data-testid="electronic-contract-unavailable"
            :description="reasonText()"
            :message="$t('plugin.idc-compliance.contract.unavailable')"
            show-icon
            type="warning"
          />

          <section
            v-if="onboarding"
            :aria-label="$t('plugin.idc-compliance.contract.progressLabel')"
            class="step-rail"
          >
            <Steps
              :current="currentStep"
              :items="[
                {
                  title: $t('plugin.idc-compliance.contract.steps.profile'),
                },
                {
                  title: $t('plugin.idc-compliance.contract.steps.tencent'),
                },
                {
                  title: $t('plugin.idc-compliance.contract.steps.recheck'),
                },
                {
                  title: $t('plugin.idc-compliance.contract.steps.permissions'),
                },
              ]"
              responsive
            />
          </section>

          <main v-if="onboarding" class="onboarding-grid">
            <article
              class="step-card"
              data-testid="electronic-contract-profile"
            >
              <div class="step-heading">
                <b>01</b>
                <div>
                  <small>{{
                    $t("plugin.idc-compliance.contract.step1.kicker")
                  }}</small>
                  <h2>
                    {{ $t("plugin.idc-compliance.contract.steps.profile") }}
                  </h2>
                </div>
              </div>
              <p class="step-description">
                {{ $t("plugin.idc-compliance.contract.step1.description") }}
              </p>
              <dl class="profile-list">
                <div>
                  <dt>
                    {{ $t("plugin.idc-compliance.contract.fields.legalName") }}
                  </dt>
                  <dd>{{ onboarding.merchant.legalName || "—" }}</dd>
                </div>
                <div>
                  <dt>
                    {{ $t("plugin.idc-compliance.contract.fields.region") }}
                  </dt>
                  <dd>{{ onboarding.merchant.countryCode || "—" }}</dd>
                </div>
                <div>
                  <dt>
                    {{ $t("plugin.idc-compliance.contract.fields.merchantNo") }}
                  </dt>
                  <dd>{{ onboarding.merchant.merchantNo || "—" }}</dd>
                </div>
              </dl>
              <div class="privacy-note">
                {{ $t("plugin.idc-compliance.contract.step1.privacy") }}
              </div>
            </article>

            <article
              class="step-card step-card--accent"
              data-testid="electronic-contract-tencent"
            >
              <div class="step-heading">
                <b>02</b>
                <div>
                  <small>{{
                    $t("plugin.idc-compliance.contract.step2.kicker")
                  }}</small>
                  <h2>
                    {{ $t("plugin.idc-compliance.contract.steps.tencent") }}
                  </h2>
                </div>
              </div>
              <p class="step-description">
                {{ $t("plugin.idc-compliance.contract.step2.description") }}
              </p>
              <div class="role-callout">
                <strong>{{
                  $t("plugin.idc-compliance.contract.step2.roleTitle")
                }}</strong>
                <span>{{
                  $t("plugin.idc-compliance.contract.step2.roleDescription")
                }}</span>
              </div>
              <Button
                block
                data-testid="electronic-contract-open-tencent"
                :disabled="
                  !canOnboard || !profileComplete || temporarilyBlocked
                "
                :loading="opening"
                size="large"
                type="primary"
                @click="openTencent"
              >
                {{ $t("plugin.idc-compliance.contract.actions.openTencent") }}
              </Button>
              <small class="button-note">{{
                $t("plugin.idc-compliance.contract.actions.newWindow")
              }}</small>
            </article>

            <article
              class="step-card"
              data-testid="electronic-contract-recheck"
            >
              <div class="step-heading">
                <b>03</b>
                <div>
                  <small>{{
                    $t("plugin.idc-compliance.contract.step3.kicker")
                  }}</small>
                  <h2>
                    {{ $t("plugin.idc-compliance.contract.steps.recheck") }}
                  </h2>
                </div>
              </div>
              <p class="step-description">
                {{ $t("plugin.idc-compliance.contract.step3.description") }}
              </p>
              <div class="readiness-list">
                <div>
                  <span>{{
                    $t("plugin.idc-compliance.contract.fields.organization")
                  }}</span>
                  <Tag
                    :color="onboarding.organizationReady ? 'green' : 'orange'"
                  >
                    {{
                      onboarding.organizationReady
                        ? $t("plugin.idc-compliance.contract.ready")
                        : $t("plugin.idc-compliance.contract.waiting")
                    }}
                  </Tag>
                </div>
                <div>
                  <span>{{
                    $t("plugin.idc-compliance.contract.fields.operator")
                  }}</span>
                  <Tag :color="onboarding.operatorReady ? 'green' : 'orange'">
                    {{
                      onboarding.operatorReady
                        ? $t("plugin.idc-compliance.contract.ready")
                        : $t("plugin.idc-compliance.contract.waiting")
                    }}
                  </Tag>
                </div>
              </div>
              <Button
                block
                data-testid="electronic-contract-recheck-button"
                :disabled="!canOnboard || !profileComplete"
                :loading="rechecking"
                size="large"
                @click="recheck"
              >
                {{ $t("plugin.idc-compliance.contract.actions.recheck") }}
              </Button>
            </article>

            <article
              class="step-card"
              data-testid="electronic-contract-permissions"
            >
              <div class="step-heading">
                <b>04</b>
                <div>
                  <small>{{
                    $t("plugin.idc-compliance.contract.step4.kicker")
                  }}</small>
                  <h2>
                    {{ $t("plugin.idc-compliance.contract.steps.permissions") }}
                  </h2>
                </div>
              </div>
              <p class="step-description">
                {{ $t("plugin.idc-compliance.contract.step4.description") }}
              </p>
              <div class="capability-list">
                <div>
                  <span>{{
                    $t("plugin.idc-compliance.contract.capability.seal")
                  }}</span>
                  <Tag :color="onboarding.operatorReady ? 'green' : 'default'">
                    {{
                      onboarding.operatorReady
                        ? $t(
                            "plugin.idc-compliance.contract.capability.tencentManaged",
                          )
                        : $t(
                            "plugin.idc-compliance.contract.capability.unchecked",
                          )
                    }}
                  </Tag>
                </div>
                <div>
                  <span>{{
                    $t("plugin.idc-compliance.contract.capability.download")
                  }}</span>
                  <Tag :color="capabilityColor(capability('flow.download'))">
                    {{ capabilityText(capability("flow.download")) }}
                  </Tag>
                </div>
                <div>
                  <span>{{
                    $t("plugin.idc-compliance.contract.capability.autoSign")
                  }}</span>
                  <Tag
                    :color="capabilityColor(capability('auto_sign.enterprise'))"
                  >
                    {{ capabilityText(capability("auto_sign.enterprise")) }}
                  </Tag>
                </div>
              </div>
              <div class="owner-note">
                {{ $t("plugin.idc-compliance.contract.step4.owner") }}
              </div>
            </article>
          </main>
        </div>
      </TabPane>
      <TabPane
        v-if="canViewTemplates"
        key="templates"
        :tab="$t('plugin.idc-compliance.contract.tabs.templates')"
      >
        <ElectronicContractTemplateManager
          :can-manage="canManageTemplates"
          :refresh-token="templateRefreshToken"
          @open-editor="openTemplateEditor"
        />
      </TabPane>
      <TabPane
        v-if="canViewContracts"
        key="contracts"
        :tab="$t('plugin.idc-compliance.contract.tabs.contracts')"
      >
        <ElectronicContractManager
          :can-cancel-order="canCancelFailedContractOrder"
          :can-configure-failure-policy="canConfigureFailurePolicy"
          :can-create="canCreateContracts"
          :can-reissue="canReissueContracts"
          :can-terminate="canTerminateContracts"
        />
      </TabPane>
      <TabPane
        v-if="canManageAutoSign"
        key="auto-sign"
        :tab="$t('plugin.idc-compliance.contract.tabs.autoSign')"
      >
        <ElectronicContractAutoSignManager />
      </TabPane>
    </Tabs>
    <TemplateEditorDrawer>
      <ElectronicContractTemplateEditor
        v-if="editorMode"
        :item="editorItem"
        :mode="editorMode"
        @close="closeTemplateEditor"
      />
    </TemplateEditorDrawer>
  </Page>
</template>

<style scoped>
.contract-tabs :deep(.ant-tabs-nav) {
  flex: none;
  margin-bottom: 12px;
  padding-inline: 12px;
}
.contract-tabs :deep(.ant-tabs-nav-list) {
  padding-left: 0;
}
.contract-tabs {
  box-sizing: border-box;
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
  width: 100%;
}
.contract-tabs :deep(.ant-tabs-content-holder),
.contract-tabs :deep(.ant-tabs-content),
.contract-tabs :deep(.ant-tabs-tabpane-active) {
  height: 100%;
  min-height: 0;
  min-width: 0;
}
.contract-tabs :deep(.ant-tabs-content-holder) {
  overflow: hidden;
}
.contract-shell {
  --cyan: 14 165 233;
  display: grid;
  gap: 18px;
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: auto;
  transition: opacity 160ms ease;
}
.contract-shell.loading {
  opacity: 0.55;
}
.contract-toolbar :deep(.ant-card-head) {
  min-height: 48px;
}
.step-rail {
  background: hsl(var(--card));
  border: 1px solid rgb(148 163 184 / 22%);
  border-radius: 0;
  padding: 22px 26px;
}
.onboarding-grid {
  display: grid;
  gap: 18px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.step-card {
  background: hsl(var(--card));
  border: 1px solid rgb(148 163 184 / 22%);
  border-radius: 0;
  box-shadow: none;
  display: flex;
  flex-direction: column;
  min-height: 350px;
  padding: 24px;
}
.step-card--accent {
  background: hsl(var(--card));
  border-color: rgb(14 165 233 / 24%);
}
.step-heading {
  align-items: center;
  display: flex;
  gap: 13px;
}
.step-heading > b {
  color: rgb(var(--cyan));
  font-family: ui-monospace, monospace;
  font-size: 12px;
}
.step-heading small {
  color: hsl(var(--muted-foreground));
  font-family: ui-monospace, monospace;
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.step-heading h2 {
  color: hsl(var(--foreground));
  font-size: 18px;
  margin: 2px 0 0;
}
.step-description {
  color: hsl(var(--muted-foreground));
  font-size: 13px;
  line-height: 1.75;
  margin: 20px 0;
}
.profile-list,
.readiness-list,
.capability-list {
  display: grid;
  gap: 0;
  margin: 0;
}
.profile-list > div,
.readiness-list > div,
.capability-list > div {
  align-items: center;
  border-bottom: 1px dashed rgb(148 163 184 / 25%);
  display: flex;
  gap: 18px;
  justify-content: space-between;
  min-height: 48px;
}
.profile-list dt,
.readiness-list span,
.capability-list span {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}
.profile-list dd {
  color: hsl(var(--foreground));
  font-size: 13px;
  font-weight: 750;
  margin: 0;
  text-align: right;
}
.privacy-note,
.owner-note {
  background: hsl(var(--muted) / 74%);
  border-radius: 10px;
  color: hsl(var(--muted-foreground));
  font-size: 11px;
  line-height: 1.65;
  margin-top: auto;
  padding: 12px 14px;
}
.role-callout {
  border-left: 3px solid rgb(var(--cyan));
  display: grid;
  gap: 5px;
  margin: 4px 0 22px;
  padding: 5px 0 5px 14px;
}
.role-callout strong {
  color: hsl(var(--foreground));
  font-size: 13px;
}
.role-callout span,
.button-note {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
  line-height: 1.6;
}
.button-note {
  margin-top: 10px;
  text-align: center;
}
.readiness-list,
.capability-list {
  margin-bottom: 22px;
}
@media (max-width: 920px) {
  .onboarding-grid {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 640px) {
  .contract-tabs :deep(.ant-tabs-nav) {
    padding-inline: 8px;
  }
  .contract-tabs :deep(.ant-tabs-nav-list) {
    padding-left: 0;
  }
  .step-rail {
    padding: 20px 16px;
  }
  .step-card {
    min-height: auto;
    padding: 20px;
  }
  .profile-list > div {
    align-items: flex-start;
    flex-direction: column;
    gap: 4px;
    padding: 10px 0;
  }
  .profile-list dd {
    text-align: left;
  }
}
</style>

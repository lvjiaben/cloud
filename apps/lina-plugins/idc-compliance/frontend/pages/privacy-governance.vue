<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/compliance/privacy",
  title: "Privacy Governance",
};
</script>

<script setup lang="ts">
import type {
  AssessmentItem,
  DeletionItem,
  IncidentItem,
  RetentionItem,
} from "./privacy-governance-client";
import { onMounted, reactive, ref } from "vue";
import { useAccess } from "./plugin-access";
import { Page } from "@vben/common-ui";
import {
  Alert,
  Button,
  Card,
  Input,
  InputNumber,
  Modal as AntModal,
  Select,
  Spin,
  TabPane,
  Tabs,
  Tag,
  message,
} from "ant-design-vue";
import dayjs from "dayjs";
import { $t } from "#/locales";
import VbenModalCompat from "./components/vben-modal-compat.vue";
import {
  advanceIncident,
  confirmDeletion,
  createIncident,
  generateSupportExport,
  listAssessments,
  listDeletion,
  listIncidents,
  listRetention,
  publishAssessment,
  saveAssessmentDraft,
  scanDeletion,
  setRetention,
} from "./privacy-governance-client";

const { hasAccessByCodes } = useAccess();
const canConfigure = hasAccessByCodes([
  "idc:admin:compliance:privacy:configure",
]);
const canIncident = hasAccessByCodes(["idc:admin:compliance:privacy:incident"]);
const loading = ref(true);
const active = ref("assessments");
const assessments = ref<AssessmentItem[]>([]);
const retention = ref<RetentionItem[]>([]);
const deletion = ref<DeletionItem[]>([]);
const incidents = ref<IncidentItem[]>([]);
const retentionDays = reactive<Record<string, number>>({});
const assessmentTypes = [
  "personal_verification",
  "enterprise_verification",
  "electronic_contract",
  "tax_invoice",
  "local_sensitive_export",
  "ai_safe_projection",
];
const dataTypes = [
  "personal_verification_material",
  "enterprise_verification_material",
  "electronic_contract_evidence",
  "tax_invoice_evidence",
  "sensitive_access_audit",
  "local_sensitive_export",
];
const aiFields = [
  "personal_verification_status",
  "enterprise_verification_status",
  "contract_status",
  "tax_invoice_status",
  "aggregate_case_count",
  "aggregate_contract_count",
  "aggregate_invoice_count",
];
const draft = reactive<Record<string, any>>({
  assessmentTypeCode: "personal_verification",
  expectedRowVersion: 0,
  aiAllowedFields: [],
});
const draftOpen = ref(false);
const incidentOpen = ref(false);
const incidentForm = reactive({
  severityCode: "medium",
  sourceCode: "manual_report",
  safeSummary: "",
});
const textFields = [
  "purpose",
  "necessity",
  "impact",
  "recipients",
  "storageLocation",
  "retentionDescription",
  "rightsEntry",
  "merchantRole",
  "thirdPartyRole",
  "processingInstruction",
  "subcontractingBoundary",
  "auditEvidenceDescription",
];
const nextStage: Record<string, string> = {
  discovered: "contained",
  contained: "assessed",
  assessed: "notification_pending",
  notification_pending: "notification_confirmed",
  notification_confirmed: "remediating",
  remediating: "resolved",
};
const key = (prefix: string) => `${prefix}-${crypto.randomUUID()}`;
const label = (scope: string, code: string) =>
  $t(`plugin.idc-compliance.privacy.${scope}.${code}`);
const formatTime = (value: number) => dayjs(value).format("YYYY-MM-DD HH:mm");

async function load() {
  loading.value = true;
  try {
    const [a, r, d, i] = await Promise.all([
      listAssessments(),
      listRetention(),
      listDeletion(),
      listIncidents(),
    ]);
    assessments.value = a.items;
    retention.value = r.items;
    deletion.value = d.items;
    incidents.value = i.items;
    for (const item of r.items)
      retentionDays[item.dataTypeCode] = item.retentionDays;
  } finally {
    loading.value = false;
  }
}
function editAssessment(type: string) {
  const existing = assessments.value.find(
    (item) => item.assessmentTypeCode === type && item.statusCode === "draft",
  );
  Object.assign(draft, {
    assessmentTypeCode: type,
    expectedRowVersion: existing?.rowVersion || 0,
    aiAllowedFields: existing?.aiAllowedFields || [],
  });
  for (const field of textFields)
    draft[field] = existing?.[field as keyof AssessmentItem] || "";
  draftOpen.value = true;
}
async function saveDraft() {
  await saveAssessmentDraft(draft as any);
  message.success($t("plugin.idc-compliance.privacy.messages.saved"));
  draftOpen.value = false;
  await load();
}
function publish(item: AssessmentItem) {
  AntModal.confirm({
    title: $t("plugin.idc-compliance.privacy.publish.confirmTitle"),
    content: $t("plugin.idc-compliance.privacy.publish.confirmText"),
    async onOk() {
      await publishAssessment({
        assessmentNo: item.assessmentNo,
        expectedRowVersion: item.rowVersion,
        confirmed: true,
        idempotencyKey: key("privacy-publish"),
      });
      message.success($t("plugin.idc-compliance.privacy.messages.published"));
      await load();
    },
  });
}
async function saveRetention(type: string) {
  await setRetention({
    dataTypeCode: type,
    retentionDays: retentionDays[type] || 0,
    idempotencyKey: key("privacy-retention"),
  });
  message.success($t("plugin.idc-compliance.privacy.messages.saved"));
  await load();
}
async function scan() {
  const result = await scanDeletion();
  message.success(
    $t("plugin.idc-compliance.privacy.messages.scanned", {
      count: result.created,
    }),
  );
  await load();
}
function deleteOriginal(item: DeletionItem) {
  AntModal.confirm({
    title: $t("plugin.idc-compliance.privacy.deletion.confirmTitle"),
    content: $t("plugin.idc-compliance.privacy.deletion.confirmText"),
    async onOk() {
      await confirmDeletion(item, key("privacy-delete"));
      await load();
    },
  });
}
async function submitIncident() {
  await createIncident({
    ...incidentForm,
    idempotencyKey: key("privacy-incident"),
  });
  incidentOpen.value = false;
  incidentForm.safeSummary = "";
  await load();
}
function advance(item: IncidentItem) {
  const stage = nextStage[item.statusCode];
  if (!stage) return;
  AntModal.confirm({
    title: label("stage", stage),
    content: $t("plugin.idc-compliance.privacy.incident.advanceText"),
    async onOk() {
      await advanceIncident(
        item,
        stage,
        $t("plugin.idc-compliance.privacy.incident.defaultNote"),
        key("privacy-stage"),
      );
      await load();
    },
  });
}
function supportExport() {
  AntModal.confirm({
    title: $t("plugin.idc-compliance.privacy.support.confirmTitle"),
    content: $t("plugin.idc-compliance.privacy.support.confirmText"),
    async onOk() {
      const result = await generateSupportExport(key("privacy-support"));
      const url = URL.createObjectURL(
        new Blob([result.content], { type: "application/json" }),
      );
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = result.fileName;
      anchor.click();
      URL.revokeObjectURL(url);
    },
  });
}
onMounted(load);
</script>

<template>
  <Page
    :auto-content-height="true"
    class="privacy-page"
    data-testid="privacy-governance-page"
    ><div class="privacy-admin">
      <header class="page-heading">
        <span>{{ $t("plugin.idc-compliance.privacy.eyebrow") }}</span>
        <h1>{{ $t("plugin.idc-compliance.privacy.title") }}</h1>
        <p>{{ $t("plugin.idc-compliance.privacy.subtitle") }}</p>
      </header>
      <section class="governance-panel">
        <div class="status-toolbar">
          <Tag color="green">{{
            $t("plugin.idc-compliance.privacy.crossBorderClosed")
          }}</Tag>
          <Button @click="load">{{
            $t("plugin.idc-compliance.privacy.actions.refresh")
          }}</Button>
        </div>
        <Alert
          show-icon
          type="info"
          :message="$t('plugin.idc-compliance.privacy.localOnly')"
        /><Spin :spinning="loading"
          ><Tabs v-model:active-key="active">
            <TabPane
              key="assessments"
              :tab="$t('plugin.idc-compliance.privacy.tabs.assessments')"
              ><div class="grid">
                <Card
                  v-for="type in assessmentTypes"
                  :key="type"
                  :title="label('assessment', type)"
                  ><template #extra
                    ><Button
                      v-if="canConfigure"
                      size="small"
                      @click="editAssessment(type)"
                      >{{
                        $t("plugin.idc-compliance.privacy.actions.edit")
                      }}</Button
                    ></template
                  >
                  <p>
                    {{
                      assessments.find(
                        (i) =>
                          i.assessmentTypeCode === type &&
                          i.statusCode === "published",
                      )?.purpose ||
                      $t("plugin.idc-compliance.privacy.notPublished")
                    }}
                  </p>
                  <Button
                    v-if="
                      canConfigure &&
                      assessments.find(
                        (i) =>
                          i.assessmentTypeCode === type &&
                          i.statusCode === 'draft',
                      )
                    "
                    type="primary"
                    @click="
                      publish(
                        assessments.find(
                          (i) =>
                            i.assessmentTypeCode === type &&
                            i.statusCode === 'draft',
                        )!,
                      )
                    "
                    >{{
                      $t("plugin.idc-compliance.privacy.actions.publish")
                    }}</Button
                  ></Card
                >
              </div></TabPane
            >
            <TabPane
              key="retention"
              :tab="$t('plugin.idc-compliance.privacy.tabs.retention')"
              ><Card v-for="type in dataTypes" :key="type" class="row"
                ><strong>{{ label("dataType", type) }}</strong
                ><InputNumber
                  v-model:value="retentionDays[type]"
                  :min="1"
                /><span>{{ $t("plugin.idc-compliance.privacy.days") }}</span
                ><Button v-if="canConfigure" @click="saveRetention(type)">{{
                  $t("plugin.idc-compliance.privacy.actions.save")
                }}</Button></Card
              ></TabPane
            >
            <TabPane
              key="deletion"
              :tab="$t('plugin.idc-compliance.privacy.tabs.deletion')"
              ><Button v-if="canConfigure" @click="scan">{{
                $t("plugin.idc-compliance.privacy.actions.scan")
              }}</Button
              ><Card v-for="item in deletion" :key="item.taskNo" class="row"
                ><div>
                  <strong>{{ label("dataType", item.dataTypeCode) }}</strong>
                  <p>{{ item.targetNo }} · {{ formatTime(item.dueAt) }}</p>
                </div>
                <Tag>{{ label("deletionStatus", item.statusCode) }}</Tag
                ><Button
                  v-if="canConfigure && item.statusCode === 'pending'"
                  danger
                  @click="deleteOriginal(item)"
                  >{{
                    $t("plugin.idc-compliance.privacy.actions.confirmDelete")
                  }}</Button
                ></Card
              ></TabPane
            >
            <TabPane
              key="incidents"
              :tab="$t('plugin.idc-compliance.privacy.tabs.incidents')"
              ><div class="toolbar">
                <Button
                  v-if="canIncident"
                  danger
                  @click="incidentOpen = true"
                  >{{
                    $t("plugin.idc-compliance.privacy.actions.reportIncident")
                  }}</Button
                ><Button v-if="canConfigure" @click="supportExport">{{
                  $t("plugin.idc-compliance.privacy.actions.supportExport")
                }}</Button>
              </div>
              <Card v-for="item in incidents" :key="item.incidentNo"
                ><template #title>{{ item.safeSummary }}</template
                ><Tag color="red">{{ label("stage", item.statusCode) }}</Tag>
                <p>{{ formatTime(item.createdAt) }}</p>
                <Button
                  v-if="canIncident && nextStage[item.statusCode]"
                  @click="advance(item)"
                  >{{
                    $t("plugin.idc-compliance.privacy.actions.nextStage")
                  }}：{{ label("stage", nextStage[item.statusCode]) }}</Button
                ></Card
              ></TabPane
            >
          </Tabs></Spin
        >
      </section>
    </div>
    <VbenModalCompat
      v-model:open="draftOpen"
      :title="label('assessment', draft.assessmentTypeCode)"
      @ok="saveDraft"
      ><div class="form">
        <label v-for="field in textFields" :key="field"
          ><span>{{ label("field", field) }}</span
          ><Input.TextArea v-model:value="draft[field]" :rows="2" /></label
        ><label v-if="draft.assessmentTypeCode === 'ai_safe_projection'"
          ><span>{{
            $t("plugin.idc-compliance.privacy.field.aiAllowedFields")
          }}</span
          ><Select
            v-model:value="draft.aiAllowedFields"
            mode="multiple"
            :options="
              aiFields.map((v) => ({ label: label('aiField', v), value: v }))
            "
        /></label></div
    ></VbenModalCompat>
    <VbenModalCompat
      v-model:open="incidentOpen"
      :title="$t('plugin.idc-compliance.privacy.actions.reportIncident')"
      @ok="submitIncident"
      ><div class="form">
        <Select
          v-model:value="incidentForm.severityCode"
          :options="
            ['low', 'medium', 'high', 'critical'].map((v) => ({
              label: label('severity', v),
              value: v,
            }))
          "
        /><Select
          v-model:value="incidentForm.sourceCode"
          :options="
            [
              'unauthorized_attempt',
              'bulk_download',
              'data_loss',
              'manual_report',
              'system_detection',
            ].map((v) => ({ label: label('source', v), value: v }))
          "
        /><Input.TextArea
          v-model:value="incidentForm.safeSummary"
          :rows="4"
        /></div
    ></VbenModalCompat>
  </Page>
</template>
<style scoped>
.privacy-page {
  width: 100%;
  min-width: 0;
  max-width: calc(100vw - 32px);
}
.privacy-admin {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 16px;
  padding: 20px;
  width: min(100%, calc(100vw - 32px));
  min-width: 0;
  max-width: 100%;
}
.privacy-admin > * {
  min-width: 0;
}
.grid,
.row,
.toolbar,
.privacy-admin :deep(.ant-card),
.privacy-admin :deep(.ant-tabs) {
  min-width: 0;
  max-width: 100%;
}
.privacy-admin :deep(.ant-card) {
  overflow-wrap: anywhere;
}
.page-heading {
  border-left: 3px solid #1677ff;
  min-width: 0;
  padding: 2px 0 2px 14px;
}
.page-heading span {
  color: #1677ff;
  display: block;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  margin-bottom: 4px;
}
.page-heading h1 {
  font-size: 20px;
  font-weight: 600;
  line-height: 1.35;
  margin: 0;
}
.page-heading p {
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
  margin: 6px 0 0;
  max-width: 880px;
}
.governance-panel {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  min-width: 0;
  padding: 20px;
}
.status-toolbar {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  margin-bottom: 16px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.row {
  margin: 10px 0;
}
.row :deep(.ant-card-body) {
  display: flex;
  align-items: center;
  gap: 12px;
  justify-content: space-between;
}
.toolbar {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
}
.form {
  display: grid;
  gap: 12px;
  max-height: 65vh;
  overflow: auto;
}
.form label {
  display: grid;
  gap: 5px;
}
@media (max-width: 760px) {
  .privacy-admin {
    padding: 16px;
    width: 100%;
  }
  .governance-panel {
    padding: 16px;
  }
  .status-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
  .grid {
    grid-template-columns: 1fr;
  }
  .row :deep(.ant-card-body) {
    flex-direction: column;
    align-items: flex-start;
  }
  .privacy-admin :deep(.ant-card-head) {
    padding-inline: 16px;
  }
  .privacy-admin :deep(.ant-card-head-wrapper) {
    align-items: flex-start;
    gap: 8px;
    flex-wrap: wrap;
  }
  .privacy-admin :deep(.ant-card-head-title) {
    overflow: visible;
    white-space: normal;
    text-overflow: clip;
  }
  .privacy-admin :deep(.ant-card-extra) {
    margin-inline-start: 0;
  }
  .toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>

<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/migration",
  title: "Migration checks",
};
</script>

<script setup lang="ts">
import type {
  MigrationBatch,
  MigrationConflict,
  MigrationPreflightResult,
  MigrationReport,
} from "./migration-client";
import type { VbenFormProps } from "#/adapter/form";
import type { VxeGridProps } from "#/adapter/vxe-table";

import { computed, h, onMounted, onBeforeUnmount, reactive, ref, watch } from "vue";
import { createNumberedPage } from "./numbered-page";

import { Page } from "@vben/common-ui";
import { preferences } from "@vben/preferences";

import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Empty,
  Input,
  message,
  Modal,
  Space,
  Tabs,
  TabPane,
  Tag,
} from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";

import {
  checkCutover,
  createBatch,
  decideConflict,
  getReport,
  listConflicts,
  reconcileBatch,
  rollbackBatch,
  runBatch,
  runPreflight,
} from "./migration-client";

const activeTab = ref("preflight");
const loading = ref(false);
const preflight = ref<MigrationPreflightResult>();
const batch = ref<MigrationBatch>();
const conflicts = ref<MigrationConflict[]>([]);
const conflictPage = ref({ currentPage: 1, pageSize: 10, total: 0 });
let evidenceSequence = 0;
const conflictPager = createNumberedPage<MigrationConflict>((state) => {
  conflicts.value = state.items;
  conflictPage.value = state;
  conflictGridApi.setGridOptions({ pagerConfig: { currentPage: state.currentPage, pageSize: state.pageSize, total: state.total, enabled: true } });
}, () => {
  evidenceSequence++;
  report.value = undefined;
});
const report = ref<MigrationReport>();
const lastResult = ref("");

const form = reactive({
  batchNo: `MIG-${new Date().toISOString().slice(0, 10).replaceAll("-", "")}`,
  charset: "UTF-8",
  currencies: ["CNY"],
  mode: "dry_run" as "dry_run" | "import",
  sourceSystem: "zjmf-finance",
  sourceVersion: "v10",
  timezone: "Asia/Shanghai",
  watermark: "",
});

const canCreate = computed(() => preflight.value?.supported === true);
const hasBatch = computed(() => Number(batch.value?.id || 0) > 0);

const capabilityLabel: Record<string, string> = {
  attachments: "attachments",
  balances: "balances",
  billing_invoice: "billingInvoice",
  catalog: "catalog",
  commission: "commission",
  credit: "credit",
  identity: "identity",
  orders: "orders",
  payments: "payments",
  refunds: "refunds",
  services: "services",
  tickets: "tickets",
};

const statusColor: Record<string, string> = {
  blocked: "error",
  cutover: "success",
  matched: "success",
  preflight: "processing",
  ready: "success",
  rolled_back: "default",
  running: "processing",
};

function sourceFormSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Select",
      componentProps: {
        options: [
          {
            label: $t("plugin.idc-zjmfmigrate.migration.sources.zjmf"),
            value: "zjmf-finance",
          },
          {
            label: $t("plugin.idc-zjmfmigrate.migration.sources.v10"),
            value: "v10",
          },
        ],
      },
      fieldName: "sourceSystem",
      label: $t("plugin.idc-zjmfmigrate.migration.fields.sourceSystem"),
      rules: z.string().min(1),
    },
    {
      component: "Input",
      fieldName: "sourceVersion",
      label: $t("plugin.idc-zjmfmigrate.migration.fields.sourceVersion"),
      rules: z.string().trim().min(1),
    },
    {
      component: "Input",
      fieldName: "charset",
      label: $t("plugin.idc-zjmfmigrate.migration.fields.charset"),
      rules: z.string().trim().min(1),
    },
    {
      component: "Input",
      fieldName: "timezone",
      label: $t("plugin.idc-zjmfmigrate.migration.fields.timezone"),
      rules: z.string().trim().min(1),
    },
    {
      component: "Select",
      componentProps: {
        mode: "tags",
        options: [{ label: "CNY", value: "CNY" }],
      },
      fieldName: "currencies",
      label: $t("plugin.idc-zjmfmigrate.migration.fields.currencies"),
      rules: z.array(z.string()).min(1),
    },
  ];
}

function batchFormSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Input",
      fieldName: "batchNo",
      label: $t("plugin.idc-zjmfmigrate.migration.fields.batchNo"),
      rules: z.string().trim().min(1),
    },
    {
      component: "RadioGroup",
      componentProps: {
        options: [
          {
            label: $t("plugin.idc-zjmfmigrate.migration.modes.dryRun"),
            value: "dry_run",
          },
          {
            label: $t("plugin.idc-zjmfmigrate.migration.modes.import"),
            value: "import",
          },
        ],
      },
      fieldName: "mode",
      label: $t("plugin.idc-zjmfmigrate.migration.fields.mode"),
      rules: z.enum(["dry_run", "import"]),
    },
  ];
}

function conflictColumns(): VxeGridProps<MigrationConflict>["columns"] {
  return [
    {
      field: "objectType",
      title: $t("plugin.idc-zjmfmigrate.migration.columns.objectType"),
    },
    {
      field: "sourceId",
      title: $t("plugin.idc-zjmfmigrate.migration.columns.sourceId"),
    },
    {
      field: "summary",
      minWidth: 260,
      title: $t("plugin.idc-zjmfmigrate.migration.columns.summary"),
    },
    {
      field: "decision",
      title: $t("plugin.idc-zjmfmigrate.migration.columns.decision"),
    },
    fixedRightActionColumn({
      field: "actions",
      slots: { default: "actions" },
      title: $t("plugin.idc-zjmfmigrate.migration.columns.actions"),
      width: 140,
    }),
  ];
}

const [SourceForm, sourceFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: sourceFormSchema(),
  showDefaultActions: false,
  wrapperClass: "grid-cols-1 md:grid-cols-2 gap-x-4",
});
const [BatchForm, batchFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: batchFormSchema(),
  showDefaultActions: false,
  wrapperClass: "grid-cols-1 md:grid-cols-2 gap-x-4",
});
const [ConflictGrid, conflictGridApi] = useVbenVxeGrid({
  gridEvents: { pageChange: ({ currentPage, pageSize }: { currentPage: number; pageSize: number }) => loadConflicts(currentPage, pageSize) },
  gridOptions: {
    columns: conflictColumns(),
    data: [],
    height: 420,
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0, pageSizes: [10, 20, 50, 100] },
    rowConfig: { keyField: "id" },
    showOverflow: true,
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-migration-conflicts",
  },
});

function translatedStatus(value?: string) {
  return $t(`plugin.idc-zjmfmigrate.migration.status.${value || "unknown"}`);
}

function translatedEvidence(value: string) {
  return $t(`plugin.idc-zjmfmigrate.migration.evidence.${value}`);
}

function translatedMessage(value: string) {
  const mapping: Record<string, string> = {
    "charset needs explicit conversion review": "charsetReview",
    "core mappings complete": "coreMappingsComplete",
    "no external side effects": "noExternalSideEffects",
    "source credentials are task-scoped and never persisted":
      "credentialsNotPersisted",
    "source freeze confirmed": "sourceFreezeConfirmed",
    "source system is not in the verified adapter matrix": "unsupportedSource",
    "source version is required": "sourceVersionRequired",
    "timezone is missing; timestamps require manual review": "timezoneReview",
    "unresolved conflicts or reconciliation differences":
      "unresolvedDifferences",
  };
  const key = mapping[value];
  return key ? $t(`plugin.idc-zjmfmigrate.migration.messages.${key}`) : value;
}

async function handlePreflight() {
  const validation = await sourceFormApi.validate();
  if (!validation.valid) return;
  const values = await sourceFormApi.getValues();
  form.charset = String(values.charset || "").trim();
  form.currencies = Array.isArray(values.currencies)
    ? values.currencies.map(String)
    : [];
  form.sourceSystem = String(values.sourceSystem || "").trim();
  form.sourceVersion = String(values.sourceVersion || "").trim();
  form.timezone = String(values.timezone || "").trim();
  loading.value = true;
  try {
    preflight.value = await runPreflight({
      charset: form.charset,
      currencies: form.currencies,
      sourceSystem: form.sourceSystem,
      sourceVersion: form.sourceVersion,
      timezone: form.timezone,
    });
    const resultMessage = preflight.value.supported
      ? $t("plugin.idc-zjmfmigrate.migration.messages.preflightPassed")
      : $t("plugin.idc-zjmfmigrate.migration.messages.preflightBlocked");
    if (preflight.value.supported) {
      message.success(resultMessage);
    } else {
      message.warning(resultMessage);
    }
  } finally {
    loading.value = false;
  }
}

async function handleCreateBatch() {
  const validation = await batchFormApi.validate();
  if (!validation.valid) return;
  const values = await batchFormApi.getValues();
  form.batchNo = String(values.batchNo || "").trim();
  form.mode = values.mode === "import" ? "import" : "dry_run";
  loading.value = true;
  try {
    batch.value = await createBatch({
      batchNo: form.batchNo,
      mode: form.mode,
      sourceSystem: form.sourceSystem,
      sourceVersion: form.sourceVersion,
    });
    activeTab.value = "execution";
    message.success(
      $t("plugin.idc-zjmfmigrate.migration.messages.batchCreated"),
    );
  } finally {
    loading.value = false;
  }
}

async function refreshBatchEvidence() {
  if (!batch.value) return;
  const sequence = ++evidenceSequence;
  const target = batch.value.id;
  const [, reportResult] = await Promise.all([
    loadConflicts(),
    getReport(target),
  ]);
  if (batch.value?.id === target && sequence === evidenceSequence) report.value = reportResult;
}

async function loadConflicts(currentPage = conflictPage.value.currentPage, pageSize = conflictPage.value.pageSize) {
  const target = batch.value?.id;
  if (!target) return;
  await conflictPager.query({ currentPage, pageSize }, async (page, size) => {
    const result = await listConflicts(target, page, size);
    return { items: result.list, total: result.total };
  });
}

watch(() => batch.value?.id, () => { evidenceSequence++; conflictPager.reset(); report.value = undefined; });
onBeforeUnmount(() => { evidenceSequence++; conflictPager.invalidate(); });

async function handleRun() {
  if (!batch.value) return;
  loading.value = true;
  try {
    const result = await runBatch(batch.value.id, form.watermark);
    batch.value.status = result.status;
    lastResult.value = $t(
      "plugin.idc-zjmfmigrate.migration.messages.runCompleted",
      { count: result.mapped },
    );
    await refreshBatchEvidence();
  } finally {
    loading.value = false;
  }
}

async function handleReconcile() {
  if (!batch.value) return;
  loading.value = true;
  try {
    const result = await reconcileBatch(batch.value.id);
    lastResult.value = $t(
      "plugin.idc-zjmfmigrate.migration.messages.reconcileCompleted",
      { count: result.differences },
    );
    await refreshBatchEvidence();
  } finally {
    loading.value = false;
  }
}

async function handleCutover() {
  if (!batch.value) return;
  loading.value = true;
  try {
    const result = await checkCutover(batch.value.id);
    lastResult.value = result.checks.map(translatedMessage).join("；");
    if (result.allowed) batch.value.status = "cutover";
    message[result.allowed ? "success" : "warning"](
      $t(
        result.allowed
          ? "plugin.idc-zjmfmigrate.migration.messages.cutoverPassed"
          : "plugin.idc-zjmfmigrate.migration.messages.cutoverBlocked",
      ),
    );
    await refreshBatchEvidence();
  } finally {
    loading.value = false;
  }
}

function handleRollback() {
  if (!batch.value) return;
  Modal.confirm({
    content: $t("plugin.idc-zjmfmigrate.migration.messages.rollbackConfirm"),
    title: $t("plugin.idc-zjmfmigrate.migration.actions.rollback"),
    async onOk() {
      const result = await rollbackBatch(batch.value!.id);
      batch.value!.status = result.status;
      await refreshBatchEvidence();
    },
  });
}

function handleConflict(record: MigrationConflict) {
  let reason = "";
  Modal.confirm({
    content: () =>
      h(Input.TextArea, {
        maxlength: 500,
        onChange: (event: Event) => {
          reason = (event.target as HTMLTextAreaElement).value;
        },
        placeholder: $t(
          "plugin.idc-zjmfmigrate.migration.fields.decisionReasonPlaceholder",
        ),
        rows: 3,
      }),
    title: $t("plugin.idc-zjmfmigrate.migration.actions.resolveConflict"),
    async onOk() {
      if (reason.trim().length < 2) {
        message.warning(
          $t("plugin.idc-zjmfmigrate.migration.messages.reasonRequired"),
        );
        return Promise.reject(new Error("reason required"));
      }
      await decideConflict(record.id, "keep_source", reason.trim());
      await refreshBatchEvidence();
    },
  });
}

watch(conflicts, (items) => conflictGridApi.setGridOptions({ data: items }), {
  deep: true,
});
watch(
  () => preferences.app.locale,
  async () => {
    sourceFormApi.setState({ schema: sourceFormSchema() });
    batchFormApi.setState({ schema: batchFormSchema() });
    conflictGridApi.setGridOptions({ columns: conflictColumns() });
  },
);
onMounted(async () => {
  await sourceFormApi.setValues({
    charset: form.charset,
    currencies: [...form.currencies],
    sourceSystem: form.sourceSystem,
    sourceVersion: form.sourceVersion,
    timezone: form.timezone,
  });
  await batchFormApi.setValues({ batchNo: form.batchNo, mode: form.mode });
});
</script>

<template>
  <Page :auto-content-height="true" class="migration-page">
    <div class="migration-shell">
      <div class="page-heading">
        <div>
          <h1>{{ $t("plugin.idc-zjmfmigrate.migration.title") }}</h1>
          <p>{{ $t("plugin.idc-zjmfmigrate.migration.description") }}</p>
        </div>
        <Tag color="blue">
          {{ $t("plugin.idc-zjmfmigrate.migration.readOnlyFirst") }}
        </Tag>
      </div>

      <Alert
        banner
        class="boundary-alert"
        show-icon
        type="info"
        :message="$t('plugin.idc-zjmfmigrate.migration.boundaryTitle')"
        :description="
          $t('plugin.idc-zjmfmigrate.migration.boundaryDescription')
        "
      />

      <Tabs v-model:active-key="activeTab" class="migration-tabs">
        <TabPane
          key="preflight"
          :tab="$t('plugin.idc-zjmfmigrate.migration.tabs.preflight')"
        >
          <div class="content-grid">
            <Card
              size="small"
              :title="$t('plugin.idc-zjmfmigrate.migration.sections.source')"
            >
              <SourceForm />
              <Button
                :loading="loading"
                type="primary"
                @click="handlePreflight"
              >
                {{ $t("plugin.idc-zjmfmigrate.migration.actions.preflight") }}
              </Button>
            </Card>

            <Card
              size="small"
              :title="$t('plugin.idc-zjmfmigrate.migration.sections.result')"
            >
              <Empty
                v-if="!preflight"
                :description="
                  $t('plugin.idc-zjmfmigrate.migration.empty.preflight')
                "
              />
              <template v-else>
                <Alert
                  show-icon
                  :type="preflight.supported ? 'success' : 'error'"
                  :message="
                    $t(
                      preflight.supported
                        ? 'plugin.idc-zjmfmigrate.migration.messages.preflightPassed'
                        : 'plugin.idc-zjmfmigrate.migration.messages.preflightBlocked',
                    )
                  "
                />
                <div v-if="preflight.blockers.length" class="result-list">
                  <strong>
                    {{ $t("plugin.idc-zjmfmigrate.migration.labels.blockers") }}
                  </strong>
                  <div v-for="item in preflight.blockers" :key="item">
                    {{ translatedMessage(item) }}
                  </div>
                </div>
                <div v-if="preflight.warnings.length" class="result-list">
                  <strong>
                    {{ $t("plugin.idc-zjmfmigrate.migration.labels.warnings") }}
                  </strong>
                  <div v-for="item in preflight.warnings" :key="item">
                    {{ translatedMessage(item) }}
                  </div>
                </div>
                <div class="result-list">
                  <strong>
                    {{
                      $t("plugin.idc-zjmfmigrate.migration.labels.capabilities")
                    }}
                  </strong>
                  <Space wrap>
                    <Tag v-for="item in preflight.capabilities" :key="item">
                      {{
                        $t(
                          `plugin.idc-zjmfmigrate.migration.capabilities.${capabilityLabel[item] || item}`,
                        )
                      }}
                    </Tag>
                  </Space>
                </div>
              </template>
            </Card>
          </div>

          <Card
            size="small"
            class="batch-card"
            :title="$t('plugin.idc-zjmfmigrate.migration.sections.batch')"
          >
            <div class="batch-form">
              <BatchForm class="batch-vben-form" />
              <Button
                :disabled="!canCreate"
                :loading="loading"
                type="primary"
                @click="handleCreateBatch"
              >
                {{ $t("plugin.idc-zjmfmigrate.migration.actions.createBatch") }}
              </Button>
            </div>
          </Card>
        </TabPane>

        <TabPane
          key="execution"
          :disabled="!hasBatch"
          :tab="$t('plugin.idc-zjmfmigrate.migration.tabs.execution')"
        >
          <Card v-if="batch" size="small">
            <Descriptions bordered size="small" :column="3">
              <DescriptionsItem
                :label="$t('plugin.idc-zjmfmigrate.migration.fields.batchNo')"
              >
                {{ batch.batchNo }}
              </DescriptionsItem>
              <DescriptionsItem
                :label="$t('plugin.idc-zjmfmigrate.migration.fields.batchId')"
              >
                {{ batch.id }}
              </DescriptionsItem>
              <DescriptionsItem
                :label="$t('plugin.idc-zjmfmigrate.migration.fields.status')"
              >
                <Tag :color="statusColor[batch.status]">
                  {{ translatedStatus(batch.status) }}
                </Tag>
              </DescriptionsItem>
            </Descriptions>
            <div class="execution-bar">
              <Input
                v-model:value="form.watermark"
                class="watermark"
                :placeholder="
                  $t(
                    'plugin.idc-zjmfmigrate.migration.fields.watermarkPlaceholder',
                  )
                "
              />
              <Space wrap>
                <Button type="primary" :loading="loading" @click="handleRun">
                  {{ $t("plugin.idc-zjmfmigrate.migration.actions.run") }}
                </Button>
                <Button :loading="loading" @click="handleReconcile">
                  {{ $t("plugin.idc-zjmfmigrate.migration.actions.reconcile") }}
                </Button>
                <Button :loading="loading" @click="handleCutover">
                  {{ $t("plugin.idc-zjmfmigrate.migration.actions.cutover") }}
                </Button>
                <Button
                  danger
                  :disabled="batch.status === 'cutover'"
                  @click="handleRollback"
                >
                  {{ $t("plugin.idc-zjmfmigrate.migration.actions.rollback") }}
                </Button>
              </Space>
            </div>
            <Alert
              v-if="lastResult"
              show-icon
              type="info"
              :message="lastResult"
            />
          </Card>
        </TabPane>

        <TabPane
          key="conflicts"
          :disabled="!hasBatch"
          :tab="$t('plugin.idc-zjmfmigrate.migration.tabs.conflicts')"
        >
          <ConflictGrid>
            <template #actions="{ row }">
              <Button
                v-if="row.decision === 'pending'"
                type="link"
                @click="handleConflict(row)"
              >
                {{
                  $t("plugin.idc-zjmfmigrate.migration.actions.resolveConflict")
                }}
              </Button>
            </template>
          </ConflictGrid>
        </TabPane>

        <TabPane
          key="report"
          :disabled="!hasBatch"
          :tab="$t('plugin.idc-zjmfmigrate.migration.tabs.report')"
        >
          <Empty
            v-if="!report"
            :description="$t('plugin.idc-zjmfmigrate.migration.empty.report')"
          />
          <Card v-else size="small">
            <Descriptions bordered size="small" :column="1">
              <DescriptionsItem
                :label="$t('plugin.idc-zjmfmigrate.migration.fields.status')"
              >
                <Tag :color="statusColor[report.status]">
                  {{ translatedStatus(report.status) }}
                </Tag>
              </DescriptionsItem>
              <DescriptionsItem
                v-for="(value, key) in report.byType"
                :key="key"
                :label="
                  $t(
                    `plugin.idc-zjmfmigrate.migration.capabilities.${capabilityLabel[key] || key}`,
                  )
                "
              >
                {{ translatedEvidence(value) }}
              </DescriptionsItem>
            </Descriptions>
          </Card>
        </TabPane>
      </Tabs>
    </div>
  </Page>
</template>

<style scoped>
.migration-shell {
  min-height: 100%;
  padding: 16px;
  background: hsl(var(--background));
}

.page-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 12px;
}

.page-heading h1 {
  margin: 0;
  color: hsl(var(--foreground));
  font-size: 20px;
  font-weight: 600;
  line-height: 28px;
}

.page-heading p {
  margin: 4px 0 0;
  color: hsl(var(--muted-foreground));
}

.boundary-alert,
.migration-tabs,
.batch-card {
  margin-top: 12px;
}

.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 3fr) minmax(320px, 2fr);
  gap: 12px;
}

.result-list {
  margin-top: 16px;
}

.result-list strong {
  display: block;
  margin-bottom: 8px;
}

.batch-form {
  display: flex;
  align-items: flex-end;
  gap: 16px;
}

.batch-vben-form {
  flex: 1;
  min-width: 0;
}

.batch-form :deep(.ant-form-item) {
  min-width: 240px;
  margin-bottom: 0;
}

.execution-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 16px 0;
}

.watermark {
  max-width: 360px;
}

:deep(.ant-card),
:deep(.ant-alert),
:deep(.ant-input),
:deep(.ant-select-selector),
:deep(.ant-btn) {
  border-radius: 0 !important;
}

@media (max-width: 960px) {
  .content-grid {
    grid-template-columns: 1fr;
  }

  .batch-form,
  .execution-bar {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>

<script lang="ts">
export const pluginPageMeta = {
  routePath: "system/staff-access",
  title: "Staff Permissions",
};
</script>

<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type {
  StaffAccessGrant,
  StaffAccessGrantInput,
  StaffCandidate,
} from "./staff-access-client";

import { computed, nextTick, onMounted, ref } from "vue";

import { useAccess } from "@vben/access";
import { Page, useVbenModal } from "@vben/common-ui";

import { Alert, Space, Tag, message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import {
  saveStaffAccessGrant,
  staffAccessGrantList,
  staffCandidateList,
} from "./staff-access-client";

const managePermission = "system:staff-access:manage";
const { hasAccessByCodes } = useAccess();
const canManage = computed(() => hasAccessByCodes([managePermission]));

const capabilityCodes = [
  "idc.support.handle",
  "idc.support.lead",
  "idc.support.internal-notify",
] as const;

const total = ref(0);
const editing = ref(false);
const candidatesLoading = ref(false);
const candidates = ref<StaffCandidate[]>([]);
let candidateRequestSequence = 0;
const candidateOptions = computed(() =>
  candidates.value.map((item) => ({
    label: item.nickname
      ? `${item.nickname} (${item.username})`
      : item.username,
    value: item.userId,
  })),
);
const capabilityOptions = computed(() =>
  capabilityCodes.map((value) => ({
    label: capabilityLabel(value),
    value,
  })),
);
const scopeOptions = computed(() => [
  {
    label: $t("plugin.staff-access.scope.department"),
    value: "department",
  },
  {
    label: $t("plugin.staff-access.scope.global"),
    value: "global",
  },
]);

const editingGrantVersion = ref(0);

function grantDefaults() {
  return {
    capabilityCode: capabilityCodes[0],
    enabled: true,
    reason: "",
    scopeKey: "",
    scopeType: "department",
    userId: null,
    validFrom: null,
    validUntil: null,
  };
}

function grantSchema(): VbenFormProps["schema"] {
  const requiredMessage = $t("plugin.staff-access.message.required");
  return [
    {
      component: "Select",
      componentProps: {
        filterOption: false,
        loading: candidatesLoading.value,
        onSearch: loadCandidates,
        options: candidateOptions.value,
        placeholder: $t("plugin.staff-access.form.employeePlaceholder"),
        showSearch: true,
      },
      fieldName: "userId",
      label: $t("plugin.staff-access.form.employee"),
      rules: z.preprocess(
        (value) => Number(value || 0),
        z.number().positive({ message: requiredMessage }),
      ),
    },
    {
      component: "Select",
      componentProps: { options: capabilityOptions.value },
      fieldName: "capabilityCode",
      label: $t("plugin.staff-access.form.duty"),
      rules: z.string().min(1, { message: requiredMessage }),
    },
    {
      component: "Select",
      componentProps: { options: scopeOptions.value },
      fieldName: "scopeType",
      label: $t("plugin.staff-access.form.scope"),
      rules: z.string().min(1, { message: requiredMessage }),
    },
    {
      component: "Input",
      componentProps: {
        placeholder: $t("plugin.staff-access.form.departmentPlaceholder"),
      },
      dependencies: {
        show: (values) => values.scopeType === "department",
        triggerFields: ["scopeType"],
      },
      description: $t("plugin.staff-access.form.departmentHelp"),
      fieldName: "scopeKey",
      label: $t("plugin.staff-access.form.departmentCode"),
    },
    {
      component: "DatePicker",
      componentProps: { class: "w-full", showTime: true, valueFormat: "x" },
      fieldName: "validFrom",
      label: $t("plugin.staff-access.form.validFrom"),
    },
    {
      component: "DatePicker",
      componentProps: { class: "w-full", showTime: true, valueFormat: "x" },
      fieldName: "validUntil",
      label: $t("plugin.staff-access.form.validUntil"),
    },
    {
      component: "Switch",
      componentProps: {
        checkedChildren: $t("plugin.staff-access.status.enabled"),
        unCheckedChildren: $t("plugin.staff-access.status.disabled"),
      },
      fieldName: "enabled",
      label: $t("plugin.staff-access.form.status"),
    },
    {
      component: "Textarea",
      componentProps: {
        maxlength: 500,
        placeholder: $t("plugin.staff-access.form.reasonPlaceholder"),
        rows: 3,
        showCount: true,
      },
      fieldName: "reason",
      formItemClass: "md:col-span-2",
      label: $t("plugin.staff-access.form.reason"),
      rules: z.string().trim().min(1, { message: requiredMessage }),
    },
  ];
}

const [GrantForm, grantFormApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: "w-full" },
    labelWidth: 118,
  },
  schema: grantSchema(),
  showDefaultActions: false,
  wrapperClass: "grid-cols-1 md:grid-cols-2 gap-x-4",
});

function capabilityLabel(code: string) {
  const suffix = code.replace("idc.support.", "").replaceAll("-", "_");
  return $t(`plugin.staff-access.capability.${suffix}`);
}

function scopeLabel(item: StaffAccessGrant) {
  if (item.scopeType === "global") {
    return $t("plugin.staff-access.scope.global");
  }
  return $t("plugin.staff-access.scope.departmentValue", {
    code: item.scopeKey,
  });
}

function validityLabel(item: StaffAccessGrant) {
  if (!item.validFrom && !item.validUntil) {
    return $t("plugin.staff-access.validity.unlimited");
  }
  return $t("plugin.staff-access.validity.range", {
    from: item.validFrom
      ? formatTimestamp(item.validFrom)
      : $t("plugin.staff-access.validity.now"),
    until: item.validUntil
      ? formatTimestamp(item.validUntil)
      : $t("plugin.staff-access.validity.noExpiry"),
  });
}

function searchSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Input",
      componentProps: {
        placeholder: $t("plugin.staff-access.search.keywordPlaceholder"),
      },
      fieldName: "keyword",
      label: $t("plugin.staff-access.search.keyword"),
    },
    {
      component: "Select",
      componentProps: {
        options: [
          {
            label: $t("plugin.staff-access.search.allDuties"),
            value: "",
          },
          ...capabilityCodes.map((value) => ({
            label: capabilityLabel(value),
            value,
          })),
        ],
      },
      defaultValue: "",
      fieldName: "capabilityCode",
      label: $t("plugin.staff-access.search.duty"),
    },
    {
      component: "Select",
      componentProps: {
        options: [
          {
            label: $t("plugin.staff-access.search.allStatuses"),
            value: -1,
          },
          {
            label: $t("plugin.staff-access.status.enabled"),
            value: 1,
          },
          {
            label: $t("plugin.staff-access.status.disabled"),
            value: 0,
          },
        ],
      },
      defaultValue: -1,
      fieldName: "status",
      label: $t("plugin.staff-access.search.status"),
    },
  ];
}

function columns(): VxeGridProps["columns"] {
  const result: VxeGridProps["columns"] = [
    {
      field: "username",
      minWidth: 180,
      slots: { default: "employee" },
      title: $t("plugin.staff-access.table.employee"),
    },
    {
      field: "capabilityCode",
      minWidth: 180,
      slots: { default: "duty" },
      title: $t("plugin.staff-access.table.duty"),
    },
    {
      field: "scopeType",
      minWidth: 180,
      slots: { default: "scope" },
      title: $t("plugin.staff-access.table.scope"),
    },
    {
      field: "status",
      slots: { default: "status" },
      title: $t("plugin.staff-access.table.status"),
      width: 110,
    },
    {
      field: "validUntil",
      minWidth: 240,
      slots: { default: "validity" },
      title: $t("plugin.staff-access.table.validity"),
    },
    {
      field: "updatedAt",
      minWidth: 180,
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      title: $t("plugin.staff-access.table.updatedAt"),
    },
  ];
  if (canManage.value) {
    result.push({
      field: "action",
      fixed: "right",
      slots: { default: "action" },
      title: $t("plugin.staff-access.table.action"),
      width: 100,
    });
  }
  return result;
}

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: {
      componentProps: { allowClear: true },
      labelWidth: 88,
    },
    schema: searchSchema(),
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-3",
  },
  gridOptions: {
    columns: columns(),
    height: "100%",
    pagerConfig: {},
    proxyConfig: {
      ajax: {
        query: async (
          { page }: { page: { currentPage: number; pageSize: number } },
          values: Record<string, unknown> = {},
        ) => {
          const result = await staffAccessGrantList({
            capabilityCode: String(values.capabilityCode || "") || undefined,
            keyword: String(values.keyword || "").trim() || undefined,
            page: page.currentPage,
            pageSize: page.pageSize,
            status: Number(values.status ?? -1),
          });
          total.value = result.total;
          return result;
        },
      },
    },
    rowConfig: { keyField: "grantNo" },
  },
});

async function updateCandidateField() {
  await grantFormApi.updateSchema([
    {
      componentProps: {
        filterOption: false,
        loading: candidatesLoading.value,
        onSearch: loadCandidates,
        options: candidateOptions.value,
        placeholder: $t("plugin.staff-access.form.employeePlaceholder"),
        showSearch: true,
      },
      fieldName: "userId",
    },
  ]);
}

async function loadCandidates(keyword = "") {
  const requestSequence = ++candidateRequestSequence;
  candidatesLoading.value = true;
  await updateCandidateField();
  try {
    const result = await staffCandidateList({
      keyword: keyword.trim() || undefined,
      page: 1,
      pageSize: 50,
    });
    if (requestSequence === candidateRequestSequence) {
      candidates.value = result.items;
    }
  } finally {
    if (requestSequence === candidateRequestSequence) {
      candidatesLoading.value = false;
      await updateCandidateField();
    }
  }
}

async function openCreate() {
  editing.value = false;
  editingGrantVersion.value = 0;
  candidates.value = [];
  grantModalApi.open();
  await nextTick();
  await grantFormApi.resetForm();
  await grantFormApi.setValues(grantDefaults());
  await loadCandidates();
}

async function openEdit(item: StaffAccessGrant) {
  editing.value = true;
  editingGrantVersion.value = item.version;
  candidates.value = [
    {
      nickname: item.nickname,
      userId: item.userId,
      username: item.username,
    },
  ];
  grantModalApi.open();
  await nextTick();
  await updateCandidateField();
  await grantFormApi.resetForm();
  await grantFormApi.setValues({
    capabilityCode: item.capabilityCode,
    enabled: item.status === 1,
    reason: "",
    scopeKey: item.scopeKey,
    scopeType: item.scopeType,
    userId: item.userId,
    validFrom: item.validFrom ?? null,
    validUntil: item.validUntil ?? null,
  });
  await loadCandidates(item.nickname || item.username);
}

async function submit() {
  const { valid } = await grantFormApi.validate();
  if (!valid) return false;
  const values =
    await grantFormApi.getValues<
      Omit<StaffAccessGrantInput, "expectedVersion">
    >();
  const scopeType = values.scopeType === "global" ? "global" : "department";
  const scopeKey = String(values.scopeKey || "").trim();
  if (scopeType === "department" && !scopeKey) {
    message.warning($t("plugin.staff-access.message.departmentRequired"));
    return false;
  }
  if (
    values.validFrom &&
    values.validUntil &&
    Number(values.validUntil) <= Number(values.validFrom)
  ) {
    message.warning($t("plugin.staff-access.message.invalidValidity"));
    return false;
  }

  grantModalApi.lock(true);
  try {
    await saveStaffAccessGrant({
      capabilityCode: values.capabilityCode,
      enabled: values.enabled !== false,
      expectedVersion: editingGrantVersion.value,
      reason: String(values.reason || "").trim(),
      scopeKey: scopeType === "global" ? "" : scopeKey,
      scopeType,
      userId: Number(values.userId),
      validFrom: values.validFrom ? Number(values.validFrom) : null,
      validUntil: values.validUntil ? Number(values.validUntil) : null,
    });
    message.success($t("plugin.staff-access.message.saved"));
    grantModalApi.close();
    await gridApi.query();
    return true;
  } finally {
    grantModalApi.lock(false);
  }
}

const grantModalTitle = computed(() =>
  editing.value
    ? $t("plugin.staff-access.form.editTitle")
    : $t("plugin.staff-access.form.createTitle"),
);

const [GrantModal, grantModalApi] = useVbenModal({
  async onClosed() {
    candidateRequestSequence += 1;
    candidates.value = [];
    candidatesLoading.value = false;
    editing.value = false;
    editingGrantVersion.value = 0;
    await grantFormApi.resetForm();
  },
  onConfirm: submit,
});

onMounted(() => {
  gridApi.setGridOptions({ columns: columns() });
  gridApi.formApi.setState({ schema: searchSchema() });
});
</script>

<template>
  <Page
    :auto-content-height="true"
    class="staff-access-page"
    data-testid="staff-access-page"
  >
    <Grid :table-title="$t('plugin.staff-access.title')">
      <template #toolbar-tools>
        <Space>
          <span class="staff-access-total-label">
            {{ $t("plugin.staff-access.table.total") }}
            <strong data-testid="staff-access-total">{{ total }}</strong>
          </span>
          <a-button v-if="canManage" type="primary" @click="openCreate">
            {{ $t("plugin.staff-access.action.create") }}
          </a-button>
        </Space>
      </template>

      <template #employee="{ row }">
        <div class="staff-access-employee">
          <strong>{{ row.nickname || row.username }}</strong>
          <span v-if="row.nickname">{{ row.username }}</span>
        </div>
      </template>
      <template #duty="{ row }">
        {{ capabilityLabel(row.capabilityCode) }}
      </template>
      <template #scope="{ row }">
        {{ scopeLabel(row) }}
      </template>
      <template #status="{ row }">
        <Tag :color="row.status === 1 ? 'green' : 'default'">
          {{
            row.status === 1
              ? $t("plugin.staff-access.status.enabled")
              : $t("plugin.staff-access.status.disabled")
          }}
        </Tag>
      </template>
      <template #validity="{ row }">
        {{ validityLabel(row) }}
      </template>
      <template #action="{ row }">
        <a-button size="small" type="link" @click="openEdit(row)">
          {{ $t("plugin.staff-access.action.edit") }}
        </a-button>
      </template>
    </Grid>

    <GrantModal
      class="staff-access-modal top-6 max-h-[calc(100dvh-48px)] w-[720px] max-w-[calc(100vw-32px)] rounded-none!"
      :cancel-text="$t('plugin.staff-access.action.cancel')"
      :confirm-text="$t('plugin.staff-access.action.save')"
      :title="grantModalTitle"
    >
      <Alert
        class="staff-access-security-hint"
        :message="$t('plugin.staff-access.form.securityHint')"
        show-icon
        type="info"
      />
      <GrantForm />
    </GrantModal>
  </Page>
</template>

<style scoped>
.staff-access-total-label {
  color: hsl(var(--muted-foreground));
  font-size: 13px;
}

.staff-access-total-label strong {
  color: hsl(var(--foreground));
  margin-left: 6px;
}

.staff-access-employee {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.staff-access-employee span {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.staff-access-security-hint {
  margin-bottom: 18px;
}

:global(.staff-access-modal),
:global(.staff-access-modal .ant-alert),
:deep(.vxe-grid),
:deep(.vxe-toolbar) {
  border-radius: 0;
}
</style>

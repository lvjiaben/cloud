<script setup lang="ts">
import type {
  ContractPolicyCode,
  ContractPolicyLevel,
  ContractPolicyOrderRule,
  ContractPolicyRule,
  ContractPolicyTemplate,
} from "../contract-policy-client";

import { computed, ref, watch } from "vue";

import { Alert, Button, Select, Tag, message } from "ant-design-vue";

import { $t } from "#/locales";

import {
  catalogContractPolicyUpdate,
  contractPolicyTemplates,
  customerContractPolicyUpdate,
  orderContractPolicyUpdate,
} from "../contract-policy-client";

const props = defineProps<{
  disabled?: boolean;
  level: ContractPolicyLevel;
  policyCode: ContractPolicyCode;
  targetNo: string;
  templateNo: string;
  version: number;
}>();

const emit = defineEmits<{
  updated: [rule: ContractPolicyOrderRule | ContractPolicyRule];
}>();

const loading = ref(false);
const saving = ref(false);
const loadFailed = ref(false);
const templates = ref<ContractPolicyTemplate[]>([]);
const policyCode = ref<ContractPolicyCode>(props.policyCode);
const templateNo = ref(props.templateNo);

const policyOptions = computed(() => {
  const values: ContractPolicyCode[] =
    props.level === "enterprise"
      ? ["none", "required"]
      : ["inherit", "none", "required"];
  return values.map((value) => ({
    label: $t(`plugin.idc-core.contractPolicy.policy.${value}`),
    value,
  }));
});

const templateOptions = computed(() =>
  templates.value.map((item) => ({
    label: `${item.name} · ${$t(`plugin.idc-core.contractPolicy.language.${item.languageCode}`)} · ${$t(`plugin.idc-core.contractPolicy.scope.${item.scopeCode}`)}`,
    value: item.templateNo,
  })),
);

const canSave = computed(
  () =>
    !props.disabled &&
    props.version > 0 &&
    (!saving.value &&
      (policyCode.value !== "required" || Boolean(templateNo.value))),
);

watch(
  () => [props.policyCode, props.templateNo, props.version] as const,
  ([nextPolicy, nextTemplate]) => {
    policyCode.value = nextPolicy;
    templateNo.value = nextTemplate;
  },
);

watch(policyCode, (value) => {
  if (value !== "required") templateNo.value = "";
  if (value === "required" && templates.value.length === 0) void loadTemplates();
});

watch(
  () => [props.level, props.targetNo] as const,
  () => {
    templates.value = [];
    if (policyCode.value === "required") void loadTemplates();
  },
);

async function loadTemplates() {
  if (loading.value || !props.targetNo && props.level !== "product") return;
  loading.value = true;
  loadFailed.value = false;
  try {
    const result = await contractPolicyTemplates(props.level, props.targetNo);
    templates.value = result.templates;
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (!canSave.value) return;
  saving.value = true;
  try {
    const input = {
      expectedVersion: props.version,
      policyCode: policyCode.value,
      templateNo: policyCode.value === "required" ? templateNo.value : "",
    };
    let result: { rule: ContractPolicyOrderRule | ContractPolicyRule };
    if (props.level === "enterprise") {
      result = await customerContractPolicyUpdate(props.targetNo, {
        ...input,
        policyCode: input.policyCode as "none" | "required",
      });
    } else if (props.level === "product") {
      result = await catalogContractPolicyUpdate(props.targetNo, input);
    } else {
      result = await orderContractPolicyUpdate(props.targetNo, input);
    }
    policyCode.value = result.rule.policyCode;
    templateNo.value = result.rule.templateNo;
    emit("updated", result.rule);
    message.success($t("plugin.idc-core.contractPolicy.saved"));
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <section class="contract-policy-card" data-testid="contract-policy-editor">
    <div class="policy-heading">
      <div>
        <strong>{{ $t("plugin.idc-core.contractPolicy.title") }}</strong>
        <small>{{ $t(`plugin.idc-core.contractPolicy.help.${level}`) }}</small>
      </div>
      <Tag color="cyan">{{ $t(`plugin.idc-core.contractPolicy.level.${level}`) }}</Tag>
    </div>
    <div class="policy-fields">
      <label>
        <span>{{ $t("plugin.idc-core.contractPolicy.rule") }}</span>
        <Select
          v-model:value="policyCode"
          :disabled="disabled"
          :options="policyOptions"
          data-testid="contract-policy-code"
        />
      </label>
      <label v-if="policyCode === 'required'">
        <span>{{ $t("plugin.idc-core.contractPolicy.template") }}</span>
        <Select
          v-model:value="templateNo"
          :disabled="disabled"
          :loading="loading"
          :options="templateOptions"
          :placeholder="$t('plugin.idc-core.contractPolicy.templatePlaceholder')"
          data-testid="contract-policy-template"
          show-search
        />
      </label>
    </div>
    <Alert
      v-if="loadFailed"
      :message="$t('plugin.idc-core.contractPolicy.templateLoadFailed')"
      show-icon
      type="error"
    />
    <div class="policy-actions">
      <Button
        :disabled="!canSave"
        :loading="saving"
        data-testid="contract-policy-save"
        type="primary"
        @click="save"
      >
        {{ $t("plugin.idc-core.contractPolicy.save") }}
      </Button>
    </div>
  </section>
</template>

<style scoped>
.contract-policy-card {
  background: hsl(var(--muted) / 26%);
  border: 1px solid hsl(var(--border));
  border-left: 3px solid rgb(8 145 178);
  border-radius: 10px;
  display: grid;
  gap: 14px;
  padding: 16px;
}

.policy-heading,
.policy-actions {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.policy-heading > div,
.policy-fields label {
  display: grid;
  gap: 5px;
}

.policy-heading small,
.policy-fields span {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.policy-fields {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.policy-fields :deep(.ant-select) {
  width: 100%;
}

.policy-actions {
  justify-content: flex-end;
}

@media (max-width: 680px) {
  .policy-fields {
    grid-template-columns: 1fr;
  }
}
</style>

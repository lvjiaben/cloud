<script setup lang="ts">
import type { UploadRequestOption } from 'ant-design-vue/es/vc-upload/interface';
import type {
  PurchaseConfigurationView,
  PurchaseField,
  PurchaseFileItem,
  PurchaseValue,
} from '#/api/idc/purchase-configuration';

import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { Page } from '@vben/common-ui';
import { preferences } from '@vben/preferences';
import {
  Alert,
  Button,
  Card,
  Cascader,
  Checkbox,
  DatePicker,
  Form,
  FormItem,
  Input,
  message,
  Select,
  Spin,
  TreeSelect,
  Upload,
} from 'ant-design-vue';

import {
  evaluatePurchaseConfiguration,
  freezePurchaseConfiguration,
  getPurchaseConfiguration,
  quotePurchaseConfiguration,
  revokePurchaseConfigurationFile,
  uploadPurchaseConfigurationFile,
} from '#/api/idc/purchase-configuration';
import { $t } from '#/locales';
import { formatMinorAmount } from '#/utils/format-amount';
import { mapPurchaseOptions } from './purchase-options';
import {
  mergeAuthoritativeFileTokens,
  submissionSnapshotUnchanged,
} from './purchase-configuration-flow';

const UploadDragger = Upload.Dragger;
const route = useRoute();
const router = useRouter();
const loading = ref(true);
const loadFailed = ref(false);
const submitting = ref(false);
const evaluating = ref(false);
const configuration = ref<PurchaseConfigurationView>();
const values = reactive<Record<string, any>>({});
const files = reactive<Record<string, PurchaseFileItem[]>>({});
const fileTokens = reactive<Record<string, string[]>>({});
const quoteTerms = ref<Array<{ label: string; total: string }>>([]);
const issueTokens = ref(new Set<string>());
const removingTokens = ref(new Set<string>());
let evaluationTimer: ReturnType<typeof setTimeout> | undefined;
let evaluationSequence = 0;
let applyingAuthoritative = false;
let submissionChanged = false;
const publicSlug = computed(() => String(route.params.publicSlug || '').trim());
const quoteItemNo = computed(() =>
  String(route.query.quoteItemNo || '').trim(),
);
const currencyCode = computed(() =>
  String(route.query.currency || 'CNY').toUpperCase(),
);
const chinese = computed(() => preferences.app.locale === 'zh-CN');

function localized(field: PurchaseField, part: 'description' | 'label') {
  return chinese.value
    ? field[`${part}ZhCN`] || field[`${part}EnUS`]
    : field[`${part}EnUS`] || field[`${part}ZhCN`];
}

function options(field: PurchaseField) {
  return mapPurchaseOptions(field.options ?? [], chinese.value);
}
function optionPathFilter(input: string, path: Array<{ label?: unknown }>) {
  const value = input.toLocaleLowerCase();
  return path.some((option) =>
    String(option.label ?? '')
      .toLocaleLowerCase()
      .includes(value),
  );
}

function rows(): PurchaseValue[] {
  return (configuration.value?.fields ?? [])
    .filter((field) => field.visible)
    .flatMap((field) => {
      const value = values[field.fieldToken];
      if (field.type === 'file') {
        const tokens = fileTokens[field.fieldToken] ?? [];
        return tokens.length
          ? [
              {
                fieldToken: field.fieldToken,
                type: field.type,
                fileTokens: [...tokens],
              },
            ]
          : [];
      }
      if (
        value === undefined ||
        value === '' ||
        (Array.isArray(value) && value.length === 0)
      )
        return [];
      const row: PurchaseValue = {
        fieldToken: field.fieldToken,
        type: field.type,
      };
      if (['single_line', 'multi_line'].includes(field.type))
        row.textValue = String(value);
      else if (field.type === 'integer') row.integerValue = String(value);
      else if (field.type === 'decimal') row.decimalValue = String(value);
      else if (field.type === 'boolean') row.booleanValue = value === true;
      else if (field.type === 'date') row.dateValue = String(value);
      else if (field.type === 'datetime') row.dateTimeValue = String(value);
      else if (['single_select', 'multi_select'].includes(field.type))
        row.optionTokens = Array.isArray(value) ? value : [String(value)];
      else if (field.type === 'tree_select')
        row.treeOptionTokens = Array.isArray(value) ? value : [String(value)];
      else if (field.type === 'address')
        row.addressOptionTokens = Array.isArray(value)
          ? value
          : [String(value)];
      return [row];
    });
}

function applyRows(
  input: PurchaseValue[],
  previousFields: PurchaseField[] = [],
) {
  for (const key of Object.keys(values)) delete values[key];
  const authoritativeFiles = mergeAuthoritativeFileTokens(
    configuration.value?.fields ?? [],
    previousFields,
    input,
    fileTokens,
  );
  for (const field of configuration.value?.fields ?? []) {
    if (field.type !== 'file' || !field.visible) continue;
    const next = authoritativeFiles[field.fieldToken] ?? [];
    fileTokens[field.fieldToken] = next;
    files[field.fieldToken] = (files[field.fieldToken] ?? []).filter((item) =>
      next.includes(item.fileToken),
    );
  }
  for (const row of input) {
    if (row.type === 'file') continue;
    const value =
      row.textValue ??
      row.integerValue ??
      row.decimalValue ??
      row.booleanValue ??
      row.dateValue ??
      row.dateTimeValue ??
      row.optionTokens ??
      row.treeOptionTokens ??
      row.addressOptionTokens ??
      row.fileTokens;
    if (value !== undefined) values[row.fieldToken] = value;
  }
}

async function load() {
  loading.value = true;
  loadFailed.value = false;
  try {
    configuration.value = await getPurchaseConfiguration(publicSlug.value);
    applyingAuthoritative = true;
    applyRows(configuration.value.values ?? []);
    await nextTick();
    applyingAuthoritative = false;
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
}

function fieldHelp(field: PurchaseField) {
  return issueTokens.value.has(field.fieldToken)
    ? $t('purchaseConfiguration.messages.fieldInvalid')
    : localized(field, 'description');
}

async function evaluateCurrent(scheduledSequence?: number) {
  if (!configuration.value || loadFailed.value) return;
  const sequence = scheduledSequence ?? ++evaluationSequence;
  evaluating.value = true;
  try {
    const result = await evaluatePurchaseConfiguration(
      publicSlug.value,
      configuration.value.configurationRevision,
      rows(),
    );
    if (sequence !== evaluationSequence) return;
    applyingAuthoritative = true;
    const previousFields = configuration.value.fields;
    configuration.value = {
      configurationRevision: result.configurationRevision,
      fields: result.fields,
      values: result.values,
    };
    applyRows(result.values, previousFields);
    issueTokens.value = new Set(
      result.issues.flatMap((issue) =>
        issue.fieldToken ? [issue.fieldToken] : [],
      ),
    );
    await nextTick();
    applyingAuthoritative = false;
  } catch {
    if (sequence === evaluationSequence)
      message.error($t('purchaseConfiguration.messages.evaluateFailed'));
  } finally {
    if (sequence === evaluationSequence) evaluating.value = false;
  }
}

function scheduleEvaluation() {
  quoteTerms.value = [];
  issueTokens.value = new Set();
  if (submitting.value) {
    submissionChanged = true;
    evaluationSequence += 1;
    return;
  }
  if (applyingAuthoritative || !configuration.value) return;
  const sequence = ++evaluationSequence;
  if (evaluationTimer) clearTimeout(evaluationTimer);
  evaluationTimer = setTimeout(() => evaluateCurrent(sequence), 250);
}

watch(values, scheduleEvaluation, { deep: true });
watch(fileTokens, scheduleEvaluation, { deep: true });
onBeforeUnmount(() => {
  if (evaluationTimer) clearTimeout(evaluationTimer);
  evaluationSequence += 1;
});

function uploadPolicyText(field: PurchaseField) {
  const policy = field.uploadPolicy;
  if (!policy) return '';
  const megabytes = Math.max(
    1,
    Math.floor(policy.maximumFileBytes / 1024 / 1024),
  );
  return $t('purchaseConfiguration.file.policy', {
    count: policy.maximumFiles,
    extensions: policy.allowedExtensions.join('、'),
    size: megabytes,
  });
}

async function upload(field: PurchaseField, option: UploadRequestOption) {
  const file = option.file as File;
  const current = files[field.fieldToken] ?? [];
  if (
    !field.uploadPolicy ||
    current.length >= field.uploadPolicy.maximumFiles
  ) {
    option.onError?.(new Error('file_count_exceeded'));
    return;
  }
  try {
    const item = await uploadPurchaseConfigurationFile(
      publicSlug.value,
      configuration.value!.configurationRevision,
      field.fieldToken,
      file,
    );
    const next = [...current, item];
    files[field.fieldToken] = next;
    fileTokens[field.fieldToken] = next.map((entry) => entry.fileToken);
    option.onSuccess?.(item);
    message.success($t('purchaseConfiguration.file.uploaded'));
  } catch (error) {
    option.onError?.(error as Error);
    message.error($t('purchaseConfiguration.file.uploadFailed'));
  }
}

async function removeFile(field: PurchaseField, item: PurchaseFileItem) {
  if (removingTokens.value.has(item.fileToken)) return;
  removingTokens.value = new Set(removingTokens.value).add(item.fileToken);
  try {
    await revokePurchaseConfigurationFile(item.fileToken);
    const next = (files[field.fieldToken] ?? []).filter(
      (entry) => entry.fileToken !== item.fileToken,
    );
    files[field.fieldToken] = next;
    fileTokens[field.fieldToken] = next.map((entry) => entry.fileToken);
    message.success($t('purchaseConfiguration.file.revoked'));
  } catch {
    message.error($t('purchaseConfiguration.file.removeFailed'));
  } finally {
    const next = new Set(removingTokens.value);
    next.delete(item.fileToken);
    removingTokens.value = next;
  }
}

async function submit() {
  if (!configuration.value || submitting.value) return;
  submitting.value = true;
  submissionChanged = false;
  if (evaluationTimer) clearTimeout(evaluationTimer);
  evaluationTimer = undefined;
  evaluationSequence += 1;
  const submittedRevision = configuration.value.configurationRevision;
  const submittedRows = rows();
  const submittedDigest = JSON.stringify(submittedRows);
  const unchanged = () =>
    submissionSnapshotUnchanged(
      submittedRevision,
      submittedRows,
      configuration.value?.configurationRevision ?? '',
      rows(),
      submissionChanged,
    ) && JSON.stringify(rows()) === submittedDigest;
  try {
    let evaluation: Awaited<ReturnType<typeof evaluatePurchaseConfiguration>>;
    try {
      evaluation = await evaluatePurchaseConfiguration(
        publicSlug.value,
        configuration.value.configurationRevision,
        rows(),
      );
    } catch {
      message.error($t('purchaseConfiguration.messages.evaluateFailed'));
      return;
    }
    if (!unchanged()) {
      message.warning($t('purchaseConfiguration.messages.changedDuringSubmit'));
      return;
    }
    if (!evaluation.valid) {
      issueTokens.value = new Set(
        evaluation.issues.flatMap((issue) =>
          issue.fieldToken ? [issue.fieldToken] : [],
        ),
      );
      message.error($t('purchaseConfiguration.messages.invalid'));
      return;
    }
    configuration.value = {
      configurationRevision: evaluation.configurationRevision,
      fields: evaluation.fields,
      values: evaluation.values,
    };
    let quote: Awaited<ReturnType<typeof quotePurchaseConfiguration>>;
    try {
      quote = await quotePurchaseConfiguration(
        publicSlug.value,
        configuration.value.configurationRevision,
        currencyCode.value,
        evaluation.values,
      );
    } catch {
      message.error($t('purchaseConfiguration.messages.quoteFailed'));
      return;
    }
    if (!unchanged()) {
      message.warning($t('purchaseConfiguration.messages.changedDuringSubmit'));
      return;
    }
    quoteTerms.value = quote.terms.map((term) => ({
      label: chinese.value
        ? term.labelZhCN || term.labelEnUS
        : term.labelEnUS || term.labelZhCN,
      total: formatMinorAmount(term.totalAmount, quote.currencyCode),
    }));
    if (quoteItemNo.value) {
      try {
        await freezePurchaseConfiguration(
          quoteItemNo.value,
          publicSlug.value,
          configuration.value.configurationRevision,
          evaluation.values,
        );
      } catch {
        message.error($t('purchaseConfiguration.messages.freezeFailed'));
        return;
      }
      if (!unchanged()) {
        message.warning(
          $t('purchaseConfiguration.messages.changedDuringSubmit'),
        );
        return;
      }
      message.success($t('purchaseConfiguration.messages.frozen'));
      try {
        await router.push('/account/cart');
      } catch {
        message.warning($t('purchaseConfiguration.messages.navigationFailed'));
      }
    } else {
      message.success($t('purchaseConfiguration.messages.quoted'));
    }
  } finally {
    submitting.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('purchaseConfiguration.title')">
    <Card>
      <Spin :spinning="loading">
        <Alert
          v-if="loadFailed"
          class="mb-4"
          :message="$t('purchaseConfiguration.messages.loadFailed')"
          show-icon
          type="error"
        >
          <template #action
            ><Button size="small" @click="load">{{
              $t('purchaseConfiguration.catalog.retry')
            }}</Button></template
          >
        </Alert>
        <Alert
          v-if="!quoteItemNo"
          class="mb-4"
          :message="$t('purchaseConfiguration.messages.previewOnly')"
          show-icon
          type="info"
        />
        <Form
          v-if="configuration && !loadFailed"
          class="grid grid-cols-1 gap-x-4 md:grid-cols-2"
          layout="vertical"
        >
          <FormItem
            v-for="field in configuration.fields.filter((item) => item.visible)"
            :key="field.fieldToken"
            :class="{
              'md:col-span-2': [
                'address',
                'file',
                'multi_line',
                'tree_select',
              ].includes(field.type),
            }"
            :help="fieldHelp(field)"
            :label="localized(field, 'label')"
            :required="field.required"
            :validate-status="
              issueTokens.has(field.fieldToken) ? 'error' : undefined
            "
          >
            <Input.TextArea
              v-if="field.type === 'multi_line'"
              v-model:value="values[field.fieldToken]"
              :disabled="field.disabled || submitting"
              :maxlength="field.maximumLength"
              :rows="4"
              show-count
            />
            <Input
              v-else-if="
                ['single_line', 'integer', 'decimal'].includes(field.type)
              "
              v-model:value="values[field.fieldToken]"
              :disabled="field.disabled || submitting"
              :maxlength="field.maximumLength"
            />
            <Checkbox
              v-else-if="field.type === 'boolean'"
              v-model:checked="values[field.fieldToken]"
              :disabled="field.disabled || submitting"
              >{{ localized(field, 'description') }}</Checkbox
            >
            <Select
              v-else-if="['single_select', 'multi_select'].includes(field.type)"
              v-model:value="values[field.fieldToken]"
              class="w-full"
              :disabled="field.disabled || submitting"
              :mode="field.type === 'multi_select' ? 'multiple' : undefined"
              :options="options(field)"
              option-filter-prop="label"
              show-search
            />
            <DatePicker
              v-else-if="['date', 'datetime'].includes(field.type)"
              v-model:value="values[field.fieldToken]"
              class="w-full"
              :disabled="field.disabled || submitting"
              :show-time="field.type === 'datetime'"
              :value-format="
                field.type === 'datetime'
                  ? 'YYYY-MM-DDTHH:mm:ssZ'
                  : 'YYYY-MM-DD'
              "
            />
            <TreeSelect
              v-else-if="field.type === 'tree_select'"
              v-model:value="values[field.fieldToken]"
              class="w-full"
              :disabled="field.disabled || submitting"
              :tree-data="options(field)"
              show-search
              tree-default-expand-all
              tree-node-filter-prop="label"
            />
            <Cascader
              v-else-if="field.type === 'address'"
              v-model:value="values[field.fieldToken]"
              class="w-full"
              :disabled="field.disabled || submitting"
              :options="options(field)"
              :show-search="{ filter: optionPathFilter }"
            />
            <div v-else-if="field.type === 'file'">
              <UploadDragger
                :accept="field.uploadPolicy?.allowedExtensions.join(',')"
                :custom-request="(option) => upload(field, option)"
                :disabled="field.disabled || submitting"
                :file-list="[]"
                :multiple="(field.uploadPolicy?.maximumFiles ?? 1) > 1"
                data-testid="purchase-configuration-file-upload"
              >
                <p>{{ $t('purchaseConfiguration.file.drag') }}</p>
                <p class="text-muted-foreground">
                  {{ uploadPolicyText(field) }}
                </p>
              </UploadDragger>
              <div
                v-for="item in (files[field.fieldToken] ?? []).filter((entry) =>
                  (fileTokens[field.fieldToken] ?? []).includes(
                    entry.fileToken,
                  ),
                )"
                :key="item.fileToken"
                class="mt-2 flex items-center justify-between rounded border px-3 py-2"
              >
                <span>{{ item.fileName }}</span>
                <Button
                  danger
                  :disabled="submitting"
                  :loading="removingTokens.has(item.fileToken)"
                  size="small"
                  type="link"
                  @click="removeFile(field, item)"
                  >{{ $t('purchaseConfiguration.file.remove') }}</Button
                >
              </div>
            </div>
          </FormItem>
        </Form>
        <div
          v-if="quoteTerms.length"
          class="mt-4 rounded border p-4"
          data-testid="purchase-configuration-quote"
        >
          <div
            v-for="term in quoteTerms"
            :key="term.label"
            class="flex justify-between"
          >
            <span>{{ term.label }}</span
            ><strong>{{ term.total }}</strong>
          </div>
        </div>
        <div class="mt-4 flex justify-end gap-2">
          <Button @click="router.back()">{{
            $t('purchaseConfiguration.actions.back')
          }}</Button>
          <Button
            :disabled="loadFailed || evaluating"
            :loading="submitting"
            type="primary"
            @click="submit"
            >{{
              quoteItemNo
                ? $t('purchaseConfiguration.actions.freeze')
                : $t('purchaseConfiguration.actions.quote')
            }}</Button
          >
        </div>
      </Spin>
    </Card>
  </Page>
</template>

<script lang="ts" setup>
import type {
  CustomerSupportCustomFieldDefinition,
  CustomerTicketCustomFieldSubmission,
} from '#/api/idc/support';

import { computed, reactive, watch } from 'vue';

import { preferences } from '@vben/preferences';

import { Form, Input, Radio, Select } from 'ant-design-vue';

import { $t } from '#/locales';

const props = withDefaults(
  defineProps<{
    definitions: CustomerSupportCustomFieldDefinition[];
    disabled?: boolean;
    testIdPrefix?: string;
  }>(),
  {
    disabled: false,
    testIdPrefix: 'support-custom-field',
  },
);

const values = reactive<Record<string, string | string[] | undefined>>({});
const errors = reactive<Record<string, string>>({});
const numberPattern = /^-?[0-9]{1,20}(?:\.[0-9]{1,10})?$/u;
const datePattern = /^(\d{4})-(\d{2})-(\d{2})$/u;

const orderedDefinitions = computed(() =>
  [...props.definitions].sort(
    (left, right) => left.sortOrder - right.sortOrder,
  ),
);

watch(
  () => props.definitions,
  (definitions) => {
    const active = new Set(definitions.map((item) => item.fieldNo));
    for (const fieldNo of Object.keys(values)) {
      if (!active.has(fieldNo)) delete values[fieldNo];
    }
    for (const fieldNo of Object.keys(errors)) {
      if (!active.has(fieldNo)) delete errors[fieldNo];
    }
  },
  { deep: true, immediate: true },
);

function localizedName(item: { nameEnUS: string; nameZhCN: string }) {
  return preferences.app.locale === 'en-US'
    ? item.nameEnUS || item.nameZhCN
    : item.nameZhCN || item.nameEnUS;
}

function localizedDescription(item: {
  descriptionEnUS?: string;
  descriptionZhCN?: string;
}) {
  return preferences.app.locale === 'en-US'
    ? item.descriptionEnUS || item.descriptionZhCN || ''
    : item.descriptionZhCN || item.descriptionEnUS || '';
}

function isEmpty(value: string | string[] | undefined) {
  return (
    value === undefined ||
    value === '' ||
    (Array.isArray(value) && value.length === 0)
  );
}

function isValidDate(value: string) {
  const matched = datePattern.exec(value);
  if (!matched) return false;
  const year = Number(matched[1]);
  const month = Number(matched[2]);
  const day = Number(matched[3]);
  const date = new Date(Date.UTC(year, month - 1, day));
  return (
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day
  );
}

function stringValue(fieldNo: string) {
  const value = values[fieldNo];
  return Array.isArray(value) ? '' : value;
}

function setStringValue(fieldNo: string, value: string) {
  values[fieldNo] = value;
}

function validate() {
  let valid = true;
  for (const field of orderedDefinitions.value) {
    const value = values[field.fieldNo];
    errors[field.fieldNo] = '';
    if (field.required && isEmpty(value)) {
      errors[field.fieldNo] = $t('support.customFieldRequired', {
        name: localizedName(field),
      });
      valid = false;
      continue;
    }
    if (isEmpty(value)) continue;
    if (
      field.fieldType === 'single_line' &&
      String(value).trim().length > 1024
    ) {
      errors[field.fieldNo] = $t('support.customFieldSingleLineTooLong');
      valid = false;
    } else if (
      field.fieldType === 'multi_line' &&
      String(value).trim().length > 65_536
    ) {
      errors[field.fieldNo] = $t('support.customFieldMultiLineTooLong');
      valid = false;
    } else if (
      field.fieldType === 'number' &&
      !numberPattern.test(String(value).trim())
    ) {
      errors[field.fieldNo] = $t('support.customFieldNumberInvalid');
      valid = false;
    } else if (
      field.fieldType === 'date' &&
      !isValidDate(String(value).trim())
    ) {
      errors[field.fieldNo] = $t('support.customFieldDateInvalid');
      valid = false;
    } else if (
      ['single_select', 'multi_select'].includes(field.fieldType)
    ) {
      const selected = (Array.isArray(value) ? value : [value]).filter(
        (optionNo): optionNo is string => typeof optionNo === 'string',
      );
      const allowed = new Set(field.options.map((option) => option.optionNo));
      if (selected.some((optionNo) => !allowed.has(optionNo))) {
        errors[field.fieldNo] = $t('support.customFieldOptionInvalid');
        valid = false;
      }
    }
  }
  return valid;
}

function submissions() {
  return orderedDefinitions.value.flatMap<CustomerTicketCustomFieldSubmission>(
    (field) => {
      const value = values[field.fieldNo];
      if (isEmpty(value)) return [];
      switch (field.fieldType) {
        case 'boolean':
          return [{ booleanValue: value === 'true', fieldNo: field.fieldNo }];
        case 'date':
          return [{ dateValue: String(value), fieldNo: field.fieldNo }];
        case 'multi_select':
        case 'single_select':
          return [
            {
              fieldNo: field.fieldNo,
              optionNos: Array.isArray(value) ? value : [String(value)],
            },
          ];
        case 'number':
          return [
            { fieldNo: field.fieldNo, numberValue: String(value).trim() },
          ];
        default:
          return [{ fieldNo: field.fieldNo, textValue: String(value).trim() }];
      }
    },
  );
}

function reset() {
  for (const fieldNo of Object.keys(values)) delete values[fieldNo];
  for (const fieldNo of Object.keys(errors)) delete errors[fieldNo];
}

defineExpose({ reset, submissions, validate });
</script>

<template>
  <section
    v-if="orderedDefinitions.length"
    class="grid gap-1"
    :data-testid="`${testIdPrefix}-group`"
  >
    <div class="mb-3">
      <h2 class="m-0 text-base font-semibold">
        {{ $t('support.additionalInformation') }}
      </h2>
      <p class="mb-0 mt-1 text-sm text-muted-foreground">
        {{ $t('support.additionalInformationHint') }}
      </p>
    </div>

    <Form.Item
      v-for="field in orderedDefinitions"
      :key="field.fieldNo"
      :data-testid="`${testIdPrefix}-${field.fieldNo}`"
      :extra="localizedDescription(field)"
      :help="errors[field.fieldNo]"
      :label="localizedName(field)"
      :required="field.required"
      :validate-status="errors[field.fieldNo] ? 'error' : undefined"
    >
      <Input.TextArea
        v-if="field.fieldType === 'multi_line'"
        :value="stringValue(field.fieldNo)"
        :auto-size="{ minRows: 4, maxRows: 10 }"
        :disabled="disabled"
        :maxlength="65_536"
        @update:value="setStringValue(field.fieldNo, $event)"
      />
      <Input
        v-else-if="field.fieldType === 'single_line'"
        :value="stringValue(field.fieldNo)"
        :disabled="disabled"
        :maxlength="1024"
        @update:value="setStringValue(field.fieldNo, $event)"
      />
      <Input
        v-else-if="field.fieldType === 'number'"
        :value="stringValue(field.fieldNo)"
        :disabled="disabled"
        inputmode="decimal"
        :maxlength="32"
        @update:value="setStringValue(field.fieldNo, $event)"
      />
      <Input
        v-else-if="field.fieldType === 'date'"
        :value="stringValue(field.fieldNo)"
        :disabled="disabled"
        type="date"
        @update:value="setStringValue(field.fieldNo, $event)"
      />
      <Radio.Group
        v-else-if="field.fieldType === 'boolean'"
        v-model:value="values[field.fieldNo]"
        :disabled="disabled"
      >
        <Radio.Button value="true">{{ $t('support.yes') }}</Radio.Button>
        <Radio.Button value="false">{{ $t('support.no') }}</Radio.Button>
      </Radio.Group>
      <Select
        v-else
        v-model:value="values[field.fieldNo]"
        :disabled="disabled"
        :mode="field.fieldType === 'multi_select' ? 'multiple' : undefined"
        :options="
          field.options.map((option) => ({
            label: localizedName(option),
            value: option.optionNo,
          }))
        "
        :placeholder="$t('support.chooseCustomFieldOption')"
      />
    </Form.Item>
  </section>
</template>

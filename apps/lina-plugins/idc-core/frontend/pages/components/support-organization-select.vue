<script setup lang="ts">
import type { PluginHostMountContext } from "../../src/host-context";
import type { SupportOrganizationDepartment } from "../../src/support/api";
import { computed, onBeforeUnmount, ref } from "vue";
import { Alert, Button, Pagination, Select } from "ant-design-vue";
import { $t } from "#/locales";
import { createSupportAPI } from "../../src/support/api";

const props = defineProps<{
  modelValue?: number;
  selectedName?: string;
  context: PluginHostMountContext;
  disabled?: boolean;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: number | undefined];
}>();
const api = createSupportAPI(() => props.context);
const items = ref<SupportOrganizationDepartment[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const keyword = ref("");
const loading = ref(false);
const failed = ref(false);
const selected = ref<SupportOrganizationDepartment>();
let timer: ReturnType<typeof setTimeout> | undefined;
let request = 0;
const options = computed(() => {
  const values = items.value.map((item) => ({
    value: item.deptId,
    label: item.deptName,
  }));
  if (
    props.modelValue &&
    !values.some((item) => item.value === props.modelValue)
  ) {
    values.unshift({
      value: props.modelValue,
      label:
        selected.value?.deptId === props.modelValue
          ? selected.value.deptName
          : props.selectedName || String(props.modelValue),
    });
  }
  return values;
});
function tr(key: string) {
  return $t(`plugin.idc-core.supportOrganization.${key}`);
}
async function load() {
  const current = ++request;
  loading.value = true;
  failed.value = false;
  try {
    const result = await api.organizationDepartments(
      page.value,
      pageSize.value,
      keyword.value,
    );
    if (current !== request) return;
    items.value = result.items;
    total.value = result.total;
  } catch {
    if (current !== request) return;
    failed.value = true;
    items.value = [];
    total.value = 0;
  } finally {
    if (current === request) loading.value = false;
  }
}
function search(value: string) {
  clearTimeout(timer);
  timer = setTimeout(() => {
    keyword.value = value;
    page.value = 1;
    void load();
  }, 250);
}
function change(value: unknown) {
  const id = typeof value === "number" ? value : undefined;
  selected.value = items.value.find((item) => item.deptId === id);
  emit("update:modelValue", id);
}
onBeforeUnmount(() => {
  clearTimeout(timer);
  request++;
});
</script>

<template>
  <Select
    :value="modelValue || undefined"
    :options="options"
    :loading="loading"
    :disabled="disabled"
    :placeholder="tr('choose')"
    :aria-label="tr('department')"
    show-search
    :filter-option="false"
    class="w-full"
    data-testid="support-organization-select"
    @search="search"
    @change="change"
    @dropdown-visible-change="
      (open) => {
        if (open) load();
      }
    "
  >
    <template #dropdownRender="{ menuNode }">
      <component :is="menuNode" />
      <div class="organization-pagination" @mousedown.prevent>
        <Alert v-if="failed" type="error" :message="tr('loadFailed')" show-icon>
          <template #action
            ><Button size="small" @click="load">{{
              tr("retry")
            }}</Button></template
          >
        </Alert>
        <Pagination
          v-else
          v-model:current="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-size-options="['10', '20', '50']"
          size="small"
          show-size-changer
          :show-total="
            (value) =>
              $t('plugin.idc-core.supportOrganization.total', { total: value })
          "
          @change="load"
        />
      </div>
    </template>
  </Select>
</template>

<style scoped>
.organization-pagination {
  padding: 8px;
  border-top: 1px solid hsl(var(--border));
  min-width: 300px;
}
</style>

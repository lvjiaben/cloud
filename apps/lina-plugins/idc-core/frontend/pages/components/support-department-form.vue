<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { PluginHostMountContext } from "../../src/host-context";
import type {
  SupportDepartmentSetting,
  SupportDepartmentSettingInput,
} from "../../src/support/api";
import { onMounted, watch } from "vue";
import { Alert, Button } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { createTranslator } from "../../src/runtime/i18n";
import { supportMessages } from "../../src/support/messages";
import SupportOrganizationSelect from "./support-organization-select.vue";

const props = defineProps<{
  item: SupportDepartmentSetting;
  departments: SupportDepartmentSetting[];
  context: PluginHostMountContext;
  saving: boolean;
}>();
const emit = defineEmits<{
  save: [body: SupportDepartmentSettingInput];
  cancel: [];
}>();
const tr = createTranslator(() => props.context, supportMessages);
function org(key: string) {
  return $t(`plugin.idc-core.supportOrganization.${key}`);
}
function schema(): VbenFormProps["schema"] {
  return [
    {
      fieldName: "orgDeptId",
      component: SupportOrganizationSelect,
      label: org("department"),
      componentProps: {
        context: props.context,
        selectedName: props.item.orgDeptName,
      },
      rules: z
        .number({ required_error: org("required") })
        .positive(org("required")),
    },
    {
      fieldName: "nameZhCN",
      component: "Input",
      label: tr("support.settings.nameZhCN"),
      rules: "required",
      componentProps: { "aria-label": tr("support.settings.nameZhCN") },
    },
    {
      fieldName: "nameEnUS",
      component: "Input",
      label: tr("support.settings.nameEnUS"),
      rules: "required",
      componentProps: { "aria-label": tr("support.settings.nameEnUS") },
    },
    {
      fieldName: "parentDepartmentNo",
      component: "Select",
      label: tr("support.settings.parentDepartment"),
      componentProps: {
        allowClear: true,
        options: props.departments
          .filter((item) => item.departmentNo !== props.item.departmentNo)
          .map((item) => ({
            value: item.departmentNo,
            label: props.context.locale.startsWith("zh")
              ? item.nameZhCN
              : item.nameEnUS,
          })),
      },
    },
    {
      fieldName: "status",
      component: "RadioGroup",
      label: tr("support.settings.settingState"),
      rules: "required",
      componentProps: {
        optionType: "button",
        buttonStyle: "solid",
        options: [
          { value: "active", label: tr("support.settings.active") },
          { value: "disabled", label: tr("support.settings.disabled") },
        ],
      },
    },
  ];
}
const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 108 },
  schema: [],
  showDefaultActions: false,
});
async function initialize() {
  await formApi.setState({ schema: schema() });
  await formApi.setValues({
    orgDeptId: props.item.orgDeptId || undefined,
    nameZhCN: props.item.nameZhCN,
    nameEnUS: props.item.nameEnUS,
    parentDepartmentNo: props.item.parentDepartmentNo || undefined,
    status: props.item.status,
  });
}
async function save() {
  if (props.saving || !(await formApi.validate()).valid) return;
  const values = await formApi.getValues<SupportDepartmentSettingInput>();
  emit("save", {
    orgDeptId: values.orgDeptId,
    nameZhCN: values.nameZhCN.trim(),
    nameEnUS: values.nameEnUS.trim(),
    parentDepartmentNo: values.parentDepartmentNo || "",
    status: values.status,
    assignmentMode: "manual_claim",
  });
}
onMounted(initialize);
watch(() => props.item, initialize, { deep: true });
watch(
  () => props.departments,
  () => formApi.setState({ schema: schema() }),
);
watch(
  () => props.context.locale,
  () => formApi.setState({ schema: schema() }),
);
</script>

<template>
  <div class="settings-form" data-testid="support-settings-department-form">
    <h3>
      {{
        item.departmentNo
          ? tr("support.settings.editDepartment")
          : tr("support.settings.newDepartment")
      }}
    </h3>
    <Alert
      v-if="item.departmentNo && !item.orgDeptId"
      type="warning"
      show-icon
      :message="org('unlinked')"
      class="mb-4"
    />
    <p class="mb-4 text-sm text-muted-foreground">{{ org("manualClaim") }}</p>
    <Form />
    <div class="flex justify-end gap-2">
      <Button
        v-if="item.departmentNo"
        :disabled="saving"
        @click="emit('cancel')"
        >{{ tr("support.action.cancel") }}</Button
      >
      <Button type="primary" :loading="saving" @click="save">{{
        tr("support.settings.save")
      }}</Button>
    </div>
  </div>
</template>

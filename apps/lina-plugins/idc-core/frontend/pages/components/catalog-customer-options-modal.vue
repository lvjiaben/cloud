<script setup lang="ts">
import type { VbenFormSchema } from "#/adapter/form";
import type { CatalogProductVersionConfiguration, CatalogCustomerFieldSelection } from "../catalog-client";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Alert } from "ant-design-vue";
import { useVbenForm } from "#/adapter/form";
import { catalogProductVersionConfigurationUpdate } from "../catalog-client";

const emit = defineEmits<{ saved: [] }>();
let versionNo = "";
let configuration: CatalogProductVersionConfiguration | undefined;
const zh = () => preferences.app.locale === "zh-CN";
const [Form, formApi] = useVbenForm({ schema: [], showDefaultActions: false });
const [Modal, modalApi] = useVbenModal({
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{ versionNo: string; configuration: CatalogProductVersionConfiguration }>();
    versionNo = data.versionNo;
    configuration = data.configuration;
    const schema: VbenFormSchema[] = [];
    const values: Record<string, unknown> = {};
    for (const [index, field] of (configuration.customerConfigurableFields ?? []).entries()) {
      const selection = configuration.customerFieldSelections?.find(item => item.fieldKey === field.key);
      const prefix = `option_${index}`;
      const label = zh() ? field.label?.zhCN : field.label?.enUS;
      schema.push({ fieldName: `${prefix}_enabled`, label: label ?? field.key, component: "Switch", componentProps: { "data-testid": `${prefix}_enabled` } });
      values[`${prefix}_enabled`] = Boolean(selection);
      if (field.type === "integer") {
        for (const bound of ["minimum", "maximum"] as const) {
          schema.push({
            fieldName: `${prefix}_${bound}`, label: bound === "minimum" ? (zh() ? "最小可选数量" : "Minimum quantity") : (zh() ? "最大可选数量" : "Maximum quantity"),
            component: "InputNumber", componentProps: { "data-testid": `${prefix}_${bound}`, precision: 0, min: Number(field.minimum), max: Number(field.maximum), class: "w-full" },
            dependencies: { triggerFields: [`${prefix}_enabled`], show: current => Boolean(current[`${prefix}_enabled`]), required: current => Boolean(current[`${prefix}_enabled`]) },
          });
          values[`${prefix}_${bound}`] = selection?.[bound] === undefined ? undefined : Number(selection[bound]);
        }
      } else {
        schema.push({ fieldName: `${prefix}_candidates`, label: zh() ? "允许购买" : "Allowed choices", component: "Select", componentProps: { mode: "multiple", options: (field.staticCandidates ?? []).map(candidate => ({ value: candidate.value, label: zh() ? candidate.label?.zhCN : candidate.label?.enUS })) }, dependencies: { triggerFields: [`${prefix}_enabled`], show: current => Boolean(current[`${prefix}_enabled`]), required: current => Boolean(current[`${prefix}_enabled`]) } });
        values[`${prefix}_candidates`] = selection?.candidateValues ?? [];
      }
    }
    formApi.setState({ schema });
    await formApi.setValues(values);
  },
  async onConfirm() {
    const result = await formApi.validate();
    if (!result.valid || !configuration) return;
    const values = await formApi.getValues();
    const selections: CatalogCustomerFieldSelection[] = [];
    for (const [index, field] of (configuration.customerConfigurableFields ?? []).entries()) {
      const prefix = `option_${index}`;
      if (!values[`${prefix}_enabled`]) continue;
      selections.push(field.type === "integer" ? { fieldKey: field.key, minimum: String(values[`${prefix}_minimum`]), maximum: String(values[`${prefix}_maximum`]) } : { fieldKey: field.key, candidateValues: values[`${prefix}_candidates`] as string[] });
    }
    modalApi.lock();
    try {
      await catalogProductVersionConfigurationUpdate(versionNo, { expectedVersion: configuration.version, merchantValues: configuration.merchantValues, customerFieldSelections: selections });
      emit("saved");
      await modalApi.close();
    } finally { modalApi.unlock(); }
  },
});
</script>
<template>
  <Modal data-testid="catalog-customer-options-modal" :title="zh() ? '客户可选配置' : 'Purchase options'" class="w-[560px]">
    <Alert class="mb-4" show-icon type="info" :message="zh() ? '只开启上游实际支持的规格，并填写可售范围。保存后请在价格页为每个配置设置价格（免费也要填0），未设完不能发布。未开启的配置沿用套餐默认值。' : 'Enable only supplier-supported options and sale bounds. Then explicitly price each option for every sale currency and period, including zero for free options. Disabled options keep package defaults.'" />
    <Form />
  </Modal>
</template>

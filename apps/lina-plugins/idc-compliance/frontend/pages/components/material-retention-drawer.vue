<script setup lang="ts">
import type { DeletionItem } from "../material-retention-client";
import { computed, reactive, ref } from "vue";
import { useVbenDrawer } from "@vben/common-ui";
import { Alert, Button, InputNumber, Modal, Popconfirm, Space, message } from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import { useAccess } from "../plugin-access";
import { changeMaterialHold, confirmDeletion, listDeletion, listRetention, scanDeletion, setRetention } from "../material-retention-client";

const { hasAccessByCodes } = useAccess();
const canConfigure = computed(() => hasAccessByCodes(["idc:admin:compliance:privacy:configure"]));
const busy = ref(false);
const needsRecovery = ref(false);
const currentPage = ref(1);
const pageSize = ref(20);
const days = reactive<Record<string, number>>({});
let loadSequence = 0;
const categories = ["personal_verification_material", "enterprise_verification_material", "tax_invoice_evidence"];
const key = () => `material-retention-${crypto.randomUUID()}`;
const label = (type: string) => $t(`plugin.idc-compliance.privacy.dataType.${type}`);
const [Grid, gridApi] = useVbenVxeGrid({
 gridEvents: { pageChange: async (event: { currentPage: number; pageSize: number }) => { currentPage.value = event.currentPage; pageSize.value = event.pageSize; await load(); } },
 gridOptions: { height: 360, pagerConfig: { enabled: true, currentPage: 1, pageSize: 20, total: 0, pageSizes: [10, 20, 50, 100] }, rowConfig: { keyField: "taskNo" }, columns: [] },
});
async function load() {
 const sequence = ++loadSequence;
 const page = currentPage.value;
 const size = pageSize.value;
 busy.value = true;
 try {
  const [policies, tasks] = await Promise.all([listRetention(), listDeletion(page,size)]);
  if (sequence !== loadSequence) return;
  for (const item of policies.items) days[item.dataTypeCode] = item.retentionDays;
  needsRecovery.value = tasks.items.some(item => item.statusCode === "confirmed");
  gridApi.setGridOptions({ data: tasks.items, pagerConfig: { enabled: true, currentPage: page, pageSize: size, total: tasks.total } });
 } finally { if (sequence === loadSequence) busy.value = false; }
}
async function save(type: string) {
 busy.value = true;
 try { await setRetention({ dataTypeCode: type, retentionDays: days[type] || 0, idempotencyKey: key() }); await load(); message.success($t("plugin.idc-compliance.privacy.messages.saved")); }
 finally { busy.value = false; }
}
async function scan() {
 busy.value = true;
 try { await scanDeletion(); currentPage.value = 1; await load(); }
 finally { busy.value = false; }
}
function remove(item: DeletionItem) {
 Modal.confirm({ title: item.statusCode === "confirmed" ? $t("plugin.idc-compliance.privacy.deletion.resumeTitle") : $t("plugin.idc-compliance.privacy.deletion.confirmTitle"), content: item.statusCode === "confirmed" ? $t("plugin.idc-compliance.privacy.deletion.resumeText") : $t("plugin.idc-compliance.privacy.deletion.confirmText"), async onOk() {
  busy.value = true;
  try {
   try { await confirmDeletion(item, key()); }
   catch { await load(); return; }
   await load();
  }
  finally { busy.value = false; }
 } });
}
function hold(item: DeletionItem, release: boolean) {
 Modal.confirm({ title: $t(`plugin.idc-compliance.materialRetention.${release ? "releaseHold" : "placeHold"}`), content: $t("plugin.idc-compliance.materialRetention.holdHelp"), async onOk() {
  busy.value = true;
  try { await changeMaterialHold(item, release, key()); await load(); }
  finally { busy.value = false; }
 } });
}
const [Drawer] = useVbenDrawer({ class: "w-[1000px] max-w-[100vw]", showConfirmButton: false, async onOpenChange(open) {
 if (!open) { ++loadSequence; busy.value = false; return; }
 gridApi.setGridOptions({ columns: [
  { field: "dataTypeCode", minWidth: 160, title: $t("plugin.idc-compliance.materialRetention.category"), formatter: ({ cellValue }: { cellValue: string }) => label(cellValue) },
  { field: "targetNo", minWidth: 240, title: $t("plugin.idc-compliance.materialRetention.target") },
  { field: "dueAt", width: 170, title: $t("plugin.idc-compliance.materialRetention.due"), formatter: ({ cellValue }: { cellValue: number }) => formatTimestamp(cellValue) },
  { field: "statusCode", width: 110, title: $t("plugin.idc-compliance.materialRetention.status"), formatter: ({ cellValue }: { cellValue: string }) => $t(`plugin.idc-compliance.privacy.deletionStatus.${cellValue}`) },
  { field: "holdReasonCode", width: 130, title: $t("plugin.idc-compliance.materialRetention.hold"), formatter: ({ cellValue }: { cellValue: string }) => cellValue ? $t(`plugin.idc-compliance.privacy.hold.${cellValue}`) : "—" },
  { field: "action", width: 230, fixed: "right", title: $t("plugin.idc-compliance.materialRetention.action"), slots: { default: "action" } },
 ] });
 await load();
} });
</script>
<template>
 <Drawer :title="$t('plugin.idc-compliance.materialRetention.title')">
  <div class="space-y-4" data-testid="material-retention-drawer">
   <Alert type="info" :message="$t('plugin.idc-compliance.materialRetention.help')" />
   <Alert v-if="needsRecovery" type="warning" :message="$t('plugin.idc-compliance.materialRetention.recoveryHelp')" />
   <div v-for="type in categories" :key="type" class="flex items-center gap-3">
    <span class="min-w-52">{{ label(type) }}</span>
    <InputNumber v-model:value="days[type]" :min="1" :precision="0" :disabled="!canConfigure || busy" />
    <span>{{ $t('plugin.idc-compliance.materialRetention.days') }}</span>
    <Popconfirm v-if="canConfigure" :title="$t('plugin.idc-compliance.materialRetention.saveConfirm')" @confirm="save(type)">
     <Button :disabled="busy || !days[type]">{{ $t('plugin.idc-compliance.privacy.actions.save') }}</Button>
    </Popconfirm>
   </div>
   <Space><Button :loading="busy" @click="load">{{ $t('plugin.idc-compliance.materialRetention.refresh') }}</Button><Button v-if="canConfigure" :disabled="busy" @click="scan">{{ $t('plugin.idc-compliance.privacy.actions.scan') }}</Button></Space>
   <p>{{ $t('plugin.idc-compliance.materialRetention.limit') }}</p>
   <Grid :loading="busy"><template #action="{ row }"><Space v-if="canConfigure && row.statusCode !== 'completed'" wrap><ghost-button v-if="row.statusCode === 'pending'" :disabled="busy" @click="remove(row)">{{ $t('plugin.idc-compliance.privacy.actions.confirmDelete') }}</ghost-button><ghost-button v-if="row.statusCode === 'confirmed'" :disabled="busy" @click="remove(row)">{{ $t('plugin.idc-compliance.privacy.actions.resumeDelete') }}</ghost-button><ghost-button v-if="row.statusCode === 'pending'" :disabled="busy" @click="hold(row, false)">{{ $t('plugin.idc-compliance.materialRetention.placeHold') }}</ghost-button><ghost-button v-if="row.statusCode === 'blocked' && row.holdReasonCode === 'legal_hold'" :disabled="busy" @click="hold(row, true)">{{ $t('plugin.idc-compliance.materialRetention.releaseHold') }}</ghost-button></Space></template></Grid>
  </div>
 </Drawer>
</template>

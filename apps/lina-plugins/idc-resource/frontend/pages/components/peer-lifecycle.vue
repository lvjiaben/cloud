<script setup lang="ts">
import { computed, ref } from 'vue';
import { useAccess } from '../plugin-access';
import { useVbenDrawer, useVbenModal } from '@vben/common-ui';
import { Alert, Button, Card } from 'ant-design-vue';
import { requestClient } from '#/api/request';
import { $t } from '#/locales';

type Detail = { id: string; installed: number; enabled: number; type: string; distribution: string; runtimeState: string; effectiveVersion: string; discoveredVersion: string; version: string; upgradeAvailable: boolean; lastUpgradeFailure?: unknown; newBusinessState: string };
type Impact = { id: string; blockingCount: number; items: { totalCount: number; blockingCount: number }[] };
const tr = (key: string) => $t(`plugin.idc-resource.peerLifecycle.${key}`);
const { hasAccessByCodes } = useAccess();
const visible = computed(() => ['plugin:query', 'plugin:business-impact'].every(code => hasAccessByCodes([code])));
const canConfirm = computed(() => visible.value && hasAccessByCodes(['plugin:enable']));
const busy = ref(false);
const error = ref('');
const impact = ref<Impact>();
const detail = ref<Detail>();
const [PanelDrawer, panel] = useVbenDrawer({ showConfirmButton: false });
function ready(value: Detail) {
  return value?.id === 'idc-peer' && value.type === 'dynamic' && value.distribution === 'managed' && value.installed === 1 && value.enabled === 1 && value.runtimeState === 'normal' && !!value.effectiveVersion && value.effectiveVersion === value.discoveredVersion && value.effectiveVersion === value.version && value.upgradeAvailable === false && !value.lastUpgradeFailure && ['accepting', 'stopped'].includes(value.newBusinessState);
}
async function readDetail() {
  const value = await requestClient.get<Detail>('/plugins/idc-peer');
  if (!ready(value)) throw new Error('state');
  return value;
}
async function readImpact() {
  const value = await requestClient.get<Impact>('/plugins/idc-peer/business-impact');
  if (value.id !== 'idc-peer' || !Number.isSafeInteger(value.blockingCount) || value.blockingCount < 0 || !Array.isArray(value.items) || value.items.length > 8 || value.items.reduce((total, item) => total + item.blockingCount, 0) !== value.blockingCount || value.items.some(item => !Number.isSafeInteger(item.totalCount) || item.totalCount < 0 || !Number.isSafeInteger(item.blockingCount) || item.blockingCount < 0 || item.blockingCount > item.totalCount)) throw new Error('impact');
  return value;
}
async function check() {
  if (busy.value || !visible.value) return;
  busy.value = true; impact.value = undefined; error.value = ''; detail.value = undefined;
  try { detail.value = await readDetail(); impact.value = await readImpact(); }
  catch { error.value = tr('checkFailed'); }
  finally { busy.value = false; }
}
let confirmed: Detail | undefined;
const [ConfirmModal, modal] = useVbenModal({
  onConfirm: reconfirm,
  onClosed: () => { confirmed = undefined; },
});
async function openConfirm() {
  if (busy.value || !canConfirm.value) return;
  busy.value = true; impact.value = undefined; error.value = '';
  try { confirmed = await readDetail(); detail.value = confirmed; modal.open(); }
  catch { detail.value = undefined; error.value = tr('stateFailed'); }
  finally { busy.value = false; }
}
async function reconfirm() {
  if (busy.value || !canConfirm.value || !confirmed) return;
  busy.value = true; impact.value = undefined; error.value = ''; modal.lock();
  try {
    const before = await readDetail();
    if (before.effectiveVersion !== confirmed.effectiveVersion || before.newBusinessState !== confirmed.newBusinessState) throw new Error('changed');
    await requestClient.put('/plugins/idc-peer/enable', {});
    const after = await readDetail();
    if (after.effectiveVersion !== before.effectiveVersion || after.newBusinessState !== before.newBusinessState) throw new Error('changed');
    const result = await readImpact();
    detail.value = after; impact.value = result; modal.close();
  } catch { detail.value = undefined; error.value = tr('confirmFailed'); modal.close(); }
  finally { modal.unlock(); busy.value = false; }
}
</script>
<template>
  <Button v-if="visible" @click="panel.open()" data-testid="peer-lifecycle-entry">{{ tr('title') }}</Button>
  <PanelDrawer v-if="visible" :title="tr('title')">
  <Card :title="tr('title')" :bordered="false" class="mb-4" data-testid="peer-lifecycle">
    <p class="mb-3 text-muted-foreground">{{ tr('description') }}</p>
    <div class="mb-3 flex flex-wrap gap-2">
      <Button :loading="busy" :disabled="busy" @click="check">{{ tr('check') }}</Button>
      <Button v-if="canConfirm" :disabled="busy" @click="openConfirm">{{ tr('reconfirm') }}</Button>
    </div>
    <Alert v-if="error" :message="error" type="error" show-icon data-testid="peer-lifecycle-error" />
    <Alert v-if="impact" :type="impact.blockingCount ? 'warning' : 'success'" :message="impact.blockingCount ? tr('blocked') : tr('drained')" show-icon data-testid="peer-lifecycle-result">
      <template #description><p>{{ tr('count') }}：{{ impact.blockingCount }}</p><p v-if="detail?.newBusinessState === 'stopped'">{{ tr('stopped') }}</p><p>{{ tr('recheck') }}</p></template>
    </Alert>
  </Card>
  </PanelDrawer>
    <ConfirmModal data-testid="peer-lifecycle-confirm-modal" :title="tr('reconfirm')" :confirm-text="tr('confirm')" :close-on-click-modal="false" :close-on-press-escape="!busy" :closable="!busy" :cancel-button-props="{ disabled: busy }">
      <p>{{ tr('confirmation') }}</p>
    </ConfirmModal>
</template>

<script setup lang="ts">
import type {
  CustomerServer,
  CustomerServerDetail,
  IncomingServerTransfer,
  OperationIntent,
  ServerAccessory,
  ServerAction,
  ServerOperationHistory,
  ServerResizeOptions,
} from '#/api/idc/customer-servers';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { computed, h, onMounted, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page, useVbenDrawer } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Checkbox,
  Collapse,
  CollapsePanel,
  Descriptions,
  DescriptionsItem,
  Empty,
  Form,
  FormItem,
  Input,
  InputNumber,
  message,
  Modal as AntModal,
  Radio,
  RadioGroup,
  Select,
  SelectOption,
  Space,
  Spin,
  Tag,
} from 'ant-design-vue';
import dayjs from 'dayjs';
import VbenModalCompat from '#/components/vben-modal-compat.vue';

import {
  authorizeServerOperation,
  checkoutServerAccessoryQuote,
  checkoutServerIPChangeQuote,
  checkoutServerResizeQuote,
  createServerAccessoryQuote,
  createServerIPChangeQuote,
  createServerOperation,
  createServerResizeQuote,
  confirmIncomingServerTransfer,
  getCustomerServer,
  getServerResizeOptions,
  listCustomerServers,
  listIncomingServerTransfers,
  redeemServerOperationOutput,
  requestCustomerServerTransfer,
  updateCustomerServerMetadata,
} from '#/api/idc/customer-servers';
import { $t } from '#/locales';
import { useVbenForm } from '#/adapter/form';
import { preferences } from '@vben/preferences';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import TrafficPackagePurchase from './traffic-package-purchase.vue';

const [TrafficPackageDrawer, trafficPackageDrawerApi] = useVbenDrawer({ connectedComponent: TrafficPackagePurchase });

const route = useRoute();
const router = useRouter();
const loading = ref(false);
const failed = ref(false);
const items = ref<CustomerServer[]>([]);
const detail = ref<CustomerServerDetail>();
const actionOpen = ref(false);
const actionBusy = ref(false);
const metadataOpen = ref(false);
const metadataBusy = ref(false);
const activeAction = ref<ServerAction>('start');
const locallyConsumed = reactive(new Set<string>());
const incomingTransfers = ref<IncomingServerTransfer[]>([]);
const transferOpen = ref(false);
const transferBusy = ref(false);
const recipientEmail = ref('');
const resizeOpen = ref(false);
const resizeBusy = ref(false);
const resizeOptions = ref<ServerResizeOptions>();
const resizeSelections = reactive<Record<string, string[]>>({});
const resizeQuantities = reactive<Record<string, number>>({});

const form = reactive({
  confirmPassword: '',
  confirmedServiceNo: '',
  externalPort: undefined as number | undefined,
  formatDataDisk: false,
  image: '',
  internalPort: undefined as number | undefined,
  mode: 'graceful',
  name: '',
  objectNo: '',
  password: '',
  port: undefined as number | undefined,
  protocol: 'tcp',
  systemDiskSize: undefined as number | undefined,
});
const metadataForm = reactive({ alias: '', remark: '' });

const [MetadataForm, metadataFormApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    labelWidth: 110,
  },
  handleValuesChange(values) {
    metadataForm.alias = String(values.alias || '');
    metadataForm.remark = String(values.remark || '');
  },
  schema: [
    {
      component: 'Input',
      componentProps: { maxlength: 128 },
      fieldName: 'alias',
      label: $t('servers.metadata.alias'),
    },
    {
      component: 'Textarea',
      componentProps: { maxlength: 1000, rows: 4, showCount: true },
      fieldName: 'remark',
      label: $t('servers.metadata.remark'),
    },
  ],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1',
});

const [TransferForm, transferFormApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: 'w-full' },
    labelWidth: 110,
  },
  handleValuesChange(values) {
    recipientEmail.value = String(values.recipientEmail || '');
  },
  schema: [
    {
      component: 'Input',
      componentProps: { autocomplete: 'off', maxlength: 320, type: 'email' },
      fieldName: 'recipientEmail',
      label: $t('servers.transfer.email'),
      rules: 'required',
    },
  ],
  showDefaultActions: false,
  wrapperClass: 'grid-cols-1',
});

const serviceNo = computed(() => String(route.params.serviceNo || '').trim());
const isEnglish = computed(() => preferences.app.locale.startsWith('en'));
const pageTitle = computed(() =>
  serviceNo.value ? $t('servers.detail.title') : $t('servers.list.title'),
);
const accessoryActions = new Set<ServerAction>([
  'backup_create',
  'backup_delete',
  'backup_restore',
  'nat_create',
  'nat_delete',
  'security_group_attach',
  'security_group_detach',
  'snapshot_create',
  'snapshot_delete',
  'snapshot_restore',
  'traffic_add',
  'virtual_media_mount',
  'virtual_media_unmount',
]);
const quotedAccessoryActions = new Set<ServerAction>([
  'backup_create',
  'snapshot_create',
  'traffic_add',
]);
const primaryAvailableActions = computed(() =>
  (detail.value?.server.availableActions || []).filter(
    (action) => !accessoryActions.has(action),
  ),
);
const accessoryAvailableActions = computed(() =>
  (detail.value?.server.availableActions || []).filter((action) =>
    accessoryActions.has(action),
  ),
);
const accessoryDirectoryFresh = computed(
  () =>
    !!detail.value?.accessoriesExpiresAt &&
    detail.value.accessoriesExpiresAt > Date.now(),
);
const trafficSnapshot = computed(() => {
  const item = detail.value?.accessories.find(
    (entry) => entry.kind === 'traffic',
  );
  if (!item) return;
  const number = (key: string) => {
    const value = Number(item.projection[key]);
    return Number.isFinite(value) && value >= 0 ? value : undefined;
  };
  const total = number('total_gb');
  const base = number('quota_gb');
  const temporary = number('temporary_gb');
  if (total === undefined || base === undefined || temporary === undefined)
    return;
  return { base, temporary, total };
});

function listColumns(): VxeGridProps<CustomerServer>['columns'] {
  return [
    {
      field: 'product',
      minWidth: 180,
      slots: { default: 'product' },
      title: $t('servers.fields.product'),
    },
    {
      field: 'serviceNo',
      minWidth: 180,
      title: $t('servers.fields.serviceNo'),
    },
    {
      field: 'runtimeStatus',
      slots: { default: 'runtimeStatus' },
      title: $t('servers.fields.runtime'),
      width: 140,
    },
    {
      field: 'entitlementStatus',
      slots: { default: 'entitlementStatus' },
      title: $t('servers.fields.status'),
      width: 140,
    },
    {
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      title: $t('pages.common.actions'),
      width: 100,
    },
  ];
}

function historyColumns(): VxeGridProps<ServerOperationHistory>['columns'] {
  return [
    {
      field: 'action',
      fixed: 'right',
      slots: { default: 'historyAction' },
      title: $t('servers.detail.actions'),
      width: 180,
    },
    {
      field: 'status',
      slots: { default: 'historyStatus' },
      title: $t('servers.fields.status'),
    },
    {
      field: 'createdAt',
      slots: { default: 'historyCreatedAt' },
      title: $t('servers.operation.submittedAt'),
      width: 180,
    },
    {
      field: 'output',
      fixed: 'right',
      slots: { default: 'historyOutput' },
      title: $t('pages.common.actions'),
      width: 140,
    },
  ];
}

const [ServerGrid, serverGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: listColumns(),
    data: [],
    emptyRender: { name: 'Empty' },
    height: 480,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: 'serviceNo' },
    toolbarConfig: { custom: true, refresh: true, zoom: true },
    id: 'customer-server-list',
  },
});

const [HistoryGrid, historyGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: historyColumns(),
    data: [],
    emptyRender: { name: 'Empty' },
    height: 360,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: 'operationNo' },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: 'customer-server-operation-history',
  },
});

watch(items, (rows) => serverGridApi.setGridOptions({ data: rows }), {
  deep: true,
});
watch(
  () => detail.value?.operations || [],
  (rows) => historyGridApi.setGridOptions({ data: rows }),
  { deep: true },
);
watch(
  () => preferences.app.locale,
  () => {
    serverGridApi.setGridOptions({ columns: listColumns() });
    historyGridApi.setGridOptions({ columns: historyColumns() });
  },
);

function statusText(value: string) {
  const key = `servers.status.${value}`;
  const translated = $t(key);
  return translated === key ? value : translated;
}

function actionText(value: ServerAction) {
  return $t(`servers.actions.${value}`);
}

function productName(server: CustomerServer | Record<string, any>) {
  return isEnglish.value
    ? String(server.productNameEnUS || '')
    : String(server.productNameZhCN || '');
}

function serverDisplayName(server: CustomerServer | Record<string, any>) {
  return String(server.customerAlias || '').trim() || productName(server);
}

function openMetadata() {
  if (!detail.value?.server) return;
  metadataForm.alias = detail.value.server.customerAlias || '';
  metadataForm.remark = detail.value.server.customerRemark || '';
  metadataFormApi.setValues({ ...metadataForm });
  metadataOpen.value = true;
}

async function saveMetadata() {
  const server = detail.value?.server;
  if (!server || metadataBusy.value) return;
  metadataBusy.value = true;
  try {
    const result = await updateCustomerServerMetadata(server.serviceNo, {
      alias: metadataForm.alias.trim(),
      expectedVersion: server.version,
      remark: metadataForm.remark.trim(),
    });
    server.customerAlias = result.metadata.alias;
    server.customerRemark = result.metadata.remark;
    server.version = result.metadata.version;
    metadataOpen.value = false;
    message.success($t('servers.metadata.saved'));
  } catch {
    message.error($t('servers.metadata.saveFailed'));
    await load();
  } finally {
    metadataBusy.value = false;
  }
}

function formatTime(value?: number) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—';
}

function formatAmount(minor: number, currency: string) {
  return new Intl.NumberFormat(isEnglish.value ? 'en-US' : 'zh-CN', {
    currency,
    style: 'currency',
  }).format(minor / 100);
}

function accessoryOptions(kind: string, state?: string) {
  return (detail.value?.accessories || []).filter(
    (item) => item.kind === kind && (!state || item.state === state),
  );
}

function accessoryKindText(kind: string) {
  const key = `servers.accessories.kind.${kind}`;
  const translated = $t(key);
  return translated === key ? kind : translated;
}

function accessoryProjectionText(item: ServerAccessory) {
  if (item.kind === 'ip') return String(item.projection.address || item.name);
  if (item.kind === 'disk') {
    const size = Number(item.projection.sizeGiB || 0);
    return size > 0 ? `${item.name} · ${size} GiB` : item.name;
  }
  return item.name;
}

function randomRequestNo() {
  return `server-${crypto.randomUUID()}`;
}

async function load() {
  if (loading.value) return;
  loading.value = true;
  failed.value = false;
  try {
    if (serviceNo.value) {
      const result = await getCustomerServer(serviceNo.value);
      detail.value = result.detail;
    } else {
      const result = await listCustomerServers();
      items.value = result.items || [];
      const transfers = await listIncomingServerTransfers();
      incomingTransfers.value = transfers.items || [];
      detail.value = undefined;
    }
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

function openTransfer() {
  recipientEmail.value = '';
  transferFormApi.setValues({ recipientEmail: '' });
  transferOpen.value = true;
}

async function openResize() {
  const server = detail.value?.server;
  if (!server || resizeBusy.value) return;
  resizeBusy.value = true;
  try {
    const value = await getServerResizeOptions(server.serviceNo);
    resizeOptions.value = value;
    for (const key of Object.keys(resizeSelections))
      delete resizeSelections[key];
    for (const key of Object.keys(resizeQuantities))
      delete resizeQuantities[key];
    const current = new Map(
      value.currentSelections.map((item) => [item.optionNo, item.quantity]),
    );
    for (const group of value.groups) {
      const selected = group.options
        .filter((option) => current.has(option.optionNo))
        .map((option) => option.optionNo);
      resizeSelections[group.groupNo] = selected;
      for (const option of group.options) {
        resizeQuantities[option.optionNo] =
          current.get(option.optionNo) || option.defaultQuantity || 1;
      }
    }
    resizeOpen.value = true;
  } catch {
    message.error($t('servers.resize.loadFailed'));
  } finally {
    resizeBusy.value = false;
  }
}

function normalizeResizeSelection(groupNo: string) {
  const group = resizeOptions.value?.groups.find(
    (item) => item.groupNo === groupNo,
  );
  if (!group || group.selectionMode !== 'single') return;
  const selected = resizeSelections[groupNo] || [];
  if (selected.length > 1) resizeSelections[groupNo] = selected.slice(-1);
}

async function submitResize() {
  const server = detail.value?.server;
  const options = resizeOptions.value;
  if (!server || !options || resizeBusy.value) return;
  const selections = options.groups.flatMap((group) =>
    (resizeSelections[group.groupNo] || []).map((optionNo) => ({
      optionNo,
      quantity: resizeQuantities[optionNo] || 1,
    })),
  );
  if (
    options.groups.some((group) => {
      const count = resizeSelections[group.groupNo]?.length || 0;
      return count < group.minimumSelected || count > group.maximumSelected;
    })
  ) {
    message.warning($t('servers.resize.selectionRequired'));
    return;
  }
  resizeBusy.value = true;
  const requestNo = `resize-${crypto.randomUUID()}`;
  try {
    const { quote } = await createServerResizeQuote(server.serviceNo, {
      currencyCode: options.currencyCode,
      idempotencyKey: requestNo,
      revision: options.revision,
      selections,
    });
    resizeOpen.value = false;
    await new Promise<void>((resolve, reject) => {
      AntModal.confirm({
        cancelText: $t('servers.common.cancel'),
        content:
          quote.direction === 'upgrade'
            ? $t('servers.resize.upgradeSummary', {
                charge: formatAmount(
                  quote.chargeAmountMinor,
                  quote.currencyCode,
                ),
                recurring: formatAmount(
                  quote.targetRecurringAmountMinor,
                  quote.currencyCode,
                ),
              })
            : $t('servers.resize.downgradeSummary', {
                effectiveAt: formatTime(quote.effectiveAt),
                recurring: formatAmount(
                  quote.targetRecurringAmountMinor,
                  quote.currencyCode,
                ),
              }),
        okText: $t('servers.resize.confirm'),
        title: $t(`servers.resize.${quote.direction}Title`),
        onCancel: () => reject(new Error('resize-cancelled')),
        onOk: async () => {
          try {
            const { checkout } = await checkoutServerResizeQuote(
              quote.quoteNo,
              `${requestNo}-checkout`,
            );
            if (checkout.invoiceNo) {
              message.info($t('servers.resize.paymentRequired'));
              await router.push(
                `/account/billing/invoices/${encodeURIComponent(checkout.invoiceNo)}`,
              );
            } else {
              message.success($t('servers.resize.scheduled'));
              await load();
            }
            resolve();
          } catch (error) {
            reject(error);
            throw error;
          }
        },
      });
    });
  } catch (error) {
    if (!(error instanceof Error && error.message === 'resize-cancelled')) {
      message.error($t('servers.resize.failed'));
    }
  } finally {
    resizeBusy.value = false;
  }
}

async function submitTransfer() {
  const server = detail.value?.server;
  if (!server || transferBusy.value || !recipientEmail.value.trim()) return;
  transferBusy.value = true;
  try {
    await requestCustomerServerTransfer(
      server.serviceNo,
      recipientEmail.value.trim(),
      `transfer-${crypto.randomUUID()}`,
    );
    transferOpen.value = false;
    message.success($t('servers.transfer.accepted'));
  } catch {
    message.error($t('servers.transfer.failed'));
  } finally {
    transferBusy.value = false;
  }
}

function confirmTransfer(item: IncomingServerTransfer) {
  AntModal.confirm({
    cancelText: $t('servers.common.cancel'),
    content: $t('servers.transfer.confirmDescription', {
      serviceNo: item.serviceNo,
    }),
    okText: $t('servers.transfer.confirm'),
    title: $t('servers.transfer.confirmTitle'),
    async onOk() {
      try {
        await confirmIncomingServerTransfer(
          item.transferNo,
          `confirm-${crypto.randomUUID()}`,
        );
        message.success($t('servers.transfer.confirmed'));
        await load();
      } catch {
        message.error($t('servers.transfer.confirmFailed'));
      }
    },
  });
}

function resetForm() {
  form.confirmPassword = '';
  form.confirmedServiceNo = '';
  form.externalPort = undefined;
  form.formatDataDisk = false;
  form.image = '';
  form.internalPort = undefined;
  form.mode = 'graceful';
  form.name = '';
  form.objectNo = '';
  form.password = '';
  form.port = undefined;
  form.protocol = 'tcp';
  form.systemDiskSize = undefined;
}

function openAction(action: ServerAction) {
  if (action === 'traffic_add' && detail.value?.server) {
    trafficPackageDrawerApi.setData({ serviceNo: detail.value.server.serviceNo }).open();
    return;
  }
  activeAction.value = action;
  resetForm();
  if (action === 'start') {
    void runOperation({});
    return;
  }
  actionOpen.value = true;
}

function operationParameters(): Record<
  string,
  boolean | number | string
> | null {
  if (activeAction.value === 'stop' || activeAction.value === 'reboot') {
    return { mode: form.mode };
  }
  if (activeAction.value === 'reinstall') {
    if (!form.image) {
      message.warning($t('servers.form.selectImage'));
      return null;
    }
    const parameters: Record<string, boolean | number | string> = {
      os: Number(form.image),
    };
    if (form.port) parameters.port = form.port;
    if (form.formatDataDisk) parameters.format_data_disk = '1';
    if (form.systemDiskSize) parameters.system_disk_size = form.systemDiskSize;
    return parameters;
  }
  if (activeAction.value === 'reset_password') {
    if (form.password.length < 8) {
      message.warning($t('servers.form.passwordRequired'));
      return null;
    }
    if (form.password !== form.confirmPassword) {
      message.warning($t('servers.form.passwordMismatch'));
      return null;
    }
  }
  if (
    activeAction.value === 'snapshot_create' ||
    activeAction.value === 'backup_create'
  ) {
    if (!form.objectNo || !form.name.trim()) {
      message.warning($t('servers.form.completeAccessory'));
      return null;
    }
    return { diskObjectNo: form.objectNo, name: form.name.trim() };
  }
  if (
    activeAction.value === 'snapshot_delete' ||
    activeAction.value === 'backup_delete'
  ) {
    if (!form.objectNo) {
      message.warning($t('servers.form.selectAccessory'));
      return null;
    }
    return { objectNo: form.objectNo };
  }
  if (
    activeAction.value === 'snapshot_restore' ||
    activeAction.value === 'backup_restore'
  ) {
    if (
      !form.objectNo ||
      form.confirmedServiceNo.trim() !== detail.value?.server.serviceNo
    ) {
      message.warning($t('servers.form.confirmServiceNoMismatch'));
      return null;
    }
    return {
      confirmedServiceNo: form.confirmedServiceNo.trim(),
      objectNo: form.objectNo,
    };
  }
  if (activeAction.value === 'security_group_attach') {
    if (!form.objectNo) message.warning($t('servers.form.selectAccessory'));
    return form.objectNo ? { objectNo: form.objectNo } : null;
  }
  if (
    activeAction.value === 'security_group_detach' ||
    activeAction.value === 'virtual_media_unmount'
  ) {
    return {};
  }
  if (activeAction.value === 'virtual_media_mount') {
    if (!form.objectNo) message.warning($t('servers.form.selectAccessory'));
    return form.objectNo ? { objectNo: form.objectNo } : null;
  }
  if (activeAction.value === 'nat_create') {
    if (!form.name.trim() || !form.externalPort || !form.internalPort) {
      message.warning($t('servers.form.completeAccessory'));
      return null;
    }
    return {
      externalPort: form.externalPort,
      internalPort: form.internalPort,
      name: form.name.trim(),
      protocol: form.protocol,
    };
  }
  if (activeAction.value === 'nat_delete') {
    if (!form.objectNo) message.warning($t('servers.form.selectAccessory'));
    return form.objectNo ? { objectNo: form.objectNo } : null;
  }
  return {};
}

function confirmAndRun() {
  const parameters = operationParameters();
  if (!parameters || actionBusy.value || !detail.value?.server) return;
  let confirmed = false;
  AntModal.confirm({
    afterClose: () => {
      if (confirmed) void runOperation(parameters).catch(() => undefined);
    },
    cancelText: $t('servers.common.cancel'),
    content: $t(`servers.confirm.${activeAction.value}`),
    okButtonProps: { danger: activeAction.value === 'reinstall' },
    okText: $t('servers.common.confirm'),
    title: $t('servers.confirm.title'),
    onOk: () => {
      confirmed = true;
    },
  });
}

async function runOperation(
  parameters: Record<string, boolean | number | string>,
) {
  if (!detail.value?.server || actionBusy.value) return;
  actionBusy.value = true;
  const intent: OperationIntent = {
    action: activeAction.value,
    parameters,
    requestNo: randomRequestNo(),
  };
  const password =
    activeAction.value === 'reset_password' ? form.password : undefined;
  try {
    if (activeAction.value === 'ip_change') {
      const quoteResult = await createServerIPChangeQuote(
        detail.value.server.serviceNo,
        intent.requestNo,
      );
      const quote = quoteResult.quote;
      const checkoutRequestNo = `${intent.requestNo}-checkout`;
      const confirmedIntent: OperationIntent = {
        action: 'ip_change',
        parameters: { ip_num: quote.ipQuantity },
        requestNo: checkoutRequestNo,
      };
      const authorization = await authorizeServerOperation(
        detail.value.server.serviceNo,
        confirmedIntent,
      );
      actionOpen.value = false;
      await new Promise<void>((resolve, reject) => {
        AntModal.confirm({
          cancelText: $t('servers.common.cancel'),
          content: $t('servers.ipChange.quoteSummary', {
            amount: formatAmount(quote.amountMinor, quote.currencyCode),
            quantity: quote.ipQuantity,
          }),
          okButtonProps: { danger: true },
          okText: $t('servers.ipChange.checkout'),
          onCancel: () => reject(new Error('checkout-cancelled')),
          onOk: async () => {
            try {
              const result = await checkoutServerIPChangeQuote(
                quote.quoteNo,
                checkoutRequestNo,
                authorization.required ? authorization.proofToken : '',
              );
              if (
                result.checkout.paymentRequired &&
                result.checkout.invoiceNo
              ) {
                message.info($t('servers.messages.paymentRequired'));
                await router.push(
                  `/account/billing/invoices/${encodeURIComponent(result.checkout.invoiceNo)}`,
                );
              } else {
                message.success($t('servers.messages.created'));
                await load();
              }
              resolve();
            } catch (error) {
              reject(error);
              throw error;
            }
          },
        });
      });
      resetForm();
      return;
    }
    if (quotedAccessoryActions.has(activeAction.value)) {
      const quoteResult = await createServerAccessoryQuote(
        detail.value.server.serviceNo,
        activeAction.value,
        parameters,
        intent.requestNo,
      );
      const quote = quoteResult.quote;
      actionOpen.value = false;
      await new Promise<void>((resolve, reject) => {
        AntModal.confirm({
          cancelText: $t('servers.common.cancel'),
          content: $t('servers.quote.summary', {
            amount: formatAmount(quote.totalAmountMinor, quote.currencyCode),
            quantity: quote.requestedQuantity,
          }),
          okText: $t('servers.quote.checkout'),
          onCancel: () => reject(new Error('checkout-cancelled')),
          onOk: async () => {
            try {
              const result = await checkoutServerAccessoryQuote(
                quote.quoteNo,
                `${intent.requestNo}-checkout`,
              );
              if (
                result.checkout.paymentRequired &&
                result.checkout.invoiceNo
              ) {
                message.info($t('servers.messages.paymentRequired'));
                await router.push(
                  `/account/billing/invoices/${encodeURIComponent(result.checkout.invoiceNo)}`,
                );
              } else {
                message.success($t('servers.messages.created'));
                await load();
              }
              resolve();
            } catch (error) {
              reject(error);
              throw error;
            }
          },
        });
      });
      resetForm();
      return;
    }
    const authorization = await authorizeServerOperation(
      detail.value.server.serviceNo,
      intent,
    );
    await createServerOperation(detail.value.server.serviceNo, intent, {
      password,
      proofToken: authorization.required ? authorization.proofToken : undefined,
    });
    message.success($t('servers.messages.created'));
    actionOpen.value = false;
    resetForm();
    await load();
  } catch (error) {
    if (!(error instanceof Error && error.message === 'checkout-cancelled')) {
      throw error;
    }
  } finally {
    form.password = '';
    form.confirmPassword = '';
    actionBusy.value = false;
  }
}

function revealOutput(value: ServerOperationHistory | Record<string, any>) {
  const operation = value as ServerOperationHistory;
  if (!detail.value?.server || locallyConsumed.has(operation.operationNo))
    return;
  let revealedPayload: Record<string, unknown> | undefined;
  let revealSucceeded = false;
  AntModal.confirm({
    afterClose: () => {
      if (!revealSucceeded || !revealedPayload) return;
      const payload = revealedPayload;
      revealedPayload = undefined;
      if (operation.action === 'console') showConsole(payload);
      else showCredential(payload);
    },
    cancelText: $t('servers.common.cancel'),
    content: $t('servers.credential.revealConfirm'),
    okText:
      operation.action === 'console'
        ? $t('servers.operation.openConsole')
        : $t('servers.credential.reveal'),
    title:
      operation.action === 'console'
        ? $t('servers.actions.console')
        : $t('servers.credential.title'),
    onOk: async () => {
      try {
        const result = await redeemServerOperationOutput(
          detail.value!.server.serviceNo,
          operation.operationNo,
        );
        locallyConsumed.add(operation.operationNo);
        operation.confidentialOutputAvailable = false;
        revealedPayload = result.payload;
        revealSucceeded = true;
      } catch {
        await load();
        message.warning($t('servers.messages.outputUnavailable'));
      }
    },
  });
}

function safeText(payload: Record<string, unknown>, key: string) {
  const value = payload[key];
  return typeof value === 'string' || typeof value === 'number'
    ? String(value)
    : '—';
}

function showCredential(payload: Record<string, unknown>) {
  AntModal.info({
    afterClose: () => {
      for (const key of Object.keys(payload)) delete payload[key];
    },
    content: () =>
      h('div', { 'data-testid': 'server-credential-content' }, [
        h(
          'p',
          `${$t('servers.credential.username')}: ${safeText(payload, 'username')}`,
        ),
        h(
          'p',
          `${$t('servers.credential.password')}: ${safeText(payload, 'password')}`,
        ),
        h(
          'p',
          `${$t('servers.credential.port')}: ${safeText(payload, 'port')}`,
        ),
        h(
          'p',
          { class: 'mt-4 text-red-500' },
          $t('servers.credential.closeWarning'),
        ),
      ]),
    okText: $t('servers.credential.savedAndClose'),
    title: $t('servers.credential.title'),
  });
}

function showConsole(payload: Record<string, unknown>) {
  const candidate = ['vnc_url_https', 'vnc_url']
    .map((key) => safeText(payload, key))
    .find((value) => value !== '—');
  if (!candidate) {
    message.warning($t('servers.messages.outputUnavailable'));
    return;
  }
  try {
    const url = new URL(candidate);
    if (url.protocol !== 'https:') throw new Error('unsupported console URL');
    window.open(url.toString(), '_blank', 'noopener,noreferrer');
  } catch {
    message.warning($t('servers.messages.outputUnavailable'));
  }
}

watch(serviceNo, load);
onMounted(load);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="pageTitle" data-testid="customer-servers-page">
    <Spin :spinning="loading">
      <Alert
        v-if="failed"
        class="mb-4"
        show-icon
        type="error"
        :message="$t('servers.common.loadFailed')"
      >
        <template #action
          ><Button size="small" @click="load">{{
            $t('servers.common.refresh')
          }}</Button></template
        >
      </Alert>

      <template v-if="!serviceNo">
        <Card
          v-if="incomingTransfers.length"
          class="mb-4"
          data-testid="incoming-server-transfers"
          :title="$t('servers.transfer.incomingTitle')"
        >
          <Descriptions bordered :column="1">
            <DescriptionsItem
              v-for="item in incomingTransfers"
              :key="item.transferNo"
              :label="item.serviceNo"
            >
              {{ $t('servers.transfer.expiresAt') }}：{{
                formatTime(item.expiresAt)
              }}
              <Button
                class="ml-3"
                type="primary"
                @click="confirmTransfer(item)"
              >
                {{ $t('servers.transfer.confirm') }}
              </Button>
            </DescriptionsItem>
          </Descriptions>
        </Card>
        <Card v-if="items.length" data-testid="customer-server-list">
          <ServerGrid>
            <template #product="{ row }">{{ serverDisplayName(row) }}</template>
            <template #runtimeStatus="{ row }"
              ><Tag>{{ statusText(row.runtimeStatus) }}</Tag></template
            >
            <template #entitlementStatus="{ row }">{{
              statusText(row.entitlementStatus)
            }}</template>
            <template #action="{ row }">
              <Button
                type="link"
                @click="
                  router.push(
                    `/account/servers/${encodeURIComponent(row.serviceNo)}`,
                  )
                "
                >{{ $t('servers.list.open') }}</Button
              >
            </template>
          </ServerGrid>
        </Card>
      </template>

      <template v-else-if="detail">
        <Space class="mb-4"
          ><Button @click="router.push('/account/servers')">{{
            $t('servers.common.back')
          }}</Button
          ><Button @click="load">{{
            $t('servers.common.refresh')
          }}</Button></Space
        >
        <Card
          class="mb-4"
          data-testid="customer-server-detail"
          :title="serverDisplayName(detail.server)"
        >
          <template #extra>
            <Space>
              <Button
                data-testid="server-transfer-open"
                type="link"
                @click="openTransfer"
              >
                {{ $t('servers.transfer.open') }}
              </Button>
              <Button
                data-testid="server-metadata-edit"
                type="link"
                @click="openMetadata"
                >{{ $t('servers.metadata.edit') }}</Button
              >
            </Space>
          </template>
          <Descriptions bordered :column="{ xs: 1, sm: 2, lg: 4 }">
            <DescriptionsItem :label="$t('servers.fields.serviceNo')">{{
              detail.server.serviceNo
            }}</DescriptionsItem>
            <DescriptionsItem :label="$t('servers.fields.runtime')"
              ><Tag>{{
                statusText(detail.server.runtimeStatus)
              }}</Tag></DescriptionsItem
            >
            <DescriptionsItem :label="$t('servers.fields.status')">{{
              statusText(detail.server.entitlementStatus)
            }}</DescriptionsItem>
            <DescriptionsItem :label="$t('servers.fields.expiresAt')">{{
              formatTime(detail.server.entitlementExpiresAt)
            }}</DescriptionsItem>
            <DescriptionsItem :label="$t('servers.metadata.product')">{{
              productName(detail.server)
            }}</DescriptionsItem>
            <DescriptionsItem
              :label="$t('servers.metadata.remark')"
              :span="3"
              >{{
                detail.server.customerRemark || $t('servers.metadata.empty')
              }}</DescriptionsItem
            >
            <DescriptionsItem :label="$t('servers.renewal.autoRenew')">
              {{
                detail.server.autoRenewEnabled
                  ? $t('servers.renewal.enabled', {
                      cycles: detail.server.renewalCycles,
                    })
                  : $t('servers.renewal.disabled')
              }}
              <Button
                class="ml-2"
                type="link"
                @click="router.push('/account/billing/renewals')"
                >{{ $t('servers.renewal.manage') }}</Button
              >
            </DescriptionsItem>
            <DescriptionsItem :label="$t('servers.refund.title')">
              <template v-if="detail.server.activeRefundNo">
                {{ detail.server.activeRefundNo }}
                <Button
                  class="ml-2"
                  type="link"
                  @click="router.push('/account/refunds')"
                  >{{ $t('servers.refund.open') }}</Button
                >
              </template>
              <template v-else>{{ $t('servers.refund.none') }}</template>
            </DescriptionsItem>
          </Descriptions>
        </Card>

        <Card
          v-if="trafficSnapshot"
          class="mb-4"
          data-testid="customer-server-traffic"
          :title="$t('servers.traffic.title')"
        >
          <Alert
            class="mb-4"
            show-icon
            type="info"
            :message="$t('servers.traffic.observational')"
          />
          <Descriptions bordered :column="{ xs: 1, sm: 2, lg: 4 }">
            <DescriptionsItem :label="$t('servers.traffic.total')">
              {{ trafficSnapshot.total }} GB
            </DescriptionsItem>
            <DescriptionsItem :label="$t('servers.traffic.base')">
              {{ trafficSnapshot.base }} GB
            </DescriptionsItem>
            <DescriptionsItem :label="$t('servers.traffic.temporary')">
              {{ trafficSnapshot.temporary }} GB
            </DescriptionsItem>
            <DescriptionsItem :label="$t('servers.traffic.used')">
              {{ $t('servers.traffic.unavailable') }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('servers.traffic.resetAt')">
              {{ $t('servers.traffic.unavailable') }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('servers.traffic.updatedAt')">
              {{ formatTime(detail.accessoriesObservedAt) }}
            </DescriptionsItem>
          </Descriptions>
        </Card>

        <Card class="mb-4" :title="$t('servers.detail.actions')">
          <Alert
            v-if="detail.server.activeOperation"
            class="mb-4"
            show-icon
            type="info"
            :message="$t('servers.operation.processing')"
          />
          <Space wrap>
            <Button
              data-testid="server-resize-open"
              :disabled="!!detail.server.activeOperation"
              :loading="resizeBusy"
              @click="openResize"
              >{{ $t('servers.resize.open') }}</Button
            >
            <Button
              v-for="action in primaryAvailableActions"
              :key="action"
              :data-testid="`server-action-${action}`"
              :danger="action === 'reinstall'"
              :disabled="!!detail.server.activeOperation"
              @click="openAction(action)"
              >{{ actionText(action) }}</Button
            >
          </Space>
        </Card>

        <Card
          class="mb-4"
          data-testid="customer-server-accessories"
          :title="$t('servers.accessories.title')"
        >
          <Alert
            v-if="!accessoryDirectoryFresh"
            class="mb-4"
            show-icon
            type="warning"
            :message="$t('servers.accessories.stale')"
          />
          <Space v-if="accessoryAvailableActions.length" class="mb-4" wrap>
            <Button
              v-for="action in accessoryAvailableActions"
              :key="action"
              :data-testid="`server-action-${action}`"
              :danger="
                action.endsWith('_delete') || action.endsWith('_restore')
              "
              :disabled="!!detail.server.activeOperation"
              @click="openAction(action)"
              >{{ actionText(action) }}</Button
            >
          </Space>
          <Alert
            v-else
            class="mb-4"
            show-icon
            type="info"
            :message="$t('servers.accessories.unavailable')"
          >
            <template #action>
              <Button
                data-testid="server-accessory-support"
                size="small"
                @click="
                  router.push({
                    path: '/account/support/new',
                    query: { serviceNo: detail.server.serviceNo },
                  })
                "
              >
                {{ $t('servers.accessories.createTicket') }}
              </Button>
            </template>
          </Alert>
          <Empty
            v-if="!detail.accessories.length"
            :description="$t('servers.accessories.empty')"
          />
          <Descriptions v-else bordered :column="{ xs: 1, sm: 2 }">
            <DescriptionsItem
              v-for="item in detail.accessories"
              :key="item.objectNo"
              :label="accessoryKindText(item.kind)"
            >
              {{ accessoryProjectionText(item) }} · {{ statusText(item.state) }}
            </DescriptionsItem>
          </Descriptions>
          <div class="mt-3 text-xs text-gray-500">
            {{ $t('servers.accessories.observedAt') }}:
            {{ formatTime(detail.accessoriesObservedAt) }}
          </div>
        </Card>

        <Card class="mb-4" :title="$t('servers.detail.configuration')">
          <Empty
            v-if="!detail.configuration.length"
            :description="$t('servers.detail.emptyConfiguration')"
          />
          <Descriptions v-else bordered :column="{ xs: 1, sm: 2 }">
            <DescriptionsItem
              v-for="item in detail.configuration"
              :key="`${item.componentType}-${item.optionNameZhCN}`"
              :label="isEnglish ? item.groupNameEnUS : item.groupNameZhCN"
            >
              {{ isEnglish ? item.optionNameEnUS : item.optionNameZhCN }} ×
              {{ item.quantity }}
            </DescriptionsItem>
          </Descriptions>
        </Card>

        <Card :title="$t('servers.detail.history')">
          <HistoryGrid>
            <template #historyAction="{ row }">{{
              actionText(row.action)
            }}</template>
            <template #historyStatus="{ row }"
              ><Tag>{{ statusText(row.status) }}</Tag></template
            >
            <template #historyCreatedAt="{ row }">{{
              formatTime(row.createdAt)
            }}</template>
            <template #historyOutput="{ row }">
              <Button
                v-if="
                  row.confidentialOutputAvailable &&
                  !locallyConsumed.has(row.operationNo)
                "
                type="link"
                :data-testid="`server-output-${row.operationNo}`"
                @click="revealOutput(row)"
                >{{
                  row.action === 'console'
                    ? $t('servers.operation.openConsole')
                    : $t('servers.credential.reveal')
                }}</Button
              >
            </template>
          </HistoryGrid>
        </Card>
      </template>
    </Spin>

    <VbenModalCompat
      v-model:open="metadataOpen"
      data-testid="server-metadata-modal"
      :confirm-loading="metadataBusy"
      :ok-text="$t('servers.common.save')"
      :cancel-text="$t('servers.common.cancel')"
      :title="$t('servers.metadata.title')"
      @ok="saveMetadata"
    >
      <div>
        <MetadataForm />
        <Alert show-icon type="info" :message="$t('servers.metadata.hint')" />
      </div>
    </VbenModalCompat>

    <VbenModalCompat
      v-model:open="transferOpen"
      data-testid="server-transfer-modal"
      :confirm-loading="transferBusy"
      :ok-text="$t('servers.transfer.submit')"
      :cancel-text="$t('servers.common.cancel')"
      :title="$t('servers.transfer.title')"
      @ok="submitTransfer"
    >
      <Alert
        class="mb-4"
        show-icon
        type="warning"
        :message="$t('servers.transfer.description')"
      />
      <TransferForm />
    </VbenModalCompat>

    <VbenModalCompat
      v-model:open="resizeOpen"
      data-testid="server-resize-modal"
      :confirm-loading="resizeBusy"
      :ok-text="$t('servers.resize.quote')"
      :cancel-text="$t('servers.common.cancel')"
      :title="$t('servers.resize.title')"
      width="720px"
      @ok="submitResize"
    >
      <Alert
        class="mb-4"
        show-icon
        type="warning"
        :message="$t('servers.resize.diskWarning')"
      />
      <Form v-if="resizeOptions" layout="vertical">
        <FormItem
          v-for="group in resizeOptions.groups"
          :key="group.groupNo"
          :label="isEnglish ? group.nameEnUS : group.nameZhCN"
        >
          <Select
            v-model:value="resizeSelections[group.groupNo]"
            :data-testid="`server-resize-group-${group.groupNo}`"
            mode="multiple"
            @change="normalizeResizeSelection(group.groupNo)"
          >
            <SelectOption
              v-for="option in group.options"
              :key="option.optionNo"
              :value="option.optionNo"
            >
              {{ isEnglish ? option.nameEnUS : option.nameZhCN }}
            </SelectOption>
          </Select>
          <div
            v-for="option in group.options.filter((item) =>
              (resizeSelections[group.groupNo] || []).includes(item.optionNo),
            )"
            :key="`${option.optionNo}-quantity`"
            class="mt-2 flex items-center gap-2"
          >
            <span>{{ isEnglish ? option.nameEnUS : option.nameZhCN }}</span>
            <InputNumber
              v-if="option.maximumQuantity > 1"
              v-model:value="resizeQuantities[option.optionNo]"
              :data-testid="`server-resize-quantity-${option.optionNo}`"
              :min="option.minimumQuantity"
              :max="option.maximumQuantity"
            />
          </div>
        </FormItem>
      </Form>
    </VbenModalCompat>

    <VbenModalCompat
      v-model:open="actionOpen"
      data-testid="server-operation-modal"
      :confirm-loading="actionBusy"
      :ok-text="$t('servers.common.confirm')"
      :cancel-text="$t('servers.common.cancel')"
      :title="actionText(activeAction)"
      @ok="confirmAndRun"
    >
      <Alert
        class="mb-4"
        show-icon
        :type="activeAction === 'reinstall' ? 'error' : 'warning'"
        :message="$t(`servers.confirm.${activeAction}`)"
      />
      <Form layout="vertical">
        <template v-if="activeAction === 'stop' || activeAction === 'reboot'">
          <FormItem :label="$t('servers.form.mode')"
            ><RadioGroup v-model:value="form.mode"
              ><Radio value="graceful">{{ $t('servers.form.graceful') }}</Radio
              ><Radio value="hard">{{
                $t('servers.form.force')
              }}</Radio></RadioGroup
            ></FormItem
          >
        </template>
        <template v-if="activeAction === 'reset_password'">
          <FormItem :label="$t('servers.form.newPassword')"
            ><Input.Password
              v-model:value="form.password"
              autocomplete="new-password"
          /></FormItem>
          <FormItem :label="$t('servers.form.confirmPassword')"
            ><Input.Password
              v-model:value="form.confirmPassword"
              autocomplete="new-password"
          /></FormItem>
        </template>
        <template v-if="activeAction === 'reinstall'">
          <Alert
            v-if="!detail?.reinstallImages.some((image) => image.selectable)"
            class="mb-4"
            show-icon
            type="warning"
            :message="$t('servers.detail.imageUnavailable')"
          />
          <FormItem :label="$t('servers.form.image')">
            <Select
              v-model:value="form.image"
              data-testid="server-reinstall-image"
              :placeholder="$t('servers.form.selectImage')"
              show-search
              option-filter-prop="label"
            >
              <SelectOption
                v-for="image in detail?.reinstallImages || []"
                :key="image.reference"
                :value="image.reference"
                :disabled="!image.selectable"
                :label="`${image.group} ${image.name}`"
              >
                {{ image.group ? `${image.group} / ` : '' }}{{ image.name }} ·
                {{ image.osType || '—' }} · {{ image.arch || '—' }} ·
                {{ image.bootMode || '—' }}
              </SelectOption>
            </Select>
          </FormItem>
          <Collapse data-testid="server-reinstall-advanced" ghost>
            <CollapsePanel
              key="advanced"
              :header="$t('servers.advanced.title')"
            >
              <FormItem :label="$t('servers.advanced.port')"
                ><InputNumber
                  v-model:value="form.port"
                  :min="1"
                  :max="65535"
                  class="w-full"
              /></FormItem>
              <FormItem
                ><Checkbox v-model:checked="form.formatDataDisk">{{
                  $t('servers.advanced.formatDataDisk')
                }}</Checkbox></FormItem
              >
              <FormItem :label="$t('servers.advanced.systemDiskSize')"
                ><InputNumber
                  v-model:value="form.systemDiskSize"
                  :min="1"
                  class="w-full"
              /></FormItem>
            </CollapsePanel>
          </Collapse>
        </template>
        <template
          v-if="
            activeAction === 'snapshot_create' ||
            activeAction === 'backup_create'
          "
        >
          <FormItem :label="$t('servers.form.disk')">
            <Select
              v-model:value="form.objectNo"
              data-testid="server-accessory-object"
              :placeholder="$t('servers.form.selectAccessory')"
            >
              <SelectOption
                v-for="item in accessoryOptions('disk', 'attached')"
                :key="item.objectNo"
                :value="item.objectNo"
                >{{ accessoryProjectionText(item) }}</SelectOption
              >
            </Select>
          </FormItem>
          <FormItem :label="$t('servers.form.name')">
            <Input
              v-model:value="form.name"
              data-testid="server-accessory-name"
              :maxlength="128"
            />
          </FormItem>
        </template>
        <template
          v-if="
            [
              'snapshot_delete',
              'snapshot_restore',
              'backup_delete',
              'backup_restore',
            ].includes(activeAction)
          "
        >
          <FormItem :label="$t('servers.form.accessory')">
            <Select
              v-model:value="form.objectNo"
              data-testid="server-accessory-object"
              :placeholder="$t('servers.form.selectAccessory')"
            >
              <SelectOption
                v-for="item in accessoryOptions(
                  activeAction.startsWith('snapshot') ? 'snapshot' : 'backup',
                  'available',
                )"
                :key="item.objectNo"
                :value="item.objectNo"
                >{{ accessoryProjectionText(item) }}</SelectOption
              >
            </Select>
          </FormItem>
          <FormItem
            v-if="activeAction.endsWith('_restore')"
            :label="$t('servers.form.confirmServiceNo')"
          >
            <Input
              v-model:value="form.confirmedServiceNo"
              :placeholder="detail?.server.serviceNo"
              autocomplete="off"
            />
          </FormItem>
        </template>
        <template v-if="activeAction === 'security_group_attach'">
          <FormItem :label="$t('servers.form.securityGroup')">
            <Select
              v-model:value="form.objectNo"
              data-testid="server-accessory-object"
              :placeholder="$t('servers.form.selectAccessory')"
            >
              <SelectOption
                v-for="item in accessoryOptions('security_group', 'available')"
                :key="item.objectNo"
                :value="item.objectNo"
                >{{ accessoryProjectionText(item) }}</SelectOption
              >
            </Select>
          </FormItem>
        </template>
        <template v-if="activeAction === 'nat_create'">
          <FormItem :label="$t('servers.form.name')"
            ><Input v-model:value="form.name" :maxlength="128"
          /></FormItem>
          <FormItem :label="$t('servers.form.externalPort')"
            ><InputNumber
              v-model:value="form.externalPort"
              :min="1"
              :max="65535"
              class="w-full"
          /></FormItem>
          <FormItem :label="$t('servers.form.internalPort')"
            ><InputNumber
              v-model:value="form.internalPort"
              :min="1"
              :max="65535"
              class="w-full"
          /></FormItem>
          <FormItem :label="$t('servers.form.protocol')">
            <Select v-model:value="form.protocol">
              <SelectOption value="tcp">TCP</SelectOption>
              <SelectOption value="udp">UDP</SelectOption>
              <SelectOption value="tcp_udp">TCP + UDP</SelectOption>
            </Select>
          </FormItem>
        </template>
        <template v-if="activeAction === 'nat_delete'">
          <FormItem :label="$t('servers.form.natRule')">
            <Select
              v-model:value="form.objectNo"
              data-testid="server-accessory-object"
              :placeholder="$t('servers.form.selectAccessory')"
            >
              <SelectOption
                v-for="item in accessoryOptions('nat', 'active')"
                :key="item.objectNo"
                :value="item.objectNo"
                >{{ accessoryProjectionText(item) }}</SelectOption
              >
            </Select>
          </FormItem>
        </template>
        <template v-if="activeAction === 'virtual_media_mount'">
          <FormItem :label="$t('servers.form.virtualMedia')">
            <Select
              v-model:value="form.objectNo"
              data-testid="server-accessory-object"
              :placeholder="$t('servers.form.selectAccessory')"
            >
              <SelectOption
                v-for="item in accessoryOptions('virtual_media', 'available')"
                :key="item.objectNo"
                :value="item.objectNo"
                >{{ accessoryProjectionText(item) }}</SelectOption
              >
            </Select>
          </FormItem>
        </template>
      </Form>
    </VbenModalCompat>
    <TrafficPackageDrawer @success="load" />
  </Page>
</template>

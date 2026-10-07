<script setup lang="ts">
import type { AutoSignAuthorization } from "../electronic-contract-auto-sign-client";
import type { Dayjs } from "dayjs";
import type { VxeGridProps } from "#/adapter/vxe-table";

import { computed, h, onMounted, onBeforeUnmount, reactive, ref, watch } from "vue";
import { createNumberedPage } from "../numbered-page";
import { preferences } from "@vben/preferences";

import {
  Alert,
  Button,
  Card,
  Checkbox,
  DatePicker,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Tag,
  message,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";

import { $t } from "#/locales";

import {
  autoSignAuthorizationCreate,
  autoSignAuthorizationList,
  autoSignAuthorizationRevoke,
} from "../electronic-contract-auto-sign-client";

const loading = ref(false);
const saving = ref(false);
const page = ref(1);
const total = ref(0);
const pageSize = ref(10);
let loadSequence = 0;
const pager = createNumberedPage<AutoSignAuthorization>((state) => {
  items.value = state.items;
  total.value = state.total;
  page.value = state.currentPage;
  pageSize.value = state.pageSize;
  gridApi.setGridOptions({ pagerConfig: { enabled: true, currentPage: state.currentPage, pageSize: state.pageSize, total: state.total } });
}, () => { candidates.value = []; form.templateKeys = []; });
onBeforeUnmount(() => { loadSequence++; pager.invalidate(); });
const items = ref<AutoSignAuthorization[]>([]);
const candidates = ref<
  Array<{ name: string; templateNo: string; versionNo: number }>
>([]);
const form = reactive({
  allowManualOptional: false,
  allowRequiredOrder: true,
  currencyCode: "CNY",
  expiresAt: undefined as Dayjs | undefined,
  maxAmountMajor: undefined as string | undefined,
  successLimit: 100,
  templateKeys: [] as string[],
});

const candidateOptions = computed(() =>
  candidates.value.map((item) => ({
    label: `${item.name} · v${item.versionNo}`,
    value: `${item.templateNo}:${item.versionNo}`,
  })),
);
const canSubmit = computed(
  () =>
    form.templateKeys.length > 0 &&
    (form.allowRequiredOrder || form.allowManualOptional) &&
    Boolean(form.currencyCode.match(/^[A-Z]{3}$/)) &&
    Boolean(toMinorUnits(form.maxAmountMajor ?? "", form.currencyCode)) &&
    form.successLimit > 0 &&
    Boolean(form.expiresAt),
);
function columns(): VxeGridProps<AutoSignAuthorization>["columns"] { return [
  { field: "currencyCode", width: 100, title: $t("plugin.idc-compliance.contract.autoSign.currency") },
  { field: "sources", slots: { default: "sources" }, minWidth: 160, title: $t("plugin.idc-compliance.contract.autoSign.sourceTitle") },
  { field: "limits", slots: { default: "limits" }, minWidth: 150, title: $t("plugin.idc-compliance.contract.autoSign.limits") },
  { field: "templates", slots: { default: "templates" }, minWidth: 220, title: $t("plugin.idc-compliance.contract.autoSign.templates") },
  { field: "expires", slots: { default: "expires" }, width: 180, title: $t("plugin.idc-compliance.contract.autoSign.expiresAt") },
  { field: "status", slots: { default: "status" }, width: 120, title: $t("plugin.idc-compliance.contract.autoSign.status") },
  { field: "actions", fixed: "right", slots: { default: "actions" }, width: 120, title: $t("plugin.idc-compliance.contract.autoSign.actions") },
]; }
const [Grid, gridApi] = useVbenVxeGrid({ gridEvents: { pageChange: ({ currentPage, pageSize }: { currentPage: number; pageSize: number }) => load(currentPage,pageSize) }, gridOptions: { columns: columns(), data: [], emptyRender: { name: "Empty" }, height: "100%", pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0, pageSizes: [10,20,50,100] }, rowConfig: { keyField: "authorizationNo" }, toolbarConfig: { custom: true, refresh: false, zoom: true }, id: "idc-electronic-contract-auto-sign" } });
watch(items, (rows) => gridApi.setGridOptions({ data: rows }), { deep: true });
watch(() => preferences.app.locale, () => gridApi.setGridOptions({ columns: columns() }));

function key(prefix: string) {
  return `${prefix}-${crypto.randomUUID()}`;
}

function minorUnitDigits(currency: string) {
  return (
    new Intl.NumberFormat(undefined, {
      currency,
      style: "currency",
    }).resolvedOptions().maximumFractionDigits ?? 2
  );
}

function toMinorUnits(value: string, currency: string) {
  const normalized = value.trim();
  if (!/^\d+(?:\.\d+)?$/.test(normalized)) return "";
  const digits = minorUnitDigits(currency);
  const [whole = "0", fraction = ""] = normalized.split(".");
  if (fraction.length > digits) return "";
  const minor = `${whole}${fraction.padEnd(digits, "0")}`.replace(
    /^0+(?=\d)/,
    "",
  );
  if (!/^\d+$/.test(minor) || minor === "0" || minor.length > 18) return "";
  return minor;
}

function formatMoney(value: string, currency: string) {
  const digits = minorUnitDigits(currency);
  const scale = 10n ** BigInt(digits);
  const minor = BigInt(value);
  const whole = minor / scale;
  const fraction = minor % scale;
  const groupedWhole = new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 0,
    useGrouping: true,
  }).format(whole);
  const decimal = digits ? `.${fraction.toString().padStart(digits, "0")}` : "";
  return `${currency} ${groupedWhole}${decimal}`;
}

function formatTime(value?: number) {
  return value
    ? new Intl.DateTimeFormat(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(value)
    : "—";
}

function disabledExpiryDate(value: Dayjs) {
  const maximum = new Date();
  maximum.setFullYear(maximum.getFullYear() + 1);
  return (
    value.endOf("day").valueOf() < Date.now() ||
    value.startOf("day").valueOf() > maximum.getTime()
  );
}

function sourceText(item: AutoSignAuthorization) {
  const values: string[] = [];
  if (item.allowRequiredOrder)
    values.push(
      $t("plugin.idc-compliance.contract.autoSign.sources.required"),
    );
  if (item.allowManualOptional)
    values.push(
      $t("plugin.idc-compliance.contract.autoSign.sources.optional"),
    );
  return values.join("、");
}

async function load(requestedPage = page.value, requestedSize = pageSize.value) {
  const sequence = ++loadSequence;
  loading.value = true;
  try {
    await pager.query({ currentPage: requestedPage, pageSize: requestedSize }, async (page, pageSize) => {
      const result = await autoSignAuthorizationList(page, pageSize);
      if(sequence === loadSequence) candidates.value = result.candidates;
      return result;
    });
  } finally {
    if(sequence === loadSequence) loading.value = false;
  }
}

function createAuthorization() {
  if (!canSubmit.value || saving.value) return;
  const amount = toMinorUnits(form.maxAmountMajor!, form.currencyCode);
  if (!amount) {
    message.error(
      $t("plugin.idc-compliance.contract.autoSign.invalidAmount"),
    );
    return;
  }
  const selected = form.templateKeys.map((value) => {
    const separator = value.lastIndexOf(":");
    return {
      templateNo: value.slice(0, separator),
      versionNo: Number(value.slice(separator + 1)),
    };
  });
  const selectedNames = selected.map((item) => {
    const candidate = candidates.value.find(
      (value) =>
        value.templateNo === item.templateNo &&
        value.versionNo === item.versionNo,
    );
    return `${candidate?.name ?? item.templateNo} · v${item.versionNo}`;
  });
  const selectedSources = [
    form.allowRequiredOrder
      ? $t("plugin.idc-compliance.contract.autoSign.sources.required")
      : "",
    form.allowManualOptional
      ? $t("plugin.idc-compliance.contract.autoSign.sources.optional")
      : "",
  ].filter(Boolean);
  const details = [
    [
      $t("plugin.idc-compliance.contract.autoSign.templates"),
      selectedNames.join("；"),
    ],
    [
      $t("plugin.idc-compliance.contract.autoSign.sourceTitle"),
      selectedSources.join("；"),
    ],
    [
      $t("plugin.idc-compliance.contract.autoSign.expiresAt"),
      formatTime(form.expiresAt!.valueOf()),
    ],
    [
      $t("plugin.idc-compliance.contract.autoSign.currency"),
      form.currencyCode,
    ],
    [
      $t("plugin.idc-compliance.contract.autoSign.maxAmount"),
      formatMoney(amount, form.currencyCode),
    ],
    [
      $t("plugin.idc-compliance.contract.autoSign.successLimit"),
      String(form.successLimit),
    ],
  ];
  Modal.confirm({
    title: $t("plugin.idc-compliance.contract.autoSign.confirmTitle"),
    content: h("div", { style: { display: "grid", gap: "12px" } }, [
      ...details.map(([label, value]) =>
        h("div", { style: { display: "grid", gap: "2px" } }, [
          h("strong", label),
          h("span", value),
        ]),
      ),
      h(
        "p",
        { style: { margin: "4px 0 0" } },
        $t(
          "plugin.idc-compliance.contract.autoSign.confirmImmutability",
        ),
      ),
    ]),
    okText: $t("plugin.idc-compliance.contract.autoSign.enable"),
    async onOk() {
      saving.value = true;
      try {
        await autoSignAuthorizationCreate({
          allowManualOptional: form.allowManualOptional,
          allowRequiredOrder: form.allowRequiredOrder,
          confirmed: true,
          currencyCode: form.currencyCode,
          expiresAt: form.expiresAt!.valueOf(),
          idempotencyKey: key("contract-auto-sign-enable"),
          maxAmountMinor: amount,
          successLimit: form.successLimit,
          templates: selected,
        });
        message.success(
          $t("plugin.idc-compliance.contract.autoSign.enabled"),
        );
        page.value = 1;
        await load();
      } finally {
        saving.value = false;
      }
    },
  });
}

function revoke(item: AutoSignAuthorization) {
  Modal.confirm({
    title: $t("plugin.idc-compliance.contract.autoSign.revokeTitle"),
    content: $t(
      "plugin.idc-compliance.contract.autoSign.revokeDescription",
    ),
    okButtonProps: { danger: true },
    okText: $t("plugin.idc-compliance.contract.autoSign.revoke"),
    async onOk() {
      await autoSignAuthorizationRevoke(
        item.authorizationNo,
        item.version,
        key("contract-auto-sign-revoke"),
      );
      message.success(
        $t("plugin.idc-compliance.contract.autoSign.revoked"),
      );
      await load();
    },
  });
}

onMounted(load);
</script>

<template>
  <div class="auto-sign-shell" data-testid="contract-auto-sign-manager">
    <Alert
      show-icon
      type="warning"
      :message="
        $t('plugin.idc-compliance.contract.autoSign.warningTitle')
      "
      :description="
        $t('plugin.idc-compliance.contract.autoSign.warningDescription')
      "
    />
    <Card
      class="auto-sign-create"
      :title="$t('plugin.idc-compliance.contract.autoSign.createTitle')"
    >
      <div class="form-grid">
        <label class="wide">
          <span>{{
            $t("plugin.idc-compliance.contract.autoSign.templates")
          }}</span>
          <Select
            v-model:value="form.templateKeys"
            data-testid="contract-auto-sign-templates"
            mode="multiple"
            :options="candidateOptions"
            :placeholder="
              $t(
                'plugin.idc-compliance.contract.autoSign.templatesPlaceholder',
              )
            "
          />
        </label>
        <label class="wide">
          <span>{{
            $t("plugin.idc-compliance.contract.autoSign.sourceTitle")
          }}</span>
          <Space wrap>
            <Checkbox
              v-model:checked="form.allowRequiredOrder"
              data-testid="contract-auto-sign-required"
            >
              {{
                $t(
                  "plugin.idc-compliance.contract.autoSign.sources.required",
                )
              }}
            </Checkbox>
            <Checkbox
              v-model:checked="form.allowManualOptional"
              data-testid="contract-auto-sign-optional"
            >
              {{
                $t(
                  "plugin.idc-compliance.contract.autoSign.sources.optional",
                )
              }}
            </Checkbox>
          </Space>
        </label>
        <label>
          <span>{{
            $t("plugin.idc-compliance.contract.autoSign.currency")
          }}</span>
          <Input
            v-model:value="form.currencyCode"
            :maxlength="3"
            data-testid="contract-auto-sign-currency"
          />
        </label>
        <label>
          <span>{{
            $t("plugin.idc-compliance.contract.autoSign.maxAmount")
          }}</span>
          <InputNumber
            v-model:value="form.maxAmountMajor"
            :min="0.01"
            string-mode
            data-testid="contract-auto-sign-amount"
          />
        </label>
        <label>
          <span>{{
            $t("plugin.idc-compliance.contract.autoSign.successLimit")
          }}</span>
          <InputNumber
            v-model:value="form.successLimit"
            :min="1"
            :precision="0"
            data-testid="contract-auto-sign-limit"
          />
        </label>
        <label>
          <span>{{
            $t("plugin.idc-compliance.contract.autoSign.expiresAt")
          }}</span>
          <div data-testid="contract-auto-sign-expiry">
            <DatePicker
              v-model:value="form.expiresAt"
              :disabled-date="disabledExpiryDate"
              show-time
            />
          </div>
          <small>{{
            $t("plugin.idc-compliance.contract.autoSign.maxExpiryHelp")
          }}</small>
        </label>
      </div>
      <Button
        type="primary"
        :disabled="!canSubmit"
        :loading="saving"
        data-testid="contract-auto-sign-enable"
        @click="createAuthorization"
      >
        {{ $t("plugin.idc-compliance.contract.autoSign.enable") }}
      </Button>
    </Card>
    <Card
      class="auto-sign-history"
      :title="
        $t('plugin.idc-compliance.contract.autoSign.historyTitle')
      "
    >
      <Grid :loading="loading" class="min-h-0 min-w-0">
          <template #sources="{ row: record }">{{ sourceText(record) }}</template>
          <template #limits="{ row: record }">
            <div>
              {{ formatMoney(record.maxAmountMinor, record.currencyCode) }}
            </div>
            <small>{{ record.successCount }} / {{ record.successLimit }}</small>
          </template>
          <template #templates="{ row: record }">
            <div
              v-for="template in record.templates"
              :key="`${template.templateNo}:${template.versionNo}`"
            >
              {{ template.name }} · v{{ template.versionNo }}
            </div>
          </template>
          <template #expires="{ row: record }">{{
            formatTime(record.expiresAt)
          }}</template>
          <template #status="{ row: record }"
            ><Tag
              :color="record.statusCode === 'active' ? 'green' : 'default'"
              >{{
                $t(
                  `plugin.idc-compliance.contract.autoSign.statuses.${record.statusCode}`,
                )
              }}</Tag
            ></template
          >
          <template #actions="{ row: record }">
            <Button
              v-if="record.statusCode === 'active'"
              danger
              type="link"
              data-testid="contract-auto-sign-revoke"
              @click="revoke(record)"
              >{{
                $t("plugin.idc-compliance.contract.autoSign.revoke")
              }}</Button
            >
          </template>
      </Grid>
    </Card>
  </div>
</template>

<style scoped>
.auto-sign-shell {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}
.auto-sign-shell > :deep(.ant-alert) { flex: none; }
.auto-sign-create { flex: 0 0 120px; min-height: 120px; overflow: auto; }
.auto-sign-history { display: flex; flex: 1; flex-direction: column; min-height: 0; min-width: 0; overflow: hidden; }
.auto-sign-history :deep(.ant-card-head) { flex: none; }
.auto-sign-history :deep(.ant-card-body) { flex: 1; min-height: 0; min-width: 0; padding: 0; }

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 18px;
}

.form-grid label {
  display: grid;
  gap: 8px;
}

.form-grid label > span {
  font-size: 12px;
  font-weight: 700;
  color: hsl(var(--muted-foreground));
}

.form-grid label > small {
  font-size: 11px;
  color: hsl(var(--muted-foreground));
}

.form-grid .wide {
  grid-column: 1 / -1;
}

.form-grid :deep(.ant-input-number),
.form-grid :deep(.ant-picker) {
  width: 100%;
}

@media (max-width: 760px) {
  .form-grid {
    grid-template-columns: 1fr;
  }

  .form-grid .wide {
    grid-column: auto;
  }
}
</style>

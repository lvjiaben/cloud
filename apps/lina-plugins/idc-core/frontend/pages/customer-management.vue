<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/customers",
  title: "Customer Management",
};
</script>

<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { VbenFormSchema } from "#/adapter/form";

import type { VxeGridProps } from "#/adapter/vxe-table";

import type {
  CustomerListParams,
  CustomerSegment,
  CustomerStatus,
  CustomerSummary,
  CustomerType,
  VerificationStatus,
} from "./customer-client";

import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
} from "vue";

import { useAccess } from "./plugin-access";
import { createNumberedPage } from "./numbered-page";
import { Page, useVbenDrawer, useVbenModal } from "@vben/common-ui";
import { $te } from "@vben/locales";
import { preferences } from "@vben/preferences";

import {
  Alert as AAlert,
  Badge,
  Button as AButton,
  Modal,
  Space,
  Tag,
  message,
} from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid, vxeCheckboxChecked } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";

import enPluginMessages from "../../manifest/i18n/en-US/plugin.json";
import zhPluginMessages from "../../manifest/i18n/zh-CN/plugin.json";
import CustomerOverviewDrawer from "./components/customer-overview-drawer.vue";
import CustomerTagBatchModal from "./components/customer-tag-batch-modal.vue";
import {
  customerActivationCreate,
  customerActivationResend,
  customerDirectCreate,
  customerList,
  customerPasswordReset,
  customerSegments,
} from "./customer-client";
import { buildCountryOptions, callingCodeOptions } from "./country-options";

const accessCodes = {
  create: "idc:admin:customer:create",
  query: "idc:admin:customer:query",
  tagBatch: "idc:admin:customer:tag:batch",
  update: "idc:admin:customer:update",
};

const { hasAccessByCodes } = useAccess();
const pagination = createNumberedPage<CustomerSummary>(
  ({ currentPage, pageSize, total }) => gridApi.setGridOptions({ pagerConfig: { currentPage, pageSize, total } }),
  () => { customerOverviewDrawerApi.close(); customerTagBatchModalApi.close(); void gridApi.grid?.clearCheckboxRow(); },
);
let activeFilterSignature = "";
const groupSegmentParams = reactive({ namePrefix: "" });
const tagSegmentParams = reactive({ namePrefix: "" });
const segmentSearchTimers: Partial<
  Record<"group" | "tag", ReturnType<typeof setTimeout>>
> = {};

const canViewOverview = computed(() => hasAccessByCodes([accessCodes.query]));
const canBatchTags = computed(() => hasAccessByCodes([accessCodes.tagBatch]));
const canCreateCustomer = computed(() =>
  hasAccessByCodes([accessCodes.create]),
);
const canResetPassword = computed(() => hasAccessByCodes([accessCodes.update]));
const countryOptions = computed(() =>
  buildCountryOptions(preferences.app.locale || "zh-CN"),
);
const activationSaving = ref(false);
const activationResending = ref("");
const activationRequestKey = ref("");
const directSaving = ref(false);
const directRequestKey = ref("");
const passwordResetSaving = ref(false);
const passwordResetCustomer = ref<CustomerSummary | null>(null);

function baseCustomerFormSchema(
  mode: "activation" | "direct",
): VbenFormSchema[] {
  return [
    {
      component: "Input",
      componentProps: {
        "data-testid":
          mode === "activation"
            ? "customer-activation-name"
            : "customer-direct-name",
        maxlength: 256,
      },
      fieldName: "displayName",
      label: $t("plugin.idc-core.customer.activation.displayName"),
      rules: z
        .string()
        .trim()
        .min(1, {
          message: $t(
            "plugin.idc-core.customer.validation.displayNameRequired",
          ),
        })
        .max(256, {
          message: $t("plugin.idc-core.customer.validation.displayNameTooLong"),
        }),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid":
          mode === "activation" ? "customer-activation-country" : undefined,
        optionFilterProp: "label",
        options: countryOptions.value,
        showSearch: true,
      },
      defaultValue: "CN",
      fieldName: "countryCode",
      label: $t("plugin.idc-core.customer.activation.country"),
      rules: z.string().length(2, {
        message: $t("plugin.idc-core.customer.validation.countryRequired"),
      }),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid":
          mode === "activation" ? "customer-activation-locale" : undefined,
        options: [
          {
            label: $t("plugin.idc-core.customer.activation.localeZhCN"),
            value: "zh-CN",
          },
          {
            label: $t("plugin.idc-core.customer.activation.localeEnUS"),
            value: "en-US",
          },
        ],
      },
      defaultValue: "zh-CN",
      fieldName: "preferredLocale",
      label: $t("plugin.idc-core.customer.activation.locale"),
      rules: z.enum(["zh-CN", "en-US"]),
    },
    {
      component: "RadioGroup",
      componentProps: {
        "data-testid":
          mode === "activation"
            ? "customer-activation-destination-kind"
            : undefined,
        buttonStyle: "solid",
        optionType: "button",
        options: [
          {
            label: $t("plugin.idc-core.customer.activation.email"),
            value: "email",
          },
          {
            label: $t("plugin.idc-core.customer.activation.sms"),
            value: "sms",
          },
        ],
      },
      defaultValue: "email",
      fieldName: "destinationKind",
      label:
        mode === "activation"
          ? $t("plugin.idc-core.customer.activation.destination")
          : directText("login"),
      rules: z.enum(["email", "sms"]),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid":
          mode === "activation"
            ? "customer-activation-email"
            : "customer-direct-email",
      },
      dependencies: {
        show: (values) => values.destinationKind === "email",
        triggerFields: ["destinationKind"],
      },
      fieldName: "email",
      label: $t("plugin.idc-core.customer.activation.email"),
      rules: z
        .string()
        .trim()
        .email({
          message: $t("plugin.idc-core.customer.validation.emailInvalid"),
        })
        .optional()
        .or(z.literal("")),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid":
          mode === "activation" ? "customer-activation-phone-code" : undefined,
        optionFilterProp: "label",
        options: callingCodeOptions,
        showSearch: true,
      },
      defaultValue: "+86",
      dependencies: {
        show: (values) => values.destinationKind === "sms",
        triggerFields: ["destinationKind"],
      },
      fieldName: "phoneCountryCode",
      label: directText("phone"),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid":
          mode === "activation"
            ? "customer-activation-phone"
            : "customer-direct-phone",
      },
      dependencies: {
        show: (values) => values.destinationKind === "sms",
        triggerFields: ["destinationKind"],
      },
      fieldName: "phone",
      label: $t("plugin.idc-core.customer.activation.sms"),
    },
  ];
}

function activationFormSchema(): VbenFormSchema[] {
  return [
    ...baseCustomerFormSchema("activation"),
    {
      component: "Textarea",
      componentProps: {
        "data-testid": "customer-activation-reason",
        maxlength: 512,
        rows: 3,
        showCount: true,
      },
      fieldName: "reason",
      label: $t("plugin.idc-core.customer.activation.reason"),
      rules: z
        .string()
        .trim()
        .min(10, {
          message: $t("plugin.idc-core.customer.validation.reasonLength"),
        })
        .max(512, {
          message: $t("plugin.idc-core.customer.validation.reasonLength"),
        }),
    },
    {
      component: "Checkbox",
      componentProps: { "data-testid": "customer-activation-bind-existing" },
      defaultValue: false,
      fieldName: "bindExistingIdentity",
      label: $t("plugin.idc-core.customer.activation.bindExisting"),
    },
  ];
}

function directFormSchema(): VbenFormSchema[] {
  const schema = baseCustomerFormSchema("direct");
  schema.push(
    {
      component: "InputPassword",
      componentProps: {
        "data-testid": "customer-direct-password",
        autocomplete: "new-password",
        maxlength: 128,
      },
      fieldName: "initialPassword",
      label: directText("password"),
      rules: z
        .string()
        .min(8, {
          message: $t("plugin.idc-core.customer.validation.passwordLength"),
        })
        .max(128, {
          message: $t("plugin.idc-core.customer.validation.passwordLength"),
        }),
    },
    {
      component: "Textarea",
      componentProps: {
        "data-testid": "customer-direct-reason",
        maxlength: 512,
        rows: 3,
        showCount: true,
      },
      fieldName: "reason",
      label: $t("plugin.idc-core.customer.activation.reason"),
      rules: z
        .string()
        .trim()
        .min(10, {
          message: $t("plugin.idc-core.customer.validation.reasonLength"),
        })
        .max(512, {
          message: $t("plugin.idc-core.customer.validation.reasonLength"),
        }),
    },
  );
  return schema;
}

function passwordResetFormSchema(): VbenFormSchema[] {
  return [
    {
      component: "InputPassword",
      componentProps: {
        "data-testid": "customer-password-reset-password",
        autocomplete: "new-password",
        maxlength: 128,
      },
      fieldName: "newPassword",
      label: $t("plugin.idc-core.customer.password.new"),
      rules: z
        .string()
        .min(8, {
          message: $t("plugin.idc-core.customer.validation.passwordLength"),
        })
        .max(128, {
          message: $t("plugin.idc-core.customer.validation.passwordLength"),
        }),
    },
    {
      component: "Textarea",
      componentProps: {
        "data-testid": "customer-password-reset-reason",
        maxlength: 512,
        rows: 3,
        showCount: true,
      },
      fieldName: "reason",
      label: $t("plugin.idc-core.customer.password.reason"),
      rules: z
        .string()
        .trim()
        .min(10, {
          message: $t("plugin.idc-core.customer.validation.reasonLength"),
        })
        .max(512, {
          message: $t("plugin.idc-core.customer.validation.reasonLength"),
        }),
    },
  ];
}

const [CustomerActivationForm, activationFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 120 },
  schema: [],
  showDefaultActions: false,
});
const [DirectCustomerForm, directFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 120 },
  schema: [],
  showDefaultActions: false,
});
const [CustomerPasswordResetForm, passwordResetFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 120 },
  schema: [],
  showDefaultActions: false,
});

const [ActivationModal, activationModalApi] = useVbenModal({
  class: "w-[min(96vw,620px)]",
  onConfirm: createCustomerActivation,
  title: $t("plugin.idc-core.customer.activation.title"),
});
const [DirectModal, directModalApi] = useVbenModal({
  class: "w-[min(96vw,680px)]",
  onConfirm: createDirectCustomer,
  title: directText("title"),
});
const [PasswordResetModal, passwordResetModalApi] = useVbenModal({
  class: "w-[min(96vw,560px)]",
  onConfirm: resetCustomerPassword,
  title: $t("plugin.idc-core.customer.password.title"),
});

async function openPasswordReset(row: CustomerSummary) {
  passwordResetCustomer.value = row;
  passwordResetModalApi.open();
  await nextTick();
  try {
    await passwordResetFormApi.setState({ schema: passwordResetFormSchema() });
    await passwordResetFormApi.resetForm();
  } catch (error) {
    passwordResetModalApi.close();
    throw error;
  }
}

async function resetCustomerPassword() {
  const row = passwordResetCustomer.value;
  if (!row) return false;
  const { valid } = await passwordResetFormApi.validate();
  if (!valid) {
    message.warning($t("plugin.idc-core.customer.password.required"));
    return false;
  }
  const values = await passwordResetFormApi.getValues<{
    newPassword: string;
    reason: string;
  }>();
  passwordResetSaving.value = true;
  passwordResetModalApi.lock(true);
  try {
    await customerPasswordReset(row.customerNo, {
      idempotencyKey: `customer-password-reset-${crypto.randomUUID()}`,
      newPassword: values.newPassword,
      reason: values.reason.trim(),
    });
    passwordResetModalApi.close();
    message.success($t("plugin.idc-core.customer.password.saved"));
  } finally {
    await passwordResetFormApi.resetForm();
    passwordResetSaving.value = false;
    passwordResetModalApi.lock(false);
  }
}

type DirectMessageKey =
  | "created"
  | "description"
  | "login"
  | "notice"
  | "open"
  | "password"
  | "phone"
  | "required"
  | "title";

function directText(key: DirectMessageKey) {
  const messageKey = `plugin.idc-core.customer.direct.${key}`;
  if ($te(messageKey)) return $t(messageKey);
  const fallbackMessages = preferences.app.locale
    ?.toLowerCase()
    .startsWith("zh")
    ? zhPluginMessages
    : enPluginMessages;
  return fallbackMessages[messageKey as keyof typeof fallbackMessages];
}

async function openDirectCustomer() {
  directRequestKey.value = `customer-direct-${crypto.randomUUID()}`;
  directModalApi.open();
  await nextTick();
  try {
    await directFormApi.setState({ schema: directFormSchema() });
    await directFormApi.resetForm();
    await directFormApi.setValues({
      countryCode: "CN",
      destinationKind: "email",
      displayName: "",
      email: "",
      initialPassword: "",
      phone: "",
      phoneCountryCode: "+86",
      preferredLocale: "zh-CN",
      reason: "",
    });
  } catch (error) {
    directModalApi.close();
    throw error;
  }
}

async function createDirectCustomer() {
  if (directSaving.value) return false;
  const { valid } = await directFormApi.validate();
  const values = await directFormApi.getValues<{
    countryCode: string;
    destinationKind: "email" | "sms";
    displayName: string;
    email?: string;
    initialPassword: string;
    phone?: string;
    phoneCountryCode?: string;
    preferredLocale: "en-US" | "zh-CN";
    reason: string;
  }>();
  if (
    !valid ||
    (values.destinationKind === "email"
      ? !values.email?.trim()
      : !values.phone?.trim())
  ) {
    message.warning(directText("required"));
    return false;
  }
  directSaving.value = true;
  directModalApi.lock(true);
  try {
    await customerDirectCreate({
      countryCode: values.countryCode.trim().toUpperCase(),
      customerType: "individual",
      displayName: values.displayName.trim(),
      email:
        values.destinationKind === "email" ? values.email?.trim() : undefined,
      idempotencyKey: directRequestKey.value,
      initialPassword: values.initialPassword,
      phone:
        values.destinationKind === "sms" ? values.phone?.trim() : undefined,
      phoneCountryCode:
        values.destinationKind === "sms" ? values.phoneCountryCode : undefined,
      preferredLocale: values.preferredLocale,
      reason: values.reason.trim(),
    });
    directModalApi.close();
    message.success(directText("created"));
    await gridApi.query();
  } finally {
    await directFormApi.setFieldValue("initialPassword", "");
    directSaving.value = false;
    directModalApi.lock(false);
  }
}

async function openCustomerActivation() {
  activationRequestKey.value = `customer-activation-${crypto.randomUUID()}`;
  activationModalApi.open();
  await nextTick();
  try {
    await activationFormApi.setState({ schema: activationFormSchema() });
    await activationFormApi.resetForm();
    await activationFormApi.setValues({
      bindExistingIdentity: false,
      countryCode: "CN",
      destinationKind: "email",
      displayName: "",
      email: "",
      phone: "",
      phoneCountryCode: "+86",
      preferredLocale: "zh-CN",
      reason: "",
    });
  } catch (error) {
    activationModalApi.close();
    throw error;
  }
}

async function createCustomerActivation() {
  if (activationSaving.value) return false;
  const { valid } = await activationFormApi.validate();
  const values = await activationFormApi.getValues<{
    bindExistingIdentity?: boolean;
    countryCode: string;
    destinationKind: "email" | "sms";
    displayName: string;
    email?: string;
    phone?: string;
    phoneCountryCode?: string;
    preferredLocale: "en-US" | "zh-CN";
    reason: string;
  }>();
  if (
    !valid ||
    (values.destinationKind === "email"
      ? !values.email?.trim()
      : !values.phone?.trim())
  ) {
    message.warning($t("plugin.idc-core.customer.activation.required"));
    return false;
  }
  activationSaving.value = true;
  activationModalApi.lock(true);
  try {
    const result = await customerActivationCreate({
      bindExistingIdentity: Boolean(values.bindExistingIdentity),
      countryCode: values.countryCode.trim().toUpperCase(),
      customerType: "individual",
      displayName: values.displayName.trim(),
      email:
        values.destinationKind === "email" ? values.email?.trim() : undefined,
      idempotencyKey: activationRequestKey.value,
      phone:
        values.destinationKind === "sms" ? values.phone?.trim() : undefined,
      phoneCountryCode:
        values.destinationKind === "sms"
          ? values.phoneCountryCode?.trim()
          : undefined,
      preferredLocale: values.preferredLocale,
      reason: values.reason.trim(),
    });
    activationModalApi.close();
    message.success(
      $t("plugin.idc-core.customer.activation.sent", {
        destination: result.destinationHint,
      }),
    );
    await gridApi.query();
  } finally {
    activationSaving.value = false;
    activationModalApi.lock(false);
  }
}

function resendCustomerActivation(row: CustomerSummary) {
  if (activationResending.value) return;
  Modal.confirm({
    content: $t("plugin.idc-core.customer.activation.resendDescription"),
    okText: $t("plugin.idc-core.customer.activation.resendConfirm"),
    title: $t("plugin.idc-core.customer.activation.resendTitle"),
    async onOk() {
      activationResending.value = row.customerNo;
      try {
        const result = await customerActivationResend(row.customerNo, {
          idempotencyKey: `customer-activation-resend-${crypto.randomUUID()}`,
          reason: $t("plugin.idc-core.customer.activation.resendReason"),
        });
        message.success(
          $t("plugin.idc-core.customer.activation.resent", {
            destination: result.destinationHint,
          }),
        );
      } finally {
        activationResending.value = "";
      }
    },
  });
}

function buildSearchSchema(): VbenFormProps["schema"] {
  const allOption = {
    label: $t("plugin.idc-core.customer.search.all"),
    value: "",
  };
  return [
    {
      component: "Input",
      componentProps: {
        "data-testid": "customer-search-code",
        maxlength: 18,
        placeholder: $t("plugin.idc-core.customer.search.codePlaceholder"),
      },
      fieldName: "customerCode",
      label: $t("plugin.idc-core.customer.search.code"),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "customer-search-phone",
        maxlength: 20,
        placeholder: $t("plugin.idc-core.customer.search.phonePlaceholder"),
      },
      fieldName: "phone",
      label: $t("plugin.idc-core.customer.search.phone"),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "customer-search-name",
        maxlength: 100,
        placeholder: $t("plugin.idc-core.customer.search.namePlaceholder"),
      },
      fieldName: "namePrefix",
      label: $t("plugin.idc-core.customer.search.name"),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": "customer-search-type",
        options: [
          allOption,
          {
            label: $t("plugin.idc-core.customer.type.individual"),
            value: "individual",
          },
          {
            label: $t("plugin.idc-core.customer.type.enterprise"),
            value: "enterprise",
          },
        ],
      },
      defaultValue: "",
      fieldName: "type",
      label: $t("plugin.idc-core.customer.search.type"),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": "customer-search-status",
        options: [
          allOption,
          ...[
            "pending_activation",
            "active",
            "restricted",
            "disabled",
            "anonymized",
          ].map((value) => ({
            label: $t(`plugin.idc-core.customer.status.${value}`),
            value,
          })),
        ],
      },
      defaultValue: "",
      fieldName: "status",
      label: $t("plugin.idc-core.customer.search.status"),
    },
    {
      component: "ApiSelect",
      componentProps: {
        "data-testid": "customer-search-group",
        api: (params: { namePrefix?: string }) =>
          segmentOptions("group", params.namePrefix || ""),
        filterOption: false,
        labelField: "label",
        onSearch: (value: string) => scheduleSegmentSearch("group", value),
        params: groupSegmentParams,
        showSearch: true,
        valueField: "value",
      },
      defaultValue: "",
      fieldName: "groupNo",
      label: $t("plugin.idc-core.customer.search.group"),
    },
    {
      component: "ApiSelect",
      componentProps: {
        "data-testid": "customer-search-tag",
        api: (params: { namePrefix?: string }) =>
          segmentOptions("tag", params.namePrefix || ""),
        filterOption: false,
        labelField: "label",
        onSearch: (value: string) => scheduleSegmentSearch("tag", value),
        params: tagSegmentParams,
        showSearch: true,
        valueField: "value",
      },
      defaultValue: "",
      fieldName: "tagNo",
      label: $t("plugin.idc-core.customer.search.tag"),
    },
  ];
}

function scheduleSegmentSearch(kind: "group" | "tag", value: string) {
  const active = segmentSearchTimers[kind];
  if (active) clearTimeout(active);
  segmentSearchTimers[kind] = setTimeout(() => {
    const target = kind === "group" ? groupSegmentParams : tagSegmentParams;
    target.namePrefix = value.trim();
    delete segmentSearchTimers[kind];
  }, 250);
}

function localizedSegmentName(item: CustomerSegment) {
  return preferences.app.locale === "zh-CN"
    ? item.nameZhCn || item.nameEnUs || item.code
    : item.nameEnUs || item.nameZhCn || item.code;
}

async function segmentOptions(kind: "group" | "tag", namePrefix: string) {
  const locale = preferences.app.locale === "zh-CN" ? "zh-CN" : "en-US";
  const result = await customerSegments({
    kind,
    limit: 25,
    locale,
    namePrefix,
  });
  return [
    {
      label: $t("plugin.idc-core.customer.search.all"),
      value: "",
    },
    ...result.items.map((item) => ({
      label: localizedSegmentName(item),
      value: item.number,
    })),
  ];
}

function buildColumns(): VxeGridProps["columns"] {
  return [
    { type: "checkbox", width: 52 },
    {
      field: "customerCode",
      width: 96,
      title: $t("plugin.idc-core.customer.table.customerNo"),
    },
    {
      field: "primaryPhone",
      width: 150,
      title: $t("plugin.idc-core.customer.table.phone"),
    },
    {
      field: "displayName",
      minWidth: 160,
      title: $t("plugin.idc-core.customer.table.customer"),
    },
    {
      field: "type",
      slots: { default: "type" },
      title: $t("plugin.idc-core.customer.table.type"),
      width: 220,
    },
    {
      field: "status",
      slots: { default: "status" },
      title: $t("plugin.idc-core.customer.table.status"),
      width: 142,
    },
    {
      field: "verification",
      slots: { default: "verification" },
      title: $t("plugin.idc-core.customer.table.verification"),
      width: 130,
    },
    {
      field: "level",
      slots: { default: "level" },
      title: $t("plugin.idc-core.customer.table.level"),
      width: 150,
    },
    {
      field: "tagCount",
      slots: { default: "tagCount" },
      title: $t("plugin.idc-core.customer.table.tags"),
      width: 100,
    },
    {
      field: "createdAt",
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      minWidth: 180,
      title: $t("plugin.idc-core.customer.table.createdAt"),
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: $t("plugin.idc-core.customer.table.actions"),
      width: 220,
    }),
  ];
}

function normalizeFilters(values: Record<string, unknown>): CustomerListParams {
  return {
    customerCode: String(values.customerCode || "").trim(),
    namePrefix: String(values.namePrefix || "").trim(),
    phone: String(values.phone || "").trim(),
    groupNo: String(values.groupNo || ""),
    status: String(values.status || "") as CustomerStatus | "",
    tagNo: String(values.tagNo || ""),
    type: String(values.type || "") as CustomerType | "",
  };
}

function filterSignature(filters: CustomerListParams) {
  return JSON.stringify([
    filters.customerCode || "",
    filters.namePrefix || "",
    filters.phone || "",
    filters.type || "",
    filters.status || "",
    filters.groupNo || "",
    filters.tagNo || "",
  ]);
}

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: {
      componentProps: { allowClear: true },
      labelWidth: 90,
    },
    schema: [],
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-3",
  },
  gridOptions: {
    checkboxConfig: { highlight: true, reserve: true },
    columns: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async ({ page }: { page: { currentPage: number; pageSize: number } }, formValues = {}) => {
          const filters = normalizeFilters(formValues);
          const signature = filterSignature(filters);
          if (signature !== activeFilterSignature) {
            activeFilterSignature = signature;
            page.currentPage = 1;
          }

          return pagination.query(page, async (currentPage, pageSize) => {
          const result = await customerList({
            ...filters,
            page: currentPage,
            limit: pageSize,
          });
          return { items: result.customers, total: result.total };
          });
        },
      },
    },
    rowConfig: { keyField: "customerNo" },
    toolbarConfig: { custom: true, refresh: true, zoom: true },
    id: "idc-customer-management",
  },
});

const [CustomerOverviewDrawerRef, customerOverviewDrawerApi] = useVbenDrawer({
  connectedComponent: CustomerOverviewDrawer,
});

const [CustomerTagBatchModalRef, customerTagBatchModalApi] = useVbenModal({
  connectedComponent: CustomerTagBatchModal,
});

function localizedLevelName(row: CustomerSummary) {
  if (!row.level) return "—";
  return preferences.app.locale === "zh-CN"
    ? row.level.nameZhCn || row.level.nameEnUs || row.level.code
    : row.level.nameEnUs || row.level.nameZhCn || row.level.code;
}

function statusBadge(status: CustomerStatus) {
  if (status === "active") return "success";
  if (status === "pending_activation") return "processing";
  if (status === "restricted") return "warning";
  return "default";
}

function verificationColor(status: VerificationStatus) {
  if (status === "verified") return "success";
  if (status === "pending") return "processing";
  if (status === "unverified") return "default";
  return "error";
}

function openOverview(row: CustomerSummary) {
  customerOverviewDrawerApi.setData({ customerNo: row.customerNo });
  customerOverviewDrawerApi.open();
}

function openTagBatch() {
  const rows = (gridApi.grid?.getCheckboxRecords?.() ??
    []) as CustomerSummary[];
  if (rows.length === 0) return;
  if (rows.length > 100) {
    message.warning(
      $t("plugin.idc-core.customer.tagBatch.tooMany", { count: 100 }),
    );
    return;
  }
  customerTagBatchModalApi.setData({ rows });
  customerTagBatchModalApi.open();
}

async function handleTagBatchSuccess() {
  await gridApi.grid?.clearCheckboxRow?.();
  await gridApi.query();
}


onMounted(async () => {
  gridApi.formApi.setState({ schema: buildSearchSchema() });
  gridApi.setGridOptions({ columns: buildColumns() });
  await gridApi.query();
});

onBeforeUnmount(() => {
  pagination.invalidate();
  Object.values(segmentSearchTimers).forEach((timer) => clearTimeout(timer));
});
</script>

<template>
  <Page :auto-content-height="true" data-testid="customer-management-page">
    <div class="customer-shell">
      <div class="customer-grid">
        <Grid>
          <template #toolbar-tools>
            <Space wrap>
              <a-button
                v-if="canCreateCustomer"
                data-testid="customer-activation-open"
                type="primary"
                @click="openCustomerActivation"
              >
                {{ $t("plugin.idc-core.customer.activation.open") }}
              </a-button>
              <a-button
                v-if="canCreateCustomer"
                data-testid="customer-direct-open"
                @click="openDirectCustomer"
              >
                {{ directText("open") }}
              </a-button>
              <a-button
                v-if="canBatchTags"
                data-testid="customer-tag-batch-open"
                :disabled="!vxeCheckboxChecked(gridApi)"
                @click="openTagBatch"
              >
                {{ $t("plugin.idc-core.customer.actions.batchTags") }}
              </a-button>
            </Space>
          </template>

          <template #type="{ row }">
            <Tag>{{ $t(`plugin.idc-core.customer.type.${row.type}`) }}</Tag>
          </template>

          <template #status="{ row }">
            <Badge
              :status="statusBadge(row.status)"
              :text="$t(`plugin.idc-core.customer.status.${row.status}`)"
            />
          </template>

          <template #verification="{ row }">
            <Tag :color="verificationColor(row.verification)">
              {{
                $t(`plugin.idc-core.customer.verification.${row.verification}`)
              }}
            </Tag>
          </template>

          <template #level="{ row }">
            <span class="level-name">{{ localizedLevelName(row) }}</span>
          </template>

          <template #tagCount="{ row }">
            <span class="tag-count">{{ row.tagCount }}</span>
          </template>

          <template #action="{ row }">
            <Space>
              <ghost-button
                v-if="canViewOverview"
                :data-testid="`customer-overview-${row.customerNo}`"
                @click="openOverview(row)"
              >
                {{ $t("plugin.idc-core.customer.actions.overview") }}
              </ghost-button>
              <a-button
                v-if="canCreateCustomer && row.status === 'pending_activation'"
                :data-testid="`customer-activation-resend-${row.customerNo}`"
                :loading="activationResending === row.customerNo"
                size="small"
                type="link"
                @click="resendCustomerActivation(row)"
              >
                {{ $t("plugin.idc-core.customer.activation.resend") }}
              </a-button>
              <a-button
                v-if="canResetPassword && row.status === 'active'"
                :data-testid="`customer-password-reset-${row.customerNo}`"
                size="small"
                type="link"
                @click="openPasswordReset(row)"
              >
                {{ $t("plugin.idc-core.customer.password.action") }}
              </a-button>
            </Space>
          </template>
        </Grid>
      </div>
    </div>
    <CustomerOverviewDrawerRef />
    <CustomerTagBatchModalRef @success="handleTagBatchSuccess" />
    <ActivationModal>
      <a-alert
        class="mb-4"
        :description="$t('plugin.idc-core.customer.activation.description')"
        :message="$t('plugin.idc-core.customer.activation.noPassword')"
        show-icon
        type="info"
      />
      <CustomerActivationForm />
      <p class="activation-help">
        {{ $t("plugin.idc-core.customer.activation.separateIdentity") }}
      </p>
    </ActivationModal>
    <DirectModal>
      <a-alert
        class="mb-4"
        :description="directText('description')"
        :message="directText('notice')"
        show-icon
        type="warning"
      />
      <DirectCustomerForm />
    </DirectModal>
    <PasswordResetModal>
      <a-alert
        class="mb-4"
        :message="$t('plugin.idc-core.customer.password.notice')"
        show-icon
        type="warning"
      />
      <CustomerPasswordResetForm />
    </PasswordResetModal>
  </Page>
</template>

<style scoped>
.customer-shell {
  display: grid;
  grid-template-rows: minmax(0, 1fr);
  height: 100%;
  max-width: 100%;
  min-height: 0;
  min-width: 0;
  padding: 16px;
}

.page-indicator {
  color: hsl(var(--muted-foreground));
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.customer-grid {
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}

.customer-grid :deep(.vxe-grid) {
  max-width: 100%;
  min-width: 0;
  width: 100% !important;
}

.activation-grid {
  display: grid;
  gap: 0 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.activation-help {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  margin: 8px 0 0 24px;
}

@media (width <= 640px) {
  .customer-shell {
    padding: 16px;
  }

  .activation-grid {
    grid-template-columns: 1fr;
  }
}

.level-name {
  font-weight: 600;
}

.tag-count {
  background: hsl(var(--muted));
  border-radius: 999px;
  display: inline-grid;
  font-variant-numeric: tabular-nums;
  min-width: 28px;
  padding: 2px 8px;
  place-items: center;
}
</style>

<style>
.idc-plugin-form-surface .ant-form-item-label > label {
  align-items: center !important;
  display: inline-flex !important;
  flex-wrap: nowrap !important;
  white-space: nowrap;
}

.idc-plugin-form-surface .ant-form-item-required::before {
  flex: 0 0 auto;
}
</style>

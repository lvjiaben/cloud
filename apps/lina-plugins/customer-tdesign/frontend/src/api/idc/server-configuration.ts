import { pluginApiPath, requestClient } from "#/api/request";
export type ConfigurationValue = {
  fieldToken: string;
  type: string;
  textValue?: string;
  integerValue?: string;
  decimalValue?: string;
  booleanValue?: boolean;
  dateValue?: string;
  dateTimeValue?: string;
  optionTokens?: string[];
  treeOptionTokens?: string[];
  addressOptionTokens?: string[];
  fileTokens?: string[];
};
export type ConfigurationOption = {
  optionToken: string;
  labelZhCN: string;
  labelEnUS: string;
  children?: ConfigurationOption[];
};
export type ConfigurationField = {
  fieldToken: string;
  type: string;
  labelZhCN: string;
  labelEnUS: string;
  descriptionZhCN: string;
  descriptionEnUS: string;
  visible: boolean;
  required: boolean;
  disabled: boolean;
  options?: ConfigurationOption[];
  minimumInteger?: string;
  maximumInteger?: string;
  minimumDecimal?: string;
  maximumDecimal?: string;
  minimumLength?: number;
  maximumLength?: number;
  groupZhCN?: string;
  groupEnUS?: string;
  unit?: string;
  advanced?: boolean;
  numericChoices?: string[];
  numericLabels?: Record<string, { zhCN: string; enUS: string }>;
  quantityStep?: string;
  displayControl?: string;
};
export type ConfigurationContext = {
  entitlementNo: string;
  configurationRevision: string;
  currencyCode: string;
  purchaseCycles: number;
  ready: boolean;
  billingTermToken: string;
  fields: ConfigurationField[];
  values: ConfigurationValue[];
  issues: { fieldToken?: string; issueCode: string }[];
};
const path = (serverNo: string, suffix: string) =>
  pluginApiPath(
    "idc-resource",
    `account/native-servers/${encodeURIComponent(serverNo)}/${suffix}`,
  );
export function getServerConfiguration(
  serverNo: string,
  configurationRevision?: string,
  values?: ConfigurationValue[],
) {
  return requestClient.get<ConfigurationContext>(
    path(serverNo, "configuration-context"),
    {
      params: {
        configurationRevision,
        ...(values === undefined ? {} : { values: JSON.stringify(values) }),
      },
      silentErrorMessage: true,
    },
  );
}
export function freezeServerConfiguration(
  serverNo: string,
  context: ConfigurationContext,
) {
  return requestClient.post<{ quoteNo: string }>(
    path(serverNo, "target-quotes"),
    {
      configurationRevision: context.configurationRevision,
      billingTermToken: context.billingTermToken,
      values: context.values,
    },
    { silentErrorMessage: true },
  );
}

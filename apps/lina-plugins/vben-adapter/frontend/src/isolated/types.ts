export interface IsolatedTableColumn<Row extends Record<string, unknown>> {
  field: string;
  title: string;
  width?: number;
  minWidth?: number;
  fixed?: 'left' | 'right';
  align?: 'left' | 'center' | 'right';
  cell?: (row: Row) => Node | string | Array<Node | string>;
}

export interface IsolatedTableOptions<Row extends Record<string, unknown>> {
  columns: IsolatedTableColumn<Row>[];
  rows: Row[];
  rowKey: keyof Row & string;
  height?: number;
  locale?: string;
  tableTitle?: string;
  loading?: boolean;
  emptyText?: string;
  pagination?: IsolatedTablePagination;
  actions?: Array<{ key: string; label: string; disabled?: boolean; loading?: boolean; onClick: () => void | Promise<void> }>;
  /** Official Vben search form; the owner fetches data and commits its page. */
  search?: {
    fields: IsolatedFormField[];
    values?: Record<string, unknown>;
    onSubmit: (values: Record<string, unknown>) => void | Promise<void>;
  };
}

/** Caller-owned exact pagination; omitted for unpaged detail tables and trees. */
export interface IsolatedTablePagination {
  currentPage: number;
  pageSize: number;
  total: number;
  onChange: (page: { currentPage: number; pageSize: number }) => void;
}

export interface IsolatedTableHandle<Row extends Record<string, unknown>> {
  ready: Promise<void>;
  update(options: Partial<IsolatedTableOptions<Row>>): void;
  destroy(): void;
}

/** Plain-data fields for the current connector/pool configuration drawers. */
export interface IsolatedFormField {
  name: string;
  label: string;
  component: 'Input' | 'InputPassword' | 'Textarea' | 'InputNumber' | 'Select' | 'TreeSelect' | 'Cascader' | 'Switch' | 'DatePicker';
  required?: boolean;
  description?: string;
  props?: Record<string, unknown>;
}

export interface IsolatedFormDrawerOptions {
  title: string;
  description?: string;
  /** Already-localized business error; the runtime never interprets API errors. */
  errorText?: string;
  locale?: string;
  fields: IsolatedFormField[];
  values: Record<string, unknown>;
  readOnly?: boolean;
  /** Mount trusted owner UI, not HTML; cleanup runs on close or replacement. */
  mountContent?: (container: HTMLElement) => void | (() => void);
  /** Secondary form actions rendered with the official button component. */
  actions?: Array<{ key: string; label: string; disabled?: boolean; onClick: (values: Record<string, unknown>) => void | Promise<void> }>;
  onValuesChange?: (values: Record<string, unknown>) => void;
  /** Return false on a business failure to keep the employee's entered values. */
  onSubmit?: (values: Record<string, unknown>) => Promise<boolean>;
  onClosed?: () => void;
  onError?: (error: unknown) => void;
}

export interface IsolatedFormDrawerHandle {
  ready: Promise<void>;
  update(options: Partial<IsolatedFormDrawerOptions>): void;
  close(): void;
  destroy(): void;
}

/** Plain ECharts option; the isolated runtime registers the needed chart parts. */
export type IsolatedEchartsOption = Record<string, unknown>;

export interface IsolatedEchartsOptions {
  option: IsolatedEchartsOption;
  height?: number;
  width?: string;
  locale?: string;
}

export interface IsolatedEchartsHandle {
  ready: Promise<void>;
  update(options: Partial<IsolatedEchartsOptions>): void;
  resize(): void;
  destroy(): void;
}

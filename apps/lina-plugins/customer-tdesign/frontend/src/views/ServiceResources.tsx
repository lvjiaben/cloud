// Shared resource rows for the overview and merchant-defined product tabs.
import { useState } from "react";
import { Alert, Button, Form, Textarea } from "tdesign-react";
import { EditIcon } from "tdesign-icons-react";
import { Link } from "react-router-dom";
import Dialog from "#/adapter/Dialog";
import NavigationIcon from "#/components/NavigationIcon";
import { expiryDisplay, operatingSystemIcon } from "./service-display";
import {
  BusinessTable,
  ErrorNote,
  money,
  stateLabel,
  useAction,
} from "#/components/business";
import {
  listDirectoryServices,
  saveCustomerServiceNote,
  type DirectoryService,
} from "#/api/idc/product-directories";
import {
  listNativeServerFacts,
  type NativeServerFact,
} from "#/api/idc/native-servers";

function canManage(row: DirectoryService) {
  return (
    row.entitlementOwnerId === "idc-resource" &&
    row.statusCode !== "pending_activation" &&
    (!row.managementPreparation || row.managementPreparation === "ready")
  );
}
export function serviceTarget(row: DirectoryService) {
  return canManage(row)
    ? `/account/native-servers/by-entitlement/${row.entitlementNo}`
    : `/account/orders/${row.orderNo}`;
}
export async function loadDirectoryResources(
  directory: string,
  page: number,
  size: number,
  keyword: string,
  nativeEnabled: boolean,
) {
  const result = await listDirectoryServices(directory, page, size, keyword);
  const ids = result.items
    .filter((v) => v.entitlementOwnerId === "idc-resource")
    .map((v) => v.entitlementNo);
  let facts: NativeServerFact[] = [],
    factsError = "";
  if (nativeEnabled && ids.length) {
    try {
      facts = (await listNativeServerFacts(ids)).items;
    } catch (error) {
      factsError =
        error instanceof Error
          ? error.message
          : "实例信息暂时无法读取，请刷新重试";
    }
  }
  return { ...result, facts, factsError };
}
export function ServiceNoteDialog({
  item,
  onClose,
  onSaved,
}: {
  item: DirectoryService;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [remark, setRemark] = useState(item.customerRemark || "");
  const action = useAction();
  return (
    <Dialog
      visible
      header="编辑备注"
      onClose={() => !action.busy && onClose()}
      closeOnOverlayClick={false}
      footer={
        <>
          <Button
            tag="button"
            variant="outline"
            disabled={action.busy}
            onClick={onClose}
          >
            取消
          </Button>
          <Button
            tag="button"
            loading={action.busy}
            onClick={() =>
              void action.run(async () => {
                await saveCustomerServiceNote(
                  item.entitlementNo,
                  remark,
                  item.customerRemarkRevision || 1,
                );
                onSaved();
                onClose();
              })
            }
          >
            保存备注
          </Button>
        </>
      }
    >
      <p>{item.productNameZhCn || item.productNameEnUs}</p>
      <Form labelAlign="top">
        <Form.FormItem label="服务备注">
          <Textarea
            value={remark}
            onChange={setRemark}
            maxlength={1000}
            autosize={{ minRows: 3, maxRows: 6 }}
            placeholder="例如：生产网站、测试环境。请勿填写密码或密钥。"
          />
        </Form.FormItem>
      </Form>
      <ErrorNote error={action.error} />
    </Dialog>
  );
}
export function ResourceTable({
  items,
  facts = [],
  editable = false,
  total,
  page,
  size,
  onPage,
  onReload,
}: {
  items: DirectoryService[];
  facts?: NativeServerFact[];
  editable?: boolean;
  total?: number;
  page?: number;
  size?: number;
  onPage?: (page: number, size: number) => void;
  onReload?: () => void;
}) {
  const [note, setNote] = useState<DirectoryService>();
  const byEntitlement = new Map(facts.map((v) => [v.entitlementNo, v]));
  const nativeOnly =
    items.length > 0 &&
    items.every((item) => item.entitlementOwnerId === "idc-resource");
  return (
    <>
      <BusinessTable
        rowKey="entitlementNo"
        rows={items}
        total={total}
        page={page}
        size={size}
        onPage={onPage}
        columns={[
          {
            colKey: "productNameZhCn",
            title: nativeOnly ? "产品 / 主机名" : "产品",
            minWidth: 230,
            cell: ({ row }) => (
              <div className="resource-identity">
                <Link to={serviceTarget(row)}>
                  <strong>{row.productNameZhCn || row.productNameEnUs}</strong>
                </Link>
                <small className="subline">
                  {byEntitlement.get(row.entitlementNo)?.hostname ||
                    row.entitlementNo}
                </small>
              </div>
            ),
          },
          {
            colKey: "statusCode",
            title: "状态",
            width: 110,
            cell: ({ row }) => (
              <div>
                <span
                  className={`service-lifecycle service-lifecycle-${row.statusCode}`}
                >
                  <span className="service-status-dot" aria-hidden="true" />
                  {row.statusCode === "active"
                    ? "已激活"
                    : stateLabel(row.statusCode)}
                </span>
                {byEntitlement.get(row.entitlementNo)?.stale && (
                  <small className="subline">上次确认</small>
                )}
                {row.statusCode === "pending_activation" &&
                  row.requiresManualProcessing && (
                    <small className="subline">待人工处理</small>
                  )}
              </div>
            ),
          },
          ...(nativeOnly
            ? [
                {
                  colKey: "addresses",
                  title: "IP 地址",
                  minWidth: 150,
                  cell: ({ row }: { row: DirectoryService }) => {
                    const fact = byEntitlement.get(row.entitlementNo);
                    return fact?.addresses.length ? (
                      <span className="resource-addresses">
                        {fact.addresses.join(" / ")}
                      </span>
                    ) : (
                      <span className="muted">
                        {fact?.addressesConfirmed ? "暂未分配" : "尚未确认"}
                      </span>
                    );
                  },
                },
              ]
            : []),
          {
            colKey: "expiresAt",
            title: "到期时间",
            width: 180,
            cell: ({ row }) => {
              const expiry = expiryDisplay(row.expiresAt, row.statusCode);
              return (
                <div>
                  {expiry.date}
                  {expiry.remaining && (
                    <small className={`subline service-expiry-${expiry.tone}`}>
                      {expiry.remaining}
                    </small>
                  )}
                </div>
              );
            },
          },
          {
            colKey: "currentPriceAmountMinor",
            title: "费用",
            minWidth: 155,
            cell: ({ row }) => (
              <div>
                {row.currentPriceAmountMinor != null
                  ? money(row.currentPriceAmountMinor, row.currencyCode)
                  : "—"}
                <small className="subline">
                  {row.currentPriceAmountMinor != null
                    ? `当前价格${row.periodCount && row.chargeCycles ? ` · ${row.periodCount * row.chargeCycles}${({ hour: "小时", day: "天", week: "周", month: "个月", year: "年" } as Record<string, string>)[row.periodUnitCode || ""] || "个周期"}` : ""}`
                    : "以账单为准"}
                </small>
              </div>
            ),
          },
          ...(editable
            ? [
                {
                  colKey: "osName",
                  title: "系统",
                  minWidth: 135,
                  cell: ({ row }: { row: DirectoryService }) => {
                    const name = byEntitlement.get(row.entitlementNo)?.osName;
                    return name ? (
                      <span className="service-system">
                        <NavigationIcon
                          icon={operatingSystemIcon(name)}
                          size={18}
                        />
                        <span>{name}</span>
                      </span>
                    ) : (
                      "—"
                    );
                  },
                },
                {
                  colKey: "customerRemark",
                  title: "备注",
                  minWidth: 150,
                  cell: ({ row }: { row: DirectoryService }) => (
                    <div className="resource-note">
                      <span>{row.customerRemark || "—"}</span>
                      <Button
                        tag="button"
                        variant="text"
                        shape="square"
                        aria-label={`编辑备注 ${row.productNameZhCn || row.productNameEnUs}`}
                        icon={<EditIcon />}
                        onClick={() => setNote(row)}
                      />
                    </div>
                  ),
                },
                {
                  colKey: "actions",
                  title: "操作",
                  fixed: "right" as const,
                  width: 130,
                  cell: ({ row }: { row: DirectoryService }) => (
                    <Link className="resource-manage" to={serviceTarget(row)}>
                      {row.statusCode === "pending_activation" ||
                      (row.managementPreparation &&
                        row.managementPreparation !== "ready")
                        ? "查看开通进度"
                        : row.entitlementOwnerId === "idc-resource"
                          ? "管理"
                          : "查看详情"}
                    </Link>
                  ),
                },
              ]
            : []),
        ]}
      />
      {note && (
        <ServiceNoteDialog
          item={note}
          onClose={() => setNote(undefined)}
          onSaved={() => onReload?.()}
        />
      )}
    </>
  );
}
export function ResourceFactsWarning({ message }: { message?: string }) {
  return message ? (
    <Alert
      theme="warning"
      message="部分实例信息暂时无法读取，已有服务和费用仍可查看，请刷新重试。"
    />
  ) : null;
}

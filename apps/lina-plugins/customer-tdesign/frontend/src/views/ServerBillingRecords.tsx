import { useState } from "react";
import { Link } from "react-router-dom";
import { Button, Card, Select } from "tdesign-react";
import * as api from "#/api/idc/server-billing";
import type { NativeAccessory } from "#/api/idc/native-servers";
import {
  BusinessTable,
  CursorPager,
  FetchState,
  Status,
  date,
  money,
  useCursor,
  useLoad,
} from "#/components/business";
const commercialStatus: Record<string, string> = {
  awaiting_financial_settlement: "等待付款",
  financially_settled: "已结清，等待处理",
  ready: "等待执行",
  executing: "正在执行",
  succeeded: "已完成",
  failed: "未完成",
  canceled: "已取消",
  manual_review: "需要核对",
};
export function ServerCommercialRecords({
  serverNo,
  names,
}: {
  serverNo: string;
  names: Record<string, string>;
}) {
  const cursor = useCursor();
  const data = useLoad(
    () => api.listServerCommercialRecords(serverNo, cursor.cursor),
    [serverNo, cursor.cursor],
  );
  return (
    <Card title="增购与配置变更记录">
      <div className="actions">
        <Button tag="button" variant="outline" onClick={data.reload}>
          刷新办理记录
        </Button>
      </div>
      <FetchState state={data}>
        <BusinessTable
          rowKey="operationNo"
          rows={data.data?.items || []}
          columns={[
            {
              colKey: "action",
              title: "事项",
              cell: ({ row }) => names[row.action] || "服务变更",
            },
            {
              colKey: "status",
              title: "办理状态",
              cell: ({ row }) => (
                <>
                  <span>{commercialStatus[row.status] || "状态待确认"}</span>
                  {row.status === "manual_review" && (
                    <p className="muted">请提交工单核对，勿重复办理。</p>
                  )}
                </>
              ),
            },
            {
              colKey: "amount",
              title: "本次最终金额",
              cell: ({ row }) => money(row.totalAmountMinor, row.currencyCode),
            },
            {
              colKey: "createdAt",
              title: "申请时间",
              cell: ({ row }) => date(row.createdAt),
            },
            {
              colKey: "updatedAt",
              title: "更新时间",
              cell: ({ row }) => date(row.updatedAt),
            },
            {
              colKey: "links",
              title: "费用",
              cell: ({ row }) => (
                <div className="actions">
                  {row.invoiceNo && (
                    <Link
                      to={`/account/billing/invoices/${encodeURIComponent(row.invoiceNo)}`}
                    >
                      查看账单
                    </Link>
                  )}
                  {row.orderNo && (
                    <Link
                      to={`/account/orders/${encodeURIComponent(row.orderNo)}`}
                    >
                      查看订单
                    </Link>
                  )}
                </div>
              ),
            },
          ]}
        />
        <CursorPager
          history={cursor.history}
          next={data.data?.nextCursor}
          onPrevious={cursor.previous}
          onNext={() => cursor.next(data.data!.nextCursor)}
        />
      </FetchState>
    </Card>
  );
}
export function ServerAccessoryBills({
  serverNo,
  objects,
}: {
  serverNo: string;
  objects: NativeAccessory[];
}) {
  const [kind, setKind] = useState("");
  const cursor = useCursor(kind);
  const data = useLoad(
    () => api.listServerAccessoryBills(serverNo, cursor.cursor, kind),
    [serverNo, cursor.cursor, kind],
  );
  return (
    <Card title="快照与备份账期">
      <div className="table-toolbar">
        <Select
          value={kind}
          placeholder="查看计费记录"
          options={[
            { value: "", label: "快照与备份" },
            { value: "snapshot", label: "快照" },
            { value: "backup", label: "备份" },
          ]}
          onChange={(v) => setKind(String(v))}
        />
        <Button tag="button" variant="outline" onClick={data.reload}>
          刷新账期
        </Button>
      </div>
      <FetchState state={data}>
        <BusinessTable
          rowKey="accessoryEntitlementNo"
          rows={data.data?.items || []}
          columns={[
            {
              colKey: "name",
              title: "名称",
              cell: ({ row }) => (
                <>
                  <span>
                    {objects.find((o) => o.objectNo === row.objectNo)?.name ||
                      row.objectNo}
                  </span>
                  <p className="muted">
                    {row.kind === "snapshot"
                      ? "快照"
                      : row.kind === "backup"
                        ? "备份"
                        : "资源"}
                  </p>
                </>
              ),
            },
            {
              colKey: "mode",
              title: "计费方式",
              cell: ({ row }) =>
                row.billingMode === "free"
                  ? "免费"
                  : row.billingMode === "paid"
                    ? `${row.periodCount} ${({ day: "天", month: "个月", year: "年" } as Record<string, string>)[row.periodUnit] || "（周期待确认）"}`
                    : "尚未确认",
            },
            {
              colKey: "status",
              title: "使用状态",
              cell: ({ row }) =>
                ({
                  pending_binding: "等待创建结果",
                  active: "可使用",
                  payment_pending: "待续费付款",
                  manual_review: "需要核对",
                  deleted: "已删除",
                })[row.status] || "尚未确认",
            },
            {
              colKey: "period",
              title: "当前周期到期",
              cell: ({ row }) =>
                row.billingMode === "free"
                  ? "免费对象"
                  : date(row.currentPeriodEndAt),
            },
            {
              colKey: "renew",
              title: "自动续费",
              cell: ({ row }) =>
                row.billingMode === "free"
                  ? "无需续费"
                  : row.autoRenewEnabled
                    ? "已开启"
                    : "已关闭",
            },
            {
              colKey: "invoice",
              title: "待付续费账单",
              cell: ({ row }) =>
                row.pendingInvoiceNo ? (
                  <>
                    <Link
                      to={`/account/billing/invoices/${encodeURIComponent(row.pendingInvoiceNo)}`}
                    >
                      查看并支付
                    </Link>
                    <p className="muted">
                      账单总额：
                      {row.pendingAmountMinor === null
                        ? "尚未确认"
                        : money(row.pendingAmountMinor, row.currencyCode)}
                    </p>
                    <Status value={row.pendingInvoiceStatus} />
                  </>
                ) : (
                  "暂无待付账单"
                ),
            },
          ]}
        />
        <CursorPager
          history={cursor.history}
          next={data.data?.nextCursor}
          onPrevious={cursor.previous}
          onNext={() => cursor.next(data.data!.nextCursor)}
        />
      </FetchState>
      <p className="muted">
        续费未付款时保留数据，可查看和删除；付款后仍需确认资源状态，恢复操作以实际可用为准。删除停止后续续费，已开始的周期不退款。
      </p>
    </Card>
  );
}

import { useState } from "react";
import { Alert, Button, Card, Select, Steps } from "tdesign-react";
import { Link, useNavigate, useParams } from "react-router-dom";
import * as orders from "#/api/idc/customer-commerce";
import { actionKey } from "#/api/request";
import {
  BusinessTable,
  Confirm,
  CursorPager,
  DetailList,
  ErrorNote,
  FetchState,
  Page,
  Status,
  date,
  money,
  useAction,
  useCursor,
  useLoad,
} from "#/components/business";
export default function Orders() {
  const [status, setStatus] = useState(""),
    cursor = useCursor(status);
  const data = useLoad(
    () =>
      orders.listOrders({
        cursor: cursor.cursor,
        statusCode: status,
        limit: 20,
      }),
    [cursor.cursor, status],
    "Orders:Orders:data",
  );
  return (
    <Page title="我的订单" description="查询购买记录、付款状态和服务开通进度。">
      <Card>
        <div className="table-toolbar">
          <Select
            aria-label="订单状态"
            value={status}
            options={[
              { label: "全部状态", value: "" },
              { label: "待付款", value: "awaiting_payment" },
              { label: "已付款", value: "paid" },
              { label: "已取消", value: "cancelled" },
            ]}
            onChange={(v) => setStatus(String(v))}
            style={{ width: 180 }}
          />
          <Button tag="button" variant="outline" onClick={data.reload}>
            刷新
          </Button>
        </div>
        <FetchState state={data}>
          <BusinessTable
            rowKey="orderNo"
            rows={data.data?.orders || []}
            columns={[
              {
                colKey: "orderNo",
                title: "订单编号",
                cell: ({ row }) => (
                  <Link to={`/account/orders/${row.orderNo}`}>
                    {row.orderNo}
                  </Link>
                ),
              },
              {
                colKey: "createdAt",
                title: "下单时间",
                cell: ({ row }) => date(row.createdAt),
              },
              { colKey: "itemCount", title: "商品数", width: 90 },
              {
                colKey: "totalAmountMinor",
                title: "订单金额",
                cell: ({ row }) =>
                  money(row.totalAmountMinor, row.currencyCode),
              },
              {
                colKey: "statusCode",
                title: "状态",
                cell: ({ row }) => <Status value={row.statusCode} />,
              },
              {
                colKey: "actions",
                title: "操作",
                fixed: "right",
                width: 100,
                cell: ({ row }) => (
                  <Link to={`/account/orders/${row.orderNo}`}>
                    {row.canPreparePayment ? "去付款" : "查看详情"}
                  </Link>
                ),
              },
            ]}
          />
          <CursorPager
            next={data.data?.nextCursor}
            history={cursor.history}
            onNext={() => cursor.next(data.data!.nextCursor)}
            onPrevious={cursor.previous}
          />
        </FetchState>
      </Card>
    </Page>
  );
}
export function OrderDetail() {
  const { orderNo = "" } = useParams(),
    navigate = useNavigate();
  const data = useLoad(
      () => orders.getOrder(orderNo),
      [orderNo],
      "Orders:OrderDetail:data",
    ),
    action = useAction();
  const [cancel, setCancel] = useState(false),
    [preparation, setPreparation] = useState<orders.PaymentPreparation>();
  const order = data.data?.order;
  return (
    <Page
      title="订单详情"
      actions={<Link to="/account/orders">返回订单列表</Link>}
    >
      <FetchState state={data}>
        {order && (
          <>
            <Card>
              <div className="service-title">
                <h2>{order.summary.orderNo}</h2>
                <Status value={order.summary.statusCode} />
              </div>
              <DetailList
                items={[
                  [
                    "订单金额",
                    money(
                      order.summary.totalAmountMinor,
                      order.summary.currencyCode,
                    ),
                  ],
                  ["下单时间", date(order.summary.createdAt)],
                  ["付款截止", date(order.summary.paymentDueAt)],
                ]}
              />
              <div className="actions">
                {order.summary.canPreparePayment && (
                  <Button
                    tag="button"
                    loading={action.busy}
                    onClick={() =>
                      void action.run(async () => {
                        const existing =
                          preparation?.contextNo ||
                          order.paymentPreparation?.contextNo;
                        const result = existing
                          ? await orders.resumePaymentPreparation(existing)
                          : await orders.preparePayment(orderNo);
                        setPreparation(result.preparation);
                        if (result.preparation.ready)
                          navigate(
                            `/account/billing/invoices/${result.preparation.invoiceNo}`,
                          );
                      })
                    }
                  >
                    去付款
                  </Button>
                )}
                {order.summary.canCancel && (
                  <Button
                    tag="button"
                    variant="outline"
                    onClick={() => {
                      action.clear();
                      setCancel(true);
                    }}
                  >
                    取消订单
                  </Button>
                )}
                <Button tag="button" variant="outline" onClick={data.reload}>
                  刷新进度
                </Button>
                {order.invoice && (
                  <Link
                    to={`/account/refunds?orderNo=${encodeURIComponent(orderNo)}`}
                  >
                    申请退款
                  </Link>
                )}
              </div>
              <ErrorNote error={action.error} />
              {preparation && !preparation.ready && (
                <Alert
                  theme="warning"
                  title="付款前还有一项需要完成"
                  message={
                    <span>
                      请先完成账户认证，再继续付款。
                      <Link to="/account/verification"> 账户认证</Link>
                    </span>
                  }
                />
              )}
            </Card>
            <Card title="开通进度">
              <BusinessTable
                rowKey="productNameZhCN"
                rows={order.deliveries}
                columns={[
                  {
                    colKey: "productNameZhCN",
                    title: "产品",
                    cell: ({ row }) =>
                      row.productNameZhCN || row.productNameEnUS,
                  },
                  { colKey: "quantity", title: "数量", width: 90 },
                  {
                    colKey: "statusCode",
                    title: "办理状态",
                    cell: ({ row }) => <Status value={row.statusCode} />,
                  },
                  {
                    colKey: "updatedAt",
                    title: "更新时间",
                    cell: ({ row }) => date(row.updatedAt),
                  },
                ]}
              />
            </Card>
            <Card title="已开通服务">
              <BusinessTable
                rowKey="entitlementNo"
                rows={order.serviceEntitlements}
                columns={[
                  {
                    colKey: "productNameZhCN",
                    title: "服务名称",
                    cell: ({ row }) =>
                      row.productNameZhCN || row.productNameEnUS,
                  },
                  {
                    colKey: "statusCode",
                    title: "状态",
                    cell: ({ row }) => <Status value={row.statusCode} />,
                  },
                  {
                    colKey: "expiresAt",
                    title: "到期时间",
                    cell: ({ row }) => date(row.expiresAt),
                  },
                  {
                    colKey: "actions",
                    title: "操作",
                    fixed: "right",
                    width: 150,
                    cell: ({ row }) =>
                      row.entitlementOwnerId === "idc-resource" &&
                      (!row.managementPreparation ||
                        row.managementPreparation === "ready") ? (
                        <Link
                          to={`/account/native-servers/by-entitlement/${row.entitlementNo}`}
                        >
                          管理服务
                        </Link>
                      ) : (
                        <Link to="/account/product-directories">查看服务</Link>
                      ),
                  },
                ]}
              />
            </Card>
          </>
        )}
      </FetchState>
      {cancel && (
        <Confirm
          title="取消订单"
          busy={action.busy}
          error={action.error}
          onClose={() => setCancel(false)}
          onConfirm={() =>
            void action.run(async () => {
              await orders.cancelOrder(orderNo, actionKey(`cancel-${orderNo}`));
              setCancel(false);
              data.reload();
            })
          }
        >
          <p>确认取消这笔订单？取消后如需购买，请重新下单。</p>
        </Confirm>
      )}
    </Page>
  );
}

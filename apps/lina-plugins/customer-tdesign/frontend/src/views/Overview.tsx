import {
  loadDirectoryResources,
  ResourceTable,
  ResourceFactsWarning,
} from "./ServiceResources";
import { requestClient } from "#/api/request";
import { maskContact } from "./LoginContacts";
import { useContext } from "react";
import { useFeature } from "#/runtime/features";
import { NavigationContext, productNavigation } from "#/runtime/navigation";
import { Card, Button } from "tdesign-react";
import {
  ArrowRightIcon,
  CloudIcon,
  WalletIcon,
  ServiceIcon,
  TimeIcon,
  SecuredIcon,
} from "tdesign-icons-react";
import { Link } from "react-router-dom";
import {
  getFinancialOverview,
  listFinancialRenewalEntitlements,
} from "#/api/idc/customer-finance";
import { listOrders } from "#/api/idc/customer-commerce";
import { listCustomerTickets } from "#/api/idc/support";
import {
  Page,
  useLoad,
  FetchState,
  money,
  date,
  Status,
  BusinessTable,
} from "#/components/business";
export default function Overview() {
  const compliance = useFeature("idc-compliance");
  const nativeEnabled = useFeature("idc-resource");
  const resources = useLoad(
    () => loadDirectoryResources("all", 1, 5, "", nativeEnabled),
    [nativeEnabled],
    "Overview:resources",
  );
  const products = productNavigation(useContext(NavigationContext));
  const user = useLoad(
    () =>
      requestClient.get<{
        id: number;
        username: string;
        nickname: string;
        email: string;
        phone: string;
      }>("/user/profile"),
    [],
    "Overview:account",
  );
  const finance = useLoad(
      getFinancialOverview,
      [],
      "Overview:Overview:finance",
    ),
    orders = useLoad(
      () => listOrders({ limit: 5 }),
      [],
      "Overview:Overview:orders",
    ),
    tickets = useLoad(
      () => listCustomerTickets({ page: 1, limit: 5 }),
      [],
      "Overview:Overview:tickets",
    ),
    renewals = useLoad(
      () => listFinancialRenewalEntitlements({ limit: 5 }),
      [],
      "Overview:Overview:renewals",
    );
  return (
    <Page
      title="账户概览"
      description="管理云资源，查看费用与服务进度。"
      actions={
        <Link to="/account/products/catalog">
          <Button tag="button" icon={<CloudIcon />}>
            购买服务
          </Button>
        </Link>
      }
    >
      <div className="workspace-summary">
        <section className="workspace-balance">
          <div className="section-heading">
            <span>
              <WalletIcon />
              账户余额
            </span>
            <Link to="/account/billing/transactions">
              收支明细 <ArrowRightIcon />
            </Link>
          </div>
          <FetchState state={finance}>
            <div className="workspace-balances">
              {finance.data?.overview.balances.length ? (
                finance.data.overview.balances.map((b) => (
                  <div key={b.currencyCode + b.accountTypeCode}>
                    <strong>
                      {money(b.availableBalanceMinor, b.currencyCode)}
                    </strong>
                    <small>
                      {b.accountTypeCode === "customer_balance"
                        ? "可用余额"
                        : "赠送余额"}
                    </small>
                  </div>
                ))
              ) : (
                <p className="muted">暂无账户余额</p>
              )}
            </div>
            {
              <Link to="/account/billing?recharge=1">
                <Button tag="button" variant="outline" size="small">
                  充值
                </Button>
              </Link>
            }
          </FetchState>
        </section>
        <section className="workspace-pending">
          <div className="section-heading">
            <span>
              <TimeIcon />
              待付款账单
            </span>
          </div>
          <FetchState state={finance}>
            <Link className="summary-number" to="/account/billing/invoices">
              {finance.data?.overview.openInvoiceCount ?? "—"}
              <small> 笔</small>
              <ArrowRightIcon />
            </Link>
            {finance.data?.overview.openReceivables.map((v) => (
              <span className="amount-note" key={v.currencyCode}>
                {money(v.outstandingMinor, v.currencyCode)}
              </span>
            ))}
          </FetchState>
        </section>
        <section className="workspace-support">
          <div className="section-heading">
            <span>
              <ServiceIcon />
              服务支持
            </span>
            <Link to="/account/support">
              我的工单 <ArrowRightIcon />
            </Link>
          </div>
          <p>遇到使用问题，我们会通过工单协助您处理。</p>
          <Link to="/account/support/new">
            <Button tag="button" variant="outline" size="small">
              提交工单
            </Button>
          </Link>
        </section>
      </div>
      <div className="workspace-columns">
        <div className="workspace-primary">
          <Card title="我的资源" className="resource-panel">
            {products.length ? (
              <div className="resource-grid">
                {products.map((item) => (
                  <Link
                    className="resource-entry"
                    to={item.path}
                    key={item.path}
                  >
                    <span className="resource-icon">
                      <CloudIcon size={24} />
                    </span>
                    <span>
                      <strong>{item.title}</strong>
                      <small>已购资源</small>
                    </span>
                    <b>{item.serviceCount ?? "—"}</b>
                    <ArrowRightIcon />
                  </Link>
                ))}
              </div>
            ) : (
              <div className="workspace-empty">
                <CloudIcon size={28} />
                <span>暂无已开放的产品服务菜单</span>
              </div>
            )}
          </Card>
          <Card title="资源列表">
            <FetchState state={resources}>
              <ResourceFactsWarning message={resources.data?.factsError} />
              <ResourceTable
                items={resources.data?.items || []}
                facts={resources.data?.facts}
              />
            </FetchState>
          </Card>
          <Card
            title="最近订单"
            actions={
              <Link to="/account/orders">
                查看订单 <ArrowRightIcon />
              </Link>
            }
          >
            <FetchState state={orders}>
              <BusinessTable
                rowKey="orderNo"
                rows={orders.data?.orders || []}
                columns={[
                  {
                    colKey: "orderNo",
                    title: "订单号",
                    cell: ({ row }) => (
                      <Link to={`/account/orders/${row.orderNo}`}>
                        {row.orderNo}
                      </Link>
                    ),
                  },
                  {
                    colKey: "totalAmountMinor",
                    title: "金额",
                    cell: ({ row }) =>
                      money(row.totalAmountMinor, row.currencyCode),
                  },
                  {
                    colKey: "statusCode",
                    title: "状态",
                    cell: ({ row }) => <Status value={row.statusCode} />,
                  },
                  {
                    colKey: "createdAt",
                    title: "创建时间",
                    cell: ({ row }) => date(row.createdAt),
                  },
                ]}
              />
            </FetchState>
          </Card>
          <Card
            title="续费管理"
            actions={
              <Link to="/account/billing/renewals">
                查看服务 <ArrowRightIcon />
              </Link>
            }
          >
            <FetchState state={renewals}>
              {renewals.data?.entitlements.length ? (
                renewals.data.entitlements.map((v) => (
                  <Link
                    className="summary-row"
                    to="/account/billing/renewals"
                    key={v.entitlementNo}
                  >
                    <span>{v.productNameZhCN || v.productNameEnUS}</span>
                    <small>{date(v.entitlementExpiresAt)}</small>
                    <span>续费</span>
                  </Link>
                ))
              ) : (
                <div className="workspace-empty">
                  <TimeIcon size={24} />
                  <span>暂无需要续费的服务</span>
                </div>
              )}
            </FetchState>
          </Card>
        </div>
        <aside className="workspace-secondary">
          <Card
            title="我的账户"
            actions={<Link to="/account/profile">管理</Link>}
          >
            <FetchState state={user}>
              <div className="overview-account">
                <strong>{user.data?.nickname || user.data?.username}</strong>
                <span>{user.data?.username}</span>
              </div>
              <div className="overview-account-row">
                <span>账号 ID</span>
                <strong>{user.data?.id ?? "—"}</strong>
              </div>
              <div className="overview-account-row">
                <span>登录手机</span>
                <span>
                  {maskContact(user.data?.phone, "phone") || "未绑定"}
                </span>
              </div>
              <div className="overview-account-row">
                <span>登录邮箱</span>
                <span>
                  {maskContact(user.data?.email, "email") || "未绑定"}
                </span>
              </div>
              <Link
                className="overview-account-security"
                to="/account/security"
              >
                <SecuredIcon />
                登录与安全
                <ArrowRightIcon />
              </Link>
            </FetchState>
          </Card>
          <Card title="常用操作">
            <div className="workspace-shortcuts">
              <Link to="/account/orders">
                <WalletIcon />
                订单管理
                <ArrowRightIcon />
              </Link>
              <Link to="/account/billing/renewals">
                <TimeIcon />
                服务续费
                <ArrowRightIcon />
              </Link>
              <Link to="/account/security">
                <SecuredIcon />
                账户安全
                <ArrowRightIcon />
              </Link>
              {compliance && (
                <Link to="/account/verification">
                  <SecuredIcon />
                  实名认证
                  <ArrowRightIcon />
                </Link>
              )}
            </div>
          </Card>
          <Card
            title="近期工单"
            actions={<Link to="/account/support">查看</Link>}
          >
            <FetchState state={tickets}>
              {tickets.data?.items.length ? (
                tickets.data.items.map((v) => (
                  <Link
                    className="ticket-summary"
                    to={`/account/support/${v.ticketNo}`}
                    key={v.ticketNo}
                  >
                    <strong>{v.subject}</strong>
                    <Status value={v.stage} />
                  </Link>
                ))
              ) : (
                <div className="workspace-empty">
                  <ServiceIcon size={24} />
                  <span>暂无工单</span>
                </div>
              )}
            </FetchState>
          </Card>
        </aside>
      </div>
    </Page>
  );
}

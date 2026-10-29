import Dialog from "#/adapter/Dialog";
import Tabs from "#/adapter/Tabs";
import NavigationIcon from "#/components/NavigationIcon";
import PluginServicePanel from "./PluginServicePanel";
import { ServicePagesContext } from "#/runtime/service-pages";
import { listDirectoryServices } from "#/api/idc/product-directories";
import { useContext, useState } from "react";
import { Button, Card, InputNumber } from "tdesign-react";
import { Link, Navigate, useNavigate, useParams } from "react-router-dom";
import { NavigationContext, productNavigation } from "#/runtime/navigation";
import * as renewals from "#/api/idc/customer-renewals";
import { actionKey, clearActionKey } from "#/api/request";
import {
  Page,
  useLoad,
  FetchState,
  date,
  money,
  useAction,
  ErrorNote,
  DetailList,
} from "#/components/business";
export function RenewDialog({
  entitlementNo,
  onClose,
}: {
  entitlementNo: string;
  onClose: () => void;
}) {
  const [cycles, setCycles] = useState(1),
    [quote, setQuote] = useState<renewals.ServiceRenewalQuote>();
  const action = useAction(),
    navigate = useNavigate();
  return (
    <Dialog
      visible
      header="续费服务"
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
                if (!quote) {
                  const key = `renew-quote-${entitlementNo}-${cycles}`;
                  setQuote(
                    (
                      await renewals.quoteServiceRenewal(
                        entitlementNo,
                        cycles,
                        actionKey(key),
                      )
                    ).quote,
                  );
                } else {
                  if (quote.expiresAt <= Date.now()) {
                    clearActionKey(`renew-quote-${entitlementNo}-${cycles}`);
                    setQuote(undefined);
                    throw new Error("报价已过期，请重新获取");
                  }
                  const result = await renewals.confirmServiceRenewal(
                    quote.quoteNo,
                    actionKey(`renew-confirm-${quote.quoteNo}`),
                  );
                  navigate(
                    `/account/billing/invoices/${result.operation.invoiceNo}`,
                  );
                  onClose();
                }
              })
            }
          >
            {quote ? "确认续费并付款" : "查看续费金额"}
          </Button>
        </>
      }
    >
      <p>选择续费周期，确认金额后再付款。</p>
      <label className="field">
        续费周期
        <InputNumber
          min={1}
          max={120}
          decimalPlaces={0}
          disabled={action.busy}
          value={cycles}
          onChange={(v) => {
            setCycles(Number(v) || 1);
            setQuote(undefined);
          }}
        />
      </label>
      {quote && (
        <DetailList
          items={[
            ["续费金额", money(quote.totalAmountMinor, quote.currencyCode)],
            ["续费后到期", date(quote.projectedExpiresAt)],
            ["报价有效期", date(quote.expiresAt)],
          ]}
        />
      )}
      <ErrorNote error={action.error} />
    </Dialog>
  );
}
export default function Services() {
  const { directoryNo } = useParams();
  const menus = productNavigation(useContext(NavigationContext));
  if (!directoryNo || !menus.some((item) => item.directoryNo === directoryNo)) {
    return menus[0] ? (
      <Navigate to={menus[0].path} replace />
    ) : (
      <Page title="产品与服务">
        <Card>
          <p className="muted">
            暂无已开放的产品服务菜单。如需帮助，请提交工单。
          </p>
          <Link to="/account/support/new">提交工单</Link>
        </Card>
      </Page>
    );
  }
  return <DirectoryServices key={directoryNo} directoryNo={directoryNo} />;
}
function DirectoryServices({ directoryNo }: { directoryNo: string }) {
  const menus = productNavigation(useContext(NavigationContext));
  const { pages, reload: reloadPages } = useContext(ServicePagesContext),
    navigate = useNavigate();
  const services = useLoad(
    () => listDirectoryServices(directoryNo, 1, 1),
    [directoryNo],
    "Services:providers",
  );
  return (
    <Page
      title={
        menus.find((item) => item.directoryNo === directoryNo)?.title ||
        "产品与服务"
      }
      actions={
        <Link to="/account/products/catalog">
          <Button>购买服务</Button>
        </Link>
      }
    >
      <div className="service-catalog">
        <Tabs
          value={directoryNo}
          label="服务分类"
          items={menus.map((item) => ({
            value: item.directoryNo!,
            label: (
              <span className="service-category-label">
                <NavigationIcon icon={item.icon || "lucide:folder"} size={16} />
                <span>{item.title}</span>
              </span>
            ),
          }))}
          onChange={(value) =>
            navigate(
              `/account/product-directories/${encodeURIComponent(value)}`,
            )
          }
        />
        <FetchState state={services}>
          {services.data?.providers?.map((provider) => {
            const binding = pages.find(
              (page) => page.pluginId === provider.pluginId,
            );
            return binding ? (
              <PluginServicePanel
                key={`${directoryNo}:${provider.pluginId}`}
                binding={binding}
                directoryNo={directoryNo}
              />
            ) : (
              <Card key={provider.pluginId}>
                <p>商品所属插件暂未提供可用的已购页面，请联系商家。</p>
                <Button
                  variant="outline"
                  onClick={() => {
                    reloadPages();
                    services.reload();
                  }}
                >
                  重试
                </Button>
              </Card>
            );
          })}
          {!services.data?.providers?.length && (
            <div className="workspace-empty">暂无已购服务</div>
          )}
        </FetchState>
      </div>
    </Page>
  );
}
export function ServerList() {
  return <Navigate to="/account/product-directories" replace />;
}

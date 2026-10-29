import {
  Component,
  lazy,
  Suspense,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { createRoot } from "react-dom/client";
import {
  BrowserRouter,
  Navigate,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import { Button, ConfigProvider, Loading } from "tdesign-react";
import zhCN from "tdesign-react/es/locale/zh_CN";
import "tdesign-react/es/style/index.css";
import "./style.css";
import CustomerLayout from "./layouts/CustomerLayout";
import { getSession, requestClient } from "./api/request";
import { EmptyPage, FetchState, useLoad } from "./components/business";
const Auth = lazy(() =>
  import("./views/Auth").then((m) => ({ default: m.default })),
);
const Overview = lazy(() =>
  import("./views/Overview").then((m) => ({ default: m.default })),
);
const Services = lazy(() =>
  import("./views/Services").then((m) => ({ default: m.default })),
);
const ServerDetail = lazy(() =>
  import("./views/ServerManagement").then((m) => ({ default: m.default })),
);
const ServerList = lazy(() =>
  import("./views/Services").then((m) => ({ default: m.ServerList })),
);
const Orders = lazy(() =>
  import("./views/Orders").then((m) => ({ default: m.default })),
);
const OrderDetail = lazy(() =>
  import("./views/Orders").then((m) => ({ default: m.OrderDetail })),
);
const Finance = lazy(() =>
  import("./views/Finance").then((m) => ({ default: m.default })),
);
const InvoiceDetail = lazy(() =>
  import("./views/Finance").then((m) => ({ default: m.InvoiceDetail })),
);
const PaymentReturn = lazy(() =>
  import("./views/Finance").then((m) => ({ default: m.PaymentReturn })),
);
const Renewals = lazy(() =>
  import("./views/Finance").then((m) => ({ default: m.Renewals })),
);
const Support = lazy(() =>
  import("./views/Support").then((m) => ({ default: m.default })),
);
const NewTicket = lazy(() =>
  import("./views/Support").then((m) => ({ default: m.NewTicket })),
);
const TicketDetail = lazy(() =>
  import("./views/Support").then((m) => ({ default: m.TicketDetail })),
);
const Website = lazy(() => import("./views/Website"));
const WebsiteSession = lazy(() =>
  import("./views/Website").then((m) => ({ default: m.WebsiteSession })),
);
const Profile = lazy(() =>
  import("./views/Account").then((m) => ({ default: m.default })),
);
const Addresses = lazy(() =>
  import("./views/Account").then((m) => ({ default: m.Addresses })),
);
const Referrals = lazy(() =>
  import("./views/Account").then((m) => ({ default: m.Referrals })),
);
const SSHKeys = lazy(() =>
  import("./views/Account").then((m) => ({ default: m.SSHKeys })),
);
const InvoiceProfiles = lazy(() =>
  import("./views/Compliance").then((m) => ({ default: m.InvoiceProfiles })),
);
const TaxInvoices = lazy(() =>
  import("./views/Compliance").then((m) => ({ default: m.TaxInvoices })),
);
const Verification = lazy(() =>
  import("./views/Compliance").then((m) => ({ default: m.Verification })),
);
const Credit = lazy(() =>
  import("./views/AdditionalFinance").then((m) => ({ default: m.Credit })),
);
const Refunds = lazy(() =>
  import("./views/AdditionalFinance").then((m) => ({ default: m.Refunds })),
);
const Contacts = lazy(() => import("./views/Contacts"));
const Security = lazy(() => import("./views/Security"));
const Privacy = lazy(() => import("./views/Privacy"));
class ErrorBoundary extends Component<
  { children: ReactNode },
  { error: boolean }
> {
  state = { error: false };
  static getDerivedStateFromError() {
    return { error: true };
  }
  render() {
    return this.state.error ? (
      <div className="page">
        <h1>页面暂时无法打开</h1>
        <p>请刷新后重试，您的业务记录不会受到影响。</p>
        <Button tag="button" onClick={() => location.reload()}>
          刷新页面
        </Button>
      </div>
    ) : (
      this.props.children
    );
  }
}
function CustomerAccess() {
  const gate = useLoad(() =>
    requestClient.get<{ required: boolean }>(
      "/x/idc-core/api/v1/account/initial-password/status",
    ),
  );
  return (
    <FetchState state={gate}>
      {gate.data?.required ? (
        <Navigate to="/auth/initial-password" replace />
      ) : (
        <CustomerLayout />
      )}
    </FetchState>
  );
}
function Protected() {
  const [session, setSession] = useState(getSession),
    location = useLocation();
  useEffect(() => {
    const listener = () => setSession(getSession());
    window.addEventListener("customer-session", listener);
    return () => window.removeEventListener("customer-session", listener);
  }, []);
  return session ? (
    <CustomerAccess />
  ) : (
    <Navigate
      to={`/auth/login?redirect=${encodeURIComponent(location.pathname + location.search)}`}
      replace
    />
  );
}
function App() {
  return (
    <ConfigProvider globalConfig={zhCN}>
      <ErrorBoundary>
        <BrowserRouter future={{ v7_startTransition: true }}>
          <Suspense
            fallback={
              <div className="loading">
                <Loading text="正在打开页面" />
              </div>
            }
          >
            <Routes>
              <Route
                path="/auth/website-session"
                element={<WebsiteSession />}
              />
              <Route path="/auth/:mode" element={<Auth />} />
              <Route element={<Protected />}>
                <Route path="/account" element={<Overview />} />
                <Route
                  path="/account/product-directories/:directoryNo?"
                  element={<Services />}
                />
                <Route path="/account/servers" element={<ServerList />} />
                <Route
                  path="/account/native-servers/:serverNo"
                  element={<ServerDetail />}
                />
                <Route
                  path="/account/native-servers/by-entitlement/:entitlementNo"
                  element={<ServerDetail />}
                />
                <Route path="/account/orders" element={<Orders />} />
                <Route
                  path="/account/orders/:orderNo"
                  element={<OrderDetail />}
                />
                <Route
                  path="/account/products/catalog"
                  element={<Website kind="catalog" />}
                />
                <Route
                  path="/account/products/:publicSlug/configure"
                  element={<Website kind="configure" />}
                />
                <Route path="/account/cart" element={<Website kind="cart" />} />
                <Route path="/account/billing" element={<Finance />} />
                <Route path="/account/billing/invoices" element={<Finance />} />
                <Route
                  path="/account/billing/transactions"
                  element={<Finance />}
                />
                <Route
                  path="/account/billing/invoices/:invoiceNo"
                  element={<InvoiceDetail />}
                />
                <Route
                  path="/account/billing/renewals"
                  element={<Renewals />}
                />
                <Route
                  path="/payment/return/:attemptNo"
                  element={<PaymentReturn />}
                />
                <Route path="/account/support" element={<Support />} />
                <Route path="/account/support/new" element={<NewTicket />} />
                <Route
                  path="/account/support/:ticketNo"
                  element={<TicketDetail />}
                />
                <Route path="/account/profile" element={<Profile />} />
                <Route path="/account/security" element={<Security />} />
                <Route path="/account/privacy" element={<Privacy />} />
                <Route path="/account/contacts" element={<Contacts />} />
                <Route path="/account/addresses" element={<Addresses />} />
                <Route path="/account/ssh-keys" element={<SSHKeys />} />
                <Route path="/account/referrals" element={<Referrals />} />
                <Route
                  path="/account/contracts/*"
                  element={<Navigate to="/account/profile" replace />}
                />
                <Route path="/account/identity" element={<Verification />} />
                <Route
                  path="/account/enterprise-verification"
                  element={<Verification />}
                />
                <Route
                  path="/account/billing/invoice-profiles"
                  element={<InvoiceProfiles />}
                />
                <Route
                  path="/account/verification"
                  element={<Verification />}
                />
                <Route
                  path="/account/invoice-profiles"
                  element={<InvoiceProfiles />}
                />
                <Route path="/account/tax-invoices" element={<TaxInvoices />} />
                <Route path="/account/refunds" element={<Refunds />} />
                <Route path="/account/billing/credit" element={<Credit />} />
                <Route path="*" element={<EmptyPage />} />
              </Route>
              <Route path="/" element={<Navigate to="/account" replace />} />
            </Routes>
          </Suspense>
        </BrowserRouter>
      </ErrorBoundary>
    </ConfigProvider>
  );
}
createRoot(document.getElementById("root")!).render(<App />);

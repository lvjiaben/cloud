import { Suspense, useEffect, useState } from "react";
import {
  Alert,
  Avatar,
  Button,
  Drawer,
  Dropdown,
  Layout,
  Loading,
} from "tdesign-react";
import {
  CloudIcon,
  UserIcon,
  LinkIcon,
  MenuIcon,
  ChevronRightIcon,
  ChevronDownIcon,
} from "tdesign-icons-react";
import { Link, Outlet, useLocation, useNavigate } from "react-router-dom";
import { customerRuntime, NavigationContext } from "#/runtime/navigation";
import { requestClient, saveSession } from "#/api/request";
import { FeaturesContext, requiredFeature } from "#/runtime/features";
import CustomerAnnouncements from "#/components/CustomerAnnouncements";
import { useLoad } from "#/components/business";
import NavigationIcon from "#/components/NavigationIcon";
import { ServicePagesContext } from "#/runtime/service-pages";
export default function CustomerLayout() {
  const navigate = useNavigate(),
    location = useLocation(),
    nav = useLoad(customerRuntime),
    user = useLoad(() =>
      requestClient.get<{
        realName?: string;
        username?: string;
        avatar?: string;
      }>("/user/info"),
    ),
    brand = useLoad(() =>
      requestClient.get<{
        common?: { siteName?: string; siteTitle?: string; logoUrl?: string };
      }>("/x/idc-portal/api/v1/public/customer-center/configuration"),
    );
  const [mobile, setMobile] = useState(false);
  const [opening, setOpening] = useState(false),
    [showOpening, setShowOpening] = useState(false);
  useEffect(() => {
    const ready = () => {
      setOpening(false);
      setShowOpening(false);
    };
    window.addEventListener("customer-page-ready", ready);
    return () => window.removeEventListener("customer-page-ready", ready);
  }, []);
  useEffect(() => {
    if (!opening) return;
    const timer = window.setTimeout(() => setShowOpening(true), 180);
    return () => window.clearTimeout(timer);
  }, [opening]);
  useEffect(() => setMobile(false), [location.pathname]);
  useEffect(() => {
    const listener = () => user.reload();
    window.addEventListener("customer-profile-updated", listener);
    return () =>
      window.removeEventListener("customer-profile-updated", listener);
  }, []);
  useEffect(() => {
    if (brand.data?.common?.siteTitle)
      document.title = brand.data.common.siteTitle;
  }, [brand.data]);
  const groups = nav.data?.groups || [],
    entries = groups.flatMap((group) => group.items || [group]);
  const selected = entries
    .filter(
      (v) =>
        !("external" in v && v.external) &&
        (v.path === location.pathname ||
          (v.path !== "/account" &&
            location.pathname.startsWith(v.path + "/"))),
    )
    .sort((a, b) => b.path.length - a.path.length)[0]?.path;
  const required = requiredFeature(location.pathname);
  useEffect(() => {
    if (required && nav.data && !nav.data.enabled.has(required)) {
      setOpening(false);
      setShowOpening(false);
    }
  }, [required, nav.data]);
  const navigation = (
    <nav aria-label="主导航">
      {nav.loading ? (
        <div className="nav-loading">
          <Loading text="正在加载菜单" />
        </div>
      ) : nav.error ? (
        <div className="nav-error">
          <p>菜单加载失败</p>
          <Button tag="button" variant="outline" onClick={nav.reload}>
            重试
          </Button>
        </div>
      ) : (
        <div className="nav-sections">
          {groups.map((group) => {
            const renderItem = (
              item: { path: string; title: string; external?: boolean; icon?:string; directoryNo?:string },
              top = false,
            ) =>
              item.external ? (
                <a
                  key={item.directoryNo || item.path}
                  className="nav-link"
                  href={item.path}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <NavigationIcon icon={item.icon || "lucide:external-link"} size={top?19:16}/>
                  <span>{item.title}</span>
                  <LinkIcon size={14} />
                </a>
              ) : (
                <Link
                  key={item.directoryNo || item.path}
                  className={`nav-link ${selected === item.path ? "is-current" : ""} ${top ? "nav-top-link" : ""}`}
                  aria-current={selected === item.path ? "page" : undefined}
                  to={item.path}
                >
                  <NavigationIcon icon={item.icon || group.icon} size={top?19:16}/>
                  <span>{item.title}</span>
                </Link>
              );
            return group.items ? (
              <section className="nav-section" key={group.directoryNo || group.path}>
                <div className="nav-section-title">
                  <NavigationIcon icon={group.icon} size={16}/>
                  {group.title}
                </div>
                {group.items.map((item) => renderItem(item))}
              </section>
            ) : (
              renderItem(group, true)
            );
          })}
        </div>
      )}
    </nav>
  );
  return (
    <Layout
      className="customer-layout"
      onClickCapture={(event) => {
        if (
          event.button !== 0 ||
          event.ctrlKey ||
          event.metaKey ||
          event.shiftKey ||
          event.altKey
        )
          return;
        const link = (event.target as Element).closest(
          "a[href]",
        ) as HTMLAnchorElement | null;
        if (!link || link.target === "_blank" || link.hasAttribute("download"))
          return;
        const target = new URL(link.href, window.location.origin);
        if (
          target.origin === window.location.origin &&
          target.pathname + target.search !==
            location.pathname + location.search
        )
          setOpening(true);
      }}
    >
      <Layout.Header className="topbar">
        <div className="brand">
          <Button
            tag="button"
            aria-label="打开菜单"
            className="mobile-menu"
            shape="square"
            variant="text"
            icon={<MenuIcon />}
            onClick={() => setMobile(true)}
          />
          <Link to="/account">
            {brand.data?.common?.logoUrl ? (
              <img alt="" src={brand.data.common.logoUrl} />
            ) : (
              <span className="brand-symbol">
                <CloudIcon size={22} />
              </span>
            )}
            <strong>{brand.data?.common?.siteName || "用户中心"}</strong>
          </Link>
          <span className="brand-divider" />
          <span className="console-label">控制台</span>
        </div>
        <div className="topbar-actions">
          <Link className="topbar-link" to="/account/billing?recharge=1">
            充值
          </Link>
          <Link className="topbar-link" to="/account/orders">
            费用
          </Link>
          <Link className="topbar-link" to="/account/support">
            工单
          </Link>
          <CustomerAnnouncements />
          <Dropdown
            options={[
              { content: "个人资料", value: "profile" },
              { content: "登录与安全", value: "security" },
              { content: "退出登录", value: "logout" },
            ]}
            trigger="click"
            onClick={async (item) => {
              if (item.value === "logout") {
                try {
                  await requestClient.post("/auth/logout");
                } catch {
                  /* Local logout remains available when the server is unreachable. */
                } finally {
                  saveSession(null);
                  navigate("/auth/login");
                }
              } else
                navigate(
                  item.value === "security"
                    ? "/account/security"
                    : "/account/profile",
                );
            }}
          >
            <Button
              tag="button"
              variant="text"
              className="avatar-button"
              aria-label="账户菜单"
            >
              <span className="account-trigger">
                <Avatar
                  size="small"
                  image={user.data?.avatar || undefined}
                  icon={<UserIcon />}
                />
                <span className="account-name">
                  {user.data?.realName || user.data?.username || "我的账户"}
                </span>
                <ChevronDownIcon size={14} aria-hidden />
              </span>
            </Button>
          </Dropdown>
        </div>
      </Layout.Header>
      <Layout className="body-layout">
        <Layout.Aside width="200px" className="desktop-sidebar">
          {navigation}
          <div className="sidebar-help">
            <span>有问题，随时联系我们</span>
            <Link to="/account/support/new">
              提交工单 <ChevronRightIcon />
            </Link>
          </div>
        </Layout.Aside>
        <Layout.Content className="main-content" aria-busy={opening}>
          {showOpening && (
            <div className="route-progress" role="status">
              <Loading size="small" />
              正在打开页面
            </div>
          )}
          <div className="route-content" {...(opening ? { inert: "" } : {})}>
            <NavigationContext.Provider value={groups}>
            <ServicePagesContext.Provider value={{pages:nav.data?.servicePages || [],reload:nav.reload}}>
              <FeaturesContext.Provider value={nav.data?.enabled || new Set()}>
                {!nav.data ? (
                  <div className="loading">
                    {nav.error ? (
                      <Alert
                        theme="error"
                        title="工作台暂时无法加载，请点击侧栏重试"
                      />
                    ) : (
                      <Loading text="正在加载工作台" />
                    )}
                  </div>
                ) : required && !nav.data?.enabled.has(required) ? (
                  <section className="page">
                    <Alert theme="info" title="商家暂未开放此功能" />
                    <Link to="/account">返回账户概览</Link>
                  </section>
                ) : (
                  <Suspense
                    fallback={
                      <div className="loading">
                        <Loading text="正在打开页面" />
                      </div>
                    }
                  >
                    <Outlet key={location.pathname} />
                  </Suspense>
                )}
              </FeaturesContext.Provider>
            </ServicePagesContext.Provider>
            </NavigationContext.Provider>
          </div>
        </Layout.Content>
      </Layout>
      <Drawer
        destroyOnClose
        visible={mobile}
        header="用户中心"
        placement="left"
        size="260px"
        footer={false}
        onClose={() => setMobile(false)}
      >
        {navigation}
      </Drawer>
    </Layout>
  );
}

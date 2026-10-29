import { useEffect, useState } from "react";
import { Alert, Button, Card, Loading } from "tdesign-react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { Page } from "#/components/business";
import {
  allowedWebsiteReturn,
  currentWebsiteSession,
  postWebsiteSession,
  websiteOrigin,
  websiteSettings,
  websiteTarget,
  type WebsiteDestination,
} from "#/runtime/website";
export default function Website({ kind }: { kind: WebsiteDestination }) {
  const { publicSlug = "" } = useParams();
  const location = useLocation();
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    let active = true;
    setError("");
    void (async () => {
      const settings = await websiteSettings();
      const target = websiteTarget(settings, kind, publicSlug, location.search);
      const token = await currentWebsiteSession();
      if (active) postWebsiteSession(target, token);
    })().catch((e) => {
      if (active)
        setError(e instanceof Error ? e.message : "官网暂时无法打开，请重试。");
    });
    return () => {
      active = false;
    };
  }, [kind, publicSlug, location.search, revision]);
  return (
    <Page
      title="前往官网"
      description="选购和购物车在官网办理，已购服务和费用在控制台管理。"
    >
      <Card>
        {error ? (
          <Alert
            theme="warning"
            title={error}
            operation={
              <Button variant="text" onClick={() => setRevision((v) => v + 1)}>
                重试
              </Button>
            }
          />
        ) : (
          <Loading text="正在前往官网" />
        )}
        <p>
          <Link to="/account">返回控制台</Link>
        </p>
      </Card>
    </Page>
  );
}
export function WebsiteSession() {
  const location = useLocation(),
    navigate = useNavigate();
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    let active = true;
    setError("");
    void (async () => {
      const params = new URLSearchParams(location.search);
      const settings = await websiteSettings();
      const origin = websiteOrigin(settings);
      if (params.get("websiteOrigin") !== origin)
        throw new Error("官网来源不匹配。");
      const nonce = params.get("nonce") || "";
      const token = await currentWebsiteSession();
      if (!active) return;
      if (window.parent !== window) {
        if (!/^[a-zA-Z0-9-]{20,80}$/.test(nonce))
          throw new Error("登录请求已失效，请返回官网重试。");
        window.parent.postMessage(
          { type: "lina-website-session", nonce, accessToken: token },
          origin,
        );
        return;
      }
      const target = allowedWebsiteReturn(
        settings,
        params.get("returnTo") || "",
      );
      if (params.get("login") === "1" && !token) {
        navigate(
          `/auth/login?redirect=${encodeURIComponent(location.pathname + location.search)}`,
          { replace: true },
        );
        return;
      }
      if (params.get("login") !== "1" && params.get("probe") !== "1")
        throw new Error("登录请求不可用，请返回官网重试。");
      postWebsiteSession(target, token);
    })().catch((e) => {
      if (active)
        setError(e instanceof Error ? e.message : "官网连接失败，请重试。");
    });
    return () => {
      active = false;
    };
  }, [location.pathname, location.search, navigate, revision]);
  return (
    <div className="website-session">
      {error ? (
        <Alert
          theme="warning"
          title={error}
          operation={
            <Button variant="text" onClick={() => setRevision((v) => v + 1)}>
              重试
            </Button>
          }
        />
      ) : (
        <Loading text="正在连接官网" />
      )}
    </div>
  );
}

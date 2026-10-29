// Isolated page host for governed plugin surfaces. Tokens stay in the parent.
import { useEffect, useRef, useState } from "react";
import { Alert, Button, Loading } from "tdesign-react";
import { useNavigate } from "react-router-dom";
import {
  getSessionGeneration,
  pluginApiPath,
  requestClient,
  safePath,
} from "#/api/request";
import type { ServicePageBinding } from "#/runtime/service-pages";
const protocol = "linapro.plugin-page-bridge",
  version = "1.0";
const methods = new Set(["GET", "POST", "PUT", "PATCH", "DELETE"]);
export default function PluginServicePanel({
  binding,
  directoryNo,
}: {
  binding: ServicePageBinding;
  directoryNo: string;
}) {
  const frame = useRef<HTMLIFrameElement>(null),
    section = useRef<HTMLElement>(null),
    navigate = useNavigate();
  const [height, setHeight] = useState(0),
    [availableHeight, setAvailableHeight] = useState(540),
    [title, setTitle] = useState("已购商品"),
    [ready, setReady] = useState(false),
    [error, setError] = useState(""),
    [attempt, setAttempt] = useState(0),
    [html, setHtml] = useState("");
  useEffect(() => {
    const fit = () => {
      const top = section.current?.getBoundingClientRect().top ?? 180;
      setAvailableHeight(Math.max(360, window.innerHeight - top - 24));
    };
    fit();
    window.addEventListener("resize", fit);
    return () => window.removeEventListener("resize", fit);
  }, [directoryNo]);
  useEffect(() => {
    const controller = new AbortController();
    setHtml("");
    void fetch(binding.source, {
      credentials: "omit",
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) throw new Error("插件页面暂时无法加载，请重试");
        const document = new DOMParser().parseFromString(
          await response.text(),
          "text/html",
        );
        document
          .querySelectorAll('base,meta[name="linapro-surface-key"]')
          .forEach((node) => node.remove());
        const base = document.createElement("base");
        base.href = new URL(binding.source, location.origin).href;
        document.head.prepend(base);
        const surface = document.createElement("meta");
        surface.name = "linapro-surface-key";
        surface.content = binding.key;
        document.head.append(surface);
        if (!controller.signal.aborted)
          setHtml("<!doctype html>" + document.documentElement.outerHTML);
      })
      .catch((cause) => {
        if (!controller.signal.aborted)
          setError(
            cause instanceof Error
              ? cause.message
              : "插件页面暂时无法加载，请重试",
          );
      });
    return () => controller.abort();
  }, [binding.source, binding.key, attempt]);
  useEffect(() => {
    let active = true;
    const session = getSessionGeneration(),
      pending = new Map<string, AbortController>(),
      seen = new Set<string>(),
      visible = new Set<string>(),
      tasks = new Set<string>(),
      tickets = new Set<string>();
    setReady(false);
    setError("");
    const timeout = setTimeout(() => {
      if (active) setError("插件页面加载超时，请重试");
    }, 30000);
    function send(payload: Record<string, unknown>) {
      if (active && session === getSessionGeneration())
        frame.current?.contentWindow?.postMessage(
          {
            protocol,
            version,
            direction: "host-to-plugin",
            pluginId: binding.pluginId,
            surfaceKey: binding.key,
            ...payload,
          },
          "*",
        );
    }
    async function onMessage(event: MessageEvent) {
      if (
        !active ||
        session !== getSessionGeneration() ||
        event.source !== frame.current?.contentWindow ||
        !["null", location.origin].includes(event.origin)
      )
        return;
      const message = event.data;
      if (
        !message ||
        message.protocol !== protocol ||
        message.version !== version ||
        message.direction !== "plugin-to-host" ||
        message.pluginId !== binding.pluginId ||
        message.surfaceKey !== binding.key
      )
        return;
      const payload = message.payload;
      if (message.capability === "host.view.v1") {
        if (message.method === "ready") {
          clearTimeout(timeout);
          setReady(true);
          send({
            capability: "host.context.v1",
            method: "sync",
            payload: {
              locale: "zh-CN",
              services: { directoryNo, pluginId: binding.pluginId },
              route: { path: "" },
              theme: {
                mode: "light",
                fontFamily: "Arial, sans-serif",
                fontSize: 14,
              },
            },
          });
        } else if (
          message.method === "set-title" &&
          typeof payload?.title === "string" &&
          payload.title.trim() &&
          payload.title.length <= 100
        ) {
          setTitle(payload.title);
        } else if (
          message.method === "resize" &&
          Number.isFinite(payload?.height)
        )
          setHeight(Math.max(320, Math.min(4000, Math.ceil(payload.height))));
        else if (
          message.method === "navigate" &&
          typeof payload?.path === "string" &&
          safePath(payload.path) &&
          payload.path.startsWith("/account/")
        )
          navigate(payload.path);
        else if (message.method === "error")
          setError("插件页面暂时无法显示，请重试");
        return;
      }
      const id = message.requestId;
      if (
        message.method !== "request" ||
        typeof id !== "string" ||
        !id ||
        id.length > 128 ||
        seen.has(id) ||
        seen.size >= 2048 ||
        pending.size >= 8 ||
        !binding.bridgeCapabilities?.includes(message.capability)
      )
        return;
      seen.add(id);
      const controller = new AbortController();
      pending.set(id, controller);
      const deadline = setTimeout(() => controller.abort(), 30000);
      try {
        if (
          !payload ||
          !methods.has(payload.method) ||
          typeof payload.path !== "string" ||
          !/^[A-Za-z0-9/_-]+$/.test(payload.path) ||
          payload.path.includes("..")
        )
          throw new Error("插件请求不可用");
        let owner = binding.pluginId,
          path = payload.path;
        let params =
          payload.params &&
          typeof payload.params === "object" &&
          !Array.isArray(payload.params)
            ? { ...payload.params }
            : {};
        const headers: Record<string, string> = {};
        if (message.capability === "purchased-services.customer.v1") {
          owner = "idc-core";
          if (
            payload.method === "GET" &&
            path === "account/purchased-services"
          ) {
            path = `account/product-directories/${encodeURIComponent(directoryNo)}/services`;
            params = {
              page: params.page,
              pageSize: params.pageSize,
              keyword: params.keyword,
              providerId: binding.pluginId,
            };
          } else {
            const note =
              /^account\/service-entitlements\/([A-Za-z0-9_-]+)\/customer-note$/.exec(
                path,
              );
            if (payload.method !== "PUT" || !note || !visible.has(note[1]))
              throw new Error("该服务不在当前列表中，请刷新重试");
          }
        } else if (binding.pluginType === "dynamic") {
          const item =
            /^account\/service-entitlements\/([A-Za-z0-9_-]+)(?:\/|$)/.exec(
              path,
            );
          const task = /^account\/service-action-tasks\/([A-Za-z0-9_-]+)$/.exec(
            path,
          );
          const ticket = /^account\/service-downloads\/([A-Za-z0-9_-]+)$/.exec(
            path,
          );
          if (
            (item && !visible.has(item[1])) ||
            (task && !tasks.has(task[1])) ||
            (ticket && !tickets.has(ticket[1]))
          )
            throw new Error("该内容不在当前插件列表中，请刷新重试");
          const grant = await requestClient.post<{
            grant: string;
            ownerPluginId: string;
          }>(
            "/plugins/page-bridge/grant",
            {
              pluginId: binding.pluginId,
              host: "customer",
              surfaceKey: binding.key,
              surfaceVersion: binding.version,
              capability: message.capability,
              method: payload.method,
              path,
            },
            { signal: controller.signal },
          );
          if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(grant.ownerPluginId))
            throw new Error("插件请求不可用");
          owner = grant.ownerPluginId;
          headers["X-Lina-Plugin-Page-Grant"] = grant.grant;
          if (
            path === "account/service-entitlements" &&
            payload.method === "GET"
          )
            params = { ...params, directoryNo, pluginId: binding.pluginId };
        } else if (!path.startsWith("account/"))
          throw new Error("插件请求不可用");
        if (
          Array.isArray(params.entitlementNos) &&
          params.entitlementNos.some(
            (id: unknown) => typeof id !== "string" || !visible.has(id),
          )
        )
          throw new Error("该服务不在当前列表中，请刷新重试");
        const options = {
          method: payload.method,
          params,
          data: payload.body,
          headers,
          signal: controller.signal,
        };
        let result: unknown;
        if (payload.download && typeof payload.download.fileName === "string") {
          const blob = await requestClient.download<Blob>(
            pluginApiPath(owner, path),
            options,
          );
          if (!active || session !== getSessionGeneration()) return;
          const href = URL.createObjectURL(blob),
            link = document.createElement("a");
          link.href = href;
          link.download = payload.download.fileName
            .replace(/[\\/\u0000-\u001f]/g, "_")
            .slice(0, 200);
          link.click();
          setTimeout(() => URL.revokeObjectURL(href), 1000);
          result = {};
        } else
          result = await requestClient.request<unknown>(
            pluginApiPath(owner, path),
            options,
          );
        const projection = result as {
          items?: { entitlementNo: string }[];
          task?: { taskNo?: string };
          ticket?: string;
        };
        if (projection?.task?.taskNo) tasks.add(projection.task.taskNo);
        if (projection?.ticket) tickets.add(projection.ticket);
        if (
          payload.method === "GET" &&
          (message.capability === "purchased-services.customer.v1" ||
            payload.path === "account/service-entitlements")
        ) {
          if (message.capability === "purchased-services.customer.v1")
            visible.clear();
          for (const item of projection.items || [])
            visible.add(item.entitlementNo);
        }
        send({ requestId: id, ok: true, result });
      } catch (cause) {
        if (!controller.signal.aborted)
          send({
            requestId: id,
            ok: false,
            result: {
              message:
                cause instanceof Error ? cause.message : "请求失败，请重试",
            },
          });
      } finally {
        clearTimeout(deadline);
        pending.delete(id);
      }
    }
    const listener = (event: MessageEvent) => {
      void onMessage(event);
    };
    window.addEventListener("message", listener);
    return () => {
      active = false;
      clearTimeout(timeout);
      window.removeEventListener("message", listener);
      for (const request of pending.values()) request.abort();
    };
  }, [binding, directoryNo, navigate, attempt]);
  return (
    <section ref={section} className="plugin-service-panel" data-plugin={binding.pluginId}>
      {error ? (
        <Alert
          theme="error"
          message={error}
          operation={
            <Button
              variant="text"
              onClick={() => setAttempt((value) => value + 1)}
            >
              重试
            </Button>
          }
        />
      ) : (
        <>
          {!ready && (
            <div className="loading">
              <Loading text="正在加载商品内容" />
            </div>
          )}
          {html && (
            <iframe
              key={`${binding.source}:${attempt}`}
              ref={frame}
              title={title}
              srcDoc={html}
              sandbox="allow-downloads allow-forms allow-modals allow-popups allow-scripts"
              referrerPolicy="no-referrer"
              style={{
                width: "100%",
                height: Math.max(height, availableHeight),
                border: 0,
                display: ready ? "block" : "none",
              }}
            />
          )}
        </>
      )}
    </section>
  );
}

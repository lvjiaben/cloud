import { useState } from "react";
import { Alert, Button } from "tdesign-react";
import Dialog from "#/adapter/Dialog";
import { DetailList, ErrorNote, useAction } from "#/components/business";
import * as api from "#/api/idc/native-servers";

export function secureConsoleUrl(ticket: api.ConsoleTicket) {
  for (const candidate of [
    ticket.vnc_url_https,
    ticket.vnc_url,
    ticket.vnc_url_http,
  ]) {
    if (!candidate) continue;
    try {
      const url = new URL(candidate);
      if (
        url.protocol === "https:" &&
        !url.username &&
        !url.password &&
        !url.hash
      )
        return url.href;
    } catch {
      /* Invalid candidates never become links. */
    }
  }
  return "";
}
export default function ServerOutputDialog({
  serverNo,
  operationNo,
  name,
  onClose,
  onConsumed,
}: {
  serverNo: string;
  operationNo: string;
  name: "generatedCredential" | "consoleTicket";
  onClose: () => void;
  onConsumed: () => void;
}) {
  const action = useAction();
  const [attempted, setAttempted] = useState(false),
    [credential, setCredential] = useState<api.GeneratedCredential>(),
    [ticket, setTicket] = useState<api.ConsoleTicket>();
  const title = name === "consoleTicket" ? "远程连接" : "新系统登录信息";
  const delivered = !!credential || !!ticket;
  return (
    <Dialog
      visible
      header={title}
      closeOnOverlayClick={false}
      onClose={() => !action.busy && onClose()}
      footer={
        <>
          <Button
            tag="button"
            variant="outline"
            disabled={action.busy}
            onClick={onClose}
          >
            {delivered ? "关闭并隐藏" : "关闭"}
          </Button>
          {!attempted && (
            <Button
              tag="button"
              loading={action.busy}
              onClick={() =>
                void action.run(async () => {
                  setAttempted(true);
                  try {
                    if (name === "generatedCredential") {
                      const value =
                        await api.revealNativeOutput<api.GeneratedCredential>(
                          serverNo,
                          operationNo,
                          name,
                        );
                      if (typeof value.password !== "string" || !value.password)
                        throw new Error(
                          "未能取得登录信息，请通过工单联系我们。",
                        );
                      setCredential(value);
                    } else {
                      const value =
                        await api.revealNativeOutput<api.ConsoleTicket>(
                          serverNo,
                          operationNo,
                          name,
                        );
                      setTicket(value);
                    }
                  } finally {
                    onConsumed();
                  }
                })
              }
            >
              确认获取
            </Button>
          )}
        </>
      }
    >
      {!attempted && (
        <p>信息仅提供一次，请确认后获取并妥善保存。关闭窗口后立即隐藏。</p>
      )}
      {credential && (
        <>
          <Alert theme="warning" message="请妥善保管新密码" />
          <DetailList
            items={[
              ["登录用户", credential.username || "尚未确认"],
              ["登录端口", credential.port || "尚未确认"],
              [
                "新密码",
                <span className="secret" data-testid="generated-password">
                  {credential.password}
                </span>,
              ],
            ]}
          />
        </>
      )}
      {ticket && (
        <>
          {secureConsoleUrl(ticket) ? (
            <>
              <p>连接地址短期有效，请及时打开。</p>
              <a
                className="console-connect-link"
                href={secureConsoleUrl(ticket)}
                target="_blank"
                rel="noopener noreferrer"
              >
                打开远程控制台
              </a>
            </>
          ) : (
            <Alert
              theme="warning"
              message="暂未取得可打开的安全连接地址，请重新发起远程连接或提交工单。"
            />
          )}
          {ticket.vnc_password && (
            <DetailList
              items={[
                [
                  "控制台密码",
                  <span className="secret">{ticket.vnc_password}</span>,
                ],
              ]}
            />
          )}
        </>
      )}
      <ErrorNote error={action.error} />
      {attempted && action.error && (
        <p className="muted">
          信息可能已经领取，当前窗口不会再次获取。请关闭后查看操作结果或提交工单。
        </p>
      )}
    </Dialog>
  );
}

import { useState } from "react";
import { Alert, Button, Card } from "tdesign-react";
import { Link } from "react-router-dom";
import * as api from "#/api/idc/customer-account";
import { actionKey, clearActionKey } from "#/api/request";
import {
  Confirm,
  ErrorNote,
  FetchState,
  Page,
  Status,
  date,
  useLoad,
  useAction,
} from "#/components/business";
const findingLabels: Record<string, string> = {
  active_services: "仍有使用中的服务",
  in_progress_operations: "仍有正在处理的操作",
  unsettled_funds: "还有资金需要结清",
  unsettled_invoices: "还有未结清的账单",
  legal_retention: "部分记录需要依法保留",
};
export default function Privacy() {
  const profile = useLoad(
      api.getCustomerProfile,
      [],
      "Privacy:Privacy:profile",
    ),
    action = useAction();
  const [confirm, setConfirm] = useState<"export" | "closure">(),
    [closure, setClosure] = useState<api.ClosureRequest>(),
    [requestNo, setRequestNo] = useState(
      () => sessionStorage.getItem("customer-tdesign.export") || "",
    );
  const result = useLoad(
    () =>
      requestNo ? api.getDataExportStatus(requestNo) : Promise.resolve(null),
    [requestNo],
    "Privacy:Privacy:result",
  );
  return (
    <Page
      title="数据与账户"
      description="申请下载账户资料，或在结清业务后申请注销。"
    >
      <FetchState state={profile}>
        <Card title="下载账户资料">
          <p>资料只供您本人下载。请先重新登录，再确认生成文件。</p>
          <div className="actions">
            <Link to="/auth/login?redirect=%2Faccount%2Fprivacy">
              <Button tag="button" variant="outline">
                重新登录
              </Button>
            </Link>
            <Button
              tag="button"
              disabled={!profile.data || action.busy}
              onClick={() => {
                action.clear();
                setConfirm("export");
              }}
            >
              申请下载资料
            </Button>
          </div>
          {requestNo && (
            <FetchState state={result}>
              {result.data && (
                <div className="section">
                  <Status value={result.data.status} />
                  <p>申请时间：{date(result.data.requestedAt)}</p>
                  <div className="actions">
                    <Button
                      tag="button"
                      variant="outline"
                      onClick={result.reload}
                    >
                      刷新进度
                    </Button>
                    {["succeeded", "completed"].includes(
                      result.data.status,
                    ) && (
                      <Button
                        tag="button"
                        disabled={action.busy}
                        onClick={() =>
                          void action.run(async () => {
                            const ticket =
                              await api.createDataExportTicket(requestNo);
                            const a = document.createElement("a");
                            a.href = api.dataExportDownloadURL(ticket.token);
                            a.download = "";
                            a.rel = "noreferrer";
                            a.click();
                          })
                        }
                      >
                        下载文件
                      </Button>
                    )}
                  </div>
                </div>
              )}
            </FetchState>
          )}
          <ErrorNote error={action.error} />
        </Card>
        <Card title="注销账户">
          <p>
            注销申请会先检查服务、账单和资金，再交由商家复核。提交申请不会立即删除账户；依法需要保留的记录仍会保留。
          </p>
          <Button
            tag="button"
            theme="danger"
            variant="outline"
            disabled={!profile.data || action.busy}
            onClick={() => {
              action.clear();
              setConfirm("closure");
            }}
          >
            申请注销
          </Button>
          {closure && (
            <Alert
              theme={closure.status === "blocked" ? "warning" : "info"}
              title={
                closure.status === "blocked"
                  ? "暂时无法申请，请先处理以下事项"
                  : "申请已提交，等待商家复核"
              }
              message={
                <ul>
                  {closure.findings.map((v, i) => (
                    <li key={i}>
                      {findingLabels[v.code] || "有待处理的业务"}（{v.count}项）
                    </li>
                  ))}
                </ul>
              }
            />
          )}
        </Card>
      </FetchState>
      {confirm && (
        <Confirm
          title={confirm === "export" ? "确认下载账户资料" : "确认申请注销"}
          busy={action.busy}
          error={action.error}
          onClose={() => setConfirm(undefined)}
          onConfirm={() =>
            void action.run(async () => {
              if (!profile.data) return;
              const version = profile.data.item.version,
                scope = `${confirm}-${version}`;
              const input = {
                confirmed: true as const,
                expectedVersion: version,
                idempotencyKey: actionKey(scope),
              };
              if (confirm === "export") {
                const proof = await api.createDataExportProof(input);
                const item = await api.requestDataExport({
                  ...input,
                  proofToken: proof.proofToken,
                });
                sessionStorage.setItem(
                  "customer-tdesign.export",
                  item.requestNo,
                );
                setRequestNo(item.requestNo);
              } else setClosure((await api.requestCustomerClosure(input)).item);
              clearActionKey(scope);
              setConfirm(undefined);
            })
          }
        >
          <p>
            {confirm === "export"
              ? "我确认由本人申请生成账户资料，并妥善保管下载文件。"
              : "我了解注销需要结清业务并经商家复核，确认提交注销申请。"}
          </p>
        </Confirm>
      )}
    </Page>
  );
}

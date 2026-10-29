import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Alert, Button, Card, Tooltip } from "tdesign-react";
import Dialog from "#/adapter/Dialog";
import OperationDialog from "./ServerOperationDialog";
import ServerOutputDialog from "./ServerOutputDialog";
import ServerPurchaseDialog from "./ServerPurchaseDialog";
import ServerOperationLog from "./ServerOperationLog";
import ServerResizeDialog from "./ServerResizeDialog";
import {
  ServerAccessoryBills,
  ServerCommercialRecords,
} from "./ServerBillingRecords";
import Tabs from "#/adapter/Tabs";
import * as api from "#/api/idc/native-servers";
import { actionKey, clearActionKey } from "#/api/request";
import { getCustomerServiceNote } from "#/api/idc/product-directories";
import {
  Page,
  FetchState,
  useLoad,
  DetailList,
  Status,
  date,
  BusinessTable,
  useAction,
  ErrorNote,
  Confirm,
} from "#/components/business";
import { RenewDialog } from "./Services";

const names: Record<string, string> = {
  start: "开机",
  stop: "关机",
  reboot: "重启",
  reset_password: "重置密码",
  console: "远程连接",
  reinstall: "重装系统",
  enter_rescue: "进入救援",
  exit_rescue: "退出救援",
  security_group_attach: "应用安全组",
  security_group_detach: "解除安全组",
  nat_create: "添加端口转发",
  nat_delete: "删除端口转发",
  virtual_media_mount: "挂载镜像",
  virtual_media_unmount: "卸载镜像",
  snapshot_create: "创建快照",
  snapshot_delete: "删除快照",
  snapshot_restore: "恢复快照",
  backup_create: "创建备份",
  backup_delete: "删除备份",
  backup_restore: "恢复备份",
  traffic_add: "购买流量包",
  ip_change: "更换 IP",
  resize: "调整配置",
};
const pending = new Set([
  "pending",
  "submitting",
  "accepted",
  "running",
  "retry_wait",
]);
const tabItems = [
  { value: "overview", label: "概览" },
  { value: "system", label: "系统与连接" },
  { value: "storage", label: "快照与备份" },
  { value: "network", label: "网络与安全" },
  { value: "logs", label: "操作记录" },
  { value: "billing", label: "费用" },
];
const unknown = (v?: string | number) =>
  v === undefined || v === "" ? "尚未确认" : v;

export default function ServerManagement() {
  const { serverNo, entitlementNo } = useParams();
  const detail = useLoad(
    async () =>
      api.getNativeServer(
        serverNo || (await api.resolveNativeServer(entitlementNo!)).serverNo,
      ),
    [serverNo, entitlementNo],
    "ServerManagement:detail",
  );
  const item = detail.data;
  return (
    <Page
      title={item?.productNameZhCn || item?.productNameEnUs || "服务器管理"}
      description={
        item?.information?.hostname || "查看资源信息，管理服务与费用。"
      }
      actions={
        <Button tag="button" variant="outline" onClick={detail.reload}>
          刷新状态
        </Button>
      }
    >
      <FetchState state={detail}>
        {item && (
          <Management key={item.serverNo} item={item} refresh={detail.reload} />
        )}
      </FetchState>
    </Page>
  );
}
function Management({
  item,
  refresh,
}: {
  item: api.NativeServerDetail;
  refresh: () => void;
}) {
  const [tab, setTab] = useState("overview"),
    [operation, setOperation] = useState<string>(),
    [selectedObject, setSelectedObject] = useState<string>(),
    [commercial, setCommercial] = useState<api.CommercialAction>(),
    [renew, setRenew] = useState(false),
    [initial, setInitial] = useState(false),
    [password, setPassword] = useState("");
  const options = useLoad(
    () => api.getNativeOptions(item.serverNo),
    [item.serverNo],
    "ServerManagement:options",
  );
  const note = useLoad(
    () => getCustomerServiceNote(item.entitlementNo),
    [item.entitlementNo],
    "ServerManagement:note",
  );
  const reveal = useAction();
  const direct = useAction();
  const progressRead = useAction();
  const [progressReference, setProgressReference] = useState("");
  const [current, setCurrent] = useState<api.NativeOperation | undefined>(() =>
    item.operations.find(
      (operation) =>
        pending.has(operation.status) ||
        ["result_unknown", "manual_takeover"].includes(operation.status),
    ),
  );
  const [pollError, setPollError] = useState("");
  const [output, setOutput] = useState<{
    operationNo: string;
    name: "generatedCredential" | "consoleTicket";
  }>();
  const [consumed, setConsumed] = useState<Set<string>>(() => new Set());
  const outputButtons = (operation: api.NativeOperation) =>
    (operation.availableOutputs || [])
      .filter(
        (o) =>
          (!o.expiresAt || o.expiresAt > Date.now()) &&
          !consumed.has(`${operation.operationNo}:${o.name}`),
      )
      .map((o) => (
        <Button
          tag="button"
          key={o.name}
          variant="text"
          onClick={() =>
            setOutput({ operationNo: operation.operationNo, name: o.name })
          }
        >
          {o.name === "consoleTicket" ? "获取连接" : "查看新密码"}
        </Button>
      ));
  useEffect(() => {
    if (!current) return;
    const latest = item.operations.find(
      (op) => op.operationNo === current.operationNo,
    );
    if (latest && latest.updatedAt > current.updatedAt) setCurrent(latest);
  }, [item.operations, current]);
  useEffect(() => {
    if (!current || !pending.has(current.status)) return;
    let alive = true;
    const timer = setTimeout(() => {
      api
        .getNativeOperation(item.serverNo, current.operationNo)
        .then((result) => {
          if (alive) {
            setCurrent((before) =>
              before?.operationNo === current.operationNo &&
              before.updatedAt <= result.operation.updatedAt
                ? result.operation
                : before,
            );
            setPollError("");
            if (!pending.has(result.operation.status)) refresh();
          }
        })
        .catch(() => {
          if (alive)
            setPollError(
              "暂时无法更新处理进度，请稍后刷新操作记录。请勿重复提交。",
            );
        });
    }, 2500);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [item.serverNo, current, refresh]);
  const can = (action: string) => item.availableActions.includes(action);
  const operationBusy =
    direct.busy ||
    (!!current &&
      (pending.has(current.status) ||
        ["result_unknown", "manual_takeover"].includes(current.status)));
  const button = (action: string) => {
    const capability = item.capabilities?.find((v) => v.action === action);
    if (!can(action) && !capability?.supported) return null;
    const reasons: Record<string, string> = {
      capability_unavailable: "暂时无法确认可用操作，请刷新后重试。",
      unsupported: "当前服务暂不支持此操作。",
      entitlement_unavailable: "当前服务状态不允许执行此操作。",
      operation_in_progress: "已有操作正在处理，请完成后再试。",
      runtime_unconfirmed: "尚未确认运行状态，请刷新后重试。",
      runtime_state_conflict: "当前运行状态不允许执行此操作。",
      choices_unavailable: "管理选项暂不可用或已过期，请刷新管理选项后重试。",
    };
    const control = (
      <Button
        tag="button"
        key={action}
        disabled={!can(action) || operationBusy}
        loading={direct.busy && ["start", "console"].includes(action)}
        variant={action === "console" ? "base" : "outline"}
        onClick={() => {
          if (!["start", "console"].includes(action)) {
            setSelectedObject(undefined);
            setOperation(action);
            return;
          }
          void direct.run(async () => {
            const scope = `native-direct-${item.serverNo}-${action}`;
            const result = await api.createNativeOperation(
              item.serverNo,
              action,
              actionKey(scope),
              true,
            );
            clearActionKey(scope);
            setCurrent(result.operation);
            setPollError("");
            refresh();
          });
        }}
      >
        {names[action] || "管理服务"}
      </Button>
    );
    return can(action) ? (
      control
    ) : (
      <Tooltip
        key={action}
        content={
          reasons[capability?.reasonCode || ""] ||
          "此操作暂时不可用，请稍后刷新。"
        }
      >
        <span>{control}</span>
      </Tooltip>
    );
  };
  const commercialButton = (action: api.CommercialAction) => {
    const capability = item.commercialCapabilities?.find(
      (v) => v.action === action,
    );
    if (!capability?.supported) return null;
    const canQuote = capability.canQuote;
    return (
      <Tooltip
        key={action}
        content={
          canQuote ? "" : "当前状态或商家规则暂不允许操作，请刷新后重试。"
        }
      >
        <span>
          <Button
            tag="button"
            variant="outline"
            disabled={!canQuote || operationBusy}
            onClick={() => setCommercial(action)}
          >
            {names[action]}
          </Button>
        </span>
      </Tooltip>
    );
  };
  const info = item.information;
  const objects = options.data?.accessories || [];
  return (
    <>
      <Card className="server-heading">
        <div className="server-heading-line">
          <div className="service-title">
            <Status value={item.runtimeState} />
            <span className="muted">{item.serverNo}</span>
          </div>
          <Link
            to={`/account/support/new?service=${encodeURIComponent(item.entitlementNo)}`}
          >
            提交工单
          </Link>
        </div>
        <div className="server-summary">
          <div>
            <span>连接地址</span>
            <strong>
              {item.addressesConfirmed
                ? item.addresses.join(" / ") || "暂未分配"
                : "尚未确认"}
            </strong>
          </div>
          <div>
            <span>操作系统</span>
            <strong>{unknown(info?.osName)}</strong>
          </div>
          <div>
            <span>到期时间</span>
            <strong>{date(item.expiresAt)}</strong>
          </div>
          <div>
            <span>服务状态</span>
            <Status value={item.entitlementStatus} />
          </div>
        </div>
        <div className="actions server-power">
          {["start", "stop", "reboot", "console"].map(button)}
          {item.expiresAt && item.entitlementStatus !== "terminated" && (
            <Button tag="button" onClick={() => setRenew(true)}>
              续费
            </Button>
          )}
        </div>
        <ErrorNote error={direct.error} />
        <p className="muted server-observed">
          信息更新时间：{date(item.observedAt)}
        </p>
      </Card>
      {current && (
        <Card className="server-progress">
          <div className="server-heading-line">
            <strong>{names[current.action] || "服务操作"}</strong>
            <Status
              value={
                current.status === "running" ? "processing" : current.status
              }
            />
          </div>
          <p>
            {pending.has(current.status)
              ? "操作已提交，正在处理。请勿重复提交。"
              : current.status === "succeeded"
                ? "操作已完成，资源信息正在更新。"
                : current.status === "result_unknown"
                  ? "结果正在确认，请等待处理或通过工单联系我们。"
                  : "请查看操作记录；如需帮助，请提交工单。"}
          </p>
          {current.failureCode === "owner_admission_denied" && (
            <Alert
              theme="warning"
              message="服务器归属、服务状态或配置已变化，请刷新核对后重新操作。"
            />
          )}
          <ErrorNote
            error={
              pollError ||
              (progressReference === current.operationNo
                ? progressRead.error
                : "")
            }
          />
          <div className="actions">{outputButtons(current)}</div>
          {current.waitingUntil && (
            <p className="muted">
              预计可继续处理时间：{date(current.waitingUntil)}
            </p>
          )}
        </Card>
      )}
      <div className="server-workspace">
        <div className="server-options-refresh">
          <Button
            tag="button"
            variant="text"
            size="small"
            loading={options.loading}
            onClick={options.reload}
          >
            刷新管理选项
          </Button>
          <span className="muted">
            {options.data?.accessoriesObservedAt
              ? `更新时间：${date(options.data.accessoriesObservedAt)}`
              : ""}
          </span>
        </div>
        <Tabs
          value={tab}
          onChange={setTab}
          items={tabItems}
          label="服务器管理"
        />
        {options.error && tab !== "storage" && (
          <Alert
            theme="warning"
            message={options.error}
            operation={
              <Button tag="button" variant="text" onClick={options.reload}>
                重新读取管理选项
              </Button>
            }
          />
        )}
        {tab === "overview" && (
          <div className="server-detail-grid">
            <Card title="实例信息">
              <div className="actions">{commercialButton("resize")}</div>
              <DetailList
                items={[
                  ["主机名", unknown(info?.hostname)],
                  ["服务编号", item.entitlementNo],
                  [
                    "处理器",
                    info?.cpuCores ? `${info.cpuCores} 核` : "尚未确认",
                  ],
                  [
                    "内存",
                    info?.memoryMiB ? `${info.memoryMiB} MiB` : "尚未确认",
                  ],
                  ["开始时间", date(item.startedAt)],
                  [
                    "备注",
                    note.loading && !note.data
                      ? "正在读取"
                      : note.error
                        ? "暂时无法读取"
                        : note.data?.note.remark || "未填写",
                  ],
                ]}
              />
            </Card>
            <Card title="连接信息">
              <DetailList
                items={[
                  [
                    "IP 地址",
                    item.addressesConfirmed
                      ? item.addresses.join(" / ") || "暂未分配"
                      : "尚未确认",
                  ],
                  ["系统", unknown(info?.osName)],
                  ["登录用户", unknown(info?.loginUsername)],
                  ["登录端口", unknown(info?.port)],
                  ["网络类型", unknown(info?.networkType)],
                  ["启动方式", unknown(info?.bootOrder)],
                ]}
              />
              <div className="actions">
                {button("console")}
                {item.credentialAvailable && (
                  <Button
                    tag="button"
                    variant="outline"
                    onClick={() => {
                      reveal.clear();
                      setInitial(true);
                    }}
                  >
                    查看初始密码
                  </Button>
                )}
              </div>
            </Card>
          </div>
        )}
        {tab === "system" && (
          <div className="server-detail-grid">
            <Card title="操作系统">
              <DetailList
                items={[
                  ["当前系统", unknown(info?.osName)],
                  ["登录用户", unknown(info?.loginUsername)],
                  ["登录端口", unknown(info?.port)],
                ]}
              />
              <p className="muted">
                重装会清除系统盘数据。请先备份需要保留的文件。
              </p>
              <div className="actions">
                {button("reinstall")}
                {button("reset_password")}
              </div>
            </Card>
            <Card title="远程连接与镜像">
              <p className="muted">
                系统无法通过网络连接时，可以尝试远程控制台。
              </p>
              <div className="actions">
                {button("console")}
                {button("virtual_media_mount")}
                {button("virtual_media_unmount")}
              </div>
              <AccessoryList
                items={objects.filter((o) => o.kind === "virtual_media")}
              />
            </Card>
            <Card title="救援模式">
              <p className="muted">
                系统无法正常启动时，可以进入临时系统检查磁盘和修复故障。完成后退出救援，恢复原系统启动。
              </p>
              <DetailList
                items={[
                  [
                    "当前状态",
                    info?.rescue === undefined
                      ? "尚未确认"
                      : info.rescue
                        ? "已进入救援"
                        : "未进入救援",
                  ],
                ]}
              />
              <div className="actions">
                {button("enter_rescue")}
                {button("exit_rescue")}
              </div>
            </Card>
          </div>
        )}
        {tab === "storage" && (
          <ServerAccessoryBills serverNo={item.serverNo} objects={objects} />
        )}
        {tab === "storage" && (
          <Card title="快照与备份">
            <div className="actions">
              {commercialButton("snapshot_create")}
              {commercialButton("backup_create")}
            </div>
            <FetchState state={options}>
              <AccessoryList
                items={objects.filter((o) =>
                  ["snapshot", "backup", "disk"].includes(o.kind),
                )}
                actions={(object) =>
                  ["snapshot", "backup"].includes(object.kind) ? (
                    <div className="actions">
                      {["restore", "delete"].map((verb) => {
                        const action = `${object.kind}_${verb}`;
                        return can(action) ? (
                          <Button
                            key={verb}
                            tag="button"
                            variant="text"
                            disabled={
                              operationBusy ||
                              (verb === "restore" &&
                                object.state !== "available")
                            }
                            onClick={() => {
                              setSelectedObject(object.objectNo);
                              setOperation(action);
                            }}
                          >
                            {verb === "restore" ? "恢复" : "删除"}
                          </Button>
                        ) : null;
                      })}
                    </div>
                  ) : null
                }
              />
            </FetchState>
          </Card>
        )}
        {tab === "network" && (
          <div className="server-detail-grid">
            <Card title="网络地址">
              <div className="actions">{commercialButton("ip_change")}</div>
              <DetailList
                items={[
                  [
                    "IP 地址",
                    item.addressesConfirmed
                      ? item.addresses.join(" / ") || "暂未分配"
                      : "尚未确认",
                  ],
                  ["网络类型", unknown(info?.networkType)],
                ]}
              />
              <AccessoryList items={objects.filter((o) => o.kind === "ip")} />
            </Card>
            <Card title="安全组">
              <div className="actions">
                {button("security_group_attach")}
                {button("security_group_detach")}
              </div>
              <AccessoryList
                items={objects.filter((o) => o.kind === "security_group")}
              />
            </Card>
            <Card title="端口转发">
              <div className="actions">
                {button("nat_create")}
                {button("nat_delete")}
              </div>
              <AccessoryList items={objects.filter((o) => o.kind === "nat")} />
            </Card>
            <Card title="流量额度" className="traffic-allowance">
              <div className="actions">{commercialButton("traffic_add")}</div>
              {objects.filter((object) => object.kind === "traffic").length ? (
                objects
                  .filter((object) => object.kind === "traffic")
                  .map((object) => (
                    <DetailList
                      key={object.objectNo}
                      items={[
                        ["基础额度", trafficAmount(object.projection.quota_gb)],
                        [
                          "临时额度",
                          trafficAmount(object.projection.temporary_gb),
                        ],
                        [
                          "当前总额度",
                          trafficAmount(object.projection.total_gb),
                        ],
                      ]}
                    />
                  ))
              ) : (
                <p className="muted">暂无已确认的流量额度。</p>
              )}
              <p className="muted">
                这里显示已分配额度，实际使用情况以监控信息为准。
              </p>
            </Card>
          </div>
        )}
        {tab === "logs" && (
          <ServerCommercialRecords serverNo={item.serverNo} names={names} />
        )}
        {tab === "logs" && (
          <ServerOperationLog
            serverNo={item.serverNo}
            names={names}
            busy={progressRead.busy}
            selected={progressReference}
            onRead={(row) => {
              setCurrent((before) =>
                before?.operationNo === row.operationNo &&
                before.updatedAt > row.updatedAt
                  ? before
                  : row,
              );
              setPollError("");
              setProgressReference(row.operationNo);
              void progressRead.run(async () => {
                const result = await api.getNativeOperation(
                  item.serverNo,
                  row.operationNo,
                );
                setCurrent((before) =>
                  before?.operationNo === row.operationNo &&
                  before.updatedAt <= result.operation.updatedAt
                    ? result.operation
                    : before,
                );
              });
            }}
            outputs={outputButtons}
          />
        )}
        {tab === "billing" && (
          <Card title="服务费用">
            <DetailList
              items={[
                ["服务状态", <Status value={item.entitlementStatus} />],
                ["开通时间", date(item.startedAt)],
                ["到期时间", date(item.expiresAt)],
              ]}
            />
            <div className="actions">
              {item.expiresAt && item.entitlementStatus !== "terminated" && (
                <Button tag="button" onClick={() => setRenew(true)}>
                  续费服务
                </Button>
              )}
              <Link to={`/account/orders/${encodeURIComponent(item.orderNo)}`}>
                查看购买订单
              </Link>
              <Link to="/account/billing">查看账单</Link>
            </div>
          </Card>
        )}
      </div>
      {operation && (
        <OperationDialog
          item={item}
          code={operation}
          title={names[operation] || "服务操作"}
          options={options.data}
          objectNo={selectedObject}
          onClose={() => setOperation(undefined)}
          onCreated={(result) => {
            setCurrent(result);
            setPollError("");
            setOperation(undefined);
            refresh();
          }}
        />
      )}
      {commercial === "resize" && (
        <ServerResizeDialog
          item={item}
          onClose={() => setCommercial(undefined)}
          onCreated={() => {
            setTab("logs");
            refresh();
            options.reload();
          }}
        />
      )}
      {commercial && commercial !== "resize" && (
        <ServerPurchaseDialog
          item={item}
          code={commercial}
          title={names[commercial]}
          options={options.data}
          onClose={() => setCommercial(undefined)}
          onCreated={() => {
            setTab("logs");
            refresh();
            options.reload();
          }}
        />
      )}
      {output && (
        <ServerOutputDialog
          {...output}
          serverNo={item.serverNo}
          onClose={() => setOutput(undefined)}
          onConsumed={() => {
            setConsumed(
              (before) =>
                new Set([...before, `${output.operationNo}:${output.name}`]),
            );
            refresh();
          }}
        />
      )}
      {renew && (
        <RenewDialog
          entitlementNo={item.entitlementNo}
          onClose={() => setRenew(false)}
        />
      )}
      {initial && (
        <Confirm
          title="查看初始密码"
          busy={reveal.busy}
          error={reveal.error}
          onClose={() => setInitial(false)}
          onConfirm={() =>
            void reveal.run(async () => {
              setPassword(
                (await api.revealInitialCredential(item.serverNo)).password,
              );
              setInitial(false);
              refresh();
            })
          }
        >
          <p>密码只提供一次，请在当前窗口妥善保存。关闭窗口后立即隐藏。</p>
        </Confirm>
      )}
      {password && (
        <Dialog
          visible
          header="初始密码"
          onClose={() => setPassword("")}
          footer={
            <Button tag="button" onClick={() => setPassword("")}>
              关闭并隐藏
            </Button>
          }
        >
          <Alert theme="warning" message="请妥善保管密码" />
          <p className="secret" data-testid="initial-password">
            {password}
          </p>
        </Dialog>
      )}
    </>
  );
}
function AccessoryList({
  items,
  actions,
}: {
  items: api.NativeAccessory[];
  actions?: (object: api.NativeAccessory) => import("react").ReactNode;
}) {
  return (
    <BusinessTable
      rowKey="objectNo"
      rows={items}
      columns={[
        {
          colKey: "kind",
          title: "类型",
          cell: ({ row }) =>
            ({
              snapshot: "快照",
              backup: "备份",
              disk: "磁盘",
              security_group: "安全组",
              ip: "IP 地址",
              nat: "端口转发",
              virtual_media: "镜像",
            })[row.kind] || "资源",
        },
        {
          colKey: "name",
          title: "名称",
          cell: ({ row }) => row.name || "未命名",
        },
        ...(items.some((item) => item.kind === "nat")
          ? [
              {
                colKey: "externalPort",
                title: "外部端口",
                cell: ({ row }: { row: api.NativeAccessory }) =>
                  scalarPort(row.projection.external_port),
              },
              {
                colKey: "internalPort",
                title: "服务器端口",
                cell: ({ row }: { row: api.NativeAccessory }) =>
                  scalarPort(row.projection.internal_port),
              },
              {
                colKey: "protocol",
                title: "协议",
                cell: ({ row }: { row: api.NativeAccessory }) =>
                  (
                    ({
                      tcp: "TCP",
                      udp: "UDP",
                      tcp_udp: "TCP 与 UDP",
                    }) as Record<string, string>
                  )[String(row.projection.protocol)] || "尚未确认",
              },
            ]
          : []),
        {
          colKey: "state",
          title: "状态",
          cell: ({ row }) =>
            ({
              available: "可用",
              attached: "已关联",
              active: "使用中",
              mounted: "已挂载",
              unavailable: "不可用",
            })[row.state] ||
            (["snapshot", "backup"].includes(row.kind)
              ? "就绪状态未确认"
              : "尚未确认"),
        },
        ...(actions
          ? [
              {
                colKey: "actions",
                title: "操作",
                cell: ({ row }: { row: api.NativeAccessory }) => actions(row),
              },
            ]
          : []),
      ]}
    />
  );
}

function scalarPort(value: unknown) {
  const text = String(value ?? "");
  return /^\d+$/.test(text) && Number(text) > 0 && Number(text) <= 65535
    ? text
    : "尚未确认";
}

function trafficAmount(value: unknown) {
  if (typeof value === "number" && Number.isSafeInteger(value) && value >= 0)
    return `${value.toLocaleString("zh-CN")} GB`;
  if (typeof value === "string" && /^\d{1,20}$/.test(value))
    return `${BigInt(value).toLocaleString("zh-CN")} GB`;
  return "尚未确认";
}

import { useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Collapse,
  Form,
  Input,
  InputNumber,
  Select,
} from "tdesign-react";
import Dialog from "#/adapter/Dialog";
import * as api from "#/api/idc/native-servers";
import { DetailList, useAction, ErrorNote } from "#/components/business";
export default function OperationDialog({
  item,
  code,
  title,
  options,
  onClose,
  onCreated,
  objectNo,
}: {
  item: api.NativeServerDetail;
  code: string;
  objectNo?: string;
  title: string;
  options?: api.NativeOptions;
  onClose: () => void;
  onCreated: (op: api.NativeOperation) => void;
}) {
  const [params, setParams] = useState<api.NativeParameters>(
      code === "stop" || code === "reboot"
        ? { mode: "graceful" }
        : objectNo
          ? { objectNo }
          : {},
    ),
    [password, setPassword] = useState(""),
    [confirmPassword, setConfirmPassword] = useState("");
  const [prepared, setPrepared] = useState<api.NativeConfirmation>(),
    [key] = useState(() => crypto.randomUUID());
  const [submitted, setSubmitted] = useState(false);
  const action = useAction();
  const ordinaryPower =
    (code === "stop" || code === "reboot") && params.mode !== "hard";
  const update = (name: string, value: string | number | boolean | undefined) =>
    setParams((before) => {
      const next = { ...before };
      if (value === undefined || value === "") delete next[name];
      else next[name] = value;
      return next;
    });
  const choices =
    code === "security_group_attach"
      ? options?.accessories.filter(
          (o) => o.kind === "security_group" && o.state === "available",
        )
      : code === "nat_delete"
        ? options?.accessories.filter((o) => o.kind === "nat")
        : code === "virtual_media_mount"
          ? options?.accessories.filter(
              (o) => o.kind === "virtual_media" && o.state === "available",
            )
          : /^(snapshot|backup)_(restore|delete)$/.test(code)
            ? options?.accessories.filter((o) => o.kind === code.split("_")[0])
            : [];
  const risk =
    code === "reinstall"
      ? "重装将清除系统盘数据，系统密码也会重新生成。"
      : code === "enter_rescue" || code === "exit_rescue"
        ? "救援模式将切换服务器启动环境并中断业务。请先确认需要保留的数据已备份。"
        : code === "reset_password"
          ? "新密码生效后，请使用新密码登录服务器。"
          : code === "stop" || code === "reboot"
            ? params.mode === "hard"
              ? "强制操作可能导致尚未写入的数据丢失。"
              : "操作会中断服务器上的连接和正在运行的业务。"
            : code.endsWith("_restore")
              ? "恢复会覆盖所选磁盘的当前数据，且服务器必须处于关机状态。请先保存需要保留的数据。恢复不收费。"
              : code.endsWith("_delete") && /snapshot|backup/.test(code)
                ? "删除后无法恢复。付费对象会停止后续续费，已开始的计费周期不退款。删除不收费。"
                : "请确认操作对象和设置后继续。";
  return (
    <Dialog
      visible
      header={prepared && !ordinaryPower ? `确认${title}` : title}
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
            theme={code === "reinstall" ? "danger" : "primary"}
            onClick={() =>
              void action.run(async () => {
                if (!prepared) {
                  if (code === "reinstall" && !params.os)
                    throw new Error("请选择需要安装的系统");
                  if (
                    ["reset_password", "enter_rescue"].includes(code) &&
                    (!password || password !== confirmPassword)
                  )
                    throw new Error("请填写并确认相同的新密码");
                  if (
                    choices &&
                    [
                      "security_group_attach",
                      "nat_delete",
                      "virtual_media_mount",
                      "snapshot_restore",
                      "snapshot_delete",
                      "backup_restore",
                      "backup_delete",
                    ].includes(code) &&
                    !params.objectNo
                  )
                    throw new Error("请选择操作对象");
                  if (
                    code.endsWith("_restore") &&
                    params.confirmedServiceNo !== item.serverNo
                  )
                    throw new Error("请输入页面显示的完整服务器编号");
                  if (
                    code === "nat_create" &&
                    (!String(params.name || "").trim() ||
                      !["tcp", "udp", "tcp_udp"].includes(
                        String(params.protocol),
                      ) ||
                      !Number.isInteger(params.externalPort) ||
                      Number(params.externalPort) < 1 ||
                      Number(params.externalPort) > 65535 ||
                      !Number.isInteger(params.internalPort) ||
                      Number(params.internalPort) < 1 ||
                      Number(params.internalPort) > 65535)
                  )
                    throw new Error(
                      "请填写名称、协议及正确的外部端口和服务器端口",
                    );
                  if (
                    [
                      "security_group_attach",
                      "nat_delete",
                      "virtual_media_mount",
                      "snapshot_restore",
                      "snapshot_delete",
                      "backup_restore",
                      "backup_delete",
                    ].includes(code) &&
                    options?.accessoriesExpiresAt &&
                    options.accessoriesExpiresAt <= Date.now()
                  )
                    throw new Error("管理选项已过期，请关闭窗口并刷新管理选项");
                  if (
                    code === "enter_rescue" &&
                    !["linux", "windows"].includes(String(params.system))
                  )
                    throw new Error("请选择救援系统");
                  const result = await api.prepareNativeOperation(
                    item.serverNo,
                    code,
                    params,
                    key,
                  );
                  if (
                    typeof result.required !== "boolean" ||
                    typeof result.proofToken !== "string" ||
                    (result.required && !result.proofToken)
                  )
                    throw new Error("暂时无法确认操作，请稍后重试");
                  setPrepared(result);
                  if (
                    (code === "stop" || code === "reboot") &&
                    params.mode !== "hard"
                  ) {
                    setSubmitted(true);
                    const created = await api.createNativeOperation(
                      item.serverNo,
                      code,
                      key,
                      true,
                      params,
                      result.proofToken,
                    );
                    onCreated(created.operation);
                    return;
                  }
                  return;
                }
                if (prepared.expiresAt && prepared.expiresAt <= Date.now()) {
                  setPrepared(undefined);
                  throw new Error("确认已过期，请重新确认操作");
                }
                setSubmitted(true);
                const result = await api.createNativeOperation(
                  item.serverNo,
                  code,
                  key,
                  true,
                  params,
                  prepared.proofToken,
                  ["reset_password", "enter_rescue"].includes(code)
                    ? password
                    : undefined,
                );
                setPassword("");
                setConfirmPassword("");
                onCreated(result.operation);
              })
            }
          >
            {action.busy
              ? prepared || ordinaryPower
                ? "正在提交"
                : "正在核对"
              : prepared
                ? submitted
                  ? "重试本次提交"
                  : "确认操作"
                : ordinaryPower
                  ? submitted
                    ? "重试本次提交"
                    : `确认${title}`
                  : "下一步"}
          </Button>
        </>
      }
    >
      <Alert
        theme={
          code === "reinstall" || params.mode === "hard" ? "warning" : "info"
        }
        message={risk}
      />
      <p className="muted">
        {item.information?.hostname || item.productNameZhCn} · {item.serverNo}
      </p>
      {prepared ? (
        <DetailList
          items={[
            ["操作", title],
            ["目标服务器", item.information?.hostname || item.serverNo],
            ...Object.entries(params).map(([key, value]): [string, string] => {
              const labels: Record<string, string> = {
                mode: "操作方式",
                system: "救援系统",
                os: "安装系统",
                objectNo: "操作对象",
                port: "登录端口",
                system_disk_size: "系统盘大小（GiB）",
                format_data_disk: "清空数据盘",
                name: "名称",
                protocol: "协议",
                externalPort: "外部端口",
                internalPort: "服务器端口",
                confirmedServiceNo: "确认服务器编号",
              };
              const display =
                key === "mode"
                  ? value === "hard"
                    ? "强制操作"
                    : "正常操作"
                  : key === "system"
                    ? value === "windows"
                      ? "Windows"
                      : "Linux"
                    : key === "os"
                      ? options?.images.find((i) => i.reference === value)
                          ?.name || "尚未确认"
                      : key === "objectNo"
                        ? choices?.find((o) => o.objectNo === value)?.name ||
                          "尚未确认"
                        : key === "protocol"
                          ? (
                              {
                                tcp: "TCP",
                                udp: "UDP",
                                tcp_udp: "TCP 与 UDP",
                              } as Record<string, string>
                            )[String(value)] || "尚未确认"
                          : key === "format_data_disk"
                            ? value === 1
                              ? "是"
                              : "否"
                            : String(value);
              return [labels[key] || "设置", display];
            }),
          ]}
        />
      ) : (
        <Form
          disabled={action.busy}
          labelAlign="top"
          className="server-operation-form"
        >
          {code === "enter_rescue" && (
            <Form.FormItem label="救援系统">
              <Select
                value={(params.system as string) || ""}
                placeholder="选择救援系统"
                options={[
                  { value: "linux", label: "Linux" },
                  { value: "windows", label: "Windows" },
                ]}
                onChange={(v) => update("system", String(v))}
              />
            </Form.FormItem>
          )}
          {(code === "stop" || code === "reboot") && (
            <Form.FormItem label="操作方式">
              <Select
                value={params.mode as string}
                options={[
                  { value: "graceful", label: "正常操作" },
                  { value: "hard", label: "强制操作" },
                ]}
                onChange={(v) => update("mode", String(v))}
              />
            </Form.FormItem>
          )}
          {["reset_password", "enter_rescue"].includes(code) && (
            <>
              <Form.FormItem
                label={code === "enter_rescue" ? "救援临时密码" : "新密码"}
              >
                <Input
                  type="password"
                  value={password}
                  onChange={setPassword}
                  autocomplete="new-password"
                  maxlength={1024}
                />
              </Form.FormItem>
              <Form.FormItem
                label={
                  code === "enter_rescue" ? "确认救援临时密码" : "确认新密码"
                }
              >
                <Input
                  type="password"
                  value={confirmPassword}
                  onChange={setConfirmPassword}
                  autocomplete="new-password"
                  maxlength={1024}
                />
              </Form.FormItem>
            </>
          )}
          {code === "reinstall" && (
            <>
              <Form.FormItem label="安装系统">
                <Select
                  filterable
                  clearable
                  value={(params.os as string) || ""}
                  placeholder="选择需要安装的系统"
                  options={
                    options?.images.map((i) => ({
                      value: i.reference,
                      label: [i.group, i.name, i.osType, i.arch, i.bootMode]
                        .filter(Boolean)
                        .join(" · "),
                      disabled: !i.selectable,
                    })) || []
                  }
                  onChange={(v) => update("os", String(v))}
                />
              </Form.FormItem>
              {!options?.images.length && (
                <p className="muted">
                  暂未获取可安装系统，请稍后刷新或提交工单。
                </p>
              )}
              <Collapse>
                <Collapse.Panel header="高级设置（选填）" value="advanced">
                  <Form.FormItem label="登录端口">
                    <InputNumber
                      value={params.port ? Number(params.port) : undefined}
                      min={1}
                      max={65535}
                      decimalPlaces={0}
                      placeholder="默认"
                      onChange={(v) =>
                        update("port", v === undefined ? undefined : Number(v))
                      }
                    />
                  </Form.FormItem>
                  <Form.FormItem label="系统盘大小（GiB）">
                    <InputNumber
                      value={
                        params.system_disk_size
                          ? Number(params.system_disk_size)
                          : undefined
                      }
                      min={1}
                      max={1048576}
                      decimalPlaces={0}
                      placeholder="默认"
                      onChange={(v) =>
                        update(
                          "system_disk_size",
                          v === undefined ? undefined : Number(v),
                        )
                      }
                    />
                  </Form.FormItem>
                  <Checkbox
                    checked={params.format_data_disk === 1}
                    onChange={(v) =>
                      update("format_data_disk", v ? 1 : undefined)
                    }
                  >
                    同时清空数据盘
                  </Checkbox>
                </Collapse.Panel>
              </Collapse>
            </>
          )}
          {[
            "security_group_attach",
            "nat_delete",
            "virtual_media_mount",
            "snapshot_restore",
            "snapshot_delete",
            "backup_restore",
            "backup_delete",
          ].includes(code) && (
            <Form.FormItem label="操作对象">
              <Select
                filterable
                value={(params.objectNo as string) || ""}
                placeholder="选择操作对象"
                options={
                  choices?.map((o) => ({
                    value: o.objectNo,
                    label: `${o.name || o.objectNo}${code.endsWith("_restore") && o.state !== "available" ? " · 状态未确认" : ""}`,
                    disabled: code.endsWith("_restore") && o.state !== "available",
                  })) || []
                }
                onChange={(v) => update("objectNo", String(v))}
              />
            </Form.FormItem>
          )}
          {code.endsWith("_restore") && (
            <Form.FormItem label={`输入服务器编号：${item.serverNo}`}>
              <Input
                value={String(params.confirmedServiceNo || "")}
                onChange={(v) => update("confirmedServiceNo", v)}
                placeholder="完整输入上方服务器编号"
                autocomplete="off"
              />
            </Form.FormItem>
          )}
          {code === "nat_create" && (
            <>
              <Form.FormItem label="名称">
                <Input
                  placeholder="例如：网站访问"
                  value={(params.name as string) || ""}
                  onChange={(v) => update("name", v)}
                  maxlength={128}
                />
              </Form.FormItem>
              <Form.FormItem label="协议">
                <Select
                  value={(params.protocol as string) || ""}
                  placeholder="选择协议"
                  options={[
                    { value: "tcp", label: "TCP" },
                    { value: "udp", label: "UDP" },
                    { value: "tcp_udp", label: "TCP 与 UDP" },
                  ]}
                  onChange={(v) => update("protocol", String(v))}
                />
              </Form.FormItem>
              {[
                ["externalPort", "外部端口"],
                ["internalPort", "服务器端口"],
              ].map(([key, label]) => (
                <Form.FormItem key={key} label={label}>
                  <InputNumber
                    placeholder={label}
                    min={1}
                    max={65535}
                    decimalPlaces={0}
                    value={params[key] ? Number(params[key]) : undefined}
                    onChange={(v) =>
                      update(key, v === undefined ? undefined : Number(v))
                    }
                  />
                </Form.FormItem>
              ))}
            </>
          )}
        </Form>
      )}
      <ErrorNote error={action.error} />
    </Dialog>
  );
}

import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Collapse,
  Form,
  Input,
  Select,
  Textarea,
  TreeSelect,
  Cascader,
  DatePicker,
} from "tdesign-react";
import Dialog from "#/adapter/Dialog";
import * as config from "#/api/idc/server-configuration";
import type { NativeServerDetail } from "#/api/idc/native-servers";
import {
  ErrorNote,
  FetchState,
  useAction,
  useLoad,
} from "#/components/business";
import ServerPurchaseDialog from "./ServerPurchaseDialog";
export default function ServerResizeDialog(props: {
  item: NativeServerDetail;
  onClose: () => void;
  onCreated: () => void;
}) {
  const data = useLoad(
    () => config.getServerConfiguration(props.item.serverNo),
    [props.item.serverNo],
  );
  return data.data ? (
    <ResizeEditor
      key={data.data.configurationRevision}
      initial={data.data}
      {...props}
    />
  ) : (
    <Dialog visible header="调整配置" footer={null} onClose={props.onClose}>
      <FetchState state={data}>{null}</FetchState>
    </Dialog>
  );
}
function ResizeEditor({
  initial,
  item,
  onClose,
  onCreated,
}: {
  initial: config.ConfigurationContext;
  item: NativeServerDetail;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [values, setValues] = useState(initial.values),
    [view, setView] = useState(initial),
    [validated, setValidated] = useState<config.ConfigurationContext>(initial),
    [edited, setEdited] = useState(false),
    [checking, setChecking] = useState(false),
    [error, setError] = useState("");
  const [target, setTarget] = useState<string>();
  const action = useAction();
  useEffect(() => {
    if (!edited) return;
    let alive = true;
    setChecking(true);
    setError("");
    const timer = setTimeout(() => {
      config
        .getServerConfiguration(
          item.serverNo,
          initial.configurationRevision,
          values,
        )
        .then((result) => {
          if (alive) {
            setView(result);
            setValidated(result);
          }
        })
        .catch((e) => {
          if (alive)
            setError(
              e instanceof Error ? e.message : "配置核对未完成，请稍后重试",
            );
        })
        .finally(() => {
          if (alive) setChecking(false);
        });
    }, 250);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [values, item.serverNo, initial.configurationRevision, edited]);
  const update = (value: config.ConfigurationValue) => {
    setEdited(true);
    setChecking(true);
    setValues((before) => [
      ...before.filter((v) => v.fieldToken !== value.fieldToken),
      value,
    ]);
  };
  type Choice = { label: string; value: string; children?: Choice[] };
  const options = (items: config.ConfigurationOption[] = []): Choice[] =>
    items.map((o) => ({
      label: o.labelZhCN || o.labelEnUS,
      value: o.optionToken,
      ...(o.children?.length ? { children: options(o.children) } : {}),
    }));
  const field = (f: config.ConfigurationField) => {
    const value = values.find((v) => v.fieldToken === f.fieldToken) || {
      fieldToken: f.fieldToken,
      type: f.type,
    };
    const set = (arm: keyof config.ConfigurationValue, next: unknown) =>
      update({ fieldToken: f.fieldToken, type: f.type, [arm]: next });
    let control;
    if (f.type === "boolean")
      control = (
        <Checkbox
          checked={value.booleanValue === true}
          onChange={(v) => set("booleanValue", v)}
        >
          启用
        </Checkbox>
      );
    else if (["single_select", "multi_select"].includes(f.type))
      control = (
        <Select
          filterable
          clearable
          multiple={f.type === "multi_select"}
          value={
            f.type === "multi_select"
              ? value.optionTokens || []
              : value.optionTokens?.[0] || ""
          }
          options={options(f.options)}
          onChange={(v) =>
            set("optionTokens", Array.isArray(v) ? v : v ? [String(v)] : [])
          }
        />
      );
    else if (f.type === "tree_select")
      control = (
        <TreeSelect
          multiple
          value={value.treeOptionTokens || []}
          data={options(f.options)}
          onChange={(v) => set("treeOptionTokens", v)}
        />
      );
    else if (f.type === "address")
      control = (
        <Cascader
          checkStrictly={false}
          valueType="full"
          value={value.addressOptionTokens || []}
          options={options(f.options)}
          onChange={(v) => set("addressOptionTokens", v)}
        />
      );
    else if (f.type === "date")
      control = (
        <DatePicker
          value={value.dateValue || ""}
          format="YYYY-MM-DD"
          onChange={(v) => set("dateValue", v)}
        />
      );
    else if (f.type === "datetime")
      control = (
        <DatePicker
          enableTimePicker
          value={value.dateTimeValue || ""}
          onChange={(v) => {
            const date = new Date(String(v));
            set(
              "dateTimeValue",
              Number.isFinite(date.getTime()) ? date.toISOString() : "",
            );
          }}
        />
      );
    else if (f.type === "multi_line")
      control = (
        <Textarea
          value={value.textValue || ""}
          maxlength={f.maximumLength}
          onChange={(v) => set("textValue", v)}
        />
      );
    else if (f.type === "file")
      control = <p className="muted">此配置需要商家协助，请提交工单。</p>;
    else if (["integer", "decimal"].includes(f.type)) {
      const arm = f.type === "integer" ? "integerValue" : "decimalValue";
      control = f.numericChoices?.length ? (
        <Select
          value={value[arm] || ""}
          options={f.numericChoices.map((n) => ({
            value: n,
            label: `${f.numericLabels?.[n]?.zhCN || f.numericLabels?.[n]?.enUS || n}${f.unit ? ` ${f.unit}` : ""}`,
          }))}
          onChange={(v) => set(arm, String(v))}
        />
      ) : (
        <Input
          value={value[arm] || ""}
          suffix={f.unit}
          onChange={(v) => set(arm, v)}
        />
      );
    } else if (f.type === "single_line")
      control = (
        <Input
          value={value.textValue || ""}
          maxlength={f.maximumLength}
          onChange={(v) => set("textValue", v)}
        />
      );
    else control = <p className="muted">暂时无法编辑此配置，请联系商家。</p>;
    const issue = view.issues.some((i) => i.fieldToken === f.fieldToken);
    return (
      <Form.FormItem
        key={f.fieldToken}
        label={f.labelZhCN || f.labelEnUS}
        requiredMark={f.required}
        help={
          issue
            ? "请检查此项配置，当前选择不符合要求。"
            : f.descriptionZhCN || f.descriptionEnUS
        }
      >
        <fieldset
          disabled={action.busy || f.disabled}
          {...(action.busy || f.disabled ? { inert: "" } : {})}
          className="configuration-field"
        >
          {control}
        </fieldset>
      </Form.FormItem>
    );
  };
  if (target)
    return (
      <ServerPurchaseDialog
        item={item}
        code="resize"
        title="确认配置调整"
        targetQuoteNo={target}
        onClose={onClose}
        onCreated={onCreated}
      />
    );
  const visible = view.fields.filter((f) => f.visible);
  return (
    <Dialog
      visible
      header="调整配置"
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
            取消
          </Button>
          <Button
            tag="button"
            loading={action.busy}
            disabled={
              !edited ||
              checking ||
              !!error ||
              !validated.ready ||
              !validated.billingTermToken ||
              view.issues.length > 0
            }
            onClick={() =>
              void action.run(async () => {
                const result = await config.freezeServerConfiguration(
                  item.serverNo,
                  validated,
                );
                if (!result.quoteNo)
                  throw new Error("暂时无法确认目标配置，请重试");
                setTarget(result.quoteNo);
              })
            }
          >
            {checking ? "正在核对配置" : "获取变更报价"}
          </Button>
        </>
      }
    >
      <Alert
        theme="info"
        message="按当前服务的计费周期计算变更费用。升级需要先支付剩余周期差价；磁盘缩容需联系商家处理。"
      />
      <Form disabled={action.busy} labelAlign="top">
        {visible.filter((f) => !f.advanced).map(field)}
        {visible.some((f) => f.advanced) && (
          <Collapse>
            <Collapse.Panel header="高级设置" value="advanced">
              {visible.filter((f) => f.advanced).map(field)}
            </Collapse.Panel>
          </Collapse>
        )}
      </Form>
      {view.issues.some((i) => !i.fieldToken) && (
        <p className="error-note">
          当前配置尚不符合要求，请检查配置或联系商家。
        </p>
      )}
      <ErrorNote error={error || action.error} />
    </Dialog>
  );
}

import { useEffect, useRef, useState } from "react";
import {
  Button,
  Card,
  Form,
  Input,
  Rate,
  Select,
  Textarea,
  Upload,
  Checkbox,
  DatePicker,
  Alert,
  Steps,
} from "tdesign-react";
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import { getCustomerServiceNote } from "#/api/idc/product-directories";
import * as support from "#/api/idc/support";
import {
  BusinessTable,
  Confirm,
  ErrorNote,
  FetchState,
  Page,
  Status,
  date,
  useAction,
  useLoad,
} from "#/components/business";
export default function Support() {
  const [page, setPage] = useState(1),
    [size, setSize] = useState(20),
    [query, setQuery] = useState(""),
    [search, setSearch] = useState("");
  const data = useLoad(
    () => support.listCustomerTickets({ page, limit: size, query: search }),
    [page, size, search],
    "Support:Support:data",
  );
  return (
    <Page
      title="我的工单"
      description="遇到问题可以提交工单，我们会在这里与您沟通。"
      actions={
        <Link to="/account/support/new">
          <Button tag="button">提交工单</Button>
        </Link>
      }
    >
      <Card>
        <div className="table-toolbar">
          <Input
            aria-label="搜索工单"
            value={query}
            onChange={setQuery}
            placeholder="搜索问题标题"
            style={{ width: 280 }}
            onEnter={() => {
              setPage(1);
              setSearch(query);
            }}
          />
          <Button
            tag="button"
            variant="outline"
            onClick={() => {
              setPage(1);
              setSearch(query);
            }}
          >
            搜索
          </Button>
        </div>
        <FetchState state={data}>
          <BusinessTable
            rowKey="ticketNo"
            rows={data.data?.items || []}
            total={data.data?.total}
            page={page}
            size={size}
            onPage={(p, s) => {
              setPage(p);
              setSize(s);
            }}
            columns={[
              {
                colKey: "subject",
                title: "问题",
                cell: ({ row }) => (
                  <Link to={`/account/support/${row.ticketNo}`}>
                    {row.subject}
                    <small className="subline">{row.ticketNo}</small>
                  </Link>
                ),
              },
              {
                colKey: "category",
                title: "问题类型",
                cell: ({ row }) =>
                  row.category?.nameZhCN || row.category?.nameEnUS || "—",
              },
              {
                colKey: "stage",
                title: "状态",
                cell: ({ row }) => <Status value={row.stage} />,
              },
              {
                colKey: "lastActivityAt",
                title: "最近更新",
                cell: ({ row }) => date(row.lastActivityAt),
              },
              {
                colKey: "actions",
                title: "操作",
                fixed: "right",
                width: 100,
                cell: ({ row }) => (
                  <Link to={`/account/support/${row.ticketNo}`}>查看</Link>
                ),
              },
            ]}
          />
        </FetchState>
      </Card>
    </Page>
  );
}
function AttachmentInput({
  files,
  setFiles,
  disabled = false,
}: {
  files: File[];
  setFiles: (files: File[]) => void;
  disabled?: boolean;
}) {
  return (
    <div className="ticket-attachments">
      <div className="field-label">附件（选填）</div>
      <Upload
        theme="file"
        multiple
        max={5}
        autoUpload={false}
        uploadButton={null}
        disabled={disabled}
        files={files.map((raw) => ({
          name: raw.name,
          size: raw.size,
          raw,
          status: "waiting" as const,
        }))}
        onChange={(items) =>
          setFiles(items.flatMap((item) => (item.raw ? [item.raw] : [])))
        }
        tips="最多选择 5 个文件，提交工单时一并上传。请勿上传密码或密钥。"
      />
    </div>
  );
}
function SupportChoice({
  kind,
  disabled = false,
  value,
  onChange,
}: {
  kind: "category" | "service";
  disabled?: boolean;
  value?: string;
  onChange?: (value: string) => void;
}) {
  type Option = { label: string; value: string };
  const [keyword, setKeyword] = useState("");
  const [options, setOptions] = useState<Option[]>([]),
    [selected, setSelected] = useState<Option>();
  const [nextCursor, setNextCursor] = useState(""),
    [loading, setLoading] = useState(false),
    [error, setError] = useState("");
  const generation = useRef(0),
    busy = useRef(false);
  async function load(cursor: string, search: string, version: number) {
    if (busy.current || version !== generation.current) return;
    busy.current = true;
    setLoading(true);
    setError("");
    try {
      const params = { keyword: search, cursor, limit: 50 };
      const result =
        kind === "category"
          ? await support.listCustomerTicketCategories(params)
          : await support.listCustomerSupportEntitlementCandidates(params);
      const rows = result.items.map((item) => ({
        label: item.nameZhCN || item.nameEnUS,
        value:
          "categoryNo" in item ? item.categoryNo : item.associationReference,
      }));
      if (version !== generation.current) return;
      setOptions((previous) =>
        cursor
          ? [
              ...new Map(
                [...previous, ...rows].map((item) => [item.value, item]),
              ).values(),
            ]
          : rows,
      );
      setNextCursor(result.nextCursor || "");
    } catch (e) {
      if (version === generation.current)
        setError(e instanceof Error ? e.message : "选项加载失败，请重试");
    } finally {
      if (version === generation.current) {
        busy.current = false;
        setLoading(false);
      }
    }
  }
  useEffect(() => {
    const version = ++generation.current;
    busy.current = false;
    setNextCursor("");
    setOptions([]);
    const timer = setTimeout(
      () => void load("", keyword, version),
      keyword ? 250 : 0,
    );
    return () => {
      clearTimeout(timer);
      generation.current++;
      busy.current = false;
    };
  }, [kind, keyword]);
  useEffect(() => {
    if (kind !== "service" || !value || selected?.value === value) return;
    let current = true;
    void support
      .listCustomerSupportEntitlementCandidates({
        associationReference: value,
        limit: 1,
      })
      .then((result) => {
        if (current) {
          const item = result.items.find(
            (item) => item.associationReference === value,
          );
          if (item)
            setSelected({
              label: item.nameZhCN || item.nameEnUS,
              value: item.associationReference,
            });
        }
      })
      .catch(() => {
        if (current) setError("关联服务暂时无法加载，请在下拉框中重新选择");
      });
    return () => {
      current = false;
    };
  }, [kind, value]);
  const shown =
    selected && !options.some((item) => item.value === selected.value)
      ? [selected, ...options]
      : options;
  return (
    <Select
      filterable
      filter={() => true}
      disabled={disabled}
      clearable={kind === "service"}
      value={value}
      options={shown}
      placeholder={
        kind === "service" ? "选择或搜索相关服务" : "选择或搜索问题类型"
      }
      loading={loading}
      onSearch={setKeyword}
      onChange={(v) => {
        const next = String(v || "");
        setSelected(shown.find((item) => item.value === next));
        onChange?.(next);
      }}
      popupProps={{
        onScrollToBottom: () => {
          if (nextCursor) void load(nextCursor, keyword, generation.current);
        },
      }}
      empty={error ? "暂时无法加载选项" : "暂无匹配结果"}
      panelBottomContent={
        error ? (
          <Button
            tag="button"
            variant="text"
            onClick={() => void load(nextCursor, keyword, generation.current)}
          >
            加载失败，点击重试
          </Button>
        ) : nextCursor ? (
          <div className="select-loading-hint">向下滚动加载更多</div>
        ) : undefined
      }
    />
  );
}
export function NewTicket() {
  const navigate = useNavigate(),
    action = useAction();
  const [form] = Form.useForm();
  const [params] = useSearchParams();
  const requestedService = params.get("service") || "";
  const [categorySelected, setCategorySelected] = useState(false);
  const related = useLoad(
    () =>
      requestedService
        ? getCustomerServiceNote(requestedService)
        : Promise.resolve(undefined),
    [requestedService],
    "Support:related-service",
  );
  useEffect(() => {
    form.setFieldsValue({ associationReference: undefined });
  }, [requestedService, form]);
  useEffect(() => {
    if (related.data?.supportAssociationReference)
      form.setFieldsValue({
        associationReference: related.data.supportAssociationReference,
      });
  }, [related.data, form]);
  const [files, setFiles] = useState<File[]>([]),
    [values, setValues] = useState<Record<string, string | boolean | string[]>>(
      {},
    );
  const settings = useLoad(
    support.listCustomerSupportCustomFields,
    [],
    "Support:NewTicket:settings",
  );
  return (
    <Page
      title="提交工单"
      description="描述您遇到的问题，提供必要的信息，帮助我们更快处理。"
      actions={<Link to="/account/support">返回工单列表</Link>}
    >
      <Alert
        theme="info"
        message="请根据问题选择类型并说明情况。已经提交过的问题，请在原工单继续回复。"
      />
      <div className="ticket-progress">
        <Steps current={categorySelected ? 1 : 0}>
          <Steps.StepItem title="选择问题类型" />
          <Steps.StepItem title="填写问题" />
          <Steps.StepItem title="等待回复" />
        </Steps>
      </div>
      {related.error && (
        <Alert
          theme="warning"
          message="关联服务暂时无法加载，请在下拉框中选择您要咨询的服务。"
        />
      )}
      <div className="ticket-compose">
        <Card className="form-card ticket-form">
          <FetchState state={settings}>
            <Form
              form={form}
              labelAlign="top"
              onValuesChange={(changed) => {
                if ("categoryNo" in changed)
                  setCategorySelected(!!changed.categoryNo);
              }}
              onSubmit={({ validateResult, fields }) => {
                if (validateResult !== true || related.loading) return;
                void action.run(async () => {
                  const customFields: support.CustomerTicketCustomFieldSubmission[] =
                    (settings.data?.items || []).map((field) => {
                      const value = values[field.fieldNo];
                      if (
                        field.required &&
                        (value === undefined ||
                          value === "" ||
                          (Array.isArray(value) && !value.length))
                      )
                        throw new Error(
                          `请填写${field.nameZhCN || field.nameEnUS}`,
                        );
                      return {
                        fieldNo: field.fieldNo,
                        ...(field.fieldType === "boolean"
                          ? { booleanValue: !!value }
                          : field.fieldType === "number"
                            ? { numberValue: String(value ?? "") }
                            : field.fieldType === "date"
                              ? { dateValue: String(value ?? "") }
                              : ["single_select", "multi_select"].includes(
                                    field.fieldType,
                                  )
                                ? {
                                    optionNos: Array.isArray(value)
                                      ? value
                                      : value
                                        ? [String(value)]
                                        : [],
                                  }
                                : { textValue: String(value ?? "") }),
                      };
                    });
                  const uploadNos =
                    await support.uploadInitialCustomerTicketAttachments(files);
                  const ticket = await support.createCustomerTicket({
                    subject: String(fields.subject),
                    body: String(fields.body),
                    categoryNo: String(fields.categoryNo),
                    associationReference: fields.associationReference
                      ? String(fields.associationReference)
                      : undefined,
                    uploadNos,
                    customFields,
                  });
                  navigate(`/account/support/${ticket.ticketNo}`);
                });
              }}
            >
              <div className="ticket-choice-grid">
                <Form.FormItem
                  name="categoryNo"
                  label="问题类型"
                  rules={[{ required: true, message: "请选择问题类型" }]}
                >
                  <SupportChoice kind="category" />
                </Form.FormItem>
                <Form.FormItem
                  name="associationReference"
                  label="相关服务（选填）"
                >
                  <SupportChoice
                    kind="service"
                    disabled={related.loading || action.busy}
                  />
                </Form.FormItem>
              </div>
              <Form.FormItem
                name="subject"
                label="问题标题"
                rules={[{ required: true, message: "请填写问题标题" }]}
              >
                <Input maxlength={200} placeholder="用一句话描述问题" />
              </Form.FormItem>
              <Form.FormItem
                name="body"
                label="详细描述"
                rules={[{ required: true, message: "请描述您遇到的问题" }]}
              >
                <Textarea
                  autosize={{ minRows: 5, maxRows: 12 }}
                  placeholder="例如：什么时候出现问题、已经尝试过哪些操作。请勿填写密码。"
                />
              </Form.FormItem>
              {settings.data?.items.map((field) => (
                <label key={field.fieldNo} className="field">
                  {field.nameZhCN || field.nameEnUS}
                  {field.required ? " *" : ""}
                  {["single_select", "multi_select"].includes(
                    field.fieldType,
                  ) ? (
                    <Select
                      multiple={field.fieldType === "multi_select"}
                      value={(values[field.fieldNo] as string | string[]) || ""}
                      options={field.options.map((v) => ({
                        label: v.nameZhCN || v.nameEnUS,
                        value: v.optionNo,
                      }))}
                      onChange={(v) =>
                        setValues({
                          ...values,
                          [field.fieldNo]: v as string | string[],
                        })
                      }
                    />
                  ) : field.fieldType === "boolean" ? (
                    <Checkbox
                      checked={!!values[field.fieldNo]}
                      onChange={(checked) =>
                        setValues({
                          ...values,
                          [field.fieldNo]: checked,
                        })
                      }
                    />
                  ) : field.fieldType === "date" ? (
                    <DatePicker
                      value={String(values[field.fieldNo] ?? "")}
                      onChange={(value) =>
                        setValues({
                          ...values,
                          [field.fieldNo]: String(value || ""),
                        })
                      }
                    />
                  ) : (
                    <Input
                      type={field.fieldType === "number" ? "number" : "text"}
                      value={String(values[field.fieldNo] ?? "")}
                      onChange={(value) =>
                        setValues({ ...values, [field.fieldNo]: value })
                      }
                    />
                  )}
                  <small>
                    {field.descriptionZhCN || field.descriptionEnUS}
                  </small>
                </label>
              ))}
              <AttachmentInput
                files={files}
                setFiles={setFiles}
                disabled={action.busy}
              />
              <ErrorNote error={action.error} />
              <div className="form-footer">
                <Button
                  tag="button"
                  type="submit"
                  loading={action.busy}
                  disabled={related.loading}
                >
                  提交工单
                </Button>
                <Link to="/account/support">取消</Link>
              </div>
            </Form>
          </FetchState>
        </Card>
        <aside className="ticket-guidance">
          <Card title="提交前请确认">
            <ol>
              <li>选择对应的问题类型，方便客服处理。</li>
              <li>服务使用问题请选择相关资源。</li>
              <li>写清出现时间、错误提示和已尝试的操作。</li>
              <li>可以附上截图，请遮住密码、密钥等信息。</li>
            </ol>
            <p>提交后，您可以在工单详情查看回复并补充说明。</p>
          </Card>
        </aside>
      </div>
    </Page>
  );
}
export function TicketDetail() {
  const { ticketNo = "" } = useParams();
  const data = useLoad(
      () => support.getCustomerTicket(ticketNo),
      [ticketNo],
      "Support:TicketDetail:data",
    ),
    action = useAction();
  const [body, setBody] = useState(""),
    [files, setFiles] = useState<File[]>([]),
    [confirm, setConfirm] = useState(""),
    [stars, setStars] = useState(5);
  const ticket = data.data;
  async function download(attachment: support.CustomerTicketAttachment) {
    await action.run(async () => {
      const blob = await support.downloadCustomerTicketAttachment(
        attachment.attachmentNo,
      );
      const url = URL.createObjectURL(blob),
        a = document.createElement("a");
      a.href = url;
      a.download = attachment.fileName;
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    });
  }
  return (
    <Page
      title={ticket?.subject || "工单详情"}
      actions={<Link to="/account/support">返回工单列表</Link>}
    >
      <FetchState state={data}>
        {ticket && (
          <>
            <Card>
              <div className="service-title">
                <Status value={ticket.stage} />
                <span>{ticket.ticketNo}</span>
                <span className="muted">{date(ticket.createdAt)}</span>
              </div>
              <div className="actions">
                {ticket.actions.canClose && (
                  <Button
                    tag="button"
                    variant="outline"
                    onClick={() => {
                      action.clear();
                      setConfirm("close");
                    }}
                  >
                    问题已解决，关闭工单
                  </Button>
                )}
                {ticket.actions.canReopen && (
                  <Button
                    tag="button"
                    variant="outline"
                    onClick={() => {
                      action.clear();
                      setConfirm("reopen");
                    }}
                  >
                    重新打开
                  </Button>
                )}
                <Button tag="button" variant="outline" onClick={data.reload}>
                  刷新
                </Button>
              </div>
            </Card>
            <Card title="沟通记录">
              <div className="conversation">
                {ticket.messages.map((message) => (
                  <article className="message" key={message.messageNo}>
                    <header>
                      <strong>
                        {message.author === "customer"
                          ? "我"
                          : message.automated
                            ? "服务通知"
                            : "客服"}
                      </strong>
                      <time>{date(message.createdAt)}</time>
                    </header>
                    <p>{message.body}</p>
                    {message.attachments.map((file) => (
                      <Button
                        tag="button"
                        key={file.attachmentNo}
                        variant="text"
                        onClick={() => void download(file)}
                      >
                        {file.fileName}
                      </Button>
                    ))}
                  </article>
                ))}
              </div>
              {ticket.actions.canReply && (
                <div className="reply-box">
                  <Textarea
                    aria-label="回复内容"
                    value={body}
                    onChange={setBody}
                    autosize={{ minRows: 4, maxRows: 10 }}
                    placeholder="填写您的回复"
                  />
                  <AttachmentInput
                    files={files}
                    setFiles={setFiles}
                    disabled={action.busy}
                  />
                  <Button
                    tag="button"
                    loading={action.busy}
                    disabled={!body.trim()}
                    onClick={() =>
                      void action.run(async () => {
                        await support.replyCustomerTicket(
                          ticketNo,
                          body,
                          ticket.version,
                          files,
                        );
                        setBody("");
                        setFiles([]);
                        data.reload();
                      })
                    }
                  >
                    发送回复
                  </Button>
                </div>
              )}
              <ErrorNote error={action.error} />
            </Card>
            {ticket.actions.canRate && (
              <Card title="评价本次服务">
                <Rate value={stars} onChange={setStars} />
                <Button
                  tag="button"
                  variant="outline"
                  loading={action.busy}
                  onClick={() =>
                    void action.run(async () => {
                      await support.rateCustomerTicket(
                        ticketNo,
                        stars,
                        ticket.currentRound,
                        ticket.version,
                      );
                      data.reload();
                    })
                  }
                >
                  提交评价
                </Button>
              </Card>
            )}
            {confirm && (
              <Confirm
                title={confirm === "close" ? "关闭工单" : "重新打开工单"}
                busy={action.busy}
                error={action.error}
                onClose={() => setConfirm("")}
                onConfirm={() =>
                  void action.run(async () => {
                    if (confirm === "close")
                      await support.closeCustomerTicket(
                        ticketNo,
                        ticket.version,
                      );
                    else
                      await support.reopenCustomerTicket(
                        ticketNo,
                        ticket.version,
                      );
                    setConfirm("");
                    data.reload();
                  })
                }
              >
                <p>
                  {confirm === "close"
                    ? "确认问题已经解决？"
                    : "问题仍未解决，确认继续与客服沟通？"}
                </p>
              </Confirm>
            )}
          </>
        )}
      </FetchState>
    </Page>
  );
}

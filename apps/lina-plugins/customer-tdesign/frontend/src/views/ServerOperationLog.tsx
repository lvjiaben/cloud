import { useState, type ReactNode } from "react";
import { Button, Card, Select } from "tdesign-react";
import {
  listNativeOperations,
  type NativeOperation,
} from "#/api/idc/native-servers";
import {
  BusinessTable,
  FetchState,
  Status,
  date,
  useLoad,
} from "#/components/business";
export default function ServerOperationLog({
  serverNo,
  names,
  busy,
  selected,
  onRead,
  outputs,
}: {
  serverNo: string;
  names: Record<string, string>;
  busy: boolean;
  selected: string;
  onRead: (operation: NativeOperation) => void;
  outputs: (operation: NativeOperation) => ReactNode;
}) {
  const [size, setSize] = useState(20);
  const [page, setPage] = useState(1),
    [action, setAction] = useState("");
  const data = useLoad(
    () => listNativeOperations(serverNo, page, action, size),
    [serverNo, page, action, size],
  );
  return (
    <Card title="操作记录">
      <div className="table-toolbar">
        <Select
          aria-label="筛选操作记录"
          value={action}
          onChange={(value) => {
            setAction(String(value));
            setPage(1);
          }}
          options={[
            { value: "", label: "全部操作" },
            ...Object.entries(names)
              .filter(
                ([value]) =>
                  ![
                    "snapshot_create",
                    "backup_create",
                    "traffic_add",
                    "ip_change",
                    "resize",
                  ].includes(value),
              )
              .map(([value, label]) => ({ value, label })),
          ]}
        />
        <Button tag="button" variant="outline" onClick={data.reload}>
          刷新记录
        </Button>
      </div>
      <FetchState state={data}>
        <BusinessTable
          rowKey="operationNo"
          rows={data.data?.items || []}
          total={data.data?.total}
          page={page}
          size={size}
          onPage={(next, size) => {
            setPage(next);
            setSize(size);
          }}
          columns={[
            {
              colKey: "action",
              title: "操作",
              cell: ({ row }) => names[row.action] || "服务操作",
            },
            {
              colKey: "status",
              title: "状态",
              cell: ({ row }) => (
                <Status
                  value={row.status === "running" ? "processing" : row.status}
                />
              ),
            },
            {
              colKey: "createdAt",
              title: "提交时间",
              cell: ({ row }) => date(row.createdAt),
            },
            {
              colKey: "updatedAt",
              title: "更新时间",
              cell: ({ row }) => date(row.updatedAt),
            },
            {
              colKey: "actions",
              title: "操作",
              cell: ({ row }) => (
                <div className="actions">
                  <Button
                    tag="button"
                    variant="text"
                    disabled={busy}
                    loading={busy && selected === row.operationNo}
                    onClick={() => onRead(row)}
                  >
                    {row.status === "succeeded" ? "查看结果" : "查看进度"}
                  </Button>
                  {outputs(row)}
                </div>
              ),
            },
          ]}
        />
      </FetchState>
    </Card>
  );
}

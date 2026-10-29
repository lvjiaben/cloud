import { useEffect, useState } from "react";
import { Button, Pagination, Tag } from "tdesign-react";
import { NotificationIcon } from "tdesign-icons-react";
import Dialog from "#/adapter/Dialog";
import * as api from "#/api/idc/announcements";
import { ErrorNote, FetchState, date, useLoad } from "#/components/business";
import { announcementHtml } from "#/utils/announcement-html";

export default function CustomerAnnouncements() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        tag="button"
        aria-label="公告"
        shape="square"
        variant="text"
        icon={<NotificationIcon />}
        onClick={() => setOpen(true)}
      />
      {open && <AnnouncementList onClose={() => setOpen(false)} />}
    </>
  );
}

function AnnouncementList({ onClose }: { onClose: () => void }) {
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(10);
  const [selected, setSelected] = useState<number>();
  const [readIds, setReadIds] = useState(new Set<number>());
  const data = useLoad(
    () => api.listCustomerAnnouncements(page, size),
    [page, size],
  );
  return (
    <>
      <Dialog
        visible
        header="公告"
        footer={false}
        width="min(720px, 94vw)"
        onClose={onClose}
      >
        <div className="announcement-list" data-testid="announcement-list">
          <FetchState state={data}>
            {!data.data?.list.length && <p className="muted">暂无公告</p>}
            {data.data?.list.map((item) => (
              <button
                key={item.id}
                type="button"
                className="announcement-row"
                onClick={() => setSelected(item.id)}
              >
                <span className="announcement-row-title">{item.title}</span>
                <Tag
                  variant="light"
                  theme={
                    item.read || readIds.has(item.id) ? "default" : "primary"
                  }
                >
                  {item.read || readIds.has(item.id) ? "已读" : "未读"}
                </Tag>
                <time>{date(item.createdAt)}</time>
              </button>
            ))}
          </FetchState>
        </div>
        <Pagination
          current={page}
          pageSize={size}
          total={data.data?.total || 0}
          pageSizeOptions={[10, 20, 50]}
          onChange={({ current, pageSize }) => {
            setPage(current);
            setSize(pageSize);
          }}
        />
      </Dialog>
      {selected !== undefined && (
        <AnnouncementDetail
          key={selected}
          id={selected}
          onClose={() => setSelected(undefined)}
          onRead={() =>
            setReadIds((previous) => new Set(previous).add(selected))
          }
        />
      )}
    </>
  );
}

function AnnouncementDetail({
  id,
  onClose,
  onRead,
}: {
  id: number;
  onClose: () => void;
  onRead: () => void;
}) {
  const [readError, setReadError] = useState("");
  const detail = useLoad(() => api.getCustomerAnnouncement(id), [id]);
  useEffect(() => {
    if (!detail.data || detail.data.read) return;
    let active = true;
    void api
      .readCustomerAnnouncement(id)
      .then(() => {
        // The list remains open after its detail closes, so it still needs the persisted state.
        onRead();
      })
      .catch(() => {
        if (active) setReadError("已读状态保存失败，请关闭后重试。");
      });
    return () => {
      active = false;
    };
  }, [detail.data, id]);
  return (
    <Dialog
      visible
      header={detail.data?.title || "公告详情"}
      width="min(800px, 94vw)"
      footer={false}
      onClose={onClose}
    >
      <FetchState state={detail}>
        {detail.data && (
          <article data-testid="announcement-detail">
            <p className="muted">{date(detail.data.createdAt)}</p>
            <div
              className="announcement-content"
              dangerouslySetInnerHTML={{
                __html: announcementHtml(detail.data.content),
              }}
            />
          </article>
        )}
      </FetchState>
      <ErrorNote error={readError} />
    </Dialog>
  );
}

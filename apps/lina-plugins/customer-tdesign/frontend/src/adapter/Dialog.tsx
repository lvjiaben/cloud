import { useEffect, useId, useRef } from "react";
import { Dialog as TDialog, type DialogProps } from "tdesign-react";
import { CloseIcon } from "tdesign-icons-react";

// Keep the library's dialog behavior while providing a named modal and keyboard access.
export default function Dialog({ header, children, ...props }: DialogProps) {
  const titleId = useId();
  const opener = useRef(document.activeElement as HTMLElement | null);
  useEffect(
    () => () => {
      if (opener.current?.isConnected) opener.current.focus();
    },
    [],
  );
  useEffect(() => {
    if (!props.visible) return;
    const keepFocus = (event: KeyboardEvent) => {
      if (event.key !== "Tab") return;
      const dialog = document
        .getElementById(titleId)
        ?.closest('[role="dialog"]');
      const controls = Array.from(
        dialog?.querySelectorAll<HTMLElement>(
          'button:not([disabled]),a[href],input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])',
        ) || [],
      ).filter(
        (element) =>
          element.tabIndex >= 0 && element.getClientRects().length > 0,
      );
      const first = controls[0],
        last = controls.at(-1);
      if (!first || !last) return;
      if (
        event.shiftKey &&
        (document.activeElement === first ||
          !dialog?.contains(document.activeElement))
      ) {
        event.preventDefault();
        last.focus();
      } else if (
        !event.shiftKey &&
        (document.activeElement === last ||
          !dialog?.contains(document.activeElement))
      ) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", keepFocus);
    return () => document.removeEventListener("keydown", keepFocus);
  }, [props.visible, titleId]);
  const semantics = {
    role: "dialog",
    "aria-modal": true,
    "aria-labelledby": titleId,
  };
  return (
    <TDialog
      {...semantics}
      placement="center"
      closeOnOverlayClick={false}
      {...props}
      header={<span id={titleId}>{header}</span>}
      closeBtn={
        <button type="button" className="dialog-close" aria-label="关闭弹窗">
          <CloseIcon />
        </button>
      }
      onOpened={() => {
        const dialog = document
          .getElementById(titleId)
          ?.closest('[role="dialog"]');
        const first = dialog?.querySelector<HTMLElement>(
          "input:not([disabled]),select:not([disabled]),textarea:not([disabled]),button:not([disabled])",
        );
        first?.focus();
        props.onOpened?.();
      }}
    >
      {children}
    </TDialog>
  );
}

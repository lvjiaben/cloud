import { Tabs as TTabs } from "tdesign-react";
import type { ReactNode } from "react";

// Keep TDesign's visual tabs and provide keyboard navigation for its label slot.
export default function Tabs({
  value,
  items,
  onChange,
  label,
}: {
  value: string;
  items: { value: string; label: ReactNode }[];
  onChange: (value: string) => void;
  label: string;
}) {
  return (
    <div role="tablist" aria-label={label}>
      <TTabs value={value} onChange={(v) => onChange(String(v))}>
        {items.map((item, index) => (
          <TTabs.TabPanel
            key={item.value}
            value={item.value}
            label={
              <button
                type="button"
                role="tab"
                className="cloud-tab-label"
                aria-selected={value === item.value}
                tabIndex={value === item.value ? 0 : -1}
                onKeyDown={(event) => {
                  const next =
                    event.key === "ArrowRight"
                      ? (index + 1) % items.length
                      : event.key === "ArrowLeft"
                        ? (index - 1 + items.length) % items.length
                        : event.key === "Home"
                          ? 0
                          : event.key === "End"
                            ? items.length - 1
                            : -1;
                  if (next < 0) return;
                  event.preventDefault();
                  onChange(items[next].value);
                  event.currentTarget
                    .closest('[role="tablist"]')
                    ?.querySelectorAll<HTMLButtonElement>('[role="tab"]')
                    [next]?.focus();
                }}
              >
                {item.label}
              </button>
            }
          />
        ))}
      </TTabs>
    </div>
  );
}

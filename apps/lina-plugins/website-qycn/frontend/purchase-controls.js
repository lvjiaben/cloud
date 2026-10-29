// Presentation only: all choices retain the public owner's opaque option token.
import { t, locale } from "./i18n.js";

const label = (item) =>
  (locale === "en-US" ? item.labelEnUS : item.labelZhCN) || item.labelZhCN || item.labelEnUS || "";
const leaves = (items = []) => items.flatMap((item) =>
  item.children?.length ? leaves(item.children) : [item]);

const families = [
  ["windows", "Windows", /windows|win(?:server)?[\s_-]*\d/i],
  ["ubuntu", "Ubuntu", /ubuntu/i],
  ["debian", "Debian", /debian/i],
  ["centos", "CentOS", /(?:^|[^a-z])centos(?=$|[^a-z])/i],
  ["rocky", "Rocky Linux", /rocky/i],
  ["alma", "AlmaLinux", /alma[\s_-]*linux/i],
  ["fedora", "Fedora", /fedora/i],
  ["alpine", "Alpine Linux", /alpine/i],
  ["opensuse", "openSUSE", /(?:open)?suse/i],
];
function family(item) {
  // Family recognition affects grouping and icons only, never the submitted value.
  return families.find(([, , match]) => match.test(`${item.labelEnUS || ""} ${item.labelZhCN || ""}`)) ||
    ["other", t("Other systems")];
}
function icon(kind) {
  const node = document.createElement("span");
  node.className = `purchase-choice-icon icon-${kind}`;
  node.setAttribute("aria-hidden", "true");
  const shapes = kind === "windows"
    ? '<path fill="currentColor" d="M2 4 11 2.8V11H2zm11-1.5L23 1v10H13zM2 13h9v8.2L2 20zm11 0h10v10l-10-1.5z"/>'
    : kind === "globe"
      ? '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c5 5 5 13 0 18-5-5-5-13 0-18ZM5 6.5c4 2 10 2 14 0M5 17.5c4-2 10-2 14 0"/>'
      : kind === "ubuntu"
        ? '<circle cx="12" cy="12" r="6"/><circle cx="12" cy="3" r="2" fill="currentColor"/><circle cx="4" cy="16.5" r="2" fill="currentColor"/><circle cx="20" cy="16.5" r="2" fill="currentColor"/>'
        : kind === "debian"
          ? '<path d="M15 19c-9 2-14-10-6-14 8-4 14 6 7 10-5 2-8-4-4-6 3-1 5 3 2 4"/>'
          : kind === "centos"
            ? '<path d="m12 2 5 5-5 5-5-5zm0 10 5 5-5 5-5-5zM2 12l5-5 5 5-5 5zm10 0 5-5 5 5-5 5z"/>'
            : '<rect x="4" y="3" width="16" height="18" rx="2"/><path d="M8 8h8M8 12h8M8 16h4"/>';
  node.innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">${shapes}</svg>`;
  return node;
}

export function operatingSystemControl(field, value) {
  const root = document.createElement("fieldset");
  root.className = "purchase-system-options";
  root.dataset.control = "choices";
  root.setAttribute("aria-label", label(field));
  const groups = new Map();
  for (const item of leaves(field.options)) {
    const [key, name] = family(item);
    if (!groups.has(key)) groups.set(key, { name, items: [] });
    groups.get(key).items.push(item);
  }
  const selected = value.optionTokens?.[0] || leaves(field.options)[0]?.optionToken;
  for (const [key, group] of groups) {
    const card = document.createElement("div");
    card.className = "purchase-system-card";
    const header = document.createElement("label");
    const radio = document.createElement("input");
    radio.type = "radio";
    radio.name = `system-${field.fieldToken}`;
    const current = group.items.find((item) => item.optionToken === selected) || group.items[0];
    radio.value = current.optionToken;
    radio.checked = current.optionToken === selected;
    radio.disabled = Boolean(field.disabled);
    radio.required = Boolean(field.required);
    const title = document.createElement("span");
    title.textContent = group.name;
    header.append(radio, icon(key), title);
    const version = document.createElement("select");
    version.setAttribute("aria-label", `${group.name} · ${t("Version")}`);
    version.disabled = Boolean(field.disabled);
    for (const item of group.items) version.add(new Option(label(item), item.optionToken, false, item === current));
    const updateSelection = () => {
      for (const node of root.querySelectorAll(".purchase-system-card"))
        node.classList.toggle("is-selected", node.querySelector('input[type="radio"]').checked);
    };
    radio.addEventListener("change", updateSelection);
    version.addEventListener("change", () => {
      radio.value = version.value;
      radio.checked = true;
      updateSelection();
    });
    card.classList.toggle("is-selected", radio.checked);
    card.append(header, version);
    root.append(card);
  }
  return root;
}

export function regionControl(field, value) {
  const root = document.createElement("fieldset");
  root.className = "purchase-options purchase-region-options";
  root.dataset.control = "choices";
  root.setAttribute("aria-label", label(field));
  const choices = leaves(field.options);
  const selected = value.optionTokens?.[0] || choices[0]?.optionToken;
  for (const item of choices) {
    const option = document.createElement("label");
    const radio = document.createElement("input");
    radio.type = "radio";
    radio.name = `region-${field.fieldToken}`;
    radio.value = item.optionToken;
    radio.checked = item.optionToken === selected;
    radio.disabled = Boolean(field.disabled);
    radio.required = Boolean(field.required);
    const caption = document.createElement("span");
    const name = document.createElement("strong");
    name.textContent = label(item);
    caption.append(icon("globe"), name);
    option.append(radio, caption);
    root.append(option);
  }
  return root;
}

export function quantitySlider(field, input) {
  const root = document.createElement("div");
  root.className = "purchase-slider";
  const track = document.createElement("div");
  track.className = "purchase-slider-track";
  const range = document.createElement("input");
  range.type = "range";
  try {
    const minimum = BigInt(input.min), maximum = BigInt(input.max), step = BigInt(input.step || "1");
    if (step > 0n && maximum >= minimum)
      input.max = String(minimum + (maximum - minimum) / step * step);
  } catch { /* Incomplete ranges remain subject to native and server validation. */ }
  range.min = input.min;
  range.max = input.max;
  range.step = input.step;
  range.disabled = Boolean(field.disabled);
  range.setAttribute("aria-label", `${label(field)} · ${t("Adjust value")}`);
  const limits = document.createElement("div");
  limits.className = "purchase-slider-limits";
  for (const value of [input.min, input.max]) {
    const node = document.createElement("span");
    node.textContent = `${value}${field.unit ? " " + field.unit : ""}`;
    limits.append(node);
  }
  const stepper = document.createElement("div");
  stepper.className = "purchase-stepper";
  const update = () => {
    range.value = input.value || input.min;
    const progress = (Number(range.value) - Number(range.min)) / (Number(range.max) - Number(range.min));
    range.style.setProperty("--slider-progress", `${Math.min(100, Math.max(0, Number.isFinite(progress) ? progress * 100 : 0))}%`);
    for (const [node, boundary] of [[decrease, input.min], [increase, input.max]])
      node.disabled = Boolean(field.disabled) || input.value === boundary;
  };
  const step = (direction) => {
    if (field.disabled) return;
    try {
      const min = BigInt(input.min), max = BigInt(input.max), increment = BigInt(input.step || "1");
      const current = BigInt(input.value || input.min);
      const offset = current > min ? current - min : 0n;
      const index = direction > 0n
        ? offset / increment + 1n
        : (offset + increment - 1n) / increment - 1n;
      const next = min + index * increment;
      input.value = String(next < min ? min : next > max ? max : next);
      update();
      input.dispatchEvent(new Event("input", { bubbles: true }));
    } catch { input.reportValidity(); }
  };
  const decrease = document.createElement("button");
  decrease.type = "button";
  decrease.textContent = "−";
  decrease.setAttribute("aria-label", `${t("Decrease")} · ${label(field)}`);
  decrease.addEventListener("click", () => step(-1n));
  const increase = document.createElement("button");
  increase.type = "button";
  increase.textContent = "+";
  increase.setAttribute("aria-label", `${t("Increase")} · ${label(field)}`);
  increase.addEventListener("click", () => step(1n));
  range.addEventListener("input", () => { input.value = range.value; update(); });
  input.addEventListener("input", update);
  track.append(range, limits);
  stepper.append(decrease, input, increase);
  root.append(track, stepper);
  update();
  return root;
}

export function generatedPassword(policy = {}) {
  const alphabets = {
    lowercase: "abcdefghijklmnopqrstuvwxyz",
    uppercase: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
    digits: "0123456789",
    symbols: "!@#$%*-_=+?",
  };
  const classes = (policy.characters?.length ? policy.characters : ["lowercase", "uppercase", "digits"])
    .map((kind) => alphabets[kind]).filter(Boolean);
  if (!classes.length) return "";
  const randomIndex = (size) => {
    const source = new Uint32Array(1), limit = Math.floor(0x100000000 / size) * size;
    do { crypto.getRandomValues(source); } while (source[0] >= limit);
    return source[0] % size;
  };
  const length = Math.min(20, Math.max(6, Number(policy.length) || 16));
  const all = classes.join("");
  const result = Array.from({ length }, (_, index) => {
    const alphabet = classes[index] || all;
    return alphabet[randomIndex(alphabet.length)];
  });
  for (let i = result.length - 1; i > 0; i--) {
    const j = randomIndex(i + 1);
    [result[i], result[j]] = [result[j], result[i]];
  }
  return result.join("");
}

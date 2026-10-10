// This connector compiles its customer entry with tsc, without a CSS bundler.
// Keep customer styles in source and apply them only inside this page root.
export function installCustomerServerAppearance(doc: Document = document) {
  const style = doc.createElement('style');
  style.textContent = `
:root{--background:216 33% 97%;--foreground:222 47% 11%;--card:0 0% 100%;--border:214 23% 88%;--primary:211 100% 55%;--primary-foreground:0 0% 100%}*{box-sizing:border-box}body{margin:0;background:hsl(var(--background));color:hsl(var(--foreground));font:14px/1.5 system-ui}button{font:inherit;cursor:pointer;padding:6px 12px;border:1px solid hsl(var(--border))}.server-list{list-style:none;margin:0;padding:0}.server-row{display:grid;grid-template-columns:2fr 1fr 1fr auto;gap:16px;padding:14px 16px;border-bottom:1px solid hsl(var(--border))}.product span{display:block}.detail{padding:16px}.detail-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr))}.detail-grid dd{margin:0}.state{padding:24px}
.zjmfcloud-servers {
  min-width: 0;
  padding: 16px;
}
.zjmfcloud-servers :is(.panel, .toolbar, .button) {
  background: hsl(var(--card, 0 0% 100%));
}
.zjmfcloud-servers :is(.panel, .button, .status, .skeleton-row) {
  border-radius: 0;
}
.zjmfcloud-servers .toolbar {
  flex-wrap: wrap;
  gap: 8px 16px;
  margin-bottom: 16px;
  padding: 12px 16px;
  border: 1px solid hsl(var(--border));
}
.zjmfcloud-servers .summary {
  min-width: 0;
  overflow-wrap: anywhere;
}
.zjmfcloud-servers .button {
  min-height: 32px;
  white-space: normal;
}
.zjmfcloud-servers .button-primary {
  background: hsl(var(--primary));
  color: hsl(var(--primary-foreground));
}
.zjmfcloud-servers .server-row {
  min-height: 64px;
}
.zjmfcloud-servers .server-row > * {
  min-width: 0;
}
.zjmfcloud-servers :is(.product strong, .product span, .location, .status) {
  overflow-wrap: anywhere;
  white-space: normal;
}
.zjmfcloud-servers .product strong {
  font-weight: 600;
  line-height: 1.5;
}
.zjmfcloud-servers .status {
  max-width: 100%;
  padding: 3px 8px;
}
.zjmfcloud-servers .detail {
  margin-top: 16px;
}
.zjmfcloud-servers .detail-head {
  flex-wrap: wrap;
  gap: 8px 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid hsl(var(--border));
}
.zjmfcloud-servers :is(.detail-head h2, .state strong) {
  font-size: 16px;
  line-height: 1.5;
}
.zjmfcloud-servers .detail-grid {
  gap: 12px 24px;
  margin-top: 12px;
}
.zjmfcloud-servers .detail-grid > div {
  min-width: 0;
  padding: 8px 0;
  border-bottom: 1px solid hsl(var(--border));
}
.zjmfcloud-servers .detail-grid dd {
  margin-top: 4px;
  line-height: 1.5;
}
@media (max-width: 720px) {
  .zjmfcloud-servers .server-row {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px 16px;
  }
  .zjmfcloud-servers .detail-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
`;
  doc.head.append(style);
}

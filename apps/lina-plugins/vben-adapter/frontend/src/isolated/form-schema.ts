import type { IsolatedFormDrawerOptions } from './types';

/** Convert caller-owned value definitions into actual Vben form schema. */
export function isolatedFormSchema(options: Pick<IsolatedFormDrawerOptions, 'fields' | 'readOnly'>) {
  return options.fields.map(field => ({
    fieldName: field.name,
    label: field.label,
    component: field.component,
    description: field.description,
    rules: !options.readOnly && field.required ? 'required' : undefined,
    componentProps: { ...field.props, disabled: options.readOnly || field.props?.disabled, autocomplete: 'off' },
  }));
}

/** Required switches may be false and required amounts may be zero. */
export function isolatedFormValuePresent(value: unknown) {
  return value !== undefined && value !== null && value !== '' && (!Array.isArray(value) || value.length > 0);
}

/** Preserve the former native type=url syntax check, without provider policy. */
export function isolatedFormURLValid(value: unknown) {
  if (!isolatedFormValuePresent(value)) return true;
  try { new URL(String(value)); return true; } catch { return false; }
}

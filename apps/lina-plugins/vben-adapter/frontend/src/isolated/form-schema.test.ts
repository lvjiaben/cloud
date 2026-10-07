import assert from 'node:assert/strict';
import test from 'node:test';
import { isolatedFormSchema, isolatedFormValuePresent, isolatedFormURLValid } from './form-schema.ts';

test('the genuine form schema preserves declared options and required validation without mutating caller data', () => {
  const props = { type: 'url', placeholder: 'Address' };
  const result = isolatedFormSchema({ fields: [{ name: 'endpoint', label: 'Endpoint', component: 'Input', required: true, props }] });
  assert.equal(result[0]!.rules, 'required');
  assert.equal(result[0]!.fieldName, 'endpoint');
  assert.equal(result[0]!.componentProps.autocomplete, 'off');
  assert.notEqual(result[0]!.componentProps, props);
  assert.deepEqual(props, { type: 'url', placeholder: 'Address' });
});

test('read-only details suppress validation and cannot override the disabled state', () => {
  const result = isolatedFormSchema({ readOnly: true, fields: [{ name: 'amount', label: 'Count', component: 'InputNumber', required: true, props: { disabled: false } }] });
  assert.equal(result[0]!.rules, undefined);
  assert.equal(result[0]!.componentProps.disabled, true);
});

test('required form values keep false switches and zero counts valid', () => {
  for (const value of [false, 0, '0', ['one']]) assert.equal(isolatedFormValuePresent(value), true);
  for (const value of [undefined, null, '', []]) assert.equal(isolatedFormValuePresent(value), false);
});

test('URL syntax rejects malformed input without inventing provider protocol policy', () => {
  for (const value of ['', 'https://api.example.invalid', 'http://127.0.0.1:8080', 'ftp://files.example.invalid']) assert.equal(isolatedFormURLValid(value), true);
  for (const value of ['not a URL', '//example.invalid']) assert.equal(isolatedFormURLValid(value), false);
});

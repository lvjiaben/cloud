import { describe, expect, it } from 'vitest';

import {
  resolvePluginSandboxRequestOwner,
  unwrapPluginSandboxResponseBody,
} from './plugin-sandbox-bridge';

describe('plugin sandbox bridge owner targeting', () => {
  it('keeps the surface plugin namespace when the grant has no owner projection', () => {
    expect(resolvePluginSandboxRequestOwner('phone-provider')).toBe(
      'phone-provider',
    );
  });

  it('uses only the owner returned by the authenticated grant issuer', () => {
    expect(resolvePluginSandboxRequestOwner('yunsu-ssl', 'idc-core')).toBe(
      'idc-core',
    );
  });

  it.each(['../idc-core', 'idc/core', 'IDC Core', '', 'idc_core'])(
    'rejects an unsafe owner projection: %s',
    (owner) => {
      expect(() =>
        resolvePluginSandboxRequestOwner('yunsu-ssl', owner),
      ).toThrow('Invalid plugin page bridge owner');
    },
  );
});

describe('plugin sandbox bridge response normalization', () => {
  it('unwraps successful source-plugin envelopes', () => {
    expect(
      unwrapPluginSandboxResponseBody({
        code: 0,
        data: { items: [], nextCursor: '' },
        message: 'OK',
      }),
    ).toEqual({ items: [], nextCursor: '' });
  });

  it('keeps dynamic-plugin guest bodies unchanged', () => {
    const body = { items: [], nextCursor: '' };
    expect(unwrapPluginSandboxResponseBody(body)).toBe(body);
  });
});

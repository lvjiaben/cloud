import type { CascaderProps } from 'ant-design-vue';

import areaData from 'area-data/pcaa.js';

type CascaderOption = NonNullable<CascaderProps['options']>[number];

const specialRegionCodes = new Set(['710000', '810000', '820000']);

function optionsFor(parentCode: string, depth: number): CascaderOption[] {
  const children = areaData[parentCode] ?? {};
  return Object.entries(children).map(([code, name]) => {
    const nested = depth > 1 ? optionsFor(code, depth - 1) : [];
    return {
      children: nested.length ? nested : undefined,
      label: name,
      value: name,
    };
  });
}

export const mainlandRegionOptions: CascaderProps['options'] = Object.entries(
  areaData['86'] ?? {},
)
  .filter(([code]) => !specialRegionCodes.has(code))
  .map(([code, name]) => {
    const cities = optionsFor(code, 2);
    return {
      children: cities.length ? cities : undefined,
      label: name,
      value: name,
    };
  });

// 在界面装载前注册随独立模板打包的标准图标，避免常用控件依赖外部图标服务。
import { addCollection } from '@iconify/vue';
import lucide from '@iconify-json/lucide/icons.json';
import fluentMdl2 from '@iconify-json/fluent-mdl2/icons.json';
import ep from '@iconify-json/ep/icons.json';

export function registerConsoleIcons() {
  addCollection(lucide);
  addCollection(fluentMdl2);
  addCollection(ep);
}

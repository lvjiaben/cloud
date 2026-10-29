import messages from './commerce-zh-CN.json' with {type:'json'};
import {getLocale,localized,translate} from './locale.js';
export function t(source){const chinese=messages[source]||source;localized({zhCN:chinese,enUS:source});return getLocale()==='en-US'?source:translate(chinese);}

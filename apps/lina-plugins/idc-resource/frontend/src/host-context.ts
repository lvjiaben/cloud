export type PluginHostMountContext={accessToken:string;container:HTMLElement;locale:string;routePath:string;route:{path?:string};t:(key:string,fallback?:string)=>string};
export type PluginHostMountInstance={unmount:()=>void;update:(context:PluginHostMountContext)=>void};

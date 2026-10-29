import { useEffect, useState } from "react";
import { CloudIcon, HomeIcon, WalletIcon, ServiceIcon, UserIcon, LinkIcon } from "tdesign-icons-react";

const icons = {
  home: HomeIcon,
  cloud: CloudIcon,
  wallet: WalletIcon,
  support: ServiceIcon,
  user: UserIcon,
  external: LinkIcon,
};
// Menu icons are data, rendered as passive SVG images rather than injected markup.
export default function NavigationIcon({icon,size=18}:{icon:string;size?:number}) {
  const [failed,setFailed]=useState(false);
  useEffect(()=>setFailed(false),[icon]);
  const Fallback=icons[icon as keyof typeof icons]||LinkIcon;
  if(failed||!/^[a-z0-9-]+:[a-z0-9-]+$/.test(icon)) return <Fallback size={size}/>;
  return <img aria-hidden="true" alt="" width={size} height={size} className="shrink-0" referrerPolicy="no-referrer"
    src={`https://api.iconify.design/${icon.replace(":","/")}.svg?color=%2364748b`} onError={()=>setFailed(true)}/>;
}

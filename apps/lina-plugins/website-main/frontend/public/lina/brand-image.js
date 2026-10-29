// Resolve only exact URLs supplied by the same fresh public identity hint.
export function brandImageURL(source,document){
 const hint=document.defaultView?.__LINA_PUBLIC_IDENTITY__;
 const mapped=hint?.images?.[source];
 return hint&&Number.isFinite(hint.savedAt)&&Date.now()>=hint.savedAt&&Date.now()-hint.savedAt<=30000&&typeof mapped==='string'&&/^\/lina\/brand-media\/[a-f0-9]{64}\.(png|jpg|gif|webp|svg|avif|ico)$/.test(mapped)?new URL(mapped,document.location.origin).href:source;
}

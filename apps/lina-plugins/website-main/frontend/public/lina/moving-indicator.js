// A single line follows direct navigation links. Panel links never move it.
const bindings=new WeakMap();
export function bindMovingIndicator(root){
 const previous=bindings.get(root);if(previous){previous.refresh();return;}
 const container=root.parentElement,line=container?.querySelector(':scope>.underline');if(!line)return;
 container.classList.add('lina-moving-line-container');root.classList.add('lina-moving-navigation');line.setAttribute('aria-hidden','true');
 const win=root.ownerDocument.defaultView;
 let current,frame,visible=false;
 const title=node=>{const a=node?.closest?.('a');return a?.parentElement?.parentElement===root&&a.getAttribute('aria-disabled')!=='true'?a:null;};
 const show=a=>{
  if(!a)return;current=a;win.cancelAnimationFrame(frame);
  const box=a.getBoundingClientRect(),offset=line.offsetParent||container,parent=offset.getBoundingClientRect(),left=box.left-parent.left+offset.scrollLeft-offset.clientLeft,top=box.bottom-parent.top+offset.scrollTop-offset.clientTop-line.getBoundingClientRect().height;
  line.style.top=top+'px';
  const hidden=!visible;visible=true;
  if(hidden){line.style.transition='none';line.style.left=left+'px';line.style.width='0px';line.style.opacity='1';line.getBoundingClientRect();line.style.removeProperty('transition');}
  const update=()=>{line.style.left=left+'px';line.style.width=box.width+'px';line.style.opacity='1';};
  if(hidden)frame=win.requestAnimationFrame(update);else update();
 };
 const refresh=()=>{if(!visible)return;if(current&&root.contains(current))show(current);else{current=undefined;visible=false;line.style.opacity='0';}};
 const leave=()=>{if(root.contains(root.ownerDocument.activeElement))return;win.cancelAnimationFrame(frame);visible=false;line.style.opacity='0';};
 root.addEventListener('pointerover',event=>show(title(event.target)));
 root.addEventListener('focusin',event=>show(title(event.target)));
 root.addEventListener('click',event=>show(title(event.target)));
 root.addEventListener('pointerleave',leave);
 root.addEventListener('focusout',()=>queueMicrotask(()=>{if(!root.matches(':hover'))leave();}));
 container.addEventListener('scroll',refresh,{passive:true});
 const observer=new win.ResizeObserver(refresh);observer.observe(container);observer.observe(root);
 bindings.set(root,{refresh});
}

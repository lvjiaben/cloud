// One public read at a time; background pages do not poll.
export function startBrandRefresh(document,load){
 const view=document.defaultView;let timer,running,pending=false,active=true;
 const visible=()=>active&&!document.hidden;
 const clear=()=>{view.clearTimeout(timer);timer=undefined;};
 const schedule=()=>{clear();if(visible())timer=view.setTimeout(refresh,30000);};
 function refresh(){
  clear();if(!visible())return Promise.resolve();
  if(running){pending=true;return running;}
  running=(async()=>{do{pending=false;await load();}while(pending&&visible());})().finally(()=>{running=undefined;schedule();});
  return running;
 }
 view.addEventListener('focus',refresh);
 document.addEventListener('visibilitychange',()=>{if(visible())refresh();else clear();});
 view.addEventListener('pagehide',()=>{active=false;pending=false;clear();});
 view.addEventListener('pageshow',event=>{active=true;if(event.persisted)refresh();});
 refresh();return refresh;
}

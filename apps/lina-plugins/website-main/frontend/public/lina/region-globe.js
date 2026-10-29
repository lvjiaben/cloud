// Original r128 sphere parameters; lifecycle and input handling owned by this plugin.
const instances = new WeakMap();
export async function initializeRegionGlobe(document) {
 const canvas=document.querySelector('#network_canvas');if(!canvas||instances.has(canvas))return;
 const view=document.defaultView,container=canvas.parentElement;instances.set(canvas,true);
 container.dataset.globeState='loading';canvas.hidden=true;
 let renderer,geometry,material,texture,frame=0,disposed=false,dragging=false,lastX=0,lastY=0,loaded=false;
 const events=new view.AbortController();
 const stop=()=>{view.cancelAnimationFrame(frame);frame=0;};
 const fallback=()=>{canvas.hidden=true;container.dataset.globeState='unavailable';dispose();};
 const dispose=()=>{if(disposed)return;disposed=true;stop();events.abort();texture?.dispose();material?.dispose();geometry?.dispose();renderer?.dispose();instances.delete(canvas);};
 const on=(target,type,fn)=>target.addEventListener(type,fn,{signal:events.signal});
 let scene,camera,sphere;
 function draw(){if(disposed||!loaded)return;renderer.render(scene,camera);}
 function tick(){frame=0;if(disposed||!loaded||document.hidden)return;if(!dragging)sphere.rotation.y+=0.0008;draw();frame=view.requestAnimationFrame(tick);}
 function resume(){stop();if(!loaded||disposed)return;draw();if(!document.hidden)frame=view.requestAnimationFrame(tick);}
 on(view,'pagehide',dispose);
 try {
  await new Promise((resolve,reject)=>{if(view.THREE)return resolve();const script=document.createElement('script');script.src=new URL('../static/vendor/three.min.js',import.meta.url).href;script.onload=resolve;script.onerror=reject;document.head.append(script);});
  if(disposed)return;const T=view.THREE;scene=new T.Scene();camera=new T.PerspectiveCamera(75,1,0.1,1000);camera.position.z=8.5;
  renderer=new T.WebGLRenderer({canvas,antialias:true,alpha:true});renderer.setSize(1037,1037);geometry=new T.SphereGeometry(5,64,64);
  material=new T.MeshBasicMaterial({transparent:true,opacity:0.9});sphere=new T.Mesh(geometry,material);sphere.rotation.y=60;sphere.rotation.x=120;scene.add(sphere);
  texture=new T.TextureLoader().load(new URL('../static/region/globe.png',import.meta.url).href,t=>{if(disposed){t.dispose();return;}material.map=t;material.needsUpdate=true;loaded=true;canvas.hidden=false;container.dataset.globeState='ready';resume();},undefined,fallback);
  on(canvas,'webglcontextlost',event=>{event.preventDefault();fallback();dispose();});
  on(canvas,'pointerdown',event=>{dragging=true;lastX=event.clientX;lastY=event.clientY;canvas.setPointerCapture(event.pointerId);});
  on(canvas,'pointermove',event=>{if(!dragging)return;const rotation=new T.Quaternion().setFromEuler(new T.Euler((event.clientY-lastY)*Math.PI/180,(event.clientX-lastX)*Math.PI/180,0,'XYZ'));sphere.quaternion.multiplyQuaternions(rotation,sphere.quaternion);lastX=event.clientX;lastY=event.clientY;draw();});
  const release=()=>{dragging=false;};on(canvas,'pointerup',release);on(canvas,'pointercancel',release);on(canvas,'lostpointercapture',release);
  on(document,'visibilitychange',resume);
 }catch{fallback();dispose();}
}

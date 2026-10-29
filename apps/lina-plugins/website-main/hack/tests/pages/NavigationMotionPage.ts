import {NavigationPage} from './NavigationPage';
export class NavigationMotionPage extends NavigationPage {
 async observe(){await this.page.evaluate(()=>{
  const events:any[]=[];(window as any).navMotion=events;
  document.querySelectorAll('.horizontal-menu-navbar-menu>li').forEach(li=>li.addEventListener('pointerenter',()=>events.push({type:'enter',at:performance.now()})));
  const original=Element.prototype.animate;
  Element.prototype.animate=function(...args:any[]){
   const animation=original.apply(this,args as any);
   if(this.matches('.lina-nav-panel')){
    const entry:any={type:'slide',at:performance.now(),samples:[],duration:animation.effect?.getTiming().duration};events.push(entry);
    const sample=()=>{entry.samples.push(this.getBoundingClientRect().height);if(animation.playState==='running')requestAnimationFrame(sample);};requestAnimationFrame(sample);
    animation.addEventListener('finish',()=>{entry.finished=performance.now();});
   }return animation;
  };
 });}
 async events(){return this.page.evaluate(()=>(window as any).navMotion);}
 async settle(){await this.page.waitForFunction(()=>!document.querySelectorAll('.lina-nav-panel').values().some(n=>n.getAnimations().length));}
 async screenshot(path:string){await this.page.screenshot({path});}
}

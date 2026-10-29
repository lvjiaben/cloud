
import {NavigationPage} from './NavigationPage';
import type {Locator} from '@playwright/test';
export class ActivationIndicatorPage extends NavigationPage {
 title(name:string){return this.page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:name});}
 movingLine(){return this.page.locator('.horizontal-menu-navbar-menu-container>.underline');}
 async lineState(){return this.movingLine().evaluate(e=>{const r=e.getBoundingClientRect(),s=getComputedStyle(e);return{left:r.left,width:r.width,opacity:Number(s.opacity),height:s.height,color:s.backgroundColor,transition:s.transitionDuration};});}
 async measureTitle(name:string){const line=await this.lineState();return this.title(name).evaluate((e,line)=>({scale:line.opacity===0?0:line.width/e.getBoundingClientRect().width,width:line.width,height:line.height,color:line.color,arrow:getComputedStyle(e,'::after').borderBottomWidth}),line);}
 async titleBox(name:string){return this.title(name).boundingBox();}
 async sampleMove(){return this.movingLine().evaluate(async e=>{const animation=e.getAnimations().find(a=>a instanceof CSSTransition&&a.transitionProperty==='left');if(!animation)return null;animation.pause();animation.currentTime=Number(animation.effect!.getTiming().duration)/2;await new Promise(requestAnimationFrame);const left=e.getBoundingClientRect().left;animation.play();return left;});}
 async keyboardFocusTitle(name:string){await this.keyboard('Tab');await this.title(name).focus();}
 category(name:string){return this.panel().getByRole('button',{name,exact:true});}
 async measure(element:Locator){return element.evaluate(e=>{const s=getComputedStyle(e,'::after'),r=e.getBoundingClientRect();return {width:parseFloat(s.width)*Math.abs(new DOMMatrixReadOnly(s.transform==='none'?'matrix(1,0,0,1,0,0)':s.transform).a),full:r.width,property:s.transitionProperty,duration:s.transitionDuration,animation:s.animationName,height:s.height,color:s.backgroundColor,origin:s.transformOrigin};});}
 async categoryLineAnimations(element:Locator){return element.evaluate(e=>e.getAnimations({subtree:true}).filter(a=>a.effect instanceof KeyframeEffect&&a.effect.pseudoElement==='::after').map(a=>({name:a instanceof CSSAnimation?a.animationName:'',property:a instanceof CSSTransition?a.transitionProperty:'',state:a.playState})));}
 async hover(element:Locator){await element.hover();}
 async focus(element:Locator){await element.focus();}
 async click(element:Locator){await element.click();}
 async moveOffCategory(){const content=this.panel().locator('.Submenu-middle-right'),box=await content.boundingBox();if(!box)throw new Error('Product category panel is not visible');await this.page.mouse.move(box.x+box.width-8,box.y+box.height-8);}
 async keyboard(key:string){await this.page.keyboard.press(key);}
 async reduce(){await this.page.emulateMedia({reducedMotion:'reduce'});}
 async captureIndicator(path:string){await this.page.screenshot({path});}
 async installProbe(){
  await this.panel().evaluate(async panel=>{await Promise.all(panel.getAnimations().map(animation=>animation.finished.catch(()=>{})));});
  await this.panel().evaluate(panel=>{const probe=document.createElement('button');probe.id='indicator-probe';probe.className='lina-indicator';probe.textContent='Reusable indicator';probe.style.cssText='position:relative;margin:12px;padding:8px;border:0;--indicator-origin:right;--indicator-duration:400ms;--indicator-color:rgb(180, 20, 70);--indicator-thickness:4px';panel.querySelector('.Submenu-page-list')!.append(probe);});
  return this.panel().locator('#indicator-probe');
 }
 async sampleReplay(element:Locator,pseudo='::after',holdAtMiddle=false){
  return element.evaluate(async(e,{pseudo,holdAtMiddle})=>{
   const transition=e.getAnimations({subtree:true}).find(a=>a instanceof CSSAnimation&&a.animationName.startsWith('lina-category-')&&a.effect instanceof KeyframeEffect&&a.effect.pseudoElement===pseudo);
   if(!transition)return null;
   const duration=Number(transition.effect!.getTiming().duration);
   transition.pause();transition.currentTime=duration/2;await new Promise(requestAnimationFrame);
   const style=getComputedStyle(e,pseudo),matrix=new DOMMatrixReadOnly(style.transform==='none'?'matrix(1,0,0,1,0,0)':style.transform),middle=parseFloat(style.width)*Math.abs(matrix.a)/e.getBoundingClientRect().width;
   if(!holdAtMiddle)transition.play();return {middle,duration};
  },{pseudo,holdAtMiddle});
 }
 async resumeReplay(element:Locator,pseudo='::after'){await element.evaluate((e,pseudo)=>{e.getAnimations({subtree:true}).filter(a=>a.effect instanceof KeyframeEffect&&a.effect.pseudoElement===pseudo).forEach(a=>a.play());},pseudo);}
 async renderAgain(){
  return this.page.evaluate(async()=>{
   const {renderNavigation}=await import('/lina/navigation.js?v=20261025-bt');
   const adapters={safeURL:(value:string)=>new URL(value,location.origin),customerLink:()=>{},request:async()=>({categories:[],products:[]})};
   renderNavigation(document,[],adapters);renderNavigation(document,[],adapters);
   return document.querySelectorAll('.horizontal-menu-navbar-menu>li').length;
  });
 }
}

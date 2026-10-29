import {NavigationPage} from './NavigationPage';
export class NavigationLayerPage extends NavigationPage{
 async positionSectionNavigation(){await this.page.locator('[data-web-solution] .et-container').evaluate(el=>scrollTo({top:scrollY+el.getBoundingClientRect().top-250,behavior:'instant'}));}
 async overlap(){return this.page.evaluate(()=>{
  const bar=document.querySelector('[data-web-solution] .et-container')!,panel=document.querySelector('.lina-open .lina-nav-panel')!;
  const r=bar.getBoundingClientRect(),p=panel.getBoundingClientRect(),x=Math.max(r.left,p.left)+Math.min(r.width,p.width)/2,y=Math.max(r.top,p.top)+5;
  return {intersects:y<Math.min(r.bottom,p.bottom),menuOnTop:panel.contains(document.elementFromPoint(x,y))};
 });}
 async selectProblem(){await this.page.locator('.et-hero-tab a[href="#web-problems"]').click();}
}

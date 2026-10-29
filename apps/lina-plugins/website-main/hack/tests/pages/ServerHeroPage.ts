import {HomeDetailPage} from './HomeDetailPage';
export class ServerHeroPage extends HomeDetailPage{
 override async motion(value:'reduce'|'no-preference'){
  await this.page.evaluate(value=>{const state=window as typeof window & {serverMotionChanged?:Promise<void>};const media=matchMedia('(prefers-reduced-motion: reduce)');state.serverMotionChanged=media.matches===(value==='reduce')?Promise.resolve():new Promise(resolve=>media.addEventListener('change',()=>resolve(),{once:true}));},value);
  await super.motion(value);
  await this.page.clock.runFor(50);
  await this.page.evaluate(()=> (window as typeof window & {serverMotionChanged?:Promise<void>}).serverMotionChanged);
 }
 async refreshServers(){await this.page.evaluate(()=>dispatchEvent(new Event('focus')));}
 async retryServers(){await this.page.locator('.lina-server-status button').click();}
 async advance(ms:number){await this.page.clock.fastForward(ms);}
 async installClock(){const time=new Date('2026-10-21T12:00:00Z');await this.page.clock.install({time});await this.page.clock.pauseAt(time);}
 async hidePage(hidden:boolean){await this.page.evaluate(hidden=>{Object.defineProperty(document,'hidden',{configurable:true,value:hidden});document.dispatchEvent(new Event('visibilitychange'));},hidden);}
}

// This small module can paint public identity before the complete site module graph loads.
import {readServerBrandIdentity,renderBrandIdentity} from './brand-identity.js';
const render=()=>{const identity=readServerBrandIdentity();if(identity&&document.querySelector('.sky-footer-container'))renderBrandIdentity(document,identity);};
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',render,{once:true});else render();

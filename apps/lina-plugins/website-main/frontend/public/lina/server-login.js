import {localized} from './locale.js';
const copy=(zhCN,enUS)=>localized({zhCN,enUS});

export function generateServerPassword(policy={}){
 const alphabets={lowercase:'abcdefghijklmnopqrstuvwxyz',uppercase:'ABCDEFGHIJKLMNOPQRSTUVWXYZ',digits:'0123456789',symbols:'!@#$%*-_=+?'};
 const classes=(policy.characters?.length?policy.characters:['lowercase','uppercase','digits']).map(kind=>alphabets[kind]).filter(Boolean);
 if(!classes.length)return '';
 const index=size=>{const bytes=new Uint32Array(1),limit=Math.floor(0x100000000/size)*size;do{crypto.getRandomValues(bytes);}while(bytes[0]>=limit);return bytes[0]%size;};
 const all=classes.join(''),length=Math.min(20,Math.max(6,Number(policy.length)||16));
 const value=Array.from({length},(_,i)=>{const chars=classes[i]||all;return chars[index(chars.length)];});
 for(let i=value.length-1;i>0;i--){const j=index(i+1);[value[i],value[j]]=[value[j],value[i]];}
 return value.join('');
}

// Password bytes remain in the live form until the private write succeeds.
export function createServerLogin(container,save){
 let policy,token='',hostname='',passwordConfigured=false,dirty=false,renderedKey='';
 const element=(tag,text,cls)=>{const node=document.createElement(tag);if(text)node.textContent=text;if(cls)node.className=cls;return node;};
 function render(next,draft={},bindingKey=''){
  const key=bindingKey+JSON.stringify(next);if(renderedKey===key)return;renderedKey=key;
  policy=next;token=draft.serverLoginToken||'';hostname=draft.serverLoginHostname||'';passwordConfigured=Boolean(draft.serverLoginPasswordConfigured);dirty=false;
  container.replaceChildren();container.hidden=!policy?.hostnameEnabled&&!policy?.passwordEnabled;
  if(container.hidden)return;
  const tip=element('p',copy('选填，留空则自动生成。','Optional. Leave blank to generate automatically.'),'form-group-text');container.append(tip);
  const add=(name,id,type)=>{
   const row=element('div','','sky-form-group sky-config-row configureproduct'),label=element('label','','sky-md sky-form-label'),body=element('div','','sky-md-100-p'),input=element('input','','form-control');
   const title=element('div','','label-title');title.append(element('span',name));label.append(title);input.id=id;input.type=type;input.autocomplete=type==='password'?'new-password':'off';label.htmlFor=id;input.maxLength=type==='password'?20:63;
   input.addEventListener('input',()=>{dirty=true;});body.append(input);row.append(label,body);container.append(row);return {input,body};
  };
  if(policy.hostnameEnabled){const {input}=add(copy('主机名','Hostname'),'server-hostname','text');input.value=hostname;input.placeholder=copy('留空自动生成','Leave blank to generate automatically');}
  if(policy.passwordEnabled){
   const {input,body}=add(copy('主机密码','Host password'),'server-password','password');
   input.placeholder=copy(passwordConfigured?'密码已设置，留空保留原密码':'留空自动生成',passwordConfigured?'Password is set. Leave blank to keep it.':'Leave blank to generate automatically');
   const actions=element('div','','sky-btn-group'),show=element('button',copy('显示密码','Show password'),'btn sky-config-btn'),random=element('button',copy('随机生成','Generate password'),'btn sky-config-btn');
   show.type=random.type='button';show.addEventListener('click',()=>{input.type=input.type==='password'?'text':'password';show.textContent=input.type==='password'?copy('显示密码','Show password'):copy('隐藏密码','Hide password');});
   random.addEventListener('click',()=>{input.value=generateServerPassword(policy.password);dirty=true;});actions.append(show,random);body.append(actions);
   if(draft.passwordNeedsReentry)tip.textContent=copy('登录后请重新填写密码，或留空自动生成。','Enter the password again after signing in, or leave blank to generate automatically.');
  }
 }
 function snapshot(){return {serverLoginToken:token,serverLoginHostname:container.querySelector('#server-hostname')?.value||hostname,serverLoginPasswordConfigured:passwordConfigured};}
 return {
  render,
  snapshot,
  hasInput:()=>Boolean(token||container.querySelector('#server-hostname')?.value||container.querySelector('#server-password')?.value),
  passwordNeedsReentry:()=>Boolean(container.querySelector('#server-password')?.value),
  clearPassword(){const input=container.querySelector('#server-password');if(input){input.value='';input.type='password';}},
  async persist(){
   const name=container.querySelector('#server-hostname')?.value||'',password=container.querySelector('#server-password')?.value||'';
   if(!dirty)return token||undefined;
   if(!token&&!name&&!password)return undefined;
   const result=await save({hostname:name,password,...(token?{previousServerLoginToken:token}:{})});
   token=result.serverLoginToken;hostname=result.hostname||'';passwordConfigured=Boolean(result.passwordConfigured);dirty=false;
   const input=container.querySelector('#server-password');if(input){input.value='';input.type='password';input.placeholder=copy('密码已设置，留空保留原密码','Password is set. Leave blank to keep it.');}
   return token;
  },
 };
}

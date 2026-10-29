
// Defaults only fill missing specification choices; backend evaluation remains authoritative.
export function applyConfigurationDefaults(configuration){
 const values=configuration.values||(configuration.values=[]);let changed=false;
 for(const field of configuration.fields||[]){
  if(!field.visible||field.disabled)continue;
  const key={single_select:'optionTokens',multi_select:'optionTokens',tree_select:'treeOptionTokens',address:'addressOptionTokens',integer:'integerValue',decimal:'decimalValue'}[field.type];
  if(!key)continue;
  const current=values.find(value=>value.fieldToken===field.fieldToken),existing=current?.[key];
  if(field.type!=='address'&&existing!==undefined&&existing!==null&&existing!==''&&(!Array.isArray(existing)||existing.length))continue;
  let value;
  const first=options=>(options||[]).find(option=>!option.disabled&&option.optionToken);
  if(field.type==='address'){
   const path=[];let options=field.options;while(options?.length){const option=options.find(option=>!option.disabled&&option.optionToken===existing?.[path.length])||first(options);if(!option)break;path.push(option.optionToken);options=option.children;}if(JSON.stringify(path)===JSON.stringify(existing))continue;if(path.length)value=path;
  }else if(['single_select','multi_select','tree_select'].includes(field.type)){
   const option=first(field.options);if(option)value=[option.optionToken];
  }else value=field.minimumInteger??field.minimumDecimal;
  if(value===undefined)continue;
  const next={fieldToken:field.fieldToken,type:field.type,[key]:value};
  if(current)values.splice(values.indexOf(current),1,next);else values.push(next);changed=true;
 }
 return changed;
}


// Interpret spawnSync results without executing a tool or changing security policy.
export function buildFailure(result,command,cwd){
 if(result.error){
  const e=result.error;
  return `Failed to start ${command} in ${cwd}: ${e.code||e.name||'unknown'}: ${e.message}${e.syscall?' (syscall: '+e.syscall+')':''}${e.path?' (path: '+e.path+')':''}`;
 }
 if(result.signal)return `${command} terminated by signal ${result.signal} in ${cwd}`;
 if(result.status!==0)return `${command} exited with status ${result.status??'unknown'} in ${cwd}`;
 return '';
}

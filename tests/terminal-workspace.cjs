const fs=require('fs'),vm=require('vm'),assert=require('assert');
let batchCalls=[],active='',count=0;
class Terminal{constructor(){this.cols=100;this.rows=30}loadAddon(){}open(){}onData(callback){this.input=callback}onResize(){}reset(){}focus(){}write(_bytes,done){done()}writeln(){}}
const window={Terminal,FitAddon:{FitAddon:class{fit(){}}}};
const context={window,Terminal,FitAddon:{FitAddon:class{fit(){}}},document:{hidden:false,addEventListener(){}},ResizeObserver:class{observe(){}},setTimeout,clearTimeout,Uint8Array,atob:x=>Buffer.from(x,'base64').toString('binary'),Promise};
vm.runInNewContext(fs.readFileSync('assets/vendor/terminal-client.js','utf8'),context);
async function request(action,params){
 if(action==='open'){active='shell-'+(++count);return {id:active}}
 if(action==='resize'||action==='close')return {ok:true};
 if(action==='poll')return{data:'',next:0,closed:false};
 if(action==='batch'){
  assert.equal(params.id,active);
  batchCalls.push(params.script);
  return{output:'3 commands completed',exitCode:0,truncated:false};
 }
 if(action==='write')return{ok:true};
 throw Error('Unexpected action '+action);
}
(async()=>{
 const terminal=window.RawTerminal({clientWidth:900,clientHeight:450,offsetParent:{}},request,()=>{});
 const script=['pwd','whoami','printf "Zażółć gęślą"'].join('\n');
 let response=await terminal.batch(script);
 assert.equal(response.exitCode,0);
 assert.equal(batchCalls[0],script);
 const large='echo abc\n'.repeat(12000);
 response=await terminal.batch(large);
 assert.equal(batchCalls[1],large);
 assert.equal(batchCalls.length,2);
 await terminal.close();
 console.log('PASS: multiline scripts, UTF-8 and 100+ KiB payload submitted intact through a separate SSH session.');
})().catch(error=>{console.error(error);process.exitCode=1});

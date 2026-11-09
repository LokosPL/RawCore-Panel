/* Local rich-text renderer. Server text is always data, never HTML. */
(function(root){
'use strict';
const palette=['#000000','#0000aa','#00aa00','#00aaaa','#aa0000','#aa00aa','#ffaa00','#aaaaaa','#555555','#5555ff','#55ff55','#55ffff','#ff5555','#ff55ff','#ffff55','#ffffff'];
const ansiPalette=[0,4,2,6,1,5,3,7,8,12,10,14,9,13,11,15].map(i=>palette[i]);
function ansiColor(n){if(n>=0&&n<16)return ansiPalette[n];if(n<232){let v=n-16,levels=[0,95,135,175,215,255];return '#'+[levels[Math.floor(v/36)],levels[Math.floor(v/6)%6],levels[v%6]].map(x=>x.toString(16).padStart(2,'0')).join('')}if(n<256){let x=(8+(n-232)*10).toString(16).padStart(2,'0');return '#'+x+x+x}return null}
function parseConsoleText(input,initial={}){
 let text=String(input??'').replace(/\x1b\][\s\S]*?(?:\x07|\x1b\\)/g,''),runs=[],style={...initial},last=0;
 const codes=/\x1b\[[0-?]*[ -/]*[@-~]|§x(?:§[0-9a-f]){6}|§#[0-9a-f]{6}|§[0-9a-fk-or]/gi;
 function push(value){if(value)runs.push({text:value.replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g,''),...style})}
 for(let match; (match=codes.exec(text));){push(text.slice(last,match.index));let code=match[0];last=codes.lastIndex;
  if(code[0]==='§'){
   let key=code[1].toLowerCase();
   if(key==='x'||key==='#'){style={color:key==='x'?'#'+code.slice(2).replaceAll('§',''):code.slice(1)};continue}
   if(/[0-9a-f]/i.test(key)){style={color:palette[parseInt(key,16)]};continue}
   if(key==='r')style={};if(key==='l')style.bold=true;if(key==='o')style.italic=true;if(key==='n')style.underline=true;if(key==='m')style.strike=true;
  }else if(code.endsWith('m')){
   let values=code.slice(2,-1).split(';').map(v=>Number(v||0));
   for(let i=0;i<values.length;i++){let n=values[i];if(n===0)style={};else if(n===1)style.bold=true;else if(n===3)style.italic=true;else if(n===4)style.underline=true;else if(n===9)style.strike=true;else if(n===22)delete style.bold;else if(n===23)delete style.italic;else if(n===24)delete style.underline;else if(n===29)delete style.strike;else if(n===39)delete style.color;else if(n>=30&&n<=37)style.color=ansiPalette[n-30];else if(n>=90&&n<=97)style.color=ansiPalette[n-90+8];else if(n===38&&values[i+1]===2&&values.slice(i+2,i+5).length===3&&values.slice(i+2,i+5).every(v=>Number.isInteger(v)&&v>=0&&v<=255)){style.color='#'+values.slice(i+2,i+5).map(v=>v.toString(16).padStart(2,'0')).join('');i+=4}else if(n===38&&values[i+1]===5&&Number.isInteger(values[i+2])){let color=ansiColor(values[i+2]);if(color)style.color=color;i+=2}}
  }
 }
 push(text.slice(last));return {runs,style};
}
function consoleSeverity(text){
 const plain=parseConsoleText(text).runs.map(r=>r.text).join('');
 if(/\b(?:ERROR|FATAL|SEVERE|CRITICAL)\b[\]: ]|\[START BLOCK\]|(?:[\w.]+(?:Exception|Error)|Error):|OutOfMemoryError|Traceback \(most recent call last\):/.test(plain)||/\[(?:error|fatal|severe|critical)\]/i.test(plain))return 'error';
 if(/\bWARN(?:ING)?\b[\]: ]|\[(?:warn|warning)\]/i.test(plain))return 'warn';
 return 'info';
}
function appendRichLines(container,text,initial={}){
 let style={...initial},lines=String(text).replace(/\r\n?/g,'\n').match(/[^\n]*\n|[^\n]+$/g)||[];
 const fragment=document.createDocumentFragment();
 for(let line of lines){let parsed=parseConsoleText(line,style),row=document.createElement('span');row.className='logline log-'+consoleSeverity(line);row._inputStyle={...style};row._outputStyle={...parsed.style};style=parsed.style;
  for(let run of parsed.runs){let span=document.createElement('span');span.textContent=run.text;if(/^#[0-9a-f]{6}$/i.test(run.color||''))span.style.color=run.color;if(run.bold)span.style.fontWeight='700';if(run.italic)span.style.fontStyle='italic';if(run.underline||run.strike)span.style.textDecoration=[run.underline?'underline':'',run.strike?'line-through':''].filter(Boolean).join(' ');row.append(span)}fragment.append(row)
 }
 container.append(fragment);return style;
}
function updateColoredConsole(container,text,previous){
 let offset=0,style={};
 if(previous&&text.startsWith(previous)&&container.lastChild?.classList?.contains('logline')){
  offset=previous.length;style=container.lastChild._outputStyle||{};
  if(!previous.endsWith('\n')){offset=previous.lastIndexOf('\n')+1;style=container.lastChild._inputStyle||{};container.lastChild.remove()}
 }else container.replaceChildren();
 appendRichLines(container,text.slice(offset),style);
}
function renderServerReply(container,command,output){container.replaceChildren();let title=document.createElement('strong'),request=document.createElement('div'),body=document.createElement('pre');title.textContent='Odpowiedź serwera Minecraft';request.className='replycommand';request.textContent='> '+command;body.className='replybody';let text=String(output||'Wysłano.');if(text.length>1048576)text=text.slice(0,1048576)+'\n[Podgląd skrócono do 1 MB]';appendRichLines(body,text);container.append(title,request,body)}
Object.assign(root,{parseConsoleText,consoleSeverity,appendRichLines,updateColoredConsole,renderServerReply});
if(typeof module!=='undefined')module.exports={parseConsoleText,consoleSeverity};
})(typeof window!=='undefined'?window:globalThis);

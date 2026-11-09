package main

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "net/http"
 "strings"
 "sync"
 "time"
)

// Disk scans are cached; log checks continue independently of the visible console.
var diskCache = struct { sync.Mutex; Key string; At time.Time; Data json.RawMessage }{}
const monitorPython = `import os,sys,json,time,re,hashlib,datetime,subprocess,concurrent.futures
request=json.loads(sys.argv[1]);now=time.time();processes=request['processes'];server=request['serverPath']
level=re.compile(r'(?:\b(?:ERROR|FATAL|SEVERE|CRITICAL)\b[\]: ]|\[START BLOCK\]|\b(?:UnhandledPromiseRejection(?:Warning)?|unhandledRejection|uncaughtException|OutOfMemoryError)\b|Traceback \(most recent call last\):|Exception in thread|(?<![\w.])(?:[A-Za-z_][\w.]*(?:Exception|Error)|Error):|(?i:\[(?:error|fatal|severe|critical)\]))')
ansi=re.compile(r'\x1b\[[0-9;]*[A-Za-z]')
def stamp(line,mtime):
 m=re.search(r'\b(\d{4}-\d\d-\d\d)[T ](\d\d:\d\d:\d\d)(?:\.\d+)?(?:\s*([+-]\d\d:\d\d|Z))?',line)
 if m:
  try: return datetime.datetime.fromisoformat(m[1]+'T'+m[2]+(m[3] or '')).timestamp()
  except ValueError: pass
 m=re.search(r'\[(\d\d:\d\d:\d\d)\s+(?:ERROR|FATAL|SEVERE|WARN|INFO)',line)
 if m:
  today=datetime.datetime.fromtimestamp(mtime);d=datetime.datetime.combine(today.date(),datetime.time.fromisoformat(m[1])).timestamp()
  return d-86400 if d>mtime+60 else d
 return mtime
def scan_logs(p):
 env=p.get('pm2_env',{});started=float(env.get('pm_uptime') or 0)/1000;events={};issues=[]
 paths=list(dict.fromkeys(x for x in [env.get('pm_out_log_path'),env.get('pm_err_log_path'),os.path.join(server,'logs','latest.log') if p['name']==request['minecraftProcess'] else None] if x))
 for path in paths:
  try:
   with open(path,'rb') as f:
    s=os.fstat(f.fileno())
    if s.st_mtime<max(started,now-900): continue
    offset=max(0,s.st_size-131072);f.seek(offset);data=f.read(131072)
    if offset: data=data.partition(b'\n')[2]
   for raw in data.decode('utf-8',errors='replace').splitlines():
    line=ansi.sub('',raw).strip();when=stamp(line,s.st_mtime)
    if when<max(started,now-900) or not level.search(line): continue
    # Keep distinct incidents, collapse periodic repetitions and the same entry in PM2/latest.log.
    message=re.sub(r'\[\d\d:\d\d:\d\d ([^\]]+)\]',r'[\1]',line)
    message=re.sub(r'^\d{4}-\d\d-\d\d[T ][\d:.]+(?:\s*[+-]\d\d:\d\d)?[ :]*','',message)[:700]
    key=hashlib.sha256((str(p['pm_id'])+'|'+str(started)+'|'+message).encode()).hexdigest()[:24]
    if key not in events or events[key]['time']<when: events[key]={'key':key,'message':message,'time':when,'source':os.path.basename(path)}
  except FileNotFoundError: pass
  except (OSError,ValueError) as e: issues.append(os.path.basename(path)+': '+str(e))
 return {'id':p['pm_id'],'name':p['name'],'errors':sorted(events.values(),key=lambda e:e['time'],reverse=True)[:12],'issues':issues}
def disk(path):
 result={'path':path,'checkedAt':now}
 try:
  if not os.path.isabs(path) or os.path.realpath(path)=='/': raise ValueError('Nie skanujemy całego /. Ustaw katalog usługi.')
  target=os.path.realpath(path)
  run=subprocess.run(['du','-B1','--max-depth=1','--',target],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
  rows=[]
  for line in run.stdout.decode('utf-8',errors='replace').splitlines():
   n,name=line.split('\t',1);display=path+name[len(target):] if name==target or name.startswith(target+'/') else name;rows.append({'path':display,'bytes':int(n)})
  root=next((r for r in rows if r['path'].rstrip('/')==path.rstrip('/')),None)
  if root is None: raise ValueError(run.stderr.decode('utf-8',errors='replace')[:200] or 'Nie można odczytać katalogu')
  result.update({'bytes':root['bytes'],'folders':sorted([r for r in rows if r is not root],key=lambda r:r['bytes'],reverse=True)[:8],'partial':run.returncode!=0})
 except (OSError,ValueError,subprocess.TimeoutExpired) as e: result['error']=str(e)[:240]
 return result
roots={}
for p in processes:
 path=server if p['name']==request['minecraftProcess'] else p.get('pm2_env',{}).get('pm_cwd','')
 if path: roots[p['pm_id']]=os.path.normpath(path)
disks=request.get('disks')
if disks is None:
 unique=list(dict.fromkeys(roots.values()))
 with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool: disks=list(pool.map(disk,unique[:6]))
 disks.extend({'path':p,'error':'Pomiar ograniczony do 6 katalogów usług','checkedAt':now} for p in unique[6:])
print(json.dumps({'checkedAt':now,'services':[dict(scan_logs(p),path=roots.get(p['pm_id'],'')) for p in processes[:24]],'disks':disks}))`

func handleMonitor(w http.ResponseWriter,r *http.Request) {
 c,e:=currentCred();if e!=nil {bad(w,e);return}
 ctx,cancel:=context.WithTimeout(r.Context(),40*time.Second);defer cancel()
 raw,e:=runRemote(ctx,c,pm2prefix()+"pm2 jlist 2>/dev/null");if e!=nil{bad(w,e);return}
 var processes []Process
 start,end:=strings.Index(raw,"["),strings.LastIndex(raw,"]")
 if start<0||end<start||json.Unmarshal([]byte(raw[start:end+1]),&processes)!=nil{bad(w,errors.New("Nie można odczytać usług do monitorowania"));return}
 if len(processes)>24{processes=processes[:24]}
 instance,e:=requestInstance(r);if e!=nil{bad(w,e);return};path,mc:=instance.ServerPath,instance.MinecraftProcess
 filtered:=make([]Process,0);for _,p:=range processes{if p.Name==instance.MinecraftProcess||p.Name==instance.WWWProcess{filtered=append(filtered,p)}};processes=filtered
 privileged,e:=currentPrivilegedCred();if e!=nil||privileged.Host!=c.Host||privileged.User!=c.User||privileged.Port!=c.Port||privileged.Password!=c.Password{bad(w,errors.New("Połączenie zostało zmienione"));return}
 key:=c.Host+":"+fmt.Sprint(c.Port)+":"+c.User+":"+path+":"+string(identityPaths(processes))
 if privileged.privileged{key+="|sudo"}
 request:=map[string]any{"processes":processes,"serverPath":path,"minecraftProcess":mc}
 diskCache.Lock();if diskCache.Key==key&&time.Since(diskCache.At)<90*time.Second&&diskCache.Data!=nil{request["disks"]=diskCache.Data};diskCache.Unlock()
 payload,_:=json.Marshal(request)
 raw,e=runRemote(ctx,privileged,remPython(monitorPython,string(payload)));if e!=nil{bad(w,e);return}
 var result struct {Disks json.RawMessage `json:"disks"`}
 if json.Unmarshal([]byte(raw),&result)!=nil{bad(w,errors.New("Niepoprawne dane monitoringu"));return}
 diskCache.Lock();if diskCache.Key!=key||request["disks"]==nil{diskCache.Key=key;diskCache.At=time.Now();diskCache.Data=append(json.RawMessage(nil),result.Disks...)};diskCache.Unlock()
 w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(raw))
}
func identityPaths(processes []Process) []byte { paths:=make([]string,0,len(processes));for _,p:=range processes{paths=append(paths,p.Name+":"+p.Env.Cwd)};b,_:=json.Marshal(paths);return b }

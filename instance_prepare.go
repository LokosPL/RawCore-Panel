package main

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "net/http"
    "path"
    "strings"
    "sync"
    "time"
)

var instancePrepareMu sync.Mutex
type InstancePreparation struct {
    Instance ServerInstance `json:"instance"`
    SourceJar string `json:"sourceJar"`
    Action string `json:"action"`
    PlanHash string `json:"planHash"`
}
const prepareInstancePython = `import os,sys,json,socket,subprocess,tempfile,shutil,secrets,stat
r=json.loads(sys.argv[1]);s=r['instance'];root=s['serverPath'];source=r['sourceJar'];items=r['existing']
if not os.path.isabs(root) or os.path.normpath(root)!=root or root=='/': raise SystemExit('Niepoprawny katalog instancji')
if os.path.lexists(root): raise SystemExit('Katalog docelowy już istnieje. Niczego nie nadpisano.')
parent=os.path.dirname(root)
if not os.path.isdir(parent) or os.path.realpath(parent)!=parent: raise SystemExit('Katalog nadrzędny musi istnieć i nie może przechodzić przez symlinki')
if os.path.realpath(source)!=source or not os.path.isfile(source) or not source.endswith('.jar'): raise SystemExit('Wskaż istniejący zwykły JAR silnika Paper, bez symlinków')
if not any(os.path.commonpath((source,i['serverPath']))==i['serverPath'] for i in items): raise SystemExit('JAR źródłowy musi należeć do zapisanej instancji')
for i in items:
 a=os.path.realpath(i['serverPath'])
 if os.path.commonpath((root,a)) in (root,a): raise SystemExit('Katalog koliduje z istniejącą instancją')
try:
 processes=json.loads(subprocess.check_output(['pm2','jlist'],timeout=8))
except FileNotFoundError: processes=[]
except Exception: raise SystemExit('Nie udało się sprawdzić procesów PM2')
if any(p.get('name')==s['minecraftProcess'] or os.path.realpath(p.get('pm2_env',{}).get('pm_cwd','/'))==root for p in processes): raise SystemExit('Konflikt nazwy lub katalogu procesu PM2')
def properties(file):
 if not os.path.exists(file): return {}
 if os.path.getsize(file)>65536 or not os.path.isfile(file): raise SystemExit('Nie można bezpiecznie odczytać konfiguracji portów istniejącej instancji')
 out={}
 with open(file,encoding='utf-8') as f:
  for raw in f:
   line=raw.strip()
   if line and not line.startswith(('#','!')) and '=' in line:
    key,value=line.split('=',1);out[key.strip()]=value.strip()
 return out
for i in items:
 props=properties(os.path.join(i['serverPath'],'server.properties'))
 configured={int(props.get('server-port','25565'))}
 if props.get('enable-rcon','false').lower()=='true': configured.add(int(props.get('rcon.port','25575')))
 if props.get('enable-query','false').lower()=='true': configured.add(int(props.get('query.port','25565')))
 if os.path.isdir(os.path.join(i['serverPath'],'plugins','Core')):
  web=properties(os.path.join(i['serverPath'],'plugins','Core','website','server.properties'));configured.add(int(web.get('port','8766')))
 if configured.intersection((s['port'],s['rconPort'],s['corePort'])): raise SystemExit('Port koliduje z konfiguracją istniejącej (również zatrzymanej) instancji')
sockets=[]
try:
 for port in (s['port'],s['rconPort'],s['corePort']):
  for family,host in ((socket.AF_INET,'0.0.0.0'),(socket.AF_INET6,'::')):
   try:
    sock=socket.socket(family,socket.SOCK_STREAM)
    if family==socket.AF_INET6: sock.setsockopt(socket.IPPROTO_IPV6,socket.IPV6_V6ONLY,1)
    sock.bind((host,port));sockets.append(sock)
   except OSError as e:
    if family==socket.AF_INET6 and e.errno in (97,99): continue
    raise SystemExit('Port '+str(port)+' jest zajęty lub niedostępny')
 java=subprocess.run([s['java'],'-version'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=8)
 if java.returncode: raise SystemExit('Java nie jest dostępna')
 javaVersion=(java.stderr+java.stdout).decode('utf-8',errors='replace')[:600]
 if r['action']=='create':
  stage=tempfile.mkdtemp(prefix='.rawcraft-instance-',dir=parent)
  try:
   shutil.copyfile(source,os.path.join(stage,'server.jar'))
   os.mkdir(os.path.join(stage,'plugins'),0o700);os.mkdir(os.path.join(stage,'plugins','Core'),0o700);os.mkdir(os.path.join(stage,'plugins','Core','website'),0o700)
   def write(name,data):
    with open(os.path.join(stage,name),'x',encoding='utf-8') as f: f.write(data);f.flush();os.fsync(f.fileno())
    os.chmod(os.path.join(stage,name),0o600)
   write('server.properties','server-port='+str(s['port'])+'\nonline-mode=true\naccepts-transfers=true\nenable-rcon=true\nrcon.port='+str(s['rconPort'])+'\nrcon.password='+secrets.token_urlsafe(32)+'\nenable-query=false\nlevel-name=world\n')
   write('eula.txt','eula=false\n')
   write('plugins/Core/website/server.properties','bind=127.0.0.1\nport='+str(s['corePort'])+'\napi-key='+secrets.token_urlsafe(40)+'\n')
   write('plugins/Core/website/integration.yml','most:\n  wlaczone: false\nplatnosci:\n  wlaczone: false\n  tryb-testowy: true\n')
   write('plugins/Core/DataBase.json',json.dumps({'enabled':True,'type':'SQLITE','sqliteFile':'core-isolated.db'},indent=2)+'\n')
   write('plugins/Core/Voting.json',json.dumps({'enabled':False,'nuVotifierEnabled':False,'lists':[{'id':i,'title':'Głosowanie '+str(i),'url':'','serviceName':'','enabled':False} for i in range(1,4)]},indent=2)+'\n')
   write('plugins/Core/Transfer.json',json.dumps({'enabled':False,'instance':s['id'] if s['id'] in ('normalny','testowy','lobby') else 'testowy','secret':'','targets':{}},indent=2)+'\n')
   write('ecosystem.config.json',json.dumps({'apps':[{'name':s['minecraftProcess'],'cwd':root,'script':s['java'],'interpreter':'none','args':['-Xms512M','-Xmx'+str(s['ram'])+'M','-jar','server.jar','--nogui'],'autorestart':False}]},indent=2)+'\n')
   write('INSTANCE-README.txt','Instancja '+s['name']+'\nZadeklarowana wersja: '+s['version']+'\nNie uruchomiono ani nie dodano procesu do PM2.\nSprawdź wersję server.jar i Java. Zainstaluj Core z GitHub Releases.\nEULA niezaakceptowana. SQL jest odrębny (SQLite); żadnych danych produkcyjnych nie skopiowano.\nDiscord, płatności, głosowania i transfer pozostają nieaktywne do osobnej konfiguracji.\nSprawdź bind RCON/firewall. Dopiero po kontroli i świadomej akceptacji EULA możesz uruchomić PM2 z ecosystem.config.json.\n')
   # No-replace directory creation: another operation must not be overwritten by rename.
   os.mkdir(root,0o700)
   for name in os.listdir(stage): os.rename(os.path.join(stage,name),os.path.join(root,name))
  finally: shutil.rmtree(stage,ignore_errors=True)
 print(json.dumps({'prepared':r['action']=='create','javaVersion':javaVersion,'path':root,'port':s['port'],'rconPort':s['rconPort'],'corePort':s['corePort'],'process':s['minecraftProcess'],'version':s['version'],'ram':s['ram'],'database':'SQLite, osobny plik core-isolated.db','started':False}))
finally:
 for sock in sockets: sock.close()
`
func handlePrepareInstance(w http.ResponseWriter,r *http.Request){
    instancePrepareMu.Lock();defer instancePrepareMu.Unlock()
    var req InstancePreparation;if e:=fromJSON(r,&req);e!=nil{bad(w,e);return}
    if req.Action!="preview"&&req.Action!="create"{bad(w,errors.New("Wybierz podgląd lub przygotowanie"));return}
    s:=req.Instance;s.Revision="";req.Instance=s
    settings.RLock();snapshot:=settings.Prefs;existing:=normalizedInstances(snapshot);settings.RUnlock()
    if e:=validateInstances(append(append([]ServerInstance(nil),existing...),s));e!=nil{bad(w,e);return}
    if s.Port<1||s.RCONPort<1||s.CorePort<1||s.RAM<512||s.RAM>65536||s.Java==""||!path.IsAbs(s.Java)||strings.ContainsAny(s.Java,"\x00\r\n")||len(s.Version)>80||s.Version==""||!strings.HasSuffix(req.SourceJar,".jar"){bad(w,errors.New("Uzupełnij porty, Java, wersję, JAR i RAM 512–65536 MB"));return}
    plan:=req;plan.Action="";plan.PlanHash="";raw,_:=json.Marshal([]any{plan,snapshot.Host,snapshot.Port,snapshot.User,existing});hash:=sha256.Sum256(raw);digest:=hex.EncodeToString(hash[:])
    if req.Action=="create"&&req.PlanHash!=digest{bad(w,errors.New("Plan zmienił się. Wykonaj podgląd ponownie."));return}
    c,e:=currentCred();if e!=nil{bad(w,e);return}
    if c.Host!=snapshot.Host||c.User!=snapshot.User||c.Port!=snapshot.Port{bad(w,errors.New("Profil SSH zmienił się"));return}
    payload,_:=json.Marshal(map[string]any{"instance":s,"sourceJar":req.SourceJar,"existing":existing,"action":req.Action})
    ctx,cancel:=context.WithTimeout(r.Context(),2*time.Minute);defer cancel()
    out,e:=runRemote(ctx,c,pm2prefix()+remPython(prepareInstancePython,string(payload)));if e!=nil{bad(w,e);return}
    var result map[string]any;if json.Unmarshal([]byte(out),&result)!=nil{bad(w,errors.New("Niepoprawna odpowiedź kreatora"));return}
    if req.Action=="create"{
        settings.Lock();p:=settings.Prefs
        if p.Host!=snapshot.Host||p.User!=snapshot.User||p.Port!=snapshot.Port{settings.Unlock();bad(w,errors.New("Pliki przygotowano, ale profil SSH zmienił się. Dodaj instancję ręcznie; niczego nie uruchomiono."));return}
        p.Instances=append(normalizedInstances(p),s)
        e=validateInstances(p.Instances);if e==nil{e=writePreferences(p)};if e==nil{settings.Prefs=p};settings.Unlock()
        if e!=nil{bad(w,errors.New("Pliki przygotowano, ale nie zapisano listy instancji: "+e.Error()));return}
    }
    result["planHash"]=digest;ok(w,result)
}

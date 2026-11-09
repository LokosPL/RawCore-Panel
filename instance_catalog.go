package main

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "net/http"
 "path"
 "regexp"
 "strings"
 "sync"
 "time"
)

type CatalogInstanceRequest struct {
 Instance ServerInstance `json:"instance"`
 Plugins []string `json:"plugins"`
 Action string `json:"action"`
 PlanHash string `json:"planHash"`
}

var catalogMu sync.Mutex
var catalogAllowed = map[string]bool{"viaversion":true,"viabackwards":true,"viarewind":true,"worldedit":true}
var catalogID = regexp.MustCompile("^[a-z0-9][a-z0-9_-]{0,31}$")

const prepareCatalogPython = `import os,sys,json,socket,subprocess,tempfile,shutil,secrets,hashlib,urllib.request,urllib.parse,urllib.error,zipfile,stat
from pathlib import Path
r=json.loads(sys.argv[1]);s=r['instance'];root=s['serverPath'];items=r['existing'];chosen=r['plugins'];action=r['action']
allowed={'viaversion':'ViaVersion','viabackwards':'ViaBackwards','viarewind':'ViaRewind','worldedit':'WorldEdit'}
if len(chosen)!=len(set(chosen)) or any(x not in allowed for x in chosen): raise SystemExit('Nieobsługiwany albo powtórzony plugin')
if 'viabackwards' in chosen and 'viaversion' not in chosen: raise SystemExit('ViaBackwards wymaga ViaVersion')
if 'viarewind' in chosen and not {'viaversion','viabackwards'}.issubset(chosen): raise SystemExit('ViaRewind wymaga ViaVersion i ViaBackwards')
if not os.path.isabs(root) or os.path.normpath(root)!=root or root=='/': raise SystemExit('Niepoprawny katalog docelowy')
if os.path.lexists(root): raise SystemExit('Katalog już istnieje — żadnych danych nie nadpisano')
parent=os.path.dirname(root);home=os.path.realpath(os.path.expanduser('~'))
if not os.path.isdir(parent):
 if not os.path.commonpath((parent,home))==home: raise SystemExit('Najpierw utwórz katalog nadrzędny poza folderem domowym')
 if os.path.islink(parent) or any(os.path.islink(os.path.join(home,*parent[len(home):].lstrip('/').split('/')[:i])) for i in range(1,len(parent[len(home):].lstrip('/').split('/'))+1)): raise SystemExit('Symlink w katalogu nadrzędnym')
else:
 if os.path.realpath(parent)!=parent: raise SystemExit('Katalog nadrzędny nie może być symlinkiem')
 if action=='create' and not os.access(parent,os.W_OK|os.X_OK): raise SystemExit('Brak praw do zapisu katalogu nadrzędnego')
for item in items:
 existing=os.path.realpath(item['serverPath'])
 if os.path.commonpath((existing,root)) in (existing,root): raise SystemExit('Nowy katalog koliduje z inną instancją')
try:
 processes=json.loads(subprocess.check_output(['pm2','jlist'],timeout=8))
except FileNotFoundError: processes=[]
except Exception: raise SystemExit('Nie udało się sprawdzić PM2')
if any(p.get('name')==s['minecraftProcess'] or os.path.realpath(p.get('pm2_env',{}).get('pm_cwd','/'))==root for p in processes): raise SystemExit('Konflikt nazwy lub katalogu PM2')
for item in items:
 props=os.path.join(item['serverPath'],'server.properties')
 if os.path.isfile(props):
  if os.path.getsize(props)>65536: raise SystemExit('Zbyt duża konfiguracja istniejącej instancji')
  lines=[line.strip().split('=',1) for line in open(props,encoding='utf-8') if '=' in line and not line.lstrip().startswith(('#','!'))]
  data={k.strip():v.strip() for k,v in lines}
  existing_ports={int(data.get('server-port','25565'))}
  if data.get('enable-rcon')=='true': existing_ports.add(int(data.get('rcon.port','25575')))
  if existing_ports.intersection((s['port'],s['rconPort'],s['corePort'])): raise SystemExit('Konflikt portów zapisanej instancji')
sockets=[]
try:
 for port in (s['port'],s['rconPort'],s['corePort']):
  for family,host in ((socket.AF_INET,'0.0.0.0'),(socket.AF_INET6,'::')):
   try:
    sock=socket.socket(family,socket.SOCK_STREAM)
    if family==socket.AF_INET6:sock.setsockopt(socket.IPPROTO_IPV6,socket.IPV6_V6ONLY,1)
    sock.bind((host,port));sockets.append(sock)
   except OSError as ex:
    if family==socket.AF_INET6 and ex.errno in (97,99):continue
    raise SystemExit('Port '+str(port)+' jest zajęty. Zatrzymaj starą usługę lub wybierz inny port.')
 java=subprocess.run([s['java'],'-version'],capture_output=True,timeout=10)
 if java.returncode:raise SystemExit('Nie można uruchomić Java')
 version=(java.stderr+java.stdout).decode('utf-8',errors='replace')
 if '25' not in version.splitlines()[0]:raise SystemExit('Paper 26.3 wymaga Java 25; wybrany interpreter: '+version[:160])
 if action=='create':
  if not os.path.isdir(parent):
   os.makedirs(parent,mode=0o700,exist_ok=True)
  if os.path.realpath(parent)!=parent:raise SystemExit('Katalog nadrzędny przechodzi przez symlink')
  stage=tempfile.mkdtemp(prefix='.rawcraft-stage-',dir=parent)
  agent='RAWCRAFT-Control/1.5.4 (https://github.com/LokosPL/RawCore-Panel)'
  class SafeRedirect(urllib.request.HTTPRedirectHandler):
   def redirect_request(self,req,fp,code,msg,headers,newurl):
    if urllib.parse.urlsplit(newurl).scheme!='https' or urllib.parse.urlsplit(newurl).hostname not in ('fill.papermc.io','cdn.papermc.io','cdn.modrinth.com'):raise RuntimeError('Nieobsługiwane przekierowanie pobierania')
    return super().redirect_request(req,fp,code,msg,headers,newurl)
  opener=urllib.request.build_opener(SafeRedirect())
  def request(url):
   req=urllib.request.Request(url,headers={'User-Agent':agent})
   return opener.open(req,timeout=35)
  def fetch(url):
   with request(url) as response:
    data=response.read(8*1024*1024+1)
   if len(data)>8*1024*1024:raise RuntimeError('Za duża odpowiedź API')
   return json.loads(data)
  def download(url,target,checksums):
   parts=urllib.parse.urlsplit(url)
   if parts.scheme!='https' or parts.hostname not in ('fill.papermc.io','cdn.papermc.io','cdn.modrinth.com'):raise RuntimeError('Nieobsługiwany serwer pobierania')
   digest=(checksums or {}).get('sha512') or (checksums or {}).get('sha256')
   algorithm='sha512' if (checksums or {}).get('sha512') else 'sha256'
   if not digest or len(digest)!=(128 if algorithm=='sha512' else 64):raise RuntimeError('Brak wiarygodnej sumy kontrolnej '+target.name)
   h=hashlib.new(algorithm);size=0
   with request(url) as response,open(target,'xb') as f:
    while True:
     chunk=response.read(1024*512)
     if not chunk:break
     size+=len(chunk)
     if size>150*1024*1024:raise RuntimeError('JAR przekracza bezpieczny limit 150 MB')
     h.update(chunk);f.write(chunk)
   if h.hexdigest().lower()!=digest.lower():raise RuntimeError('Błędna suma kontrolna '+target.name)
   if not zipfile.is_zipfile(target):raise RuntimeError('Pobrany plik nie jest prawidłowym JAR')
  try:
   builds=fetch('https://fill.papermc.io/v3/projects/paper/versions/26.3/builds')
   if not isinstance(builds,list):raise RuntimeError('Nieprawidłowa odpowiedź Paper API')
   stable=next((v for v in builds if v.get('channel')=='STABLE' and 'server:default' in v.get('downloads',{})),None)
   if not stable:raise RuntimeError('Brak stabilnej wersji Paper 26.3')
   art=stable['downloads']['server:default']
   download(art['url'],Path(stage)/'server.jar',art.get('checksums',{}))
   (Path(stage)/'plugins').mkdir(mode=0o700)
   installed=[]
   for key in chosen:
    query=urllib.parse.urlencode({'game_versions':json.dumps(['26.3']),'loaders':json.dumps(['paper']),'include_changelog':'false'})
    versions=fetch('https://api.modrinth.com/v2/project/'+key+'/version?'+query)
    releases=sorted((x for x in versions if x.get('version_type')=='release' and x.get('status','listed')=='listed'),key=lambda x:x.get('date_published',''),reverse=True)
    if not releases:raise RuntimeError('Nie ma stabilnego, kompatybilnego wydania '+allowed[key]+' dla Paper 26.3')
    jars=[j for j in releases[0].get('files',[]) if j.get('filename','').endswith('.jar')]
    if not jars:raise RuntimeError('Brak JAR dla '+allowed[key])
    file=next((j for j in jars if j.get('primary')),jars[0])
    download(file['url'],Path(stage)/'plugins'/(allowed[key]+'.jar'),file.get('hashes',{}))
    installed.append(allowed[key]+' '+str(releases[0]['version_number']))
   def write(name,data):
    with open(os.path.join(stage,name),'x',encoding='utf-8') as f:f.write(data);f.flush();os.fsync(f.fileno())
   write('server.properties','server-port='+str(s['port'])+'\nonline-mode=true\nenable-rcon=false\nrcon.port='+str(s['rconPort'])+'\nenable-query=false\nmotd=RawCore Survival 26.3\nlevel-name=world\n')
   write('eula.txt','eula=false\n')
   write('ecosystem.config.json',json.dumps({'apps':[{'name':s['minecraftProcess'],'cwd':root,'script':s['java'],'interpreter':'none','args':['-Xms512M','-Xmx'+str(s['ram'])+'M','-jar','server.jar','--nogui'],'autorestart':True,'max_restarts':10}]},indent=2)+'\n')
   write('.rawcraft-panel-created',json.dumps({'id':s['id'],'process':s['minecraftProcess'],'version':'26.3','plugins':chosen})+'\n')
   write('INSTANCE-README.txt','Paper 26.3. Wybrane pluginy: '+(', '.join(installed) if installed else 'brak')+'\nSerwer nie uruchomił się. Włącz go przyciskiem po akceptacji EULA.\nRCON wyłączony; Core i MySQL skonfiguruj osobno.\n')
   os.mkdir(root,0o700)
   for name in os.listdir(stage):os.rename(os.path.join(stage,name),os.path.join(root,name))
  finally:shutil.rmtree(stage,ignore_errors=True)
 print(json.dumps({'prepared':action=='create','started':False,'path':root,'javaVersion':version[:220],'plugins':chosen,'ram':s['ram'],'version':'Paper 26.3','database':'Brak — Core zainstalujesz osobno','process':s['minecraftProcess']}))
finally:
 for sock in sockets:sock.close()
`
const startCatalogPython = `import os,sys,json,subprocess
s=json.loads(sys.argv[1]);root=s['serverPath'];identifier=s['id'];name=s['minecraftProcess']
if not os.path.isdir(root) or os.path.realpath(root)!=root:raise SystemExit('Katalog instancji jest niedostępny')
with open(os.path.join(root,'.rawcraft-panel-created'),encoding='utf-8') as f:meta=json.load(f)
if meta.get('id')!=identifier or meta.get('process')!=name:raise SystemExit('Instancja nie jest zgodna z profilem panelu')
if not os.path.isfile(os.path.join(root,'server.jar')):raise SystemExit('Nie ma pliku Paper')
with open(os.path.join(root,'ecosystem.config.json'),encoding='utf-8') as f:app=json.load(f)['apps'][0]
if app.get('name')!=name or app.get('cwd')!=root:raise SystemExit('Zmieniono konfigurację PM2')
procs=json.loads(subprocess.check_output(['pm2','jlist'],timeout=10))
if any(p.get('name')==name for p in procs):raise SystemExit('Proces o tej nazwie już istnieje w PM2 — nie uruchamiam drugiej kopii')
eula=os.path.join(root,'eula.txt')
with open(eula,encoding='utf-8') as f:current=f.read()
if current.strip() not in ('eula=false','eula=true'):raise SystemExit('Nieoczekiwana zawartość EULA')
if current.strip()=='eula=false':
 with open(eula,'w',encoding='utf-8') as f:f.write('eula=true\n')
result=subprocess.run(['pm2','start','ecosystem.config.json','--only',name],cwd=root,text=True,capture_output=True,timeout=35)
if result.returncode:raise SystemExit('Błąd PM2: '+result.stderr[-700:])
saved=subprocess.run(['pm2','save'],text=True,capture_output=True,timeout=15)
print(json.dumps({'started':True,'process':name,'pm2Saved':saved.returncode==0,'message':result.stdout[-900:]}))
`

func availableExistingInstances(p Preferences) []ServerInstance {
 if len(p.Instances)>0{return normalizedInstances(p)}
 if p.ServerPath=="/home/"+p.User{return []ServerInstance{}}
 return normalizedInstances(p)
}

func validateCatalogRequest(req CatalogInstanceRequest) error {
 s:=req.Instance
 if !catalogID.MatchString(s.ID)||len(s.Name)==0||len(s.Name)>80||s.MinecraftProcess==""{return errors.New("Podaj prawidłową nazwę i ID serwera")}
 if !path.IsAbs(s.ServerPath)||path.Clean(s.ServerPath)!=s.ServerPath||s.ServerPath=="/"||strings.ContainsAny(s.ServerPath,"\r\n\x00"){return errors.New("Nieprawidłowy katalog instancji")}
 if s.Port<1024||s.Port>65535||s.RCONPort<1024||s.RCONPort>65535||s.CorePort<1024||s.CorePort>65535||s.Port==s.RCONPort||s.Port==s.CorePort||s.RCONPort==s.CorePort{return errors.New("Porty muszą być różne i w zakresie 1024–65535")}
 if s.RAM<1024||s.RAM>65536||!path.IsAbs(s.Java)||strings.ContainsAny(s.Java,"\r\n\x00")||len(s.Java)>1024{return errors.New("Podaj RAM 1024–65536 MB i pełną ścieżkę Java 25")}
 if s.Version!="Paper 26.3"{return errors.New("Aktualnie kreator obsługuje Paper 26.3")}
 if len(req.Plugins)>len(catalogAllowed){return errors.New("Za dużo pluginów")}
 seen:=map[string]bool{}
 for _,p:=range req.Plugins {if !catalogAllowed[p]||seen[p]{return errors.New("Nieobsługiwany lub zduplikowany plugin")};seen[p]=true}
 if seen["viabackwards"]&&!seen["viaversion"]{return errors.New("ViaBackwards wymaga ViaVersion")}
 if seen["viarewind"]&&(!seen["viaversion"]||!seen["viabackwards"]){return errors.New("ViaRewind wymaga ViaVersion i ViaBackwards")}
 return nil
}

func handleCatalogPrepare(w http.ResponseWriter,r *http.Request) {
 catalogMu.Lock();defer catalogMu.Unlock()
 var req CatalogInstanceRequest
 if e:=fromJSON(r,&req);e!=nil{bad(w,e);return}
 if req.Action!="preview"&&req.Action!="create"{bad(w,errors.New("Wybierz podgląd lub przygotowanie"));return}
 req.Instance.Revision=""
 req.Instance.WWWProcess="" // WWW is a shared service and is not copied into a Minecraft instance.
 if e:=validateCatalogRequest(req);e!=nil{bad(w,e);return}
 settings.RLock();prefs:=settings.Prefs;existing:=availableExistingInstances(prefs);settings.RUnlock()
 if e:=validateInstances(append(append([]ServerInstance{},existing...),req.Instance));e!=nil{bad(w,e);return}
 plan:=req;plan.Action="";plan.PlanHash=""
 encoded,_:=json.Marshal([]any{plan,prefs.Host,prefs.Port,prefs.User,existing})
 hash:=sha256.Sum256(encoded);digest:=hex.EncodeToString(hash[:])
 if req.Action=="create"&&req.PlanHash!=digest{bad(w,errors.New("Parametry zmieniono — najpierw wyświetl plan ponownie"));return}
 c,e:=currentCred();if e!=nil{bad(w,e);return}
 if c.Host!=prefs.Host||c.User!=prefs.User||c.Port!=prefs.Port{bad(w,errors.New("Profil SSH uległ zmianie"));return}
 payload,_:=json.Marshal(map[string]any{"instance":req.Instance,"existing":existing,"plugins":req.Plugins,"action":req.Action})
 ctx,cancel:=context.WithTimeout(r.Context(),5*time.Minute);defer cancel()
 out,e:=runRemote(ctx,c,pm2prefix()+remPython(prepareCatalogPython,string(payload)))
 if e!=nil{bad(w,e);return}
 var result map[string]any
 if json.Unmarshal([]byte(out),&result)!=nil{bad(w,errors.New("Nieprawidłowa odpowiedź instalatora"));return}
 if req.Action=="create" {
  settings.Lock();p:=settings.Prefs
  if p.Host!=prefs.Host||p.User!=prefs.User||p.Port!=prefs.Port{settings.Unlock();bad(w,errors.New("Serwer przygotowany, ale profil SSH zmienił się"));return}
  if len(p.Instances)==0 {p.Instances=append([]ServerInstance{},existing...)}
  p.Instances=append(p.Instances,req.Instance)
  e=validateInstances(p.Instances);if e==nil{e=writePreferences(p)}
  if e==nil{settings.Prefs=p}
  settings.Unlock()
  if e!=nil{bad(w,errors.New("Pliki przygotowano, ale nie zapisano profilu serwera: "+e.Error()));return}
 }
 result["planHash"]=digest;ok(w,result)
}

func handleCatalogStart(w http.ResponseWriter,r *http.Request) {
 catalogMu.Lock();defer catalogMu.Unlock()
 var req struct {
  ID string `json:"id"`
  Revision string `json:"revision"`
  EULAAccepted bool `json:"eulaAccepted"`
 }
 if e:=fromJSON(r,&req);e!=nil{bad(w,e);return}
 if !req.EULAAccepted {bad(w,errors.New("Najpierw przeczytaj i zaakceptuj EULA Minecraft"));return}
 settings.RLock();p:=settings.Prefs;settings.RUnlock()
 var chosen *ServerInstance
 for _,item:=range p.Instances {if item.ID==req.ID&&instanceRevision(p,item)==req.Revision{copy:=item;chosen=&copy;break}}
 if chosen==nil{bad(w,errors.New("Nie znaleziono instancji lub zmieniono jej konfigurację"));return}
 c,e:=currentCred();if e!=nil{bad(w,e);return}
 if c.Host!=p.Host||c.User!=p.User||c.Port!=p.Port{bad(w,errors.New("Połączenie SSH zmieniło się"));return}
 payload,_:=json.Marshal(chosen)
 ctx,cancel:=context.WithTimeout(r.Context(),60*time.Second);defer cancel()
 out,e:=runRemote(ctx,c,pm2prefix()+remPython(startCatalogPython,string(payload)))
 if e!=nil{bad(w,e);return}
 var result map[string]any
 if e=json.Unmarshal([]byte(out),&result);e!=nil{bad(w,errors.New("Niepoprawna odpowiedź PM2"));return}
 ok(w,result)
}

package main

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "io"
    "net/http"
    "path"
    "regexp"
    "strings"
)

type ServerInstance struct {
    ID string `json:"id"`
    Name string `json:"name"`
    ServerPath string `json:"serverPath"`
    MinecraftProcess string `json:"minecraftProcess"`
    WWWProcess string `json:"wwwProcess"`
    Port int `json:"port"`
    RCONPort int `json:"rconPort"`
    CorePort int `json:"corePort"`
    Version string `json:"version"`
    Java string `json:"java"`
    RAM int `json:"ram"`
    Revision string `json:"revision,omitempty"`
}
type instanceContextKey struct{}
type instancePathKey struct{}
func instanceRevision(p Preferences,s ServerInstance) string {
    s.Revision="";raw,_:=json.Marshal([]any{p.Host,p.Port,p.User,s});sum:=sha256.Sum256(raw);return hex.EncodeToString(sum[:12])
}
func normalizedInstances(p Preferences) []ServerInstance {
    items:=append([]ServerInstance(nil),p.Instances...)
    if len(items)==0 {
        if p.ServerPath=="/home/"+p.User {
            items=[]ServerInstance{{ID:"vps",Name:"VPS / SSH",ServerPath:p.ServerPath,MinecraftProcess:p.MinecraftProcess,WWWProcess:p.WWWProcess}}
        } else {
            items=[]ServerInstance{{ID:"normalny",Name:"Serwer główny",ServerPath:p.ServerPath,MinecraftProcess:p.MinecraftProcess,WWWProcess:p.WWWProcess}}
        }
    }
    for i:=range items{items[i].Revision=instanceRevision(p,items[i])};return items
}
func validateInstances(items []ServerInstance) error {
    ids,processes,ports:=map[string]bool{},map[string]bool{},map[int]bool{}
    for i,s:=range items {
        if !regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`).MatchString(s.ID)||ids[s.ID]||strings.TrimSpace(s.Name)==""||len(s.Name)>80{return errors.New("Niepoprawna lub powtórzona nazwa instancji")};ids[s.ID]=true
        if checkPath(s.ServerPath)!=nil||path.Clean(s.ServerPath)!=s.ServerPath||s.ServerPath=="/"{return errors.New("Podaj osobny pełny katalog instancji")}
        for _,other:=range items[:i]{if withinPath(s.ServerPath,other.ServerPath)||withinPath(other.ServerPath,s.ServerPath){return errors.New("Katalogi instancji nie mogą na siebie nachodzić")}}
        for _,name:=range []string{s.MinecraftProcess,s.WWWProcess}{if name==""{continue};if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`).MatchString(name)||processes[name]{return errors.New("Konflikt nazw procesów PM2")};processes[name]=true}
        if s.MinecraftProcess==""{return errors.New("Podaj nazwę procesu Minecraft")}
        for _,port:=range []int{s.Port,s.RCONPort,s.CorePort}{if port==0{continue};if port<1||port>65535||ports[port]{return errors.New("Konflikt portów instancji")};ports[port]=true}
    }
    if len(items)>32{return errors.New("Maksymalnie 32 instancje")};return nil
}
func withinPath(value,root string) bool {value=path.Clean(value);root=path.Clean(root);return value==root||strings.HasPrefix(value,root+"/")}
func requestInstance(r *http.Request) (ServerInstance,error) {
    if s,ok:=r.Context().Value(instanceContextKey{}).(ServerInstance);ok{return s,nil}
    settings.RLock();p:=settings.Prefs;items:=normalizedInstances(p);settings.RUnlock()
    if c,e:=currentCred();e==nil&&(c.Host!=p.Host||c.Port!=p.Port||c.User!=p.User){return ServerInstance{},errors.New("Zapisany profil SSH różni się od aktywnego połączenia. Połącz VPS ponownie.")}
    id,revision:=r.Header.Get("X-RAWCRAFT-Instance"),r.Header.Get("X-RAWCRAFT-Revision")
    if id==""{id=r.URL.Query().Get("instance");revision=r.URL.Query().Get("revision")}
    // Compatibility with older single-instance callers only.
    if id==""&&len(p.Instances)==0{return items[0],nil}
    for _,s:=range items{if s.ID==id&&s.Revision==revision{return s,nil}}
    return ServerInstance{},errors.New("Instancja lub jej konfiguracja zmieniła się. Wybierz serwer ponownie.")
}
func scopedEndpoint(value string) bool {
    switch value {case "/api/status","/api/monitor","/api/payments/status","/api/action","/api/logs","/api/files","/api/read","/api/save","/api/upload","/api/download","/api/delete","/api/mkdir","/api/backups","/api/latest-log","/api/minecraft/command","/api/terminal","/api/shell":return true};return false
}
func scopeInstance(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter,r *http.Request){
        if !scopedEndpoint(r.URL.Path){next(w,r);return}
        s,e:=requestInstance(r);if e!=nil{jsonOut(w,409,map[string]string{"error":e.Error()});return}
        ctx:=context.WithValue(r.Context(),instanceContextKey{},s);r=r.WithContext(ctx)
        candidate:=r.URL.Query().Get("path")
        if r.Method=="POST"&&strings.Contains(r.Header.Get("Content-Type"),"application/json"){
            data,err:=io.ReadAll(io.LimitReader(r.Body,(1<<20)+1));if err!=nil||len(data)>1<<20{bad(w,errors.New("Za duże żądanie"));return};r.Body=io.NopCloser(bytes.NewReader(data))
            var body map[string]json.RawMessage
            if json.Unmarshal(data,&body)==nil{if raw,ok:=body["path"];ok{if json.Unmarshal(raw,&candidate)!=nil{bad(w,errors.New("Niepoprawna ścieżka"));return}}}
        }
        if candidate!=""&&!withinPath(candidate,s.ServerPath){
            backup:=r.URL.Path=="/api/download"&&path.Dir(candidate)==path.Join(path.Dir(s.ServerPath),".rawcraft-backups")&&strings.HasPrefix(path.Base(candidate),path.Base(s.ServerPath)+"-")&&strings.HasSuffix(candidate,".tar.gz")
            if !backup{bad(w,errors.New("Plik należy do innej instancji. Wybierz właściwy serwer."));return}
        }
        if candidate!=""{r=r.WithContext(context.WithValue(r.Context(),instancePathKey{},candidate))}
        next(w,r)
    }
}
func handleInstances(w http.ResponseWriter,r *http.Request){
    if r.Method!="GET"{http.Error(w,"Method",405);return}
    settings.RLock();items:=normalizedInstances(settings.Prefs);settings.RUnlock();ok(w,map[string]any{"instances":items})
}
// PM2 numeric IDs may be recycled; validate the configured process name and cwd at execution time.
func instanceProcessGuard(s ServerInstance,id string) string {
    payload,_:=json.Marshal(s)
    return remPython(`import json,sys,subprocess,os
s=json.loads(sys.argv[1]);ident=int(sys.argv[2]);items=json.loads(subprocess.check_output(['pm2','jlist']))
p=next((p for p in items if p['pm_id']==ident),None)
if p is None or p['name'] not in (s['minecraftProcess'],s['wwwProcess']): raise SystemExit('Proces nie należy do wybranej instancji')
if p['name']==s['minecraftProcess'] and os.path.realpath(p.get('pm2_env',{}).get('pm_cwd',''))!=os.path.realpath(s['serverPath']): raise SystemExit('Katalog procesu nie odpowiada instancji')`,string(payload),id)+" && "
}

func instanceRemoteGuard(ctx context.Context) string {
    s,ok:=ctx.Value(instanceContextKey{}).(ServerInstance);if !ok{return ""}
    candidate,_:=ctx.Value(instancePathKey{}).(string);if candidate==""{return ""}
    return remPython(`import os,sys
root=os.path.realpath(sys.argv[1]);candidate=os.path.realpath(sys.argv[2]);backup=os.path.join(os.path.dirname(root),'.rawcraft-backups')
inside=os.path.commonpath((root,candidate))==root
archive=os.path.dirname(candidate)==backup and os.path.basename(candidate).startswith(os.path.basename(root)+'-') and candidate.endswith('.tar.gz')
if not inside and not archive: raise SystemExit('Ścieżka prowadzi poza wybraną instancję')`,s.ServerPath,candidate)+" && "
}

 // handleRemoveInstance removes only a local saved entry. It never deletes anything on VPS.
func handleRemoveInstance(w http.ResponseWriter, r *http.Request) {
    if r.Method!="POST" {http.Error(w,"Method",405);return}
    var req struct { ID string `json:"id"` }
    if e:=fromJSON(r,&req);e!=nil{bad(w,e);return}
    settings.Lock()
    p:=settings.Prefs
    at:=-1
    for i,item:=range p.Instances {if item.ID==req.ID {at=i;break}}
    if at<0 {settings.Unlock();bad(w,errors.New("Nie znaleziono zapisanej instancji"));return}
    p.Instances=append(append([]ServerInstance{},p.Instances[:at]...),p.Instances[at+1:]...)
    if len(p.Instances)==0 {
        // Keep an SSH-only workspace when both old Minecraft servers were deleted.
        p.ServerPath="/home/"+p.User
        p.MinecraftProcess="minecraft"
        if p.WWWProcess=="" {p.WWWProcess="www"}
    }
    if e:=writePreferences(p);e!=nil{settings.Unlock();bad(w,e);return}
    settings.Prefs=p
    items:=normalizedInstances(p)
    settings.Unlock()
    closeShell()
    ok(w,map[string]any{"instances":items})
}

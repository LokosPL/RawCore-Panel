package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type Process struct {
	ID    int    `json:"pm_id"`
	Name  string `json:"name"`
	PID   int    `json:"pid"`
	Monit struct {
		CPU    float64 `json:"cpu"`
		Memory int64   `json:"memory"`
	} `json:"monit"`
	Env struct {
		Status   string `json:"status"`
		Uptime   int64  `json:"pm_uptime"`
		Restarts int    `json:"restart_time"`
		Cwd      string `json:"pm_cwd"`
        OutLog string `json:"pm_out_log_path"`
        ErrorLog string `json:"pm_err_log_path"`
	} `json:"pm2_env"`
}

func route(method string, next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			http.Error(w, "Method", 405)
			return
		}
		next(w, r)
	})
}

const metricsPython = `import os,json,time,socket

def cpu():
 v=list(map(int,open('/proc/stat').readline().split()[1:9])); return sum(v),v[3]+v[4]
a=cpu();time.sleep(.2);b=cpu()
m={}
for line in open('/proc/meminfo'):
 k,v=line.split(':',1);m[k]=int(v.split()[0])*1024
s=os.statvfs('/')
print(json.dumps({'cpu':round(100*(1-(b[1]-a[1])/max(1,b[0]-a[0])),1),'cores':os.cpu_count(),'memoryTotal':m['MemTotal'],'memoryUsed':m['MemTotal']-m.get('MemAvailable',m.get('MemFree',0)),'diskTotal':s.f_blocks*s.f_frsize,'diskUsed':(s.f_blocks-s.f_bfree)*s.f_frsize,'uptime':int(float(open('/proc/uptime').read().split()[0])),'hostname':socket.gethostname()}))`

const backupsPython = `import os,sys,json,tarfile,datetime,uuid
p=os.path.realpath(sys.argv[1]);folder=os.path.join(os.path.dirname(p),'.rawcraft-backups');prefix=os.path.basename(p)+'-'
if p=='/' or not os.path.isdir(p): raise SystemExit('Wybierz istniejący katalog serwera, inny niż /')
if sys.argv[2]=='create':
 os.makedirs(folder,mode=0o700,exist_ok=True)
 name=prefix+datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%d-%H%M%S')+'-'+uuid.uuid4().hex[:6]+'.tar.gz'
 target=os.path.join(folder,name)
 try:
  with tarfile.open(target+'.part','w:gz',dereference=False) as tar: tar.add(p,arcname=os.path.basename(p))
  os.replace(target+'.part',target)
 except:
  if os.path.exists(target+'.part'): os.remove(target+'.part')
  raise
rows=[]
if os.path.isdir(folder):
 for name in sorted(os.listdir(folder),reverse=True):
  if name.startswith(prefix) and name.endswith('.tar.gz'):
   f=os.path.join(folder,name);s=os.stat(f);rows.append({'name':name,'path':f,'size':s.st_size,'mtime':int(s.st_mtime)})
print(json.dumps({'folder':folder,'items':rows}))`

func handleBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Method", 405)
		return
	}
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	instance,e:=requestInstance(r);if e!=nil{bad(w,e);return}
    path := instance.ServerPath
	if e = checkPath(path); e != nil {
		bad(w, e)
		return
	}
	mode := "list"
	if r.Method == "POST" {
		mode = "create"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()
	out, e := runRemote(ctx, c, remPython(backupsPython, path, mode))
	if e != nil {
		bad(w, e)
		return
	}
	var result any
	if json.Unmarshal([]byte(out), &result) != nil {
		bad(w, errors.New("Nie udało się odczytać kopii"))
		return
	}
	ok(w, result)
}

const rconPython = `import sys,os,socket,struct,base64,select
p=sys.argv[1];command=base64.b64decode(sys.argv[2]).decode('utf-8')
props={}
for line in open(os.path.join(p,'server.properties'),encoding='utf-8'):
 line=line.strip()
 if line and not line.startswith(('#','!')) and '=' in line:
  k,v=line.split('=',1);props[k.strip()]=v.strip()
if props.get('enable-rcon','false').lower()!='true': raise SystemExit('Włącz enable-rcon=true i ustaw rcon.password w server.properties, następnie zrestartuj serwer.')
password=props.get('rcon.password','')
if not password: raise SystemExit('Brak rcon.password w server.properties')
def receive(s,n):
 data=b''
 while len(data)<n:
  part=s.recv(n-len(data))
  if not part: raise EOFError('RCON zamknął połączenie')
  data+=part
 return data
def packet(s):
 n=struct.unpack('<i',receive(s,4))[0]
 if n<10 or n>4194304: raise RuntimeError('Niepoprawny pakiet RCON')
 data=receive(s,n);return struct.unpack('<ii',data[:8]),data[8:-2].decode('utf-8',errors='replace')
def send(s,id,kind,text):
 data=struct.pack('<ii',id,kind)+text.encode('utf-8')+b'\0\0';s.sendall(struct.pack('<i',len(data))+data)
with socket.create_connection(('127.0.0.1',int(props.get('rcon.port','25575'))),timeout=8) as s:
 send(s,10,3,password)
 for i in range(4):
  (id,kind),text=packet(s)
  if id==-1: raise RuntimeError('RCON odrzucił hasło')
  if id==10 and kind==2: break
 else: raise RuntimeError('Brak potwierdzenia RCON')
 send(s,20,2,command)
 (id,kind),text=packet(s)
 if id!=20: raise RuntimeError('Niepoprawna odpowiedź RCON')
 response=[text];total=len(text.encode())
 while select.select([s],[],[],.2)[0]:
  try: (id,kind),fragment=packet(s)
  except (EOFError,ConnectionResetError): break
  if id==20: response.append(fragment);total+=len(fragment.encode())
  if total>4194304: raise RuntimeError('Odpowiedź RCON przekracza 4 MB')
 print(''.join(response))
`

func handleMinecraftCommand(w http.ResponseWriter, r *http.Request) {
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	var req struct {
		Path    string `json:"path"`
		Command string `json:"command"`
	}
	if e = fromJSON(r, &req); e != nil {
		bad(w, e)
		return
	}
	instance,e:=requestInstance(r);if e!=nil{bad(w,e);return};req.Path=instance.ServerPath
    if e = checkPath(req.Path); e != nil {
		bad(w, e)
		return
	}
	req.Command = strings.TrimPrefix(strings.TrimSpace(req.Command), "/")
	if len(req.Command) == 0 || len(req.Command) > 1400 || strings.ContainsAny(req.Command, "\x00\r\n") {
		bad(w, errors.New("Podaj jedną komendę Minecraft (do 1400 znaków)"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	out, e := runRemote(ctx, c, remPython(rconPython, req.Path, base64.StdEncoding.EncodeToString([]byte(req.Command))))
	if e != nil {
		bad(w, e)
		return
	}
	ok(w, map[string]string{"output": out})
}

func handleLatestLog(w http.ResponseWriter, r *http.Request) {
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	instance,e:=requestInstance(r);if e!=nil{bad(w,e);return}
    path := instance.ServerPath
	if e = checkPath(path); e != nil {
		bad(w, e)
		return
	}
	script := `import os,sys
p=os.path.join(sys.argv[1],'logs','latest.log')
with open(p,'rb') as f:
 f.seek(0,2);f.seek(max(0,f.tell()-262144));sys.stdout.buffer.write(f.read())`
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	out, e := runRemote(ctx, c, remPython(script, path))
	if e != nil {
		bad(w, e)
		return
	}
	ok(w, map[string]string{"logs": out})
}

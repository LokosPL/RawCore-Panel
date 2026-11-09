package main

import (
    "context"
    "encoding/json"
    "errors"
    "net/http"
    "time"
)

// The Core key stays on the VPS. This bridge only reads a fixed, authenticated loopback status API.
const paymentsPython = `import os,sys,json,http.client,stat,time
try:
 root=os.path.realpath(sys.argv[1]);folder=root
 for part in ('plugins','Core','website'):
  folder=os.path.join(folder,part)
  if os.path.islink(folder): raise ValueError()
 file=os.path.join(folder,'server.properties')
 fd=os.open(file,os.O_RDONLY|getattr(os,'O_NOFOLLOW',0))
 with os.fdopen(fd,'r',encoding='utf-8') as handle:
  mode=os.fstat(handle.fileno())
  if not stat.S_ISREG(mode.st_mode) or mode.st_size>65536 or mode.st_mode&0o077: raise ValueError()
  props={}
  for line in handle:
   line=line.strip()
   if line and not line.startswith(('#','!')) and '=' in line:
    name,value=line.split('=',1);props[name.strip()]=value.strip()
 if props.get('bind','127.0.0.1')!='127.0.0.1': raise ValueError()
 port=int(props.get('port','8766'));key=props.get('api-key','')
 if not 1<=port<=65535 or not 32<=len(key)<=128 or any(ord(x)<33 or ord(x)>126 for x in key): raise ValueError()
 conn=http.client.HTTPConnection('127.0.0.1',port,timeout=4)
 try:
  conn.request('POST','/v1/admin/payments/status',body='{}',headers={'Authorization':'Bearer '+key,'Content-Type':'application/json'})
  reply=conn.getresponse();raw=reply.read(16385)
  if reply.status!=200 or len(raw)>16384: raise ValueError()
  data=json.loads(raw)
 finally: conn.close()
 if not isinstance(data,dict) or type(data.get('version')) is not int or data['version']!=1: raise ValueError()
 result={'version':1}
 for name in ('bridgeEnabled','paymentsEnabled','testMode','operational','bridgeConnected','acceptingTopups','paymentsAvailable'):
  if type(data.get(name)) is not bool: raise ValueError()
  result[name]=data[name]
 for name in ('lastBridgeSuccessAt','observedAt'):
  if type(data.get(name)) is not int or not 0<=data[name]<=9007199254740991: raise ValueError()
  result[name]=data[name]
 if abs(int(time.time()*1000)-result['observedAt'])>30000: raise ValueError()
 reasons=('STARTING','MAINTENANCE','BRIDGE_DISABLED','PAYMENTS_DISABLED','CONFIGURATION_ERROR','SQL_NOT_READY','BRIDGE_OFFLINE','TEST_MODE','READY')
 if data.get('reason') not in reasons: raise ValueError()
 if result['acceptingTopups'] and not all(result[x] for x in ('bridgeEnabled','paymentsEnabled','operational','bridgeConnected')): raise ValueError()
 if result['paymentsAvailable'] != (result['acceptingTopups'] and not result['testMode']): raise ValueError()
 if (data['reason']=='READY') != result['paymentsAvailable']: raise ValueError()
 result['reason']=data['reason']
 print(json.dumps(result))
except Exception:
 print(json.dumps({'error':'Nie odczytano statusu Core. Sprawdź wersję Core, połączenie i prawa 600 do website/server.properties.'}))
`

func handlePaymentStatus(w http.ResponseWriter,r *http.Request){
    c,err:=currentPrivilegedCred();if err!=nil{bad(w,err);return}
    instance,err:=requestInstance(r);if err!=nil{bad(w,err);return};path:=instance.ServerPath
    if err=checkPath(path);err!=nil{bad(w,err);return}
    ctx,cancel:=context.WithTimeout(r.Context(),8*time.Second);defer cancel()
    out,err:=runRemote(ctx,c,remPython(paymentsPython,path))
    if err!=nil{bad(w,errors.New("Nie odczytano statusu płatności przez SSH. Sprawdź połączenie i uprawnienia konta."));return}
    var result map[string]any
    if len(out)>16384||json.Unmarshal([]byte(out),&result)!=nil{bad(w,errors.New("Niepoprawna odpowiedź statusu Core"));return}
    ok(w,result)
}

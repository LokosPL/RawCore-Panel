package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

const passwordPython = `import os,sys,json,subprocess,shutil,pty,select,time,re,pwd
request=json.load(sys.stdin);user=request['user'];new=request['password'];old=request.get('old','')
if os.geteuid()==0:
 tool=shutil.which('chpasswd') or '/usr/sbin/chpasswd'
 result=subprocess.run([tool],input=(user+':'+new+'\n').encode(),stdout=subprocess.PIPE,stderr=subprocess.PIPE)
 if result.returncode: raise SystemExit(result.stderr.decode(errors='replace').replace(new,'[ukryte]').replace(old,'[ukryte]') if old else result.stderr.decode(errors='replace').replace(new,'[ukryte]'))
else:
 if pwd.getpwuid(os.geteuid()).pw_name!=user: raise SystemExit('Do zmiany hasła innego konta wymagany jest tryb administratora sudo lub root')
 tool=shutil.which('passwd') or '/usr/bin/passwd'
 pid,fd=pty.fork()
 if pid==0:
  env=os.environ.copy();env['LC_ALL']='C';os.execve(tool,[tool],env)
 buffer=b'';transcript=b'';deadline=time.monotonic()+30;sent_new=0;status=None
 try:
  while time.monotonic()<deadline:
   done,code=os.waitpid(pid,os.WNOHANG)
   if done: status=code;break
   ready,_,_=select.select([fd],[],[],.2)
   if not ready: continue
   try: data=os.read(fd,8192)
   except OSError: break
   if not data: break
   transcript=(transcript+data)[-16384:];buffer=(buffer+data)[-4096:];lower=buffer.lower()
   if re.search(rb'(current|old)[^\r\n:]*password[^\r\n:]*:\s*$',lower): os.write(fd,old.encode()+b'\n');buffer=b''
   elif re.search(rb'(retype|repeat)[^\r\n:]*password[^\r\n:]*:\s*$',lower): os.write(fd,new.encode()+b'\n');buffer=b''
   elif re.search(rb'new[^\r\n:]*password[^\r\n:]*:\s*$',lower):
    sent_new+=1
    if sent_new>1: raise RuntimeError('Hasło nie spełnia zasad serwera')
    os.write(fd,new.encode()+b'\n');buffer=b''
  while status is None and time.monotonic()<deadline:
   done,code=os.waitpid(pid,os.WNOHANG)
   if done: status=code;break
   time.sleep(.05)
  if status is None:
   os.kill(pid,15);os.waitpid(pid,0);raise RuntimeError('Zmiana hasła nie została potwierdzona w wymaganym czasie')
  if os.waitstatus_to_exitcode(status)!=0:
   text=transcript.decode(errors='replace').replace(new,'[ukryte]')
   if old: text=text.replace(old,'[ukryte]')
   raise RuntimeError('VPS odrzucił zmianę hasła. '+text[-800:])
 finally:
  os.close(fd)
  if status is None:
   try: os.kill(pid,15);os.waitpid(pid,0)
   except (ProcessLookupError,ChildProcessError): pass
print(json.dumps({'ok':True}))`

var passwordChanges sync.Mutex

func handleChangePassword(w http.ResponseWriter, r *http.Request) {
	passwordChanges.Lock()
	defer passwordChanges.Unlock()
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	var req struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Confirm  string `json:"confirm"`
		Current  string `json:"current"`
	}
	if e = fromJSON(r, &req); e != nil {
		bad(w, e)
		return
	}
	req.User = strings.TrimSpace(req.User)
	if !userRegex.MatchString(req.User) || len(req.Password) < 8 || len(req.Password) > 1024 || strings.ContainsAny(req.Password, "\x00\r\n") || req.Password != req.Confirm {
		bad(w, errors.New("Podaj poprawnego użytkownika i dwa identyczne hasła (8–1024 znaki, bez znaków końca linii)"))
		return
	}
	if req.User != c.User && !c.privileged {
		state.RLock()
		root := state.root
		state.RUnlock()
		if !root {
			bad(w, errors.New("Do zmiany hasła innego konta wymagany jest tryb administratora"))
			return
		}
	}
	old := req.Current
	if old == "" {
		old = c.Password
	}
	body, _ := json.Marshal(map[string]string{"user": req.User, "password": req.Password, "old": old})
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	var out strings.Builder
	if e = streamRemote(ctx, c, remPython(passwordPython), strings.NewReader(string(body)), &out); e != nil {
		bad(w, e)
		return
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if json.Unmarshal([]byte(out.String()), &result) != nil || !result.OK {
		bad(w, errors.New("VPS nie potwierdził zmiany hasła"))
		return
	}
	warning := ""
	if req.User == c.User {
		state.Lock()
		if state.connected && state.creds.Host == c.Host && state.creds.Port == c.Port && state.creds.User == c.User {
			previous := state.creds.Password
			state.creds.Password = req.Password
			if state.sudoPassword == previous {
				state.sudoPassword = req.Password
			}
		}
		state.Unlock()
		c.Password = req.Password
		c.privileged = false
		c.sudoPassword = ""
		settings.RLock()
		remember, auto := settings.Prefs.RememberPassword, settings.Prefs.AutoLogin
		settings.RUnlock()
		if e = rememberConnection(c, remember, auto); e != nil {
			warning = "Hasło na VPS zostało zmienione, ale nowego profilu nie udało się zapisać na komputerze. Przy kolejnym logowaniu wpisz nowe hasło."
		}
	}
	ok(w, map[string]any{"ok": true, "message": "Hasło konta " + req.User + " na VPS zostało zmienione", "warning": warning})
}

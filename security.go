package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Passwords travel only through SSH stdin. The handshake separates sudo prompts
// from file payloads, including NOPASSWD, cached tickets and timestamp_timeout=0.
const sudoPython = `import sys,os,json,base64,subprocess,threading,shutil
secret=json.loads(sys.stdin.buffer.readline()).get('password','')
command=base64.b64decode(sys.argv[1]).decode('utf-8')
sudo=shutil.which('sudo')
if not sudo: raise SystemExit('RAWCRAFT_SUDO_DENIED: Na VPS nie ma programu sudo')
prompt=b'__RAWCRAFT_SUDO_PROMPT_91f3__';ready=b'__RAWCRAFT_SUDO_READY_91f3__'
script="printf '%s' '"+ready.decode()+"' >&2; exec bash --noprofile --norc -c "+"'"+command.replace("'", "'\"'\"'")+"'"
proc=subprocess.Popen([sudo,'-S','-p',prompt.decode(),'--','bash','-c',script],stdin=subprocess.PIPE,stdout=sys.stdout.buffer,stderr=subprocess.PIPE)
event=threading.Event();authorized=[False];sent=[False]
def errors():
 pending=b''
 while True:
  part=proc.stderr.read(1)
  if not part: break
  pending+=part
  if pending.endswith(prompt):
   prefix=pending[:-len(prompt)]
   if prefix: sys.stderr.buffer.write(prefix);sys.stderr.buffer.flush()
   pending=b''
   if sent[0]:
    try: proc.stdin.close()
    except OSError: pass
    continue
   sent[0]=True
   try: proc.stdin.write(secret.encode()+b'\n');proc.stdin.flush()
   except (OSError,ValueError): pass
  elif pending.endswith(ready):
   prefix=pending[:-len(ready)]
   if prefix: sys.stderr.buffer.write(prefix);sys.stderr.buffer.flush()
   pending=b'';authorized[0]=True;event.set()
  elif b'\n' in pending or len(pending)>8192:
   sys.stderr.buffer.write(pending);sys.stderr.buffer.flush();pending=b''
 if pending: sys.stderr.buffer.write(pending);sys.stderr.buffer.flush()
 event.set()
thread=threading.Thread(target=errors,daemon=True);thread.start()
if not event.wait(15):
 proc.kill();raise SystemExit('RAWCRAFT_SUDO_DENIED: Przekroczono czas autoryzacji sudo')
if authorized[0]:
 try:
  shutil.copyfileobj(sys.stdin.buffer,proc.stdin,1048576);proc.stdin.close()
 except (BrokenPipeError,OSError,ValueError): pass
code=proc.wait();thread.join(timeout=3)
if not authorized[0]: sys.stderr.write('\nRAWCRAFT_SUDO_DENIED: Konto nie ma wymaganych praw sudo albo hasło sudo jest niepoprawne\n')
raise SystemExit(code if authorized[0] else 77)`

func remoteInput(c Credentials, in io.Reader) io.Reader {
	if !c.privileged {
		return in
	}
	header, _ := json.Marshal(map[string]string{"password": c.sudoPassword})
	if in == nil {
		in = strings.NewReader("")
	}
	return io.MultiReader(strings.NewReader(string(header)+"\n"), in)
}
func privilegedCommand(c Credentials, remote string) string {
	if !c.privileged {
		return remote
	}
	return remPython(sudoPython, base64.StdEncoding.EncodeToString([]byte(remote)))
}
func currentPrivilegedCred() (Credentials, error) {
	state.RLock()
	defer state.RUnlock()
	if !state.connected {
		return Credentials{}, errors.New("Najpierw połącz się z VPS")
	}
	c := state.creds
	c.privileged = state.admin && !state.root
	c.sudoPassword = state.sudoPassword
	return c, nil
}
func securitySnapshot() map[string]any {
	state.RLock()
	defer state.RUnlock()
	return map[string]any{"connected": state.connected, "admin": state.admin, "root": state.root, "message": state.adminMessage}
}
func negotiateAdmin(ctx context.Context, c Credentials, password string, enable bool) error {
	uid, e := runRemote(ctx, c, "printf '__RAWCRAFT_UID__%s\\n' \"$(id -u)\"")
	if e != nil {
		return e
	}
	root := rootUID(uid)
	if root {
		state.Lock()
		if !sameConnection(c) {
			state.Unlock()
			return errors.New("Połączenie SSH zostało zmienione")
		}
		state.root = true
		state.admin = true
		state.adminMessage = "Połączono jako root"
		state.Unlock()
		return nil
	}
	if !enable {
		state.Lock()
		if !sameConnection(c) {
			state.Unlock()
			return errors.New("Połączenie SSH zostało zmienione")
		}
		state.root = false
		state.admin = false
		state.sudoPassword = ""
		state.adminMessage = "Tryb użytkownika"
		state.Unlock()
		return nil
	}
	c.privileged = true
	c.sudoPassword = password
	uid, e = runRemote(ctx, c, "printf '__RAWCRAFT_UID__%s\\n' \"$(id -u)\"")
	if e != nil || !rootUID(uid) {
		state.Lock()
		if !sameConnection(c) {
			state.Unlock()
			return errors.New("Połączenie SSH zostało zmienione")
		}
		state.root = false
		state.admin = false
		state.sudoPassword = ""
		state.adminMessage = "Tryb użytkownika. Konto musi mieć prawo sudo do uruchamiania poleceń administratora."
		state.Unlock()
		if e != nil {
			return e
		}
		return errors.New("sudo nie przyznało uprawnień root")
	}
	state.Lock()
	if !sameConnection(c) {
		state.Unlock()
		return errors.New("Połączenie SSH zostało zmienione")
	}
	state.root = false
	state.admin = true
	state.sudoPassword = password
	state.adminMessage = "Administrator sudo — pliki i terminal; PM2 pozostaje na koncie SSH"
	state.Unlock()
	return nil
}
func handleSecurity(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		ok(w, securitySnapshot())
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method", 405)
		return
	}
	c, e := currentCred()
	if e != nil {
		bad(w, e)
		return
	}
	var req struct {
		Enabled  bool   `json:"enabled"`
		Password string `json:"password"`
	}
	if e = fromJSON(r, &req); e != nil {
		bad(w, e)
		return
	}
	if req.Password == "" {
		req.Password = c.Password
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	if e = negotiateAdmin(ctx, c, req.Password, req.Enabled); e != nil {
		bad(w, e)
		return
	}
	ok(w, securitySnapshot())
}

var sshAuthenticationFailure = regexp.MustCompile(`(?im)^\S+@[^\r\n]+:\s*permission denied \([^\r\n)]+\)\.?\s*$`)

func remoteFailure(err error, output string) error {
	text := strings.TrimSpace(output)
	if text == "" {
		text = err.Error()
	}
	if len(text) > 1500 {
		text = text[len(text)-1500:]
	}
	if strings.Contains(text, "RAWCRAFT_SUDO_DENIED") {
		return errors.New("Nie udało się uzyskać uprawnień sudo. Konto musi mieć takie uprawnienia; sprawdź hasło administratora w zakładce Dostęp i hasło VPS.")
	}
	var exited interface{ ExitCode() int }
	if errors.As(err, &exited) && exited.ExitCode() == 255 && sshAuthenticationFailure.MatchString(text) {
		return errors.New("Nieprawidłowe hasło lub użytkownik SSH. " + text)
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "permission denied") || strings.Contains(lower, "permissionerror") {
		return errors.New("Brak uprawnień do pliku lub polecenia na VPS. Włącz tryb administratora sudo albo użyj konta z odpowiednimi prawami.\n" + text)
	}
	return errors.New(text)
}

func sameConnection(c Credentials) bool {
	return state.connected && state.creds.Host == c.Host && state.creds.Port == c.Port && state.creds.User == c.User && state.creds.Password == c.Password
}

func rootUID(output string) bool {
 markers:=regexp.MustCompile(`__RAWCRAFT_UID__([0-9]+)`).FindAllStringSubmatch(output,-1)
 return len(markers)>0&&markers[len(markers)-1][1]=="0"
}

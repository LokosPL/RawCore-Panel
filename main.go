package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
 "rawcraft-control/shared/remotefiles"
)

// RAWCRAFT Control — one Windows EXE. Uses the Windows OpenSSH client, SSH_ASKPASS,
// and the installed Microsoft Edge to render its application window.
// Optional saved credentials are protected by Windows DPAPI. SSH verifies the host key against
// the user's normal OpenSSH known_hosts file (TOFU for first use).

type Credentials struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	privileged   bool
	sudoPassword string
}
type State struct {
	sync.RWMutex
	creds        Credentials
	connected    bool
	token        string
	csrf         string
	admin        bool
	root         bool
	sudoPassword string
	adminMessage string
}

var state State
var userRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.-]{0,63}$`)
var hostRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.:_-]{0,252}$`)
var safeID = regexp.MustCompile(`^[0-9]{1,8}$`)
var safeAction = map[string]bool{"start": true, "stop": true, "restart": true, "reload": true}
var appExe string

func token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func eq(a, b string) bool   { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'" }
func checkedCred(c Credentials) error {
	if !hostRegex.MatchString(c.Host) || strings.Contains(c.Host, "..") {
		return errors.New("Nieprawidłowy adres IP lub nazwa hosta")
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("Port SSH musi mieścić się między 1 a 65535")
	}
	if !userRegex.MatchString(c.User) {
		return errors.New("Nieprawidłowa nazwa użytkownika")
	}
	if c.Password == "" {
		return errors.New("Podaj hasło SSH")
	}
	return nil
}
func sshArgs(c Credentials, remote string) []string {
	return []string{"-T", "-p", strconv.Itoa(c.Port), "-o", "ConnectTimeout=12", "-o", "ServerAliveInterval=8",
		"-o", "StrictHostKeyChecking=accept-new", "-o", "PubkeyAuthentication=no", "-o", "PreferredAuthentications=password,keyboard-interactive",
		"-o", "NumberOfPasswordPrompts=1", "-o", "BatchMode=no", c.User + "@" + c.Host, "bash -lc " + quote(remote)}
}
func sshCmd(ctx context.Context, c Credentials, remote string) *exec.Cmd {
	program := "ssh"
	if runtime.GOOS == "windows" {
		program = "ssh.exe"
	}
	cmd := exec.CommandContext(ctx, program, sshArgs(c, privilegedCommand(c, remote))...)
	hideWindow(cmd)
	env := os.Environ()
	env = append(env, "SSH_ASKPASS="+appExe, "SSH_ASKPASS_REQUIRE=force", "DISPLAY=localhost:0", "RAWCRAFT_SSH_PASSWORD="+c.Password)
	cmd.Env = env
	cmd.Stdin = remoteInput(c, nil)
	return cmd
}
func runRemote(ctx context.Context, c Credentials, remote string) (string, error) {
    remote=instanceRemoteGuard(ctx)+remote
	cmd := sshCmd(ctx, c, remote)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", remoteFailure(err, string(out)+"\n"+stderr.String())
	}

	return string(out), nil
}
func currentCred() (Credentials, error) {
	state.RLock()
	defer state.RUnlock()
	if !state.connected {
		return Credentials{}, errors.New("Najpierw połącz się z VPS")
	}
	return state.creds, nil
}
func jsonOut(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func ok(w http.ResponseWriter, v any)      { jsonOut(w, 200, v) }
func bad(w http.ResponseWriter, err error) { jsonOut(w, 400, map[string]any{"error": err.Error()}) }
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Host, "127.0.0.1:") {
			http.Error(w, "Nieprawidłowy host", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		c, err := r.Cookie(sessionCookieName())
		if err != nil || !eq(c.Value, state.token) {
			jsonOut(w, 403, map[string]string{"error": "Sesja wymaga odnowienia", "code": "session_expired"})
			return
		}
		if r.Method != "GET" {
			if !eq(r.Header.Get("X-RAWCRAFT-CSRF"), state.csrf) {
				http.Error(w, "Brak tokenu CSRF", 403)
				return
			}
			if ori := r.Header.Get("Origin"); ori != "" && ori != "http://"+r.Host {
				http.Error(w, "Nieprawidłowe źródło żądania", 403)
				return
			}
		}
		scopeInstance(next)(w, r)
	}
}
func fromJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}
func connectCredentials(ctx context.Context, c Credentials) (string, error) {
	if e := checkedCred(c); e != nil {
		return "", e
	}
	reply, e := runRemote(ctx, c, "printf 'SSH_OK\\n'; id -un; hostname")
	if e != nil {
		return "", e
	}
	if !strings.Contains(reply, "SSH_OK") {
		return "", errors.New("Serwer nie potwierdził połączenia SSH")
	}
	state.Lock()
	state.creds = c
	state.connected = true
	state.admin = false
	state.root = false
	state.sudoPassword = ""
	state.adminMessage = "Tryb użytkownika"
	state.Unlock()
	_ = negotiateAdmin(ctx, c, c.Password, true)
	return strings.TrimSpace(reply), nil
}
func handleConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Credentials
		Remember  bool `json:"remember"`
		AutoLogin bool `json:"autoLogin"`
	}
	if e := fromJSON(r, &req); e != nil {
		bad(w, e)
		return
	}
	c := req.Credentials
	c.Host = strings.TrimSpace(c.Host)
	c.User = strings.TrimSpace(c.User)
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	reply, e := connectCredentials(ctx, c)
	if e != nil {
		bad(w, e)
		return
	}
	warning := ""
	if e = rememberConnection(c, req.Remember, req.AutoLogin); e != nil {
		warning = "Połączono, ale nie zapisano profilu: " + e.Error()
	}
	ok(w, map[string]any{"message": reply, "host": c.Host, "user": c.User, "warning": warning})
}
func handleConnectSaved(w http.ResponseWriter, r *http.Request) {
	c, e := savedCredentials()
	if e != nil {
		bad(w, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	reply, e := connectCredentials(ctx, c)
	if e != nil {
		bad(w, e)
		return
	}
	ok(w, map[string]string{"message": reply, "host": c.Host, "user": c.User})
}
func handleDisconnect(w http.ResponseWriter, r *http.Request) {
 closeShell()
	state.Lock()
	state.creds = Credentials{}
	state.connected = false
	state.admin = false
	state.root = false
	state.sudoPassword = ""
	state.Unlock()
	ok(w, map[string]bool{"ok": true})
}
func handleStatus(w http.ResponseWriter, r *http.Request) {
	c, err := currentCred()
	if err != nil {
		bad(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	remote := pm2prefix() + "if command -v pm2 >/dev/null 2>&1; then pm2 jlist 2>/dev/null || printf '[]'; else printf '[]'; fi; printf '\\n__RAWCRAFT_METRICS__\\n'; " + remPython(metricsPython)
	raw, err := runRemote(ctx, c, remote)
	if err != nil {
		bad(w, err)
		return
	}
	parts := strings.SplitN(raw, "\n__RAWCRAFT_METRICS__\n", 2)
	var processes []Process
	start, end := strings.Index(parts[0], "["), strings.LastIndex(parts[0], "]")
	if start < 0 || end < start || json.Unmarshal([]byte(parts[0][start:end+1]), &processes) != nil {
		bad(w, errors.New("Nie udało się odczytać procesów PM2"))
		return
	}
	if processes == nil {
		processes = []Process{}
	}
	var metrics map[string]any
	if len(parts) == 2 {
		_ = json.Unmarshal([]byte(parts[1]), &metrics)
	}
	instance,e:=requestInstance(r);if e!=nil{bad(w,e);return}
    filtered:=make([]Process,0);for _,p:=range processes{if p.Name==instance.MinecraftProcess||p.Name==instance.WWWProcess{filtered=append(filtered,p)}}
    ok(w, map[string]any{"processes": filtered, "metrics": metrics, "host": c.Host, "user": c.User})
}
func pm2prefix() string {
	return "command -v pm2 >/dev/null 2>&1 || { test -s \"$HOME/.nvm/nvm.sh\" && source \"$HOME/.nvm/nvm.sh\"; }; "
}
func handleAction(w http.ResponseWriter, r *http.Request) {
	c, e := currentCred()
	if e != nil {
		bad(w, e)
		return
	}
	var req struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	if err := fromJSON(r, &req); err != nil {
		bad(w, err)
		return
	}
	if !safeID.MatchString(req.ID) || !safeAction[req.Action] {
		bad(w, errors.New("Niepoprawny proces lub działanie"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	instance,err:=requestInstance(r);if err!=nil{bad(w,err);return}
    out, err := runRemote(ctx, c, pm2prefix()+instanceProcessGuard(instance,req.ID)+"pm2 "+req.Action+" "+req.ID)
	if err != nil {
		bad(w, err)
		return
	}
	ok(w, map[string]string{"message": out})
}
func handleLogs(w http.ResponseWriter, r *http.Request) {
	c, e := currentCred()
	if e != nil {
		bad(w, e)
		return
	}
	id := r.URL.Query().Get("id")
	if !safeID.MatchString(id) {
		bad(w, errors.New("Niepoprawne ID procesu"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	instance,err:=requestInstance(r);if err!=nil{bad(w,err);return}
    out, err := runRemote(ctx, c, pm2prefix()+instanceProcessGuard(instance,id)+"pm2 logs "+id+" --lines 180 --nostream --raw 2>&1")
	if err != nil {
		bad(w, err)
		return
	}
	if len(out) > 1<<20 {
		out = out[len(out)-(1<<20):]
	}
	ok(w, map[string]string{"logs": out})
}

const listPython = `import os,sys,json,stat,base64
p=base64.b64decode(sys.argv[1]).decode('utf-8')
rows=[]
for a in os.scandir(p):
 try:
  st=a.stat(follow_symlinks=False)
  rows.append({'name':a.name,'dir':stat.S_ISDIR(st.st_mode),'size':st.st_size,'mtime':int(st.st_mtime),'version':f'{st.st_dev}:{st.st_ino}:{st.st_size}:{st.st_mtime_ns}:{st.st_ctime_ns}'})
 except OSError: pass
print(json.dumps({'path':os.path.realpath(p),'items':sorted(rows,key=lambda r:(not r['dir'],r['name'].lower()))},ensure_ascii=False))`

func remPython(script string, args ...string) string {
	cmd := "python3 -c " + quote(script)
	for _, a := range args {
		cmd += " " + quote(a)
	}
	return cmd
}
func handleFiles(w http.ResponseWriter, r *http.Request) {
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	p := r.URL.Query().Get("path")
	if p == "" {
		instance,err:=requestInstance(r);if err!=nil{bad(w,err);return};p=instance.ServerPath
	}
	if !strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) {
		bad(w, errors.New("Podaj pełną ścieżkę zaczynającą się od /"))
		return
	}
	if len(p) > 2048 {
		bad(w, errors.New("Za długa ścieżka"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	out, err := runRemote(ctx, c, remPython(listPython, base64.StdEncoding.EncodeToString([]byte(p))))
	if err != nil {
		bad(w, err)
		return
	}
	var o any
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		bad(w, errors.New("Nie udało się odczytać JSON katalogu"))
		return
	}
	ok(w, o)
}
func checkPath(p string) error {
	if !strings.HasPrefix(p, "/") || len(p) > 4096 || strings.ContainsRune(p, 0) {
		return errors.New("Niepoprawna pełna ścieżka na VPS")
	}
	return nil
}
func streamRemote(ctx context.Context, c Credentials, remote string, in io.Reader, out io.Writer) error {
	cmd := sshCmd(ctx, c, instanceRemoteGuard(ctx)+remote)
	cmd.Stdin = remoteInput(c, in)
	cmd.Stdout = out
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return remoteFailure(err, stderr.String())
	}

	return nil
}
func handleUpload(w http.ResponseWriter, r *http.Request) {
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	p := r.URL.Query().Get("path")
	if err := checkPath(p); err != nil {
		bad(w, err)
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		bad(w, err)
		return
	}
	part, err := mr.NextPart()
	if err != nil {
		bad(w, err)
		return
	}
	defer part.Close()
	if part.FormName() != "file" {
		bad(w, errors.New("Wybierz plik"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Minute)
	defer cancel()
	size,e:=strconv.ParseInt(r.URL.Query().Get("size"),10,64);if e!=nil||size<0{bad(w,errors.New("Niepoprawny rozmiar przesyłanego pliku"));return}
 remote := remPython(remotefiles.Upload,p,"",strconv.FormatInt(size,10))
	if err := streamRemote(ctx, c, remote, part, io.Discard); err != nil {
		bad(w, err)
		return
	}
	ok(w, map[string]string{"message": "Przesłano na VPS: " + p})
}
func handleDownload(w http.ResponseWriter, r *http.Request) {
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	p := r.URL.Query().Get("path")
	if err := checkPath(p); err != nil {
		bad(w, err)
		return
	}
	name := filepath.Base(p)
	name = strings.Map(func(r rune) rune {
		if r == '"' || r == '\r' || r == '\n' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Minute)
	defer cancel()
	_ = streamRemote(ctx, c, "cat -- "+quote(p), nil, w)
}
func handleMkdir(w http.ResponseWriter, r *http.Request) {
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := fromJSON(r, &req); err != nil {
		bad(w, err)
		return
	}
	if err := checkPath(req.Path); err != nil {
		bad(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	_, err := runRemote(ctx, c, "mkdir -- "+quote(req.Path))
	if err != nil {
		bad(w, err)
		return
	}
	ok(w, map[string]bool{"ok": true})
}
func handleTerminal(w http.ResponseWriter, r *http.Request) {
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	var req struct {
		Command string `json:"command"`
	}
	if err := fromJSON(r, &req); err != nil {
		bad(w, err)
		return
	}
	if strings.TrimSpace(req.Command) == "" || len(req.Command) > 2000 {
		bad(w, errors.New("Podaj komendę do wykonania"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	instance,e:=requestInstance(r);if e!=nil{bad(w,e);return}
    out, err := runRemote(ctx, c, "cd -- "+quote(instance.ServerPath)+" && { "+req.Command+"\n} 2>&1")
	if err != nil {
		bad(w, err)
		return
	}
	if len(out) > 1<<20 {
		out = out[:1<<20]
	}
	ok(w, map[string]string{"output": out})
}
func servePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if eq(r.URL.Query().Get("auth"), state.token) {
		setSessionCookie(w)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	c, err := r.Cookie(sessionCookieName())
	if err != nil || !eq(c.Value, state.token) {
		http.Error(w, "Otwórz aplikację z pliku RAWCRAFT-Control.exe", 403)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	t := template.Must(template.New("ui").Parse(page))
	_ = t.Execute(w, map[string]string{"CSRF": state.csrf, "Notices": thirdPartyNotices, "Theme": currentTheme()})
}
func openApp(url string) {
	if runtime.GOOS == "windows" {
		paths := []string{}
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LOCALAPPDATA"} {
			p := os.Getenv(env)
			if p != "" {
				paths = append(paths, filepath.Join(p, "Microsoft", "Edge", "Application", "msedge.exe"))
			}
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				if startHidden(p, "--new-window", "--app="+url, "--window-size=1440,950") == nil {
					return
				}
			}
		}
		_ = startHidden("rundll32.exe", "url.dll,FileProtocolHandler", url)
		return
	}
	if runtime.GOOS == "linux" {
		_ = startHidden("xdg-open", url)
	}
}
func startHidden(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
// GUI processes must not inherit STARTF_USESHOWWINDOW/SW_HIDE used by SSH tools.
func startGUI(name string, args ...string) error {
 cmd:=exec.Command(name,args...)
 cmd.Dir=filepath.Dir(name)
 if err:=cmd.Start();err!=nil{return err}
 go cmd.Wait()
 return nil
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--askpass" {
		fmt.Println(os.Getenv("RAWCRAFT_SSH_PASSWORD"))
		return
	}
	appExe, _ = os.Executable()
	// OpenSSH invokes SSH_ASKPASS using this executable directly with one prompt
	// parameter; if an unknown argument is passed, treat it as ASKPASS.
	if len(os.Args) > 1 && os.Args[1] != "--no-open" && os.Args[1] != "--serve" {
		if os.Getenv("RAWCRAFT_SSH_PASSWORD") != "" {
			fmt.Println(os.Getenv("RAWCRAFT_SSH_PASSWORD"))
			return
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "--install-update" {
		installUpdate()
		return
	}
	configureUpdateReady(os.Args)
 syncInstalledVersion(appExe,appVersion)
	loadSettings()
	state.token = token()
	state.csrf = token()
	mux := http.NewServeMux()
	mux.HandleFunc("/", servePage)
	mux.HandleFunc("/assets/console.js",func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","text/javascript; charset=utf-8");w.Header().Set("Cache-Control","no-store");_,_=w.Write(consoleScript)})
 mux.HandleFunc("/assets/logo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		http.ServeContent(w, r, "logo.png", time.Time{}, bytes.NewReader(logo))
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		http.ServeContent(w, r, "logo.png", time.Time{}, bytes.NewReader(logo))
	})
	mux.HandleFunc("/api/instances", requireAuth(handleInstances))
    mux.HandleFunc("/api/rawcore-service", requireAuth(handleRawcoreService))
    mux.HandleFunc("/api/rawcore-logs", requireAuth(handleRawcoreLogs))
    mux.HandleFunc("/api/instances/remove", route("POST", handleRemoveInstance))
    mux.HandleFunc("/api/instances/prepare",route("POST",handlePrepareInstance))
    mux.HandleFunc("/api/security", requireAuth(handleSecurity))
	mux.HandleFunc("/api/password", route("POST", handleChangePassword))
	mux.HandleFunc("/api/theme", route("POST", handleTheme))
 mux.HandleFunc("/api/github-access",route("POST",handleGithubAccess))
 mux.HandleFunc("/api/scale",route("POST",handleScale))
	mux.HandleFunc("/api/session/renew", handleRenewSession)
	mux.HandleFunc("/api/settings", requireAuth(handleSettings))
	mux.HandleFunc("/api/update", requireAuth(handleUpdate))
	mux.HandleFunc("/api/update/apply", route("POST", handleApplyUpdate))
 mux.HandleFunc("/api/update/started", route("POST", handleUpdateStarted))
	mux.HandleFunc("/api/backups", requireAuth(handleBackups))
	mux.HandleFunc("/api/latest-log", route("GET", handleLatestLog))
	mux.HandleFunc("/api/minecraft/command", route("POST", handleMinecraftCommand))
	mux.HandleFunc("/api/connect-saved", route("POST", handleConnectSaved))
	mux.HandleFunc("/api/connect", route("POST", handleConnect))
	mux.HandleFunc("/api/disconnect", route("POST", handleDisconnect))
	mux.HandleFunc("/api/status", route("GET", handleStatus))
    mux.HandleFunc("/api/monitor", route("GET", handleMonitor))
    mux.HandleFunc("/api/payments/status", route("GET", handlePaymentStatus))
	mux.HandleFunc("/api/action", route("POST", handleAction))
	mux.HandleFunc("/api/logs", route("GET", handleLogs))
	mux.HandleFunc("/api/files", route("GET", handleFiles))
	mux.HandleFunc("/api/read", route("GET", handleRead))
	mux.HandleFunc("/api/save", route("POST", handleSave))
	mux.HandleFunc("/api/upload", route("POST", handleUpload))
	mux.HandleFunc("/api/download", route("GET", handleDownload))
	mux.HandleFunc("/api/delete",route("POST",handleDeleteFile))
 mux.HandleFunc("/api/mkdir", route("POST", handleMkdir))
	mux.HandleFunc("/api/terminal", route("POST", handleTerminal))
 mux.HandleFunc("/api/shell",route("POST",handleShell))
 mux.Handle("/assets/vendor/",http.FileServer(http.FS(terminalAssets)))
	mux.HandleFunc("/api/quit", route("POST", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]bool{"ok": true})
		go func() { time.Sleep(300 * time.Millisecond); os.Exit(0) }()
	}))
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	go updateLoop()
	url := "http://" + l.Addr().String() + "/?auth=" + state.token
	if len(os.Args) > 1 && (os.Args[1] == "--no-open" || os.Args[1] == "--serve") {
		fmt.Println(url)
		if e := http.Serve(l, mux); e != nil {
			log.Fatal(e)
		}
		return
	}
	go func() {
		if e := http.Serve(l, mux); e != nil {
			log.Print(e)
		}
	}()
	if runNativeApp(url) {
		return
	}
	openApp(url)
	select {}
}


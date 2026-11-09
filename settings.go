package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Preferences struct {
 Instances []ServerInstance `json:"instances,omitempty"`
 Scale string `json:"scale"`
	Theme            string `json:"theme"`
	UseAdmin         bool   `json:"useAdmin"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	User             string `json:"user"`
	ServerPath       string `json:"serverPath"`
	MinecraftProcess string `json:"minecraftProcess"`
	WWWProcess       string `json:"wwwProcess"`
	RememberPassword bool   `json:"rememberPassword"`
	AutoLogin        bool   `json:"autoLogin"`
	SSHCipher        string `json:"sshCipher,omitempty"`
	AutoUpdate       bool   `json:"autoUpdate"`
	TokenCipher      string `json:"tokenCipher,omitempty"`
}

var settings = struct {
	sync.RWMutex
	Prefs Preferences
	Token string
}{Prefs: Preferences{Port: 22, User: "user", ServerPath: "/opt/minecraft/server", MinecraftProcess: "minecraft", WWWProcess: "www", AutoUpdate: true, Theme: "system", UseAdmin: true, RememberPassword: true, AutoLogin: true}}

func configPath() (string, error) {
	dir, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, "RawcraftControl", "settings.json"), nil
}
func loadSettings() {
	path, e := configPath()
	if e == nil {
		if b, e := os.ReadFile(path); e == nil {
			_ = json.Unmarshal(b, &settings.Prefs)
		}
	}
	if settings.Prefs.Port == 0 {
		settings.Prefs.Port = 22
	}
	if settings.Prefs.ServerPath == "" {
		settings.Prefs.ServerPath = "/opt/minecraft/server"
	}
	if b, e := base64.StdEncoding.DecodeString(settings.Prefs.TokenCipher); e == nil && len(b) > 0 {
		if raw, e := cryptToken(b, true); e == nil {
			settings.Token = string(raw)
		}
	}
	if env := os.Getenv("RAWCRAFT_GITHUB_TOKEN"); env != "" {
		settings.Token = env
	}
}
func publicSettings() map[string]any {
	settings.RLock()
	defer settings.RUnlock()
	p := settings.Prefs
    p.Instances=normalizedInstances(p)
	p.TokenCipher = ""
	p.SSHCipher = ""
	return map[string]any{"preferences": p, "hasToken": settings.Token != "", "hasSavedPassword": settings.Prefs.SSHCipher != "", "version": appVersion, "repository": updateRepository}
}
func handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		ok(w, publicSettings())
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method", 405)
		return
	}
	var req struct {
		Preferences   Preferences `json:"preferences"`
		GitHubToken   string      `json:"githubToken"`
		ClearToken    bool        `json:"clearToken"`
		ClearPassword bool        `json:"clearPassword"`
	}
	if e := fromJSON(r, &req); e != nil {
		bad(w, e)
		return
	}
	p := req.Preferences
 if p.Scale==""{p.Scale="auto"};if !validScale(p.Scale){bad(w,errors.New("Niepoprawna skala interfejsu"));return}
	if p.Theme == "" {
		p.Theme = "system"
	}
	if !validTheme(p.Theme) {
		bad(w, errors.New("Nieprawidłowy motyw"))
		return
	}
	p.TokenCipher = ""
	if p.Host != "" && !hostRegex.MatchString(p.Host) {
		bad(w, errors.New("Niepoprawny adres VPS"))
		return
	}
	if p.Port < 1 || p.Port > 65535 || !userRegex.MatchString(p.User) {
		bad(w, errors.New("Niepoprawny port lub użytkownik"))
		return
	}
	if e := checkPath(p.ServerPath); e != nil {
		bad(w, e)
		return
	}
	if strings.TrimSpace(p.MinecraftProcess) == "" || (strings.TrimSpace(p.WWWProcess) == "" && len(p.Instances)==0) || len(p.MinecraftProcess) > 100 || len(p.WWWProcess) > 100 {
		bad(w, errors.New("Podaj nazwy procesów PM2 (do 100 znaków)"))
		return
	}
    if len(p.Instances)>0 {if e:=validateInstances(p.Instances);e!=nil{bad(w,e);return}}
	settings.Lock()
    // Old GUI/settings callers must never erase configured extra instances.
    if len(p.Instances)==0&&len(settings.Prefs.Instances)>0{p.Instances=append([]ServerInstance(nil),settings.Prefs.Instances...)}
	newToken := settings.Token
	p.TokenCipher = settings.Prefs.TokenCipher
	p.SSHCipher = settings.Prefs.SSHCipher
	if req.ClearPassword || !p.RememberPassword || p.Host != settings.Prefs.Host || p.Port != settings.Prefs.Port || p.User != settings.Prefs.User {
		p.SSHCipher = ""
	}
	if !p.RememberPassword {
		p.AutoLogin = false
	}
	if req.ClearToken {
		newToken = ""
		p.TokenCipher = ""
	}
	if req.GitHubToken != "" {
		encrypted, e := cryptToken([]byte(strings.TrimSpace(req.GitHubToken)), false)
		if e != nil {
			settings.Unlock()
			bad(w, e)
			return
		}
		newToken = strings.TrimSpace(req.GitHubToken)
		p.TokenCipher = base64.StdEncoding.EncodeToString(encrypted)
	}
	e := writePreferences(p)
	if e == nil {
		settings.Prefs = p
		settings.Token = newToken
	}
	settings.Unlock()
	if e != nil {
		bad(w, errors.New("Nie udało się zapisać ustawień: "+e.Error()))
		return
	}
	updateNativeTheme(p.Theme)
	ok(w, publicSettings())
	go checkForUpdate(false)
}

// The caller holds the settings lock while writing and committing a new snapshot.
func writePreferences(p Preferences) error {
	path, e := configPath()
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	data, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), "settings-*.tmp")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	_, e = f.Write(data)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e == nil {
		e = os.Rename(name, path)
	}
	return e
}
func rememberConnection(c Credentials, remember, auto bool) error {
	cipher := ""
	if remember {
		raw, e := json.Marshal(c)
		if e != nil {
			return e
		}
		encrypted, e := cryptToken(raw, false)
		if e != nil {
			return e
		}
		cipher = base64.StdEncoding.EncodeToString(encrypted)
	}
	settings.Lock()
	defer settings.Unlock()
	p := settings.Prefs
	p.Host = c.Host
	p.Port = c.Port
	p.User = c.User
	p.SSHCipher = cipher
	p.RememberPassword = remember
	p.AutoLogin = remember && auto
	if e := writePreferences(p); e != nil {
		return e
	}
	settings.Prefs = p
	return nil
}
func savedCredentials() (Credentials, error) {
	settings.RLock()
	p := settings.Prefs
	settings.RUnlock()
	var c Credentials
	if !p.RememberPassword || p.SSHCipher == "" {
		return c, errors.New("Brak zapamiętanego hasła. Zaloguj się ponownie.")
	}
	encrypted, e := base64.StdEncoding.DecodeString(p.SSHCipher)
	if e != nil {
		return c, e
	}
	raw, e := cryptToken(encrypted, true)
	if e != nil {
		return c, errors.New("Windows nie może odszyfrować zapamiętanego hasła na tym koncie")
	}
	if e = json.Unmarshal(raw, &c); e != nil {
		return c, e
	}
	if c.Host != p.Host || c.Port != p.Port || c.User != p.User {
		return Credentials{}, errors.New("Zapamiętane hasło dotyczy innego profilu VPS")
	}
	if e = checkedCred(c); e != nil {
		return Credentials{}, e
	}
	return c, nil
}

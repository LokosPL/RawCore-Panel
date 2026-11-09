package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const appVersion = "1.5.0"
const updateRepository = "LokosPL/RawCore-Panel"
const updateAsset = "RAWCRAFT-Control.exe"
const maxUpdateSize int64 = 256 << 20

type UpdateStatus struct {
	SHA256     string `json:"sha256,omitempty"`
	State      string `json:"state"`
	Message    string `json:"message"`
	Current    string `json:"current"`
	Latest     string `json:"latest,omitempty"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	CheckedAt  string `json:"checkedAt,omitempty"`
}

var updates = struct {
	sync.Mutex
	Status UpdateStatus
	Busy   bool
 Pending bool
	Staged string
	Hash   string
}{Status: UpdateStatus{State: "idle", Message: "Oczekiwanie na sprawdzenie", Current: appVersion}}
// Only GitHub's fixed API routes and release storage can receive updater traffic.
var githubClient = &http.Client{Timeout: 20 * time.Minute, CheckRedirect: checkUpdateRedirect}

func validUpdateAPI(u *url.URL) bool {
 if u.Scheme != "https" || u.Host != "api.github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" { return false }
 base := "/repos/"+updateRepository+"/releases/"
 if u.Path == base+"latest" { return true }
 id := strings.TrimPrefix(u.Path,base+"assets/")
 if id == u.Path || id == "" { return false }
 for _, r := range id { if r < '0' || r > '9' { return false } }
 return true
}
func checkUpdateRedirect(req *http.Request, via []*http.Request) error {
 if len(via)>8 { return errors.New("Zbyt wiele przekierowań") }
 u:=req.URL
 if u.Scheme!="https" || u.User!=nil || u.Fragment!="" { return errors.New("Nieprawidłowe przekierowanie aktualizacji") }
 if u.Host=="api.github.com" {
  if !validUpdateAPI(u) { return errors.New("Nieprawidłowy adres API aktualizacji") }
  for _, previous:=range via { if previous.URL.Host!="api.github.com" { req.Header.Del("Authorization");break } }
  return nil
 }
 req.Header.Del("Authorization")
 switch u.Host {
 case "release-assets.githubusercontent.com", "objects.githubusercontent.com", "github-releases.githubusercontent.com": return nil
 default: return errors.New("Niedozwolony serwer pobierania aktualizacji")
 }
}

func stableVersion(s string) ([3]int, error) {
	var v [3]int
	s = strings.TrimPrefix(s, "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, errors.New("Wymagany numer wersji X.Y.Z")
	}
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return v, errors.New("Niepoprawna wersja")
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return v, errors.New("Niepoprawna wersja")
			}
		}
		n, e := strconv.Atoi(p)
		if e != nil {
			return v, e
		}
		v[i] = n
	}
	return v, nil
}
func newerVersion(candidate, current string) bool {
	a, e := stableVersion(candidate)
	if e != nil {
		return false
	}
	b, e := stableVersion(current)
	if e != nil {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
func githubRequest(ctx context.Context, rawURL, token, accept string) (*http.Response, error) {
	u, e := url.Parse(rawURL)
	if e != nil {
		return nil, e
	}
	if !validUpdateAPI(u) {
		return nil, errors.New("Nieprawidłowy adres aktualizacji")
	}
	req, e := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", accept)
 req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "RAWCRAFT-Control/"+appVersion)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, e := githubClient.Do(req)
	if e != nil {
		return nil, e
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
        if resp.StatusCode==403&&resp.Header.Get("X-RateLimit-Remaining")=="0"{return nil,errors.New("Limit zapytań GitHub został wyczerpany. Aktualizator spróbuje ponownie później.")}
        if resp.StatusCode==401||resp.StatusCode==403||resp.StatusCode==404 {
            return nil, &updateAccessError{Code:resp.StatusCode}
        }
        return nil, fmt.Errorf("GitHub HTTP %d. Spróbuj ponownie za chwilę.",resp.StatusCode)
	}
	return resp, nil
}

type updateAccessError struct{ Code int }
func (e *updateAccessError) Error() string {return fmt.Sprintf("GitHub HTTP %d: token jest nieważny, wygasł albo nie ma dostępu do repozytorium RawCore-Panel (Contents: Read-only).",e.Code)}
func updateFailure(e error){var access *updateAccessError;if errors.As(e,&access){setUpdate("access_denied",e.Error());return};setUpdate("error",e.Error())}

func checksumFor(data []byte, name string) (string, error) {
 var found string
 for _, line := range strings.Split(string(data), "\n") {
  fields:=strings.Fields(line)
  if len(fields)!=2 || strings.TrimPrefix(fields[1],"*")!=name { continue }
  raw,e:=hex.DecodeString(fields[0])
  if e!=nil || len(raw)!=32 || found!="" { return "",errors.New("Niepoprawna lub powtórzona suma SHA-256 aplikacji") }
  found=strings.ToLower(fields[0])
 }
 if found!="" { return found,nil }
 return "",errors.New("Brak poprawnej sumy SHA-256 aplikacji w SHA256SUMS")
}

func updateSnapshot() UpdateStatus { updates.Lock(); defer updates.Unlock(); return updates.Status }
func setUpdate(state, message string) {
	updates.Lock()
	updates.Status.State = state
	updates.Status.Message = message
	updates.Unlock()
}
func updateLoop() {
    if dir,e:=os.UserCacheDir();e==nil{path:=filepath.Join(dir,"RawcraftControl","updates","install-error.txt");if b,e:=os.ReadFile(path);e==nil{setUpdate("install_error","Poprzednia aktualizacja nie została zainstalowana: "+string(b));_ = os.Remove(path)}else{checkForUpdate(false)}}else{checkForUpdate(false)}
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		checkForUpdate(false)
	}
}
func checkForUpdate(manual bool) {
	updates.Lock()
	if updates.Busy {if manual{updates.Pending=true};updates.Unlock();return}
 if updates.Status.State == "ready" {
		updates.Unlock()
		return
	}
	updates.Busy = true
	updates.Unlock()
	defer func() {
		updates.Lock()
		updates.Busy = false
        pending:=updates.Pending;updates.Pending=false
		updates.Status.CheckedAt = time.Now().Format(time.RFC3339)
		updates.Unlock()
        if pending{go checkForUpdate(true)}
	}()
	settings.RLock()
	enabled, token := settings.Prefs.AutoUpdate, settings.Token
	settings.RUnlock()
	if !enabled && !manual {
		setUpdate("disabled", "Automatyczne aktualizacje są wyłączone")
		return
	}
	if runtime.GOOS != "windows" {
		setUpdate("unsupported", "Automatyczne aktualizacje są dostępne dla Windows x64")
		return
	}
    if strings.TrimSpace(token)==""{setUpdate("needs_token","Wymagana jednorazowa autoryzacja prywatnych aktualizacji GitHub.");return}
	setUpdate("checking", "Sprawdzanie aktualizacji…")
    updates.Lock();updates.Status.Downloaded=0;updates.Status.Total=0;updates.Status.SHA256="";updates.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	metadataCtx,metadataCancel:=context.WithTimeout(ctx,25*time.Second)
 resp,e:=githubRequest(metadataCtx,"https://api.github.com/repos/"+updateRepository+"/releases/latest",token,"application/vnd.github+json")
	if e != nil {
        metadataCancel()
		updateFailure(e)
		return
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	e = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&release)
	resp.Body.Close()
 metadataCancel()
	if e != nil {
		setUpdate("error", "Niepoprawna odpowiedź GitHub")
		return
	}
	if _, e = stableVersion(release.Tag); e != nil {
		setUpdate("error", "Release musi mieć stabilny numer vX.Y.Z")
		return
	}
	if release.Draft || release.Prerelease {
		setUpdate("current", "Pomijanie wersji testowej")
		return
	}
	updates.Lock()
	updates.Status.Latest = release.Tag
	updates.Unlock()
	if !newerVersion(release.Tag, appVersion) {
		setUpdate("current", "Masz najnowszą wersję")
		return
	}
	var assetURL, checksumURL string
	var total int64
	for _, a := range release.Assets {
		if a.Name == updateAsset {
			assetURL = a.URL
			total = a.Size
		}
		if a.Name == "SHA256SUMS" {
			checksumURL = a.URL
		}
	}
	if assetURL == "" || checksumURL == "" || total < 4096 || total > maxUpdateSize {
		setUpdate("error", "Release nie ma aplikacji Windows i sum SHA256SUMS")
		return
	}
	resp, e = githubRequest(ctx, checksumURL, token, "application/octet-stream")
	if e != nil {
		updateFailure(e)
		return
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	resp.Body.Close()
	if e != nil || len(data) > 1<<20 {
		setUpdate("error", "Nie udało się pobrać sum kontrolnych")
		return
	}
	expected, e := checksumFor(data, updateAsset)
	if e != nil {
		updateFailure(e)
		return
	}
	updates.Lock()
	updates.Status.Total = total
	updates.Status.Downloaded = 0
	updates.Unlock()
	setUpdate("downloading", "Pobieranie nowej wersji w tle…")
	staged, e := downloadUpdate(ctx, assetURL, token, expected, total)
	if e != nil {
		updateFailure(e)
		return
	}
	updates.Lock()
	updates.Status.SHA256 = expected
	updates.Staged = staged
	updates.Hash = expected
	updates.Unlock()
	setUpdate("ready", "Nowa wersja RAWCRAFT jest gotowa. Zainstaluj aktualizację i otwórz panel ponownie.")
}

type updateProgress struct{}

func (updateProgress) Write(p []byte) (int, error) {
	updates.Lock()
	updates.Status.Downloaded += int64(len(p))
	updates.Unlock()
	return len(p), nil
}
func downloadUpdate(ctx context.Context, assetURL, token, expected string, total int64) (string, error) {
	dir, e := os.UserCacheDir()
	if e != nil {
		return "", e
	}
	dir = filepath.Join(dir, "RawcraftControl", "updates")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	f, e := os.CreateTemp(dir, "update-*.exe")
	if e != nil {
		return "", e
	}
	path := f.Name()
	success := false
	defer func() {
		f.Close()
		if !success {
			os.Remove(path)
		}
	}()
	resp, e := githubRequest(ctx, assetURL, token, "application/octet-stream")
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	hash := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, hash, updateProgress{}), io.LimitReader(resp.Body, maxUpdateSize+1))
	if e != nil {
		return "", e
	}
	if n != total || n > maxUpdateSize {
		return "", errors.New("Niepełny lub zbyt duży plik aktualizacji")
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return "", errors.New("Niezgodna suma SHA-256. Aktualizacja została odrzucona")
	}
	if e = f.Sync(); e != nil {
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	if e = verifyExecutable(path, expected); e != nil {
		return "", e
	}
	success = true
	return path, nil
}
func verifyExecutable(path, expected string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	var magic [2]byte
	if _, e = io.ReadFull(f, magic[:]); e != nil || string(magic[:]) != "MZ" {
		return errors.New("Aktualizacja nie jest aplikacją Windows")
	}
	if _, e = f.Seek(0, 0); e != nil {
		return e
	}
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, maxUpdateSize+1))
	if e != nil {
		return e
	}
	if n > maxUpdateSize || hex.EncodeToString(h.Sum(nil)) != expected {
		return errors.New("Plik aktualizacji zmienił się po pobraniu")
	}
	return nil
}
func handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		go checkForUpdate(true)
	} else if r.Method != "GET" {
		http.Error(w, "Method", 405)
		return
	}
	ok(w, updateSnapshot())
}
func handleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	updates.Lock()
	if updates.Status.State != "ready" || updates.Busy {
		updates.Unlock()
		bad(w, errors.New("Brak gotowej aktualizacji"))
		return
	}
	updates.Busy = true
	path, hash := updates.Staged, updates.Hash
	updates.Unlock()
	fail := func(e error) { updates.Lock(); updates.Busy = false; updates.Unlock(); bad(w, e) }
	if e := verifyExecutable(path, hash); e != nil {
		fail(e)
		return
	}
	targetDir:=filepath.Dir(appExe)
 probe,e:=os.CreateTemp(targetDir,".rawcraft-update-write-*")
 if e!=nil{fail(errors.New("Folder aplikacji nie pozwala na zapis. Zainstaluj panel w folderze swojego konta albo przenieś wersję portable do zapisywalnego folderu."));return}
 probeName:=probe.Name();probe.Close();os.Remove(probeName)
 dir := filepath.Dir(path)
 helper, e := os.CreateTemp(dir, "installer-*.exe")
	if e != nil {
		fail(e)
		return
	}
	helperName := helper.Name()
	helper.Close()
	if e = copyFile(path, helperName); e != nil {
		os.Remove(helperName)
		fail(e)
		return
	}
	target, e := filepath.EvalSymlinks(appExe)
	if e != nil {
		os.Remove(helperName)
		fail(e)
		return
	}
	if e = verifyExecutable(helperName,hash);e!=nil{os.Remove(helperName);fail(e);return}
 endpoint,waitReady,closeHandoff,e:=installerHandoff(hash)
 if e!=nil{os.Remove(helperName);fail(e);return};defer closeHandoff()
 if e = startGUI(helperName, "--install-update", strconv.Itoa(os.Getpid()), target, path, hash, endpoint); e != nil {
		os.Remove(helperName)
		fail(e)
		return
	}
 if e=waitReady();e!=nil{fail(e);return}
	ok(w, map[string]bool{"ok": true})
	go func() { time.Sleep(time.Second); os.Exit(0) }()
}
func copyFile(from, to string) error {
	src, e := os.Open(from)
	if e != nil {
		return e
	}
	defer src.Close()
	dst, e := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0700)
	if e != nil {
		return e
	}
	_, e = io.Copy(dst, src)
	if e == nil {
		e = dst.Sync()
	}
	ce := dst.Close()
	if e == nil {
		e = ce
	}
	return e
}
// Compatibility entry point for callers that only need the committed replacement.
func replaceExecutable(target, staged, expected string) error {
 _,err:=replaceWithBackup(target,staged,expected);return err
}

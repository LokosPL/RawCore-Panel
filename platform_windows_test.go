//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"github.com/jchv/go-webview2/webviewloader"
	"os"
 "path/filepath"
 "crypto/sha256"
 "encoding/hex"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestChildProcessesHaveNoConsoleWindow(t *testing.T) {
	cmd := exec.Command("ssh.exe")
	hideWindow(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow || cmd.SysProcAttr.CreationFlags&0x08000000 == 0 {
		t.Fatal("console window can appear")
	}
}
func TestTokenDPAPIRoundTrip(t *testing.T) {
	raw := []byte("test-token-not-a-credential")
	cipher, e := cryptToken(raw, false)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(cipher, raw) {
		t.Fatal("token stored in plaintext")
	}
	plain, e := cryptToken(cipher, true)
	if e != nil || !bytes.Equal(raw, plain) {
		t.Fatalf("DPAPI round trip failed: %v", e)
	}
}

func TestRememberedSSHSecretIsEncryptedAndBoundToProfile(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	settings.Lock()
	old := settings.Prefs
	settings.Unlock()
	defer func() { settings.Lock(); settings.Prefs = old; settings.Unlock() }()
	c := Credentials{Host: "192.0.2.50", Port: 2222, User: "minecraft", Password: "test-only-secret-123"}
	if e := rememberConnection(c, true, true); e != nil {
		t.Fatal(e)
	}
	path, _ := configPath()
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(data), c.Password) {
		t.Fatal("plaintext password persisted")
	}
	public, _ := json.Marshal(publicSettings())
	if strings.Contains(string(public), "sshCipher") || strings.Contains(string(public), c.Password) {
		t.Fatal("secret exposed to UI")
	}
	restored, e := savedCredentials()
	if e != nil || restored != c {
		t.Fatalf("cannot restore credentials: %v", e)
	}
	settings.Lock()
	settings.Prefs.Host = "192.0.2.51"
	settings.Unlock()
	if _, e = savedCredentials(); e == nil {
		t.Fatal("password reused for a different host")
	}
	if e = rememberConnection(c, false, false); e != nil {
		t.Fatal(e)
	}
	if _, e = savedCredentials(); e == nil {
		t.Fatal("password not forgotten")
	}
}
func TestNativeWindowOpensAndCloses(t *testing.T) {
	if _, e := webviewloader.GetInstalledVersion(); e != nil {
		t.Skip("WebView2 is not installed on this runner")
	}
	if !runNativeWindow("about:blank", func(hwnd uintptr) {
		go func() {
			time.Sleep(300 * time.Millisecond)
			syscall.NewLazyDLL("user32.dll").NewProc("PostMessageW").Call(hwnd, 0x10, 0, 0)
		}()
	}) {
		t.Fatal("native app window could not be created")
	}
}

func TestInstalledUpdatePreservesInstallation(t *testing.T){
 dir:=os.Getenv("RAWCRAFT_UPDATE_SMOKE_DIR");if dir==""{t.Skip("installer integration test")}
 target:=filepath.Join(dir,"RAWCRAFT-Control.exe");uninstaller:=filepath.Join(dir,"unins000.exe")
 if _,e:=os.Stat(uninstaller);e!=nil{t.Fatal(e)}
 data,e:=os.ReadFile(target);if e!=nil{t.Fatal(e)};hash:=sha256.Sum256(data)
 staged:=filepath.Join(t.TempDir(),"new.exe");if e=os.WriteFile(staged,data,0600);e!=nil{t.Fatal(e)}
 if e=replaceExecutable(target,staged,hex.EncodeToString(hash[:]));e!=nil{t.Fatal(e)}
 if _,e=os.Stat(uninstaller);e!=nil{t.Fatal("update removed uninstaller")}
 syncInstalledVersion(target,appVersion)
}

func TestGitHubAccessStoredEncryptedAndNeverReturned(t *testing.T){
 t.Setenv("APPDATA",t.TempDir())
 settings.Lock();oldPrefs,oldToken:=settings.Prefs,settings.Token;settings.Unlock()
 defer func(){settings.Lock();settings.Prefs=oldPrefs;settings.Token=oldToken;settings.Unlock()}()
 secret:="unit-test-only-github-access-value"
 if e:=storeGithubAccess(secret,false);e!=nil{t.Fatal(e)}
 path,_:=configPath();data,e:=os.ReadFile(path);if e!=nil{t.Fatal(e)}
 if bytes.Contains(data,[]byte(secret)){t.Fatal("credential persisted in plaintext")}
 public,_:=json.Marshal(publicSettings());if bytes.Contains(public,[]byte(secret))||bytes.Contains(public,[]byte("tokenCipher")){t.Fatal("credential exposed to UI")}
 settings.RLock();cipher:=settings.Prefs.TokenCipher;settings.RUnlock()
 if cipher==""{t.Fatal("missing protected credential")}
 if e:=storeGithubAccess("",true);e!=nil{t.Fatal(e)}
 settings.RLock();remaining,encrypted:=settings.Token,settings.Prefs.TokenCipher;settings.RUnlock()
 if remaining!=""||encrypted!=""{t.Fatal("credential not forgotten")}
}

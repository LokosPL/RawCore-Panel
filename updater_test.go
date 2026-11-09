package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
 "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVersionOrdering(t *testing.T) {
	for _, tc := range []struct {
		candidate, current string
		newer              bool
	}{
		{"v1.2.0", "1.1.0", true}, {"1.10.0", "1.9.9", true}, {"2.0.0", "1.99.99", true},
		{"1.1.0", "1.1.0", false}, {"v1.0.9", "1.1.0", false}, {"1.2.0-beta", "1.1.0", false},
		{"1.2", "1.1.0", false}, {"01.2.0", "1.1.0", false}, {"-1.2.0", "1.1.0", false},
	} {
		if got := newerVersion(tc.candidate, tc.current); got != tc.newer {
			t.Errorf("%s > %s = %v", tc.candidate, tc.current, got)
		}
	}
}
func TestChecksumParsing(t *testing.T) {
	sum := strings.Repeat("a", 64)
	for _, data := range []string{sum + "  RAWCRAFT-Control.exe\n", sum + " *RAWCRAFT-Control.exe\r\n"} {
		got, e := checksumFor([]byte(data), updateAsset)
		if e != nil || got != sum {
			t.Fatalf("%s: %v", got, e)
		}
	}
	for _, data := range []string{sum + "  malicious.exe", strings.Repeat("z", 64) + "  " + updateAsset, "abc " + updateAsset} {
		if _, e := checksumFor([]byte(data), updateAsset); e == nil {
			t.Fatal("accepted invalid checksum")
		}
	}
}
func TestUpdateRejectsUntrustedURLsBeforeNetwork(t *testing.T) {
	for _, u := range []string{"http://api.github.com/repos/" + updateRepository + "/releases/latest", "https://evil.example/payload", "https://api.github.com/repos/attacker/repo/releases/latest", "https://api.github.com.evil.example/repos/" + updateRepository + "/releases/latest"} {
		if resp, e := githubRequest(context.Background(), u, "secret", "application/json"); e == nil {
			resp.Body.Close()
			t.Fatal("accepted untrusted URL")
		}
	}
}
func TestVerifiedReplacementAndTamperProtection(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "app.exe")
	staged := filepath.Join(dir, "update.exe")
	old := []byte("MZold")
	payload := []byte("MZnew application")
	h := sha256.Sum256(payload)
	sum := hex.EncodeToString(h[:])
	if e := os.WriteFile(target, old, 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(staged, payload, 0600); e != nil {
		t.Fatal(e)
	}
	replacement,e := replaceWithBackup(target, staged, sum)
	if e != nil {
		t.Fatal(e)
	}
	actual, _ := os.ReadFile(target)
	backup, _ := os.ReadFile(replacement.backup)
	if string(actual) != string(payload) || string(backup) != string(old) {
		t.Fatal("replacement or backup incorrect")
	}
	if e := os.WriteFile(staged, []byte("MZtampered"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := replaceExecutable(target, staged, sum); e == nil {
		t.Fatal("tampered update was accepted")
	}
	actual, _ = os.ReadFile(target)
	if string(actual) != string(payload) {
		t.Fatal("tampered update changed target")
	}
}
func TestRejectsNonExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.exe")
	data := []byte("not an executable")
	os.WriteFile(path, data, 0600)
	h := sha256.Sum256(data)
	if verifyExecutable(path, hex.EncodeToString(h[:])) == nil {
		t.Fatal("accepted invalid executable")
	}
}

type mockTransport func(*http.Request) (*http.Response, error)

func (fn mockTransport) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }
func TestAutomaticCheckDownloadsVerifiedRelease(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("updater targets Windows")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	payload := []byte("MZ" + strings.Repeat("x", 8192))
	hash := sha256.Sum256(payload)
	sum := hex.EncodeToString(hash[:])
	oldClient := githubClient
	defer func() { githubClient = oldClient }()
	settings.Lock()
	oldPrefs, oldToken := settings.Prefs, settings.Token
	settings.Prefs.AutoUpdate = true
	settings.Token = "test-token"
	settings.Unlock()
	defer func() { settings.Lock(); settings.Prefs = oldPrefs; settings.Token = oldToken; settings.Unlock() }()
	updates.Lock()
	updates.Status = UpdateStatus{State: "idle", Current: appVersion}
	updates.Busy = false
	updates.Unlock()
	githubClient = &http.Client{Transport: mockTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing private repository authorization")
		}
		var body string
		switch r.URL.Path {
		case "/repos/" + updateRepository + "/releases/latest":
			body = fmt.Sprintf(`{"tag_name":"v99.0.0","assets":[{"name":"RAWCRAFT-Control.exe","url":"https://api.github.com/repos/%s/releases/assets/1","size":%d},{"name":"SHA256SUMS","url":"https://api.github.com/repos/%s/releases/assets/2"}]}`, updateRepository, len(payload), updateRepository)
		case "/repos/" + updateRepository + "/releases/assets/1":
			body = string(payload)
		case "/repos/" + updateRepository + "/releases/assets/2":
			body = sum + "  " + updateAsset
		default:
			t.Fatalf("unexpected URL %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	checkForUpdate(false)
	status := updateSnapshot()
	if status.State != "ready" || status.Downloaded != int64(len(payload)) {
		t.Fatalf("automatic download failed: %+v", status)
	}
	updates.Lock()
	path := updates.Staged
	updates.Unlock()
	defer os.Remove(path)
	if e := verifyExecutable(path, sum); e != nil {
		t.Fatal(e)
	}
}

func TestReplacementRecoversInterruptedRollback(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "app.exe")
	staged := filepath.Join(dir, "update.exe")
	old := []byte("MZprevious good application")
	payload := []byte("MZnew application")
	h := sha256.Sum256(payload)
	sum := hex.EncodeToString(h[:])
	os.WriteFile(target+".old", old, 0600)
	os.WriteFile(staged, payload, 0600)
	if e := replaceExecutable(target, staged, sum); e != nil {
		t.Fatal(e)
	}
	backup, _ := os.ReadFile(target + ".old")
	if string(backup) != string(old) {
		t.Fatal("lost good backup during interrupted rollback recovery")
	}
}

func TestGitHubAccessFailuresAreActionable(t *testing.T){
 old:=githubClient;defer func(){githubClient=old}()
 for _,code:=range []int{401,403,404}{
 githubClient=&http.Client{Transport:mockTransport(func(r *http.Request)(*http.Response,error){return &http.Response{StatusCode:code,Body:io.NopCloser(strings.NewReader("")),Header:make(http.Header)},nil})}
 _,e:=githubRequest(context.Background(),"https://api.github.com/repos/"+updateRepository+"/releases/latest","expired-test-token","application/json")
 if e==nil{t.Fatal("accepted denied access")};updateFailure(e);status:=updateSnapshot();if status.State!="access_denied"||!strings.Contains(status.Message,"Contents: Read-only"){t.Fatalf("unhelpful access failure: %+v",status)}
 }
}
func TestMissingTokenDoesNotPretendToBeCurrent(t *testing.T){
 if runtime.GOOS!="windows"{t.Skip("Windows updater")}
 settings.Lock();oldToken,oldAuto:=settings.Token,settings.Prefs.AutoUpdate;settings.Token="";settings.Prefs.AutoUpdate=true;settings.Unlock()
 defer func(){settings.Lock();settings.Token=oldToken;settings.Prefs.AutoUpdate=oldAuto;settings.Unlock()}()
 updates.Lock();updates.Status=UpdateStatus{State:"idle",Current:appVersion};updates.Busy=false;updates.Unlock()
 old:=githubClient;defer func(){githubClient=old}();githubClient=&http.Client{Transport:mockTransport(func(r *http.Request)(*http.Response,error){t.Fatal("request without private repository credential");return nil,nil})}
 checkForUpdate(false);if updateSnapshot().State!="needs_token"{t.Fatalf("missing token hidden: %+v",updateSnapshot())}
}


func TestUpdateRejectsAmbiguousOrUnexpectedAPIPaths(t *testing.T) {
 for _, address:=range []string{
  "https://secret@api.github.com/repos/"+updateRepository+"/releases/latest",
  "https://api.github.com/repos/"+updateRepository+"/contents/config",
  "https://api.github.com/repos/"+updateRepository+"/releases/assets/1?redirect=evil",
  "https://api.github.com/repos/"+updateRepository+"/releases/assets/../latest",
  "https://api.github.com/repos/"+updateRepository+"/releases/assets/%31",
  "https://api.github.com/repos/"+updateRepository+"/releases/latest#payload",
 } {
  if resp,e:=githubRequest(context.Background(),address,"secret","application/json");e==nil {resp.Body.Close();t.Fatal("accepted untrusted URL",address)}
 }
}
func TestUpdateRedirectsKeepTokenOnAPIOnly(t *testing.T) {
 origin,_:=http.NewRequest("GET","https://api.github.com/repos/"+updateRepository+"/releases/assets/1",nil)
 for _,host:=range []string{"release-assets.githubusercontent.com","objects.githubusercontent.com","github-releases.githubusercontent.com"} {
  req,_:=http.NewRequest("GET","https://"+host+"/asset?signature=test",nil);req.Header.Set("Authorization","Bearer secret")
  if e:=checkUpdateRedirect(req,[]*http.Request{origin});e!=nil {t.Fatal(e)}
  if req.Header.Get("Authorization")!="" {t.Fatal("leaked credential")}
 }
 for _,address:=range []string{"http://release-assets.githubusercontent.com/asset","https://evil.example/asset","https://release-assets.githubusercontent.com.evil.example/asset","https://release-assets.githubusercontent.com:444/asset","https://secret@objects.githubusercontent.com/asset","https://api.github.com/repos/attacker/repo/releases/assets/1"} {
  u,_:=url.Parse(address);req:=&http.Request{URL:u,Header:make(http.Header)}
  if checkUpdateRedirect(req,[]*http.Request{origin})==nil {t.Fatal("accepted redirect",address)}
 }
 req,_:=http.NewRequest("GET",origin.URL.String(),nil)
 if checkUpdateRedirect(req,make([]*http.Request,9))==nil {t.Fatal("accepted redirect loop")}
}
func TestUpdateRejectsDuplicateChecksum(t *testing.T) {
 sum:=strings.Repeat("a",64)+"  "+updateAsset+"\n"
 for _,data:=range []string{sum+sum,sum+strings.Repeat("b",64)+"  "+updateAsset,sum+"invalid  "+updateAsset} {
  if _,e:=checksumFor([]byte(data),updateAsset);e==nil {t.Fatal("accepted ambiguous checksum")}
 }
}


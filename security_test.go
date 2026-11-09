package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
 "runtime"
 "os"
 "path/filepath"
 "context"
 "time"
)

type fakeRemoteExit int

func (e fakeRemoteExit) Error() string { return "remote command failed" }
func (e fakeRemoteExit) ExitCode() int { return int(e) }
func TestRemoteErrorsSeparateSSHAuthenticationFromFilePermissions(t *testing.T) {
	for _, tc := range []struct {
		code       int
		text, want string
	}{
		{1, "Traceback (most recent call last):\nPermissionError: [Errno 13] Permission denied: '/opt/minecraft/server/server.properties'", "Brak uprawnień"},
		{255, "user@192.0.2.1: Permission denied (publickey,password).", "Nieprawidłowe hasło"},
		{255, "user@2001:db8::1: Permission denied (password).", "Nieprawidłowe hasło"},
		{77, "RAWCRAFT_SUDO_DENIED: not allowed", "uprawnień sudo"},
	} {
		got := remoteFailure(fakeRemoteExit(tc.code), tc.text).Error()
		if !strings.Contains(got, tc.want) {
			t.Fatalf("%q => %q", tc.text, got)
		}
		if tc.code == 1 && strings.Contains(got, "Nieprawidłowe hasło") {
			t.Fatal("filesystem denial misreported as authentication")
		}
	}
	if !strings.Contains(remoteFailure(errors.New("network issue"), "").Error(), "network issue") {
		t.Fatal("lost transport error")
	}
}
func TestSessionCookiesDoNotCollideAcrossAppInstances(t *testing.T) {
	state.token = "instance-one"
	a := sessionCookieName()
	state.token = "instance-two"
	b := sessionCookieName()
	if a == b {
		t.Fatal("cookie names collide across ports")
	}
	req := httptest.NewRequest("GET", "http://127.0.0.1:1234/api/test", nil)
	req.AddCookie(&http.Cookie{Name: a, Value: "instance-one"})
	w := httptest.NewRecorder()
	route("GET", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })(w, req)
	if w.Code != 403 {
		t.Fatal("another app instance's cookie was accepted")
	}
	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["code"] != "session_expired" {
		t.Fatal("client cannot detect a renewable session")
	}
}
func TestRenewalRequiresExistingPageSecretAndSameOrigin(t *testing.T) {
	state.token = "renewal-session"
	state.csrf = "secret-page-nonce"
	for _, tc := range []struct {
		csrf, origin string
		want         int
	}{
		{"", "", 403}, {"wrong", "http://127.0.0.1:1234", 403}, {state.csrf, "https://attacker.example", 403}, {state.csrf, "http://127.0.0.1:1234", 200},
	} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:1234/api/session/renew", nil)
		r.Header.Set("X-RAWCRAFT-CSRF", tc.csrf)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		handleRenewSession(w, r)
		if w.Code != tc.want {
			t.Fatalf("status %d want %d", w.Code, tc.want)
		}
		cookies := w.Result().Cookies()
		if tc.want == 200 {
			if len(cookies) != 1 || cookies[0].Name != sessionCookieName() || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
				t.Fatal("renewal cookie is not protected")
			}
		} else if len(cookies) > 0 {
			t.Fatal("unauthorized renewal issued a cookie")
		}
	}
}
func TestPrivilegeContextIsNotUsedForPM2OrPersistedInJSON(t *testing.T) {
	state.Lock()
	state.connected = true
	state.creds = Credentials{Host: "192.0.2.20", Port: 22, User: "minecraft", Password: "test-password"}
	state.admin = true
	state.root = false
	state.sudoPassword = "sudo-test-only"
	state.Unlock()
	defer func() {
		state.Lock()
		state.connected = false
		state.admin = false
		state.sudoPassword = ""
		state.Unlock()
	}()
	ordinary, _ := currentCred()
	elevated, _ := currentPrivilegedCred()
	if ordinary.privileged || !elevated.privileged {
		t.Fatal("PM2 user and file administrator context were mixed")
	}
	b, _ := json.Marshal(elevated)
	if strings.Contains(string(b), "sudo-test-only") || strings.Contains(string(b), "privileged") {
		t.Fatal("sudo secret exposed in JSON")
	}
}
func TestThemeValidation(t *testing.T) {
	for _, theme := range []string{"system", "dark", "light"} {
		if !validTheme(theme) {
			t.Fatal(theme)
		}
	}
	if validTheme("anything") {
		t.Fatal("accepted invalid theme")
	}
}

func TestSSHWarningsDoNotCorruptStructuredOutput(t *testing.T) {
 if runtime.GOOS == "windows" {t.Skip("uses a POSIX fake ssh executable")}
 folder:=t.TempDir()
 script:="#!/bin/sh\nprintf '{\"items\": []}'\nprintf 'Administrative policy lecture\\n' >&2\n"
 if e:=os.WriteFile(filepath.Join(folder,"ssh"),[]byte(script),0755);e!=nil{t.Fatal(e)}
 t.Setenv("PATH",folder)
 ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel()
 out,e:=runRemote(ctx,Credentials{Host:"192.0.2.20",Port:22,User:"minecraft",Password:"test-only"},"ignored by fake ssh")
 if e!=nil{t.Fatal(e)}
 var data map[string]any;if e=json.Unmarshal([]byte(out),&data);e!=nil{t.Fatalf("stderr was mixed into JSON: %q",out)}
}

func TestRootUIDProbeAndJarPolicy(t *testing.T){
 if !rootUID("profile banner__RAWCRAFT_UID__0\n"){t.Fatal("root marker lost in startup output")}
 if rootUID("__RAWCRAFT_UID__1000\n"){t.Fatal("ordinary user treated as root")}
 if !jarArchive("/plugins/Core.JAR")||jarArchive("/plugins/config.yml"){t.Fatal("JAR editing policy incorrect")}
}

package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionAndMutationProtection(t *testing.T) {
	state.token = "session"
	state.csrf = "csrf"
	handler := route("POST", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	for _, tc := range []struct {
		name, method, session, csrf, origin, host string
		want                                      int
	}{
		{"missing session", "POST", "", "csrf", "", "127.0.0.1:1234", 403},
		{"missing csrf", "POST", "session", "", "", "127.0.0.1:1234", 403},
		{"wrong origin", "POST", "session", "csrf", "https://example.com", "127.0.0.1:1234", 403},
		{"dns rebinding", "POST", "session", "csrf", "", "evil.example:1234", 403},
		{"GET cannot mutate", "GET", "session", "", "", "127.0.0.1:1234", 405},
		{"valid", "POST", "session", "csrf", "http://127.0.0.1:1234", "127.0.0.1:1234", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://"+tc.host+"/api/test", nil)
			if tc.session != "" {
				r.AddCookie(&http.Cookie{Name: sessionCookieName(), Value: tc.session})
			}
			r.Header.Set("X-RAWCRAFT-CSRF", tc.csrf)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			handler(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d", w.Code, tc.want)
			}
		})
	}
}
func TestPageTemplate(t *testing.T) {
	tpl, e := template.New("ui").Parse(page)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = tpl.Execute(&out, map[string]string{"CSRF": "test-token"}); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), "test-token") || strings.Contains(out.String(), "ZgotmplZ") {
		t.Fatal("invalid rendered template")
	}
}
func TestProcessResponseOmitsEnvironmentSecrets(t *testing.T) {
	var p Process
	if e := json.Unmarshal([]byte(`{"pm_id":4,"name":"minecraft","monit":{"cpu":12,"memory":200},"pm2_env":{"status":"online","SECRET":"hidden","env":{"PASSWORD":"hidden"}}}`), &p); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "hidden") || strings.Contains(string(b), "SECRET") {
		t.Fatal("environment secret exposed")
	}
}
func TestCredentialsAndPaths(t *testing.T) {
	good := Credentials{Host: "192.0.2.1", Port: 22, User: "minecraft", Password: "example"}
	if e := checkedCred(good); e != nil {
		t.Fatal(e)
	}
	for _, host := range []string{"-oProxyCommand=evil", "a;rm -rf /", "a\nb", ""} {
		bad := good
		bad.Host = host
		if checkedCred(bad) == nil {
			t.Fatalf("accepted host %q", host)
		}
	}
	for _, path := range []string{"relative", "/a\x00b", strings.Repeat("/", 4097)} {
		if checkPath(path) == nil {
			t.Fatalf("accepted path %q", path)
		}
	}
	if got := quote("a'b"); got != `'a'"'"'b'` {
		t.Fatalf("bad shell quoting: %s", got)
	}
}

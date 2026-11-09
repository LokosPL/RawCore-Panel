package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// Cookies are scoped to hosts, not ports. A unique name prevents two app
// instances sharing WebView2/Edge storage from invalidating one another.
func sessionCookieName() string {
	sum := sha256.Sum256([]byte(state.token))
	return "rawcraft_session_" + hex.EncodeToString(sum[:8])
}
func setSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName(), Value: state.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
}
func handleRenewSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method", 405)
		return
	}
	if !strings.HasPrefix(r.Host, "127.0.0.1:") || !eq(r.Header.Get("X-RAWCRAFT-CSRF"), state.csrf) {
		jsonOut(w, 403, map[string]string{"error": "Nie można odnowić tej sesji", "code": "csrf_invalid"})
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
		jsonOut(w, 403, map[string]string{"error": "Nieprawidłowe źródło", "code": "origin_invalid"})
		return
	}
	setSessionCookie(w)
	ok(w, map[string]bool{"ok": true})
}

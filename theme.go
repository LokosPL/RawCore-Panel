package main

import (
	"errors"
	"net/http"
)

func validTheme(theme string) bool { return theme == "system" || theme == "dark" || theme == "light" }
func handleTheme(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Theme string `json:"theme"`
	}
	if e := fromJSON(r, &req); e != nil {
		bad(w, e)
		return
	}
	if !validTheme(req.Theme) {
		bad(w, errors.New("Wybierz motyw systemowy, czarny lub jasny"))
		return
	}
	settings.Lock()
	p := settings.Prefs
	p.Theme = req.Theme
	e := writePreferences(p)
	if e == nil {
		settings.Prefs = p
	}
	settings.Unlock()
	if e != nil {
		bad(w, e)
		return
	}
	updateNativeTheme(req.Theme)
	ok(w, map[string]string{"theme": req.Theme})
}

func currentTheme() string {
	settings.RLock()
	defer settings.RUnlock()
	if validTheme(settings.Prefs.Theme) {
		return settings.Prefs.Theme
	}
	return "system"
}

func validScale(scale string)bool{return scale=="auto"||scale=="100"||scale=="125"||scale=="150"||scale=="175"||scale=="200"}
func handleScale(w http.ResponseWriter,r *http.Request){
 var req struct{Scale string `json:"scale"`}
 if e:=fromJSON(r,&req);e!=nil{bad(w,e);return}
 if !validScale(req.Scale){bad(w,errors.New("Wybierz skalę Auto lub 100–200%"));return}
 settings.Lock();p:=settings.Prefs;p.Scale=req.Scale;e:=writePreferences(p);if e==nil{settings.Prefs=p};settings.Unlock()
 if e!=nil{bad(w,e);return};ok(w,map[string]string{"scale":req.Scale})
}

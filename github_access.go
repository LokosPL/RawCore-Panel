package main

import (
 "encoding/base64"
 "errors"
 "net/http"
 "strings"
)

// Only Windows DPAPI ciphertext is persisted. Neither a release EXE nor API replies contain a PAT.
func storeGithubAccess(value string,forget bool)error{
 value=strings.TrimSpace(value)
 if !forget&&(len(value)<20||len(value)>512||strings.ContainsAny(value,"\r\n\x00\t ")){return errors.New("Wklej poprawny token dostępu GitHub")}
 settings.Lock();defer settings.Unlock()
 p:=settings.Prefs
 if forget{value="";p.TokenCipher=""}else{cipher,e:=cryptToken([]byte(value),false);if e!=nil{return e};p.TokenCipher=base64.StdEncoding.EncodeToString(cipher)}
 if e:=writePreferences(p);e!=nil{return e};settings.Prefs=p;settings.Token=value;return nil
}
func handleGithubAccess(w http.ResponseWriter,r *http.Request){
 var req struct{Token string `json:"token"`;Forget bool `json:"forget"`}
 if e:=fromJSON(r,&req);e!=nil{bad(w,e);return}
 if e:=storeGithubAccess(req.Token,req.Forget);e!=nil{bad(w,e);return}
 ok(w,map[string]bool{"authorized":!req.Forget})
 go checkForUpdate(true)
}

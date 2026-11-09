package main

import (
 "context"
 "encoding/json"
 "errors"
 "net/http"
 "strings"
 "time"
)
const deleteFilePython = `import os,sys,stat,json
p=sys.argv[1];expected=sys.argv[2]
if os.path.normpath(p)=='/': raise SystemExit('Nie można usunąć katalogu głównego')
s=os.lstat(p)
if stat.S_ISDIR(s.st_mode): raise SystemExit('Ta operacja usuwa tylko pliki, nie foldery')
version=f'{s.st_dev}:{s.st_ino}:{s.st_size}:{s.st_mtime_ns}:{s.st_ctime_ns}'
if not expected or version!=expected: raise SystemExit('Plik zmienił się od odczytania listy. Odśwież katalog przed usunięciem.')
os.unlink(p)
print(json.dumps({'ok':True}))`
func handleDeleteFile(w http.ResponseWriter,r *http.Request){
 c,e:=currentPrivilegedCred();if e!=nil{bad(w,e);return}
 var req struct{Path string `json:"path"`;Version string `json:"version"`}
 if e=fromJSON(r,&req);e!=nil{bad(w,e);return}
 if e=checkPath(req.Path);e!=nil{bad(w,e);return}
 if strings.Trim(req.Path,"/")==""||len(req.Version)==0||len(req.Version)>200{bad(w,errors.New("Niepoprawny plik lub wersja listy"));return}
 ctx,cancel:=context.WithTimeout(r.Context(),30*time.Second);defer cancel()
 out,e:=runRemote(ctx,c,remPython(deleteFilePython,req.Path,req.Version));if e!=nil{bad(w,e);return}
 var result any;if json.Unmarshal([]byte(out),&result)!=nil{bad(w,errors.New("VPS nie potwierdził usunięcia"));return};ok(w,result)
}
func jarArchive(path string) bool {return strings.HasSuffix(strings.ToLower(strings.TrimRight(path,"/")),".jar")}

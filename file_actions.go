package main

import (
 "context"
 "encoding/json"
 "errors"
 "net/http"
 "strings"
 "time"
)
const deleteFilePython = `import os,sys,stat,json,shutil
p=os.path.abspath(sys.argv[1]);expected=sys.argv[2];root=os.path.realpath(sys.argv[3])
if p=='/' or p==root or root=='/': raise SystemExit('Nie można usunąć katalogu głównego ani katalogu instancji')
s=os.lstat(p)
version=f'{s.st_dev}:{s.st_ino}:{s.st_size}:{s.st_mtime_ns}:{s.st_ctime_ns}'
if not expected or version!=expected: raise SystemExit('Plik lub folder zmienił się od odczytania listy. Odśwież katalog przed usunięciem.')
if stat.S_ISDIR(s.st_mode):
 parent=os.path.realpath(os.path.dirname(p))
 if os.path.commonpath((root,parent))!=root or os.path.commonpath((root,os.path.realpath(p)))!=root: raise SystemExit('Folder musi należeć do wybranej instancji')
 # Linux shutil.rmtree uses fd-based symlink-resistant traversal.
 shutil.rmtree(p)
else:
 os.unlink(p)
print(json.dumps({'ok':True,'directory':stat.S_ISDIR(s.st_mode)}))`
func handleDeleteFile(w http.ResponseWriter,r *http.Request){
 c,e:=currentPrivilegedCred();if e!=nil{bad(w,e);return}
 var req struct{Path string `json:"path"`;Version string `json:"version"`}
 if e=fromJSON(r,&req);e!=nil{bad(w,e);return}
 if e=checkPath(req.Path);e!=nil{bad(w,e);return}
 if strings.Trim(req.Path,"/")==""||len(req.Version)==0||len(req.Version)>200{bad(w,errors.New("Niepoprawny plik lub wersja listy"));return}
 instance,e:=requestInstance(r);if e!=nil{bad(w,e);return}
 ctx,cancel:=context.WithTimeout(r.Context(),5*time.Minute);defer cancel()
 out,e:=runRemote(ctx,c,remPython(deleteFilePython,req.Path,req.Version,instance.ServerPath));if e!=nil{bad(w,e);return}
 var result any;if json.Unmarshal([]byte(out),&result)!=nil{bad(w,errors.New("VPS nie potwierdził usunięcia"));return};ok(w,result)
}
func jarArchive(path string) bool {return strings.HasSuffix(strings.ToLower(strings.TrimRight(path,"/")),".jar")}

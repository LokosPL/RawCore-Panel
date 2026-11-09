package main

import (
    "context"
    "encoding/json"
    "errors"
    "log"
    "mime"
    "net/http"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

const downloadInfoPython = `import os,sys,json,stat
p=sys.argv[1];s=os.stat(p)
if not (stat.S_ISREG(s.st_mode) or stat.S_ISDIR(s.st_mode)):raise SystemExit('Pobierać można tylko pliki i foldery')
print(json.dumps({'directory':stat.S_ISDIR(s.st_mode),'size':s.st_size if stat.S_ISREG(s.st_mode) else None}))`
const downloadStreamPython = `import os,sys,shutil,stat,zipfile
p=sys.argv[1];folder=sys.argv[2]=='directory'
s=os.stat(p)
if folder!=stat.S_ISDIR(s.st_mode):raise SystemExit('Rodzaj pliku zmienił się przed pobraniem')
if folder:
 base=os.path.basename(os.path.normpath(p));parent=os.path.dirname(os.path.normpath(p))
 with zipfile.ZipFile(sys.stdout.buffer,'w',compression=zipfile.ZIP_DEFLATED,compresslevel=5,allowZip64=True) as archive:
  archive.writestr(base+'/',b'')
  for current,dirs,files in os.walk(p,topdown=True,followlinks=False):
   dirs[:]=sorted(d for d in dirs if not os.path.islink(os.path.join(current,d)))
   for name in dirs:
    full=os.path.join(current,name)
    archive.writestr(os.path.relpath(full,parent).replace(os.sep,'/')+'/',b'')
   for name in sorted(files):
    full=os.path.join(current,name)
    if os.path.islink(full) or not os.path.isfile(full):continue
    archive.write(full,arcname=os.path.relpath(full,parent).replace(os.sep,'/'))
else:
 if not stat.S_ISREG(s.st_mode):raise SystemExit('To nie jest zwykły plik')
 with open(p,'rb') as f:shutil.copyfileobj(f,sys.stdout.buffer,1024*1024)
sys.stdout.buffer.flush()`

type downloadInfo struct {
 Directory bool `json:"directory"`
 Size *int64 `json:"size"`
}

type countedDownloadWriter struct {
 http.ResponseWriter
 Bytes int64
}
func (w *countedDownloadWriter) Write(p []byte)(int,error){n,e:=w.ResponseWriter.Write(p);w.Bytes+=int64(n);return n,e}

func handleDownloadV2(w http.ResponseWriter,r *http.Request) {
 c,e:=currentPrivilegedCred();if e!=nil{bad(w,e);return}
 p:=r.URL.Query().Get("path")
 if e=checkPath(p);e!=nil{bad(w,e);return}
 if strings.Trim(p,"/")==""{bad(w,errors.New("Nie można pobrać katalogu głównego VPS"));return}
 ctx,cancel:=context.WithTimeout(r.Context(),45*time.Minute)
 defer cancel()
 raw,e:=runRemote(ctx,c,remPython(downloadInfoPython,p))
 if e!=nil{bad(w,e);return}
 var info downloadInfo
 if e=json.Unmarshal([]byte(raw),&info);e!=nil{bad(w,errors.New("Nie można odczytać metadanych pliku"));return}
 name:=filepath.Base(strings.TrimRight(p,"/"))
 if info.Directory {name+=".zip"}
 if name==""||name=="." {bad(w,errors.New("Nieprawidłowa nazwa pliku"));return}
 w.Header().Set("Content-Type","application/octet-stream")
 if info.Directory {w.Header().Set("Content-Type","application/zip")
 } else if info.Size!=nil&&*info.Size>=0 {w.Header().Set("Content-Length",strconv.FormatInt(*info.Size,10))}
 w.Header().Set("Content-Disposition",mime.FormatMediaType("attachment",map[string]string{"filename":name}))
 w.Header().Set("Cache-Control","no-store")
 w.Header().Set("X-Content-Type-Options","nosniff")
 writer:=&countedDownloadWriter{ResponseWriter:w}
 kind:="file";if info.Directory{kind="directory"}
 e=streamRemote(ctx,c,remPython(downloadStreamPython,p,kind),nil,writer)
 if e!=nil {
  if writer.Bytes==0 {
   w.Header().Del("Content-Disposition")
   w.Header().Del("Content-Length")
   bad(w,e)
  } else {
   log.Printf("Pobieranie pliku z VPS przerwane po %d bajtach: %v",writer.Bytes,e)
  }
 }
}

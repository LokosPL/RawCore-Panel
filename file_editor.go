package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const editorChunkSize = 256 * 1024
const editorChangeLimit = 8 * 1024 * 1024

const readChunkPython = `import os,sys,json,base64,hashlib,stat
p=os.path.realpath(sys.argv[1]);offset=int(sys.argv[2]);limit=int(sys.argv[3])
with open(p,'rb') as f:
 s=os.fstat(f.fileno())
 if not stat.S_ISREG(s.st_mode): raise SystemExit('Edytować można tylko zwykłe pliki')
 if offset<0 or offset>s.st_size: raise SystemExit('Niepoprawny zakres pliku')
 f.seek(offset);data=f.read(limit)
 try: data.decode('utf-8')
 except UnicodeDecodeError as e:
  if e.reason=='unexpected end of data' and e.start>=len(data)-3 and offset+len(data)<s.st_size: data=data[:e.start]
 version=lambda s:f'{s.st_dev}:{s.st_ino}:{s.st_size}:{s.st_mtime_ns}:{s.st_ctime_ns}'
 if version(os.fstat(f.fileno()))!=version(s) or version(os.stat(p))!=version(s): raise SystemExit('Plik zmienił się podczas odczytu. Otwórz go ponownie.')
 try:
  text=data.decode('utf-8');binary='\x00' in text
 except UnicodeDecodeError: binary=True
 print(json.dumps({'path':p,'offset':offset,'length':len(data),'size':s.st_size,'next':offset+len(data),'data':base64.b64encode(data).decode(),'hash':hashlib.sha256(data).hexdigest(),'version':version(s),'binary':binary}))`

const saveChunkPython = `import os,sys,json,base64,hashlib,stat,tempfile,shutil,fcntl
p=os.path.realpath(sys.argv[1]);offset=int(sys.argv[2]);length=int(sys.argv[3]);expected=sys.argv[4];revision=sys.argv[5]
data=sys.stdin.buffer.read(8388609)
if len(data)>8388608: raise SystemExit('Jedna zmiana może mieć do 8 MB; większe zmiany wykonaj w kolejnych fragmentach')
version=lambda s:f'{s.st_dev}:{s.st_ino}:{s.st_size}:{s.st_mtime_ns}:{s.st_ctime_ns}'
temp=None;backup=None
try:
 with open(p,'rb') as src:
  fcntl.flock(src,fcntl.LOCK_EX);s=os.fstat(src.fileno())
  if not stat.S_ISREG(s.st_mode): raise SystemExit('To nie jest zwykły plik')
  if version(s)!=revision or version(os.stat(p))!=revision: raise SystemExit('Plik zmienił się na VPS. Otwórz ponownie, aby nie nadpisać cudzych zmian.')
  if offset<0 or length<0 or offset+length>s.st_size: raise SystemExit('Niepoprawny zakres zapisu')
  src.seek(offset);original=src.read(length)
  if hashlib.sha256(original).hexdigest()!=expected: raise SystemExit('Fragment pliku zmienił się na VPS')
  fd,temp=tempfile.mkstemp(prefix='.rawcraft-edit-',dir=os.path.dirname(p))
  with os.fdopen(fd,'wb') as dst:
   try: os.fchown(dst.fileno(),s.st_uid,s.st_gid)
   except PermissionError: pass
   os.fchmod(dst.fileno(),stat.S_IMODE(s.st_mode));src.seek(0)
   remaining=offset
   while remaining:
    block=src.read(min(1048576,remaining))
    if not block: raise RuntimeError('Plik zmienił rozmiar podczas zapisu')
    dst.write(block);remaining-=len(block)
   dst.write(data);src.seek(offset+length);shutil.copyfileobj(src,dst,1048576);dst.flush();os.fsync(dst.fileno())
  fd,backup=tempfile.mkstemp(prefix='.rawcraft-backup-',dir=os.path.dirname(p))
  with os.fdopen(fd,'wb') as dst:
   try: os.fchown(dst.fileno(),s.st_uid,s.st_gid)
   except PermissionError: pass
   os.fchmod(dst.fileno(),stat.S_IMODE(s.st_mode));src.seek(0);shutil.copyfileobj(src,dst,1048576);dst.flush();os.fsync(dst.fileno())
  if version(os.fstat(src.fileno()))!=revision or version(os.stat(p))!=revision: raise SystemExit('Plik został zmieniony podczas zapisu. Zapis anulowano.')
  os.replace(backup,p+'.rawcraft.bak');backup=None;os.replace(temp,p);temp=None
 print(json.dumps({'message':'Zapisano fragment. Poprzedni cały plik: '+p+'.rawcraft.bak','path':p}))
finally:
 for name in (temp,backup):
  if name and os.path.exists(name): os.unlink(name)`

func handleRead(w http.ResponseWriter, r *http.Request) {
 if jarArchive(r.URL.Query().Get("path")){bad(w,errors.New("Pliki JAR są archiwami programu. Pobierz lub zastąp JAR zamiast edytować jego bajty."));return}
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	p := r.URL.Query().Get("path")
	if e = checkPath(p); e != nil {
		bad(w, e)
		return
	}
	offset := int64(0)
	if value := r.URL.Query().Get("offset"); value != "" {
		offset, e = strconv.ParseInt(value, 10, 64)
		if e != nil || offset < 0 {
			bad(w, errors.New("Niepoprawny offset pliku"))
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	out, e := runRemote(ctx, c, remPython(readChunkPython, p, strconv.FormatInt(offset, 10), strconv.Itoa(editorChunkSize)))
	if e != nil {
		bad(w, e)
		return
	}
	var result any
	if e = json.Unmarshal([]byte(out), &result); e != nil {
		bad(w, errors.New("Nie udało się odczytać fragmentu"))
		return
	}
	ok(w, result)
}
func handleSave(w http.ResponseWriter, r *http.Request) {
	c, e := currentPrivilegedCred()
	if e != nil {
		bad(w, e)
		return
	}
	var req struct {
		Path    string `json:"path"`
		Offset  int64  `json:"offset"`
		Length  int    `json:"length"`
		Hash    string `json:"hash"`
		Version string `json:"version"`
		Data    string `json:"data"`
	}
	if e = json.NewDecoder(io.LimitReader(r.Body, 12<<20)).Decode(&req); e != nil {
		bad(w, e)
		return
	}
	if jarArchive(req.Path){bad(w,errors.New("Edycja archiwów JAR jest wyłączona"));return}
 if e = checkPath(req.Path); e != nil {
		bad(w, e)
		return
	}
	if req.Offset < 0 || req.Length < 0 || req.Length > editorChunkSize || len(req.Hash) != 64 || len(req.Version) > 200 {
		bad(w, errors.New("Niepoprawny zakres lub wersja pliku"))
		return
	}
	data, e := base64.StdEncoding.DecodeString(req.Data)
	if e != nil || len(data) > editorChangeLimit {
		bad(w, errors.New("Niepoprawne dane fragmentu (maksymalnie 8 MB na zmianę)"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Minute)
	defer cancel()
	var out strings.Builder
	if e = streamRemote(ctx, c, remPython(saveChunkPython, req.Path, strconv.FormatInt(req.Offset, 10), strconv.Itoa(req.Length), req.Hash, req.Version), strings.NewReader(string(data)), &out); e != nil {
		bad(w, e)
		return
	}
	var result any
	if json.Unmarshal([]byte(out.String()), &result) != nil {
		bad(w, errors.New("Niepoprawna odpowiedź zapisu"))
		return
	}
	ok(w, result)
}

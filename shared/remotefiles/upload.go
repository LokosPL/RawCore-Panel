package remotefiles
// Upload stages and fsyncs the complete transfer beside its destination. A
// disconnected upload leaves the original intact; existing mode/owner survive.
const Upload = `import os,sys,stat,tempfile,shutil,json,fcntl
p=os.path.abspath(sys.argv[1]);expected=sys.argv[2];expected_size=int(sys.argv[3])
if os.path.islink(p): raise SystemExit('Zastępowanie dowiązań jest wyłączone. Wybierz plik docelowy.')
version=lambda s:f'{s.st_dev}:{s.st_ino}:{s.st_size}:{s.st_mtime_ns}:{s.st_ctime_ns}'
def current():
 try: return os.stat(p,follow_symlinks=False)
 except FileNotFoundError: return None
s=current();revision=version(s) if s else 'missing'
if expected and revision!=expected: raise SystemExit('Plik zmienił się na VPS. Odśwież listę przed nadpisaniem.')
if s and not stat.S_ISREG(s.st_mode): raise SystemExit('Można zastąpić tylko zwykły plik')
fd,temp=tempfile.mkstemp(prefix='.rawcraft-upload-',dir=os.path.dirname(p));backup=None
try:
 with os.fdopen(fd,'wb') as dst:
  shutil.copyfileobj(sys.stdin.buffer,dst,1048576);dst.flush()
  if dst.tell()!=expected_size: raise SystemExit('Niepełny transfer. Oryginalny plik pozostaje bez zmian.')
  os.fsync(dst.fileno())
  if s:
   try: os.fchown(dst.fileno(),s.st_uid,s.st_gid)
   except PermissionError: pass
   os.fchmod(dst.fileno(),stat.S_IMODE(s.st_mode))
  else: os.fchmod(dst.fileno(),0o644)
 latest=current()
 if (version(latest) if latest else 'missing')!=revision: raise SystemExit('Plik zmienił się w trakcie przesyłania. Nadpisanie anulowano.')
 if s:
  fd,backup=tempfile.mkstemp(prefix='.rawcraft-backup-',dir=os.path.dirname(p))
  with os.fdopen(fd,'wb') as dst,open(p,'rb') as src:
   fcntl.flock(src,fcntl.LOCK_EX)
   if version(os.fstat(src.fileno()))!=revision: raise SystemExit('Plik zmienił się w trakcie przesyłania')
   shutil.copyfileobj(src,dst,1048576);dst.flush();os.fsync(dst.fileno());os.fchmod(dst.fileno(),stat.S_IMODE(s.st_mode))
   try: os.fchown(dst.fileno(),s.st_uid,s.st_gid)
   except PermissionError: pass
  latest=current()
  if not latest or version(latest)!=revision: raise SystemExit('Plik zmienił się w trakcie kopii. Nadpisanie anulowano.')
  os.replace(backup,p+'.rawcraft.bak');backup=None
 os.replace(temp,p);temp=None
 print(json.dumps({'ok':True,'message':'Przesłano plik'+(' i zachowano .rawcraft.bak' if s else ''),'path':p}))
finally:
 for name in (temp,backup):
  if name and os.path.exists(name):os.unlink(name)
`

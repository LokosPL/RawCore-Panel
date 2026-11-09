package main

// mkdirTreePython creates nested directories only under the selected instance.
// It refuses symlinks and paths outside the remote root.
const mkdirTreePython = `import os,sys,json
target=os.path.abspath(sys.argv[1]);root=os.path.realpath(sys.argv[2])
if root=='/' or not os.path.isdir(root):raise SystemExit('Niepoprawny katalog instancji')
if os.path.commonpath((root,target))!=root:raise SystemExit('Folder musi należeć do wybranej instancji')
relative=os.path.relpath(target,root)
current=root
if relative!='.':
 for part in relative.split(os.sep):
  if part in ('','.','..'):raise SystemExit('Nieprawidłowa nazwa folderu')
  current=os.path.join(current,part)
  if os.path.islink(current):raise SystemExit('Folder prowadzi przez dowiązanie symboliczne')
  try:os.mkdir(current,0o755)
  except FileExistsError:pass
  if os.path.islink(current) or not os.path.isdir(current):raise SystemExit('Ścieżka jest plikiem lub dowiązaniem')
  if os.path.commonpath((root,os.path.realpath(current)))!=root:raise SystemExit('Ścieżka prowadzi poza instancję')
print(json.dumps({'ok':True,'path':target}))`

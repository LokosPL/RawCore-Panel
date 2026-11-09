package main

import (
 "errors"
 "fmt"
 "os"
 "path/filepath"
)

// Each installation owns its backup; a locked/read-only legacy .old is never
// deleted or selected as the rollback for a different installation.
type executableReplacement struct { target, backup string }

func temporaryExecutable(target,label string)(string,error){
 f,e:=os.CreateTemp(filepath.Dir(target),filepath.Base(target)+"."+label+"-*")
 if e!=nil{return "",e};name:=f.Name()
 if e=f.Close();e!=nil{_ = os.Remove(name);return "",e}
 return name,nil
}

func replaceWithBackup(target,staged,expected string)(*executableReplacement,error){
 if e:=verifyExecutable(staged,expected);e!=nil{return nil,e}
 next,e:=temporaryExecutable(target,"new");if e!=nil{return nil,e}
 defer os.Remove(next)
 if e=copyFile(staged,next);e!=nil{return nil,e}
 if e=verifyExecutable(next,expected);e!=nil{return nil,e}
 // Recover a legacy interrupted replacement by COPYING its sole good backup.
 // Reading .old works even when Windows denies deletion of that file.
 if _,e=os.Lstat(target);os.IsNotExist(e){
  legacy:=target+".old";info,readErr:=os.Lstat(legacy)
  if readErr!=nil{return nil,readErr};if !info.Mode().IsRegular(){return nil,errors.New("Nieprawidłowa kopia poprzedniej aplikacji")}
  recovery,readErr:=temporaryExecutable(target,"recovery");if readErr!=nil{return nil,readErr};defer os.Remove(recovery)
  if readErr=copyFile(legacy,recovery);readErr!=nil{return nil,fmt.Errorf("Przywrócenie poprzedniej aplikacji: %w",readErr)}
  if readErr=os.Rename(recovery,target);readErr!=nil{return nil,readErr}
 }else if e!=nil{return nil,e}
 info,e:=os.Lstat(target);if e!=nil{return nil,e}
 if !info.Mode().IsRegular(){return nil,errors.New("Plik panelu nie jest zwykłym plikiem")}
 backup,e:=temporaryExecutable(target,"rollback");if e!=nil{return nil,e}
 // Only remove our newly reserved EMPTY file, never a previous backup.
 if e=os.Remove(backup);e!=nil{return nil,e}
 if e=os.Rename(target,backup);e!=nil{return nil,fmt.Errorf("Zamknij wszystkie procesy panelu; nie można utworzyć kopii bieżącego EXE: %w",e)}
 replacement:=&executableReplacement{target:target,backup:backup}
 if e=os.Rename(next,target);e!=nil{
  if restore:=os.Rename(backup,target);restore!=nil{return replacement,errors.Join(e,fmt.Errorf("Kopia pozostaje w %s: %w",backup,restore))}
  return nil,e
 }
 return replacement,nil
}

func(r *executableReplacement)rollback()error{
 if _,e:=os.Stat(r.backup);e!=nil{return e}
 failed,e:=temporaryExecutable(r.target,"failed");if e!=nil{return e}
 if e=os.Remove(failed);e!=nil{return e}
 if e=os.Rename(r.target,failed);e!=nil{return e}
 if e=os.Rename(r.backup,r.target);e!=nil{
  if restore:=os.Rename(failed,r.target);restore!=nil{return errors.Join(e,restore)}
  return e
 }
 // Cleanup is best effort; it cannot turn a restored application into failure.
 _=os.Remove(failed);return nil
}

// Call only AFTER the new GUI has confirmed startup. Windows/AV may retain a
// handle briefly; leaving this unique backup never blocks the next update.
func(r *executableReplacement)discardBackup(){if r!=nil{_ = os.Remove(r.backup)}}

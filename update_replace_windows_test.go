//go:build windows

package main

import (
 "bytes"
 "crypto/sha256"
 "encoding/hex"
 "os"
 "path/filepath"
 "syscall"
 "testing"
)

func lockLegacyBackup(t *testing.T,path string,data []byte){
 t.Helper()
 if e:=os.WriteFile(path,data,0600);e!=nil{t.Fatal(e)}
 if e:=os.Chmod(path,0400);e!=nil{t.Fatal(e)}
 name,e:=syscall.UTF16PtrFromString(path);if e!=nil{t.Fatal(e)}
 handle,e:=syscall.CreateFile(name,syscall.GENERIC_READ,syscall.FILE_SHARE_READ,nil,syscall.OPEN_EXISTING,syscall.FILE_ATTRIBUTE_NORMAL,0)
 if e!=nil{t.Fatal(e)}
 t.Cleanup(func(){syscall.CloseHandle(handle);_=os.Chmod(path,0600);_=os.Remove(path)})
 if e=os.Remove(path);e==nil{t.Fatal("test did not reproduce Windows denial of deletion")}
}

func TestLockedLegacyBackupDoesNotBlockReplacementOrExactRollback(t *testing.T){
 dir:=t.TempDir();target:=filepath.Join(dir,"RAWCRAFT-Control.exe");staged:=filepath.Join(dir,"update.exe")
 before:=[]byte("MZcurrent application");after:=[]byte("MZnew application");legacy:=[]byte("MZunrelated older backup")
 if e:=os.WriteFile(target,before,0600);e!=nil{t.Fatal(e)};if e:=os.WriteFile(staged,after,0600);e!=nil{t.Fatal(e)}
 lockLegacyBackup(t,target+".old",legacy);lockLegacyBackup(t,target+".new",legacy)
 hash:=sha256.Sum256(after);replacement,e:=replaceWithBackup(target,staged,hex.EncodeToString(hash[:]));if e!=nil{t.Fatal(e)}
 if replacement.backup==target+".old"{t.Fatal("reused locked legacy backup")}
 saved,_:=os.ReadFile(replacement.backup);if !bytes.Equal(saved,before){t.Fatal("rollback must belong to this replacement")}
 if e=replacement.rollback();e!=nil{t.Fatal(e)}
 restored,_:=os.ReadFile(target);if !bytes.Equal(restored,before){t.Fatal("wrong version restored")}
 kept,_:=os.ReadFile(target+".old");if !bytes.Equal(kept,legacy){t.Fatal("legacy backup changed")}
}

func TestSuccessfulStartupCleanupCannotFailBecauseBackupIsLocked(t *testing.T){
 dir:=t.TempDir();target:=filepath.Join(dir,"app.exe");backup:=target+".rollback-owned"
 lockLegacyBackup(t,backup,[]byte("MZold"))
 replacement:=&executableReplacement{target:target,backup:backup};replacement.discardBackup()
 if _,e:=os.Stat(backup);e!=nil{t.Fatal("locked backup should remain for later cleanup",e)}
}

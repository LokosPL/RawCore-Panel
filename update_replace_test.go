package main

import (
 "bytes"
 "crypto/sha256"
 "encoding/hex"
 "os"
 "path/filepath"
 "testing"
)

func TestConsecutiveReplacementsKeepIndependentRollbackSnapshots(t *testing.T){
 dir:=t.TempDir();target:=filepath.Join(dir,"app.exe");staged:=filepath.Join(dir,"staged.exe")
 old:=[]byte("MZversion zero");one:=[]byte("MZversion one");two:=[]byte("MZversion two")
 os.WriteFile(target,old,0600)
 replace:=func(data []byte)*executableReplacement{
  t.Helper();os.WriteFile(staged,data,0600);sum:=sha256.Sum256(data)
  replacement,e:=replaceWithBackup(target,staged,hex.EncodeToString(sum[:]));if e!=nil{t.Fatal(e)};return replacement
 }
 first:=replace(one);second:=replace(two)
 if first.backup==second.backup{t.Fatal("backup path reused")}
 for path,want:=range map[string][]byte{first.backup:old,second.backup:one,target:two}{got,e:=os.ReadFile(path);if e!=nil||!bytes.Equal(got,want){t.Fatal("lost snapshot",path,e)}}
 if e:=second.rollback();e!=nil{t.Fatal(e)};got,_:=os.ReadFile(target);if !bytes.Equal(got,one){t.Fatal("wrong rollback")}
}

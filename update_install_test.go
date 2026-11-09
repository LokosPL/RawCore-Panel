package main

import (
 "errors"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "reflect"
 "strings"
 "testing"
)
func TestUpdateWaitsForConfirmedRestartAndRollsBack(t *testing.T){
 for _,failure:=range []string{"","wait","replace","launch","rollback"}{
  t.Run(failure,func(t *testing.T){
   var calls,states []string
   step:=func(name string)func()error{return func()error{calls=append(calls,name);if failure==name || failure=="rollback"&&name=="launch"{return errors.New(name)};return nil}}
   e:=runInstallActions(installActions{step("wait"),step("replace"),step("launch"),step("rollback")},func(state,message string,p int){states=append(states,state)})
   want:=map[string][]string{"":{"wait","replace","launch"},"wait":{"wait"},"replace":{"wait","replace"},"launch":{"wait","replace","launch","rollback"},"rollback":{"wait","replace","launch","rollback"}}[failure]
   if !reflect.DeepEqual(calls,want){t.Fatalf("%v != %v",calls,want)}
   if (e==nil)!=(failure==""){t.Fatal(e)}
   if (states[len(states)-1]=="done")!=(failure==""){t.Fatal("reported success without confirmed panel",states)}
  })
 }
}
func TestUpdateRollbackRestoresPreviousExecutable(t *testing.T){
 target:=filepath.Join(t.TempDir(),"control.exe")
 backup:=target+".rollback-test"
 os.WriteFile(target,[]byte("new"),0600);os.WriteFile(backup,[]byte("old"),0600)
 if e:=(&executableReplacement{target:target,backup:backup}).rollback();e!=nil{t.Fatal(e)}
 b,_:=os.ReadFile(target);if string(b)!="old"{t.Fatal("lost previous application")}
 if _,e:=os.Stat(target+".failed");!os.IsNotExist(e){t.Fatal("failed payload left behind")}
}
func TestPanelStartupConfirmationIsLoopbackOnlyAndOneShot(t *testing.T){
 startupUpdate.Lock();old:=startupUpdate.endpoint;startupUpdate.endpoint="";startupUpdate.Unlock();defer func(){startupUpdate.Lock();startupUpdate.endpoint=old;startupUpdate.Unlock()}()
 nonce:=strings.Repeat("a",43)
 for _,address:=range []string{"https://evil.example/rawcraft-update/"+nonce+"/ready","http://localhost:1234/rawcraft-update/"+nonce+"/ready","http://127.0.0.1:1234/admin","http://secret@127.0.0.1:1234/rawcraft-update/"+nonce+"/ready","http://127.0.0.1:1234/rawcraft-update/../ready"}{
  configureUpdateReady([]string{"app","--update-ready",address});if startupUpdate.endpoint!=""{t.Fatal("accepted unsafe callback",address)}
 }
 calls:=0;s:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){calls++;if r.Method!="POST"||r.Header.Get("X-RAWCRAFT-Version")!=appVersion{t.Error("missing confirmation context")}}));defer s.Close()
 configureUpdateReady([]string{"app","--update-ready",s.URL+"/rawcraft-update/"+nonce+"/ready"})
 for i:=0;i<2;i++ {w:=httptest.NewRecorder();handleUpdateStarted(w,httptest.NewRequest("POST","/api/update/started",strings.NewReader("{}")));if w.Code!=200{t.Fatal(w.Body.String())}}
 if calls!=1{t.Fatal("startup confirmation not one-shot",calls)}
}



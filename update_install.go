package main

import (
 "bytes"
 "context"
 _ "embed"
 "errors"
 "html/template"
 "net"
 "net/http"
 "net/url"
 "os"
 "os/exec"
 "path/filepath"
 "strconv"
 "strings"
 "sync"
 "time"
)

//go:embed assets/update.html
var updatePage string

var startupUpdate struct{ sync.Mutex; endpoint string }

func configureUpdateReady(args []string) {
 if len(args)!=3 || args[1]!="--update-ready" {return}
 u,e:=url.Parse(args[2]);if e!=nil || u.Scheme!="http" || u.User!=nil || u.RawQuery!="" || u.Fragment!="" || u.RawPath!="" {return}
 host,port,e:=net.SplitHostPort(u.Host);if e!=nil || host!="127.0.0.1" {return}
 n,e:=strconv.Atoi(port);if e!=nil || n<1 || n>65535 || !strings.HasPrefix(u.Path,"/rawcraft-update/") || !strings.HasSuffix(u.Path,"/ready"){return}
 nonce:=strings.TrimSuffix(strings.TrimPrefix(u.Path,"/rawcraft-update/"),"/ready")
 if len(nonce)!=43{return};for _,c:=range nonce{if !(c>='a'&&c<='z'||c>='A'&&c<='Z'||c>='0'&&c<='9'||c=='_'||c=='-'){return}}
 startupUpdate.Lock();startupUpdate.endpoint=u.String();startupUpdate.Unlock()
}
func handleUpdateStarted(w http.ResponseWriter,r *http.Request){
 startupUpdate.Lock();defer startupUpdate.Unlock()
 if startupUpdate.endpoint!="" {
  ctx,cancel:=context.WithTimeout(r.Context(),3*time.Second);defer cancel()
  req,e:=http.NewRequestWithContext(ctx,"POST",startupUpdate.endpoint,strings.NewReader("{}"))
  if e!=nil {bad(w,errors.New("Nie potwierdzono uruchomienia panelu"));return}
  req.Header.Set("X-RAWCRAFT-Version",appVersion)
  client:=&http.Client{Timeout:3*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
  resp,e:=client.Do(req)
  if e!=nil {bad(w,errors.New("Nie potwierdzono uruchomienia panelu"));return}
  resp.Body.Close();if resp.StatusCode!=200 {bad(w,errors.New("Nie potwierdzono uruchomienia panelu"));return}
  startupUpdate.endpoint=""
 }
 ok(w,map[string]bool{"ok":true})
}

type installActions struct {
 wait func() error
 replace func() error
 launch func() error
 rollback func() error
}
// Success requires a running, responsive new panel, not just a successful CreateProcess.
func runInstallActions(a installActions,notify func(string,string,int))error{
 notify("waiting","Zamykanie poprzedniej wersji panelu…",20)
 if e:=a.wait();e!=nil{return e}
 notify("installing","Instalowanie zweryfikowanej aktualizacji…",50)
 if e:=a.replace();e!=nil{return e}
 notify("starting","Otwieranie nowej wersji panelu…",80)
 if e:=a.launch();e!=nil {
  notify("rollback","Przywracanie poprzedniej wersji…",80)
  if rollback:=a.rollback();rollback!=nil{return errors.Join(e,rollback)}
  return e
 }
 notify("done","Panel został zaktualizowany i otwarty. Miłej pracy!",100)
 return nil
}
func installUpdate(){
 if len(os.Args)!=6&&len(os.Args)!=7{return}
 pid,e:=strconv.Atoi(os.Args[2]);if e!=nil||pid<1{return}
 target,staged,hash:=os.Args[3],os.Args[4],os.Args[5]
 endpoint:="";if len(os.Args)==7{endpoint=os.Args[6]}
 if !filepath.IsAbs(target)||!filepath.IsAbs(staged){return}
 listener,e:=net.Listen("tcp4","127.0.0.1:0");if e!=nil{return}
 base:="/rawcraft-update/"+token();origin:="http://"+listener.Addr().String()
 ready:=make(chan struct{});var readyOnce sync.Once
 var status=struct{sync.Mutex;State,Message string;Percent int}{State:"waiting",Message:"Przygotowywanie aktualizacji RAWCRAFT…",Percent:10}
 notify:=func(state,message string,percent int){status.Lock();status.State=state;status.Message=message;status.Percent=percent;status.Unlock()}
 page:=template.Must(template.New("update").Parse(updatePage));mux:=http.NewServeMux()
 mux.HandleFunc(base+"/",func(w http.ResponseWriter,r *http.Request){
  if r.Host!=listener.Addr().String(){http.Error(w,"Host",403);return}
  w.Header().Set("Content-Security-Policy","default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
  w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("Referrer-Policy","no-referrer")
  switch r.URL.Path {
  case base+"/":if r.Method!="GET"{http.Error(w,"Method",405);return};w.Header().Set("Content-Type","text/html; charset=utf-8");_=page.Execute(w,map[string]string{"Base":base,"Version":appVersion})
  case base+"/logo.png":w.Header().Set("Content-Type","image/png");http.ServeContent(w,r,"logo.png",time.Time{},bytes.NewReader(logo))
  case base+"/status":status.Lock();defer status.Unlock();jsonOut(w,200,map[string]any{"state":status.State,"message":status.Message,"percent":status.Percent})
  case base+"/ready":if r.Method!="POST"||r.Header.Get("X-RAWCRAFT-Version")!=appVersion{http.Error(w,"Confirmation",403);return};readyOnce.Do(func(){close(ready)});jsonOut(w,200,map[string]bool{"ok":true})
  default:http.NotFound(w,r)
  }
 })
 server:=&http.Server{Handler:mux,ReadHeaderTimeout:3*time.Second,ReadTimeout:5*time.Second,WriteTimeout:5*time.Second,IdleTimeout:10*time.Second}
 go server.Serve(listener);defer server.Close()
 done:=make(chan struct{});var workOnce sync.Once
 var replacement *executableReplacement
 work:=func(){workOnce.Do(func(){go func(){defer close(done)
  err:=runInstallActions(installActions{
   wait:func()error{
    if e:=verifyExecutable(staged,hash);e!=nil{return e}
    if e:=confirmInstaller(endpoint,hash);e!=nil{return e}
    return waitParent(pid)
   },
   replace:func()error{var failure error;for i:=0;i<40;i++{replacement,failure=replaceWithBackup(target,staged,hash);if failure==nil{return nil};if replacement!=nil{return failure};time.Sleep(500*time.Millisecond)};return failure},
   launch:func()error{
    cmd:=exec.Command(target,"--update-ready",origin+base+"/ready");cmd.Dir=filepath.Dir(target)
    if e:=cmd.Start();e!=nil{return e}
    exited:=make(chan struct{});go func(){_=cmd.Wait();close(exited)}()
    select {
    case <-ready:return nil
    case <-exited:return errors.New("Nowy panel zakończył pracę przed uruchomieniem")
    case <-time.After(45*time.Second):_=cmd.Process.Kill();<-exited;return errors.New("Nowy panel nie potwierdził uruchomienia")
    }
   },
   rollback:func()error{if replacement==nil{return errors.New("Brak potwierdzonej kopii tej aktualizacji")};return replacement.rollback()},
  },notify)
  if err!=nil {
   _=os.WriteFile(filepath.Join(filepath.Dir(staged),"install-error.txt"),[]byte(err.Error()),0600)
   notify("error","Aktualizacja nie powiodła się. Panel pozostaje zamknięty. Zamknij to okno i uruchom instalator z GitHub. Szczegóły: "+err.Error(),0)
  }else{
   replacement.discardBackup()
   _=os.Remove(staged);_=os.Remove(filepath.Join(filepath.Dir(staged),"install-error.txt"));time.Sleep(1500*time.Millisecond);closeUpdateWindow()
  }
 }()})}
 if !runUpdateWindow(origin+base+"/",func(uintptr){work()}) {return}
 <-done
}



